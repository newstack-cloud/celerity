//! Two nodes serving real clients over one Redis deployment, which is the shape a
//! deployment has. The other tests here take the layers apart; these put them
//! together and let the registry decide what happens.

use std::{
    net::{Ipv4Addr, SocketAddr},
    sync::Arc,
    time::Duration,
};

use axum::{
    extract::{
        ws::{Message as AxumMessage, WebSocket, WebSocketUpgrade},
        State,
    },
    response::Response,
    routing::get,
    Router,
};
use celerity_helpers::{
    redis::ConnectionWrapper,
    testing::{redis_config, redis_connection},
};

use celerity_ws_redis::{
    forwarded::ForwardedMessages,
    locations::ConnectionLocations,
    node_group::{join_or_create, node_key, NodeGroupConfig},
    pubsub::{connect, PubSubConnectionConfig},
};
use celerity_ws_registry::{
    errors::WebSocketConnError,
    registry::{
        SendContext, WebSocketConnRegistry, WebSocketConnRegistryConfig, WebSocketRegistrySend,
    },
    types::{AckWorkerConfig, MessageType},
};
use futures::{SinkExt, StreamExt};
use nanoid::nanoid;
use tokio::sync::{mpsc::channel, Mutex};
use tokio_tungstenite::tungstenite;

/// Clears anything a previous run left behind under a prefix, including the
/// message ids the run will use.
///
/// A record of a forwarded message outlives the run that wrote it, so without
/// this a second run inside the window treats its first delivery as a duplicate
/// and delivers nothing.
async fn clear(conn: &mut ConnectionWrapper, prefix: &str, message_ids: &[&str]) {
    for message_id in message_ids {
        conn.del(&format!("{prefix}:msg:{message_id}"))
            .await
            .unwrap();
    }

    let index_key = format!("{prefix}:{{group-meta}}:node-groups");
    for group_id in conn.smembers(&index_key).await.unwrap() {
        let members_key = format!("{prefix}:{{group-meta}}:node-group-members:{group_id}");
        for member in conn.smembers(&members_key).await.unwrap() {
            conn.del(&node_key(prefix, &member)).await.unwrap();
        }
        conn.del(&members_key).await.unwrap();
    }
    conn.del(&index_key).await.unwrap();
}

/// A node of the cluster, serving clients and joined up to the others.
struct ClusterNode {
    registry: Arc<WebSocketConnRegistry>,
    addr: SocketAddr,
    group_id: String,
    /// Held for the life of the node. Dropping it would close the channel the
    /// pubsub task watches for a group change, ending that arm of its select.
    _moved: tokio::sync::mpsc::Sender<celerity_ws_redis::node_group::NodeGroup>,
}

/// Starts a node with everything a deployment wires up, in the order
/// `Application::run` wires it.
async fn start_node(prefix: &str, name: &str, capacity: usize) -> ClusterNode {
    // Short enough that a test can watch a message run out of attempts.
    start_node_with_ack_timings(prefix, name, capacity, 100, 3).await
}

/// A node whose acknowledgement timings a test chooses, for one that has to
/// watch a message settle rather than watch it run out of attempts.
async fn start_node_with_ack_timings(
    prefix: &str,
    name: &str,
    capacity: usize,
    message_timeout_ms: u64,
    max_attempts: u32,
) -> ClusterNode {
    let mut conn = redis_connection().await;
    let group_config = NodeGroupConfig {
        server_node_name: name.to_string(),
        capacity,
        node_ttl_ms: 30_000,
        key_prefix: prefix.to_string(),
    };
    let group = join_or_create(&mut conn, &group_config).await.unwrap();
    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        group.id.clone(),
        group_config.node_ttl_ms,
    );

    let registry = Arc::new(WebSocketConnRegistry::new(
        WebSocketConnRegistryConfig {
            ack_worker_config: Some(AckWorkerConfig {
                message_action_check_interval_ms: Some(20),
                message_timeout_ms: Some(message_timeout_ms),
                max_attempts: Some(max_attempts),
            }),
            server_node_name: name.to_string(),
        },
        None,
    ));

    let (moved_tx, moved_rx) = channel(4);
    let (broadcaster, from_other_nodes) = connect(
        PubSubConnectionConfig {
            server_node_name: name.to_string(),
            key_prefix: prefix.to_string(),
            nodes: redis_config().nodes,
            password: redis_config().password,
            cluster_mode: redis_config().cluster_mode,
            migration_grace_ms: 5_000,
        },
        group.clone(),
        locations.clone(),
        moved_rx,
    )
    .await
    .unwrap();

    registry.set_locations(locations).unwrap();
    registry
        .set_forwarded_messages(ForwardedMessages::new(
            conn.clone(),
            prefix.to_string(),
            30_000,
        ))
        .unwrap();
    registry.set_broadcaster(broadcaster).unwrap();
    registry.clone().start_ack_worker();
    // This is started regardless of the deployment, as `Application::run` starts it,
    // since a node holding a connection tracks what its client has acknowledged whether
    // or not the message came from another node.
    registry.clone().start_client_ack_worker();
    registry.clone().listen(from_other_nodes);

    let app: Router = Router::new()
        .route("/ws", get(serve_client))
        .with_state(registry.clone());
    let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
        .await
        .unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move {
        axum::serve(listener, app).await.unwrap();
    });

    ClusterNode {
        registry,
        addr,
        group_id: group.id,
        _moved: moved_tx,
    }
}

async fn serve_client(
    State(registry): State<Arc<WebSocketConnRegistry>>,
    ws: WebSocketUpgrade,
) -> Response {
    ws.on_upgrade(move |socket| handle_client(socket, registry))
}

/// What a node does for a client, cut down to the registration either side of a
/// read loop.
async fn handle_client(socket: WebSocket, registry: Arc<WebSocketConnRegistry>) {
    let (socket_tx, mut socket_rx) = socket.split();
    let connection_id = nanoid!();
    registry
        .register_connection(connection_id.clone(), Arc::new(Mutex::new(socket_tx)))
        .await;

    while let Some(Ok(message)) = socket_rx.next().await {
        if let Some(acknowledged) = client_ack(&message) {
            registry
                .record_client_ack(connection_id.clone(), acknowledged)
                .await;
        }
    }

    registry.deregister_connection(connection_id).await;
}

/// Reads a client acknowledgement out of a text message, in the shape the
/// protocol names, which is what settles a message the holding node took on.
fn client_ack(message: &AxumMessage) -> Option<String> {
    let AxumMessage::Text(text) = message else {
        return None;
    };
    let body: serde_json::Value = serde_json::from_str(text).ok()?;
    if body.get("event").and_then(serde_json::Value::as_str) != Some("ack") {
        return None;
    }
    body.get("data")?
        .get("messageId")
        .and_then(serde_json::Value::as_str)
        .map(str::to_string)
}

async fn connect_client(
    node: &ClusterNode,
) -> (
    tokio_tungstenite::WebSocketStream<tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>>,
    String,
) {
    let before: Vec<String> = node
        .registry
        .get_connections()
        .into_iter()
        .map(|(id, _)| id)
        .collect();
    let addr = node.addr;
    let (socket, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
        .await
        .unwrap();

    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    loop {
        let now: Vec<String> = node
            .registry
            .get_connections()
            .into_iter()
            .map(|(id, _)| id)
            .collect();
        if let Some(id) = now.into_iter().find(|id| !before.contains(id)) {
            return (socket, id);
        }
        assert!(
            tokio::time::Instant::now() < deadline,
            "the connection should have been registered"
        );
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
}

/// A message for a client held by another node reaches it, and the sender hears
/// that it arrived.
#[test_log::test(tokio::test)]
async fn test_a_message_reaches_a_client_held_by_another_node() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-delivery";
    clear(&mut conn, prefix, &["m-1"]).await;

    // A group each, so the message is routed rather than overheard.
    let sender = start_node(prefix, "api-node-1", 1).await;
    let holder = start_node(prefix, "api-node-2", 1).await;
    assert_ne!(sender.group_id, holder.group_id);

    let (mut client, connection_id) = connect_client(&holder).await;

    let sent = sender
        .registry
        .send_message(
            connection_id,
            "m-1".to_string(),
            MessageType::Json,
            r#"{"event":"update"}"#.to_string(),
            Some(SendContext {
                // Waiting on the other node to say it took the message on,
                // which is the whole point of sending it this way.
                wait_for_ack: true,
                caller: None,
                inform_clients: vec![],
            }),
        )
        .await;
    assert!(
        sent.is_ok(),
        "the node holding the client should have acknowledged the message, got {sent:?}"
    );

    let received = tokio::time::timeout(Duration::from_secs(5), client.next())
        .await
        .expect("the client should have been sent the message")
        .unwrap()
        .unwrap();
    assert_eq!(
        received,
        tungstenite::Message::Text(r#"{"event":"update"}"#.to_string())
    );
}

/// A message for a client no node holds is declared lost, and the clients named
/// are told.
#[test_log::test(tokio::test)]
async fn test_a_message_for_a_client_nobody_holds_is_declared_lost() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-loss";
    clear(&mut conn, prefix, &["m-1"]).await;

    let sender = start_node(prefix, "api-node-1", 1).await;
    let _other = start_node(prefix, "api-node-2", 1).await;

    // Connected to the sending node, so it is somewhere the loss can be
    // reported to.
    let (mut watcher, watcher_id) = connect_client(&sender).await;

    // Bounded, because waiting on an acknowledgement that never comes and is
    // never given up on would hang rather than fail.
    let sent = tokio::time::timeout(
        Duration::from_secs(10),
        sender.registry.send_message(
            "a-client-that-is-not-here".to_string(),
            "m-1".to_string(),
            MessageType::Json,
            r#"{"event":"update"}"#.to_string(),
            Some(SendContext {
                wait_for_ack: true,
                caller: Some("test-caller".to_string()),
                inform_clients: vec![watcher_id],
            }),
        ),
    )
    .await
    .expect("a message nobody takes should run out of attempts rather than be waited on forever");
    assert!(
        matches!(sent, Err(WebSocketConnError::MessageLost(ref id)) if id == "m-1"),
        "a message nobody can take should be declared lost, got {sent:?}"
    );

    let received = tokio::time::timeout(Duration::from_secs(5), watcher.next())
        .await
        .expect("the client should have been told about the lost message")
        .unwrap()
        .unwrap();
    let tungstenite::Message::Binary(bytes) = received else {
        panic!("expected a binary lost message event, got {received:?}");
    };
    assert_eq!(bytes[..4], [0x1, 0x3, 0x0, 0x0]);
    let body: serde_json::Value = serde_json::from_slice(&bytes[4..]).unwrap();
    assert_eq!(body["messageId"], "m-1");
    assert_eq!(body["caller"], "test-caller");
}

/// A client's whereabouts are written when it connects and taken away when it
/// goes, which is what lets the other nodes find it and stop trying.
#[test_log::test(tokio::test)]
async fn test_a_client_is_findable_while_it_is_connected() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-locations";
    clear(&mut conn, prefix, &[]).await;

    let node = start_node(prefix, "api-node-1", 1).await;
    let (mut client, connection_id) = connect_client(&node).await;

    // Waited on rather than read once, because a connection is put in the
    // node's own map before the record of where it is has been written, and
    // connecting only waits for the first of those.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    loop {
        let recorded: String = conn
            .get(&format!("{prefix}:conn:{connection_id}"))
            .await
            .unwrap();
        if recorded == node.group_id {
            break;
        }
        assert!(
            tokio::time::Instant::now() < deadline,
            "a connected client should be recorded against the group holding it, found \
             {recorded:?}"
        );
        tokio::time::sleep(Duration::from_millis(20)).await;
    }

    client.close(None).await.unwrap();

    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    while conn
        .exists(&format!("{prefix}:conn:{connection_id}"))
        .await
        .unwrap()
    {
        assert!(
            tokio::time::Instant::now() < deadline,
            "a client that has gone should stop being findable"
        );
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
}

/// A message resent because its acknowledgement went missing is acknowledged
/// again and delivered once.
///
/// The two halves matter equally. Delivering twice is the duplicate this
/// avoids; staying silent would have the sender run out of attempts and report
/// a message that did arrive as lost.
#[test_log::test(tokio::test)]
async fn test_a_resent_message_is_acknowledged_again_and_delivered_once() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-duplicates";
    clear(&mut conn, prefix, &["m-1"]).await;

    let sender = start_node(prefix, "api-node-1", 1).await;
    let holder = start_node(prefix, "api-node-2", 1).await;
    assert_ne!(sender.group_id, holder.group_id);

    let (mut client, connection_id) = connect_client(&holder).await;

    // The same id twice, which is what the sender does when it hears nothing
    // back in time.
    for attempt in 0..2 {
        let sent = sender
            .registry
            .send_message(
                connection_id.clone(),
                "m-1".to_string(),
                MessageType::Json,
                r#"{"event":"update"}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: true,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await;
        assert!(
            sent.is_ok(),
            "attempt {attempt} should have been acknowledged, got {sent:?}"
        );
    }

    let received = tokio::time::timeout(Duration::from_secs(5), client.next())
        .await
        .expect("the client should have been sent the message")
        .unwrap()
        .unwrap();
    assert_eq!(
        received,
        tungstenite::Message::Text(r#"{"event":"update"}"#.to_string())
    );

    assert!(
        tokio::time::timeout(Duration::from_millis(500), client.next())
            .await
            .is_err(),
        "the client should have been sent the message once, not once per attempt"
    );
}

/// A message the holding node could not send is not left recorded as one it
/// forwarded.
///
/// The record is written before the message goes out, since two copies arriving
/// together must not both find it absent. A copy that then fails to send has to
/// give the record back, or the next attempt is recognised as a duplicate and
/// acknowledged without ever reaching the client, which reports a message as
/// delivered that nobody received.
#[test_log::test(tokio::test)]
async fn test_a_message_that_could_not_be_sent_is_not_recorded_as_forwarded() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-undelivered";
    clear(&mut conn, prefix, &["m-1"]).await;

    let sender = start_node(prefix, "api-node-1", 1).await;
    let holder = start_node(prefix, "api-node-2", 1).await;
    assert_ne!(sender.group_id, holder.group_id);

    let (mut client, connection_id) = connect_client(&holder).await;

    // Refused by the holding node when it comes to make a frame of it, which
    // is a delivery that fails after the message has been taken as forwarded.
    let sent = tokio::time::timeout(
        Duration::from_secs(10),
        sender.registry.send_message(
            connection_id.clone(),
            "m-1".to_string(),
            MessageType::Binary,
            "not base64 at all".to_string(),
            Some(SendContext {
                wait_for_ack: true,
                caller: None,
                inform_clients: vec![],
            }),
        ),
    )
    .await
    .expect("a message that cannot be sent should run out of attempts rather than hang");
    assert!(
        matches!(sent, Err(WebSocketConnError::MessageLost(ref id)) if id == "m-1"),
        "a message that never reached its client should be declared lost, got {sent:?}"
    );

    assert!(
        !conn.exists(&format!("{prefix}:msg:m-1")).await.unwrap(),
        "a message that was not sent should not be left recorded as forwarded"
    );

    // The same id again, as a sender retrying at the application level would.
    // It has to be treated as a first delivery rather than a duplicate.
    sender
        .registry
        .send_message(
            connection_id,
            "m-1".to_string(),
            MessageType::Json,
            r#"{"event":"second-attempt"}"#.to_string(),
            None,
        )
        .await
        .unwrap();

    let received = tokio::time::timeout(Duration::from_secs(5), client.next())
        .await
        .expect("the client should have been sent the message on the second attempt")
        .unwrap()
        .unwrap();
    assert_eq!(
        received,
        tungstenite::Message::Text(r#"{"event":"second-attempt"}"#.into()),
        "a message that failed to send once should still reach its client"
    );
}

/// A message forwarded to the node holding its client is not settled by
/// reaching the socket. That node takes it on, waits for the client, and
/// reports the outcome back, so waiting on an acknowledgement means the client
/// received it rather than that some other node wrote bytes.
#[test_log::test(tokio::test)]
async fn test_a_relayed_message_is_settled_by_its_client_not_by_the_socket() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-client-acks";
    clear(&mut conn, prefix, &["m-1"]).await;

    // Long enough that the holding node is still waiting on its client while
    // this test checks that the sender is too, rather than having spent its
    // attempts and declared the message lost.
    let sender = start_node_with_ack_timings(prefix, "api-node-1", 1, 5_000, 3).await;
    let holder = start_node_with_ack_timings(prefix, "api-node-2", 1, 5_000, 3).await;
    assert_ne!(sender.group_id, holder.group_id);

    let (mut client, connection_id) = connect_client(&holder).await;

    let registry = sender.registry.clone();
    let mut waiting = tokio::spawn(async move {
        registry
            .send_message(
                connection_id,
                "m-1".to_string(),
                MessageType::Json,
                r#"{"messageId":"m-1","ack":true}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: true,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await
    });

    let received = tokio::time::timeout(Duration::from_secs(5), client.next())
        .await
        .expect("the client should have been sent the message")
        .unwrap()
        .unwrap();
    assert_eq!(
        received,
        tungstenite::Message::Text(r#"{"messageId":"m-1","ack":true}"#.into())
    );

    // The message has reached the socket, which used to be the whole of what a
    // sender waited for. It has to keep waiting.
    assert!(
        tokio::time::timeout(Duration::from_millis(400), &mut waiting)
            .await
            .is_err(),
        "a sender should not be told a client received a message before it did"
    );

    client
        .send(tungstenite::Message::Text(
            r#"{"event":"ack","data":{"messageId":"m-1"}}"#.into(),
        ))
        .await
        .unwrap();

    let settled = tokio::time::timeout(Duration::from_secs(5), waiting)
        .await
        .expect("the acknowledgement should have settled the message")
        .unwrap();
    assert!(
        settled.is_ok(),
        "a message its client acknowledged should not be an error, got {settled:?}"
    );
}

/// A client that never acknowledges has the message declared lost by the node
/// holding it, and that outcome reaches the node waiting on it.
///
/// Only the holding node can decide this, since only it can send to that client
/// and only it receives the acknowledgement.
#[test_log::test(tokio::test)]
async fn test_a_client_that_never_acknowledges_has_the_loss_reported_back() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-client-ack-loss";
    clear(&mut conn, prefix, &["m-1"]).await;

    // The holding node spends its attempts quickly while the sender's own
    // deadline is a long way off, so only a loss reported back can settle this
    // in the time the test allows. Matching timings would let the sender's
    // deadline settle it and prove nothing about the report.
    let sender = start_node_with_ack_timings(prefix, "api-node-1", 1, 5_000, 3).await;
    let holder = start_node_with_ack_timings(prefix, "api-node-2", 1, 100, 2).await;
    assert_ne!(sender.group_id, holder.group_id);

    let (_client, connection_id) = connect_client(&holder).await;

    let sent = tokio::time::timeout(
        Duration::from_secs(3),
        sender.registry.send_message(
            connection_id,
            "m-1".to_string(),
            MessageType::Json,
            r#"{"messageId":"m-1","ack":true}"#.to_string(),
            Some(SendContext {
                wait_for_ack: true,
                caller: None,
                inform_clients: vec![],
            }),
        ),
    )
    .await
    .expect("the loss the holding node decided should reach the sender, rather than the \n     sender waiting out its own deadline");

    assert!(
        matches!(sent, Err(WebSocketConnError::MessageLost(ref id)) if id == "m-1"),
        "a message no client acknowledged should be reported lost, got {sent:?}"
    );
}
