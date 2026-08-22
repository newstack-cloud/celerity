use std::{collections::HashMap, sync::Arc, time::Duration};

use celerity_helpers::{
    redis::{get_redis_connection, ConnectionWrapper},
    testing::{redis_config, redis_connection},
};

use celerity_ws_redis::{
    locations::ConnectionLocations,
    node_group::{join_or_create, leave, node_key, NodeGroup, NodeGroupConfig},
    pubsub::{connect, PubSubConnectionConfig},
};
use celerity_ws_registry::{
    registry::ConnectionLocationStore,
    types::{AckMessage, Message, MessageType, WebSocketMessage},
};
use tokio::sync::mpsc::{channel, Receiver, Sender};

/// Clears anything a previous run left behind under a prefix, since a test that
/// fails never reaches its own cleanup.
///
/// The connections are named rather than matched, because their entries are
/// spread across a cluster's slots by design and no one request reaches all of
/// them.
async fn clear(conn: &mut ConnectionWrapper, prefix: &str, connection_ids: &[&str]) {
    for connection_id in connection_ids {
        conn.del(&format!("{prefix}:conn:{connection_id}"))
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

/// A node of a cluster, with everything a test needs to speak for it.
struct Node {
    name: String,
    group: NodeGroup,
    tx: Sender<Message>,
    rx: Receiver<Message>,
    locations: Arc<ConnectionLocations>,
    moved: Sender<NodeGroup>,
    config: NodeGroupConfig,
}

async fn start_node(prefix: &str, name: &str, capacity: usize) -> Node {
    let mut conn = redis_connection().await;
    let config = NodeGroupConfig {
        server_node_name: name.to_string(),
        capacity,
        node_ttl_ms: 30_000,
        key_prefix: prefix.to_string(),
    };
    let group = join_or_create(&mut conn, &config).await.unwrap();
    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        group.id.clone(),
        config.node_ttl_ms,
    );

    let (moved, moved_rx) = channel(4);
    let (tx, rx) = connect(
        PubSubConnectionConfig {
            server_node_name: name.to_string(),
            key_prefix: prefix.to_string(),
            nodes: redis_config().nodes,
            password: redis_config().password,
            cluster_mode: redis_config().cluster_mode,
            // Short, so a test watching the old channel go quiet is not waiting
            // on a production grace period.
            migration_grace_ms: 300,
        },
        group.clone(),
        locations.clone(),
        moved_rx,
    )
    .await
    .unwrap();

    Node {
        name: name.to_string(),
        group,
        tx,
        rx,
        locations,
        moved,
        config,
    }
}

/// Two nodes in groups of their own carry each other's traffic, and each one
/// only hears what is addressed to a connection its group holds.
#[test_log::test(tokio::test)]
async fn test_messages_reach_the_group_holding_the_connection() {
    let mut conn = redis_connection().await;
    let prefix = "test-pubsub-routing";
    clear(&mut conn, prefix, &["1", "2", "7", "probe"]).await;

    // Capacity of one, so the two nodes are in separate groups and a message
    // only arrives by being routed rather than by everyone hearing everything.
    let node1 = start_node(prefix, "api-node-1", 1).await;
    let node2 = start_node(prefix, "api-node-2", 1).await;
    // A third group, holding neither connection. Without it the test cannot
    // tell routing from every node hearing everything, since a message that
    // goes everywhere reaches the two that want it and is filtered out by the
    // one that sent it.
    let mut bystander = start_node(prefix, "api-node-3", 1).await;
    assert_ne!(node1.group.id, node2.group.id);
    assert_ne!(bystander.group.id, node1.group.id);
    assert_ne!(bystander.group.id, node2.group.id);

    node1.locations.record("1").await.unwrap();
    node2.locations.record("2").await.unwrap();

    let (collect_node1_tx, mut collect_node1_rx) = channel(1024);
    let (collect_node2_tx, mut collect_node2_rx) = channel(1024);

    let Node {
        tx: node1_tx,
        rx: mut node1_rx,
        ..
    } = node1;
    let Node {
        tx: node2_tx,
        rx: mut node2_rx,
        ..
    } = node2;

    tokio::spawn(async move {
        // Node 1 sends messages that are for a connection node 2 holds.
        send_messages_and_listen(
            vec!["2".to_string()],
            node1_tx,
            &mut node1_rx,
            "api-node-2".to_string(),
            collect_node1_tx,
        )
        .await;
    });

    tokio::spawn(async move {
        send_messages_and_listen(
            vec!["1".to_string()],
            node2_tx,
            &mut node2_rx,
            "api-node-1".to_string(),
            collect_node2_tx,
        )
        .await;
    });

    let mut received_by_connection = HashMap::<String, Vec<String>>::new();
    let mut messages_received = 0;
    let mut acks_by_node = HashMap::<String, AckMessage>::new();
    let mut acks_received = 0;
    let gathering = async {
        // A hundred messages each way, and one acknowledgement from each
        // node for the last of them.
        while messages_received < 200 || acks_received < 2 {
            tokio::select! {
                Some(message) = collect_node1_rx.recv() => {
                    match message {
                        Message::WebSocket(message) => {
                            received_by_connection
                                .entry(message.connection_id)
                                .or_default()
                                .push(message.message);
                            messages_received += 1;
                        }
                        Message::Ack(ack_message) => {
                            acks_by_node.insert("node1".to_string(), ack_message);
                            acks_received += 1;
                        }
                    }
                }
                Some(message) = collect_node2_rx.recv() => {
                    match message {
                        Message::WebSocket(message) => {
                            received_by_connection
                                .entry(message.connection_id)
                                .or_default()
                                .push(message.message);
                            messages_received += 1;
                        }
                        Message::Ack(ack_message) => {
                            acks_by_node.insert("node2".to_string(), ack_message);
                            acks_received += 1;
                        }
                    }
                }
            }
        }
    };
    tokio::time::timeout(Duration::from_secs(20), gathering)
        .await
        .expect("every message and acknowledgement should have arrived");

    assert!(
        bystander.rx.try_recv().is_err(),
        "a group holding neither connection should have been told nothing"
    );

    assert_eq!(
        received_by_connection.len(),
        2,
        "both connections should have been sent to, not one of them twice"
    );
    assert_eq!(received_by_connection["1"], build_message_list("1", 100));
    assert_eq!(received_by_connection["2"], build_message_list("2", 100));
    assert_eq!(acks_by_node.len(), 2);
    // Node 1 hears about the message it sent to the group holding connection 2,
    // which means the acknowledgement found its way back to the sender's own
    // group rather than to the group that sent it.
    assert_eq!(
        acks_by_node["node1"],
        AckMessage {
            message_id: "conn-2-msg-99".to_string(),
            message_node: "api-node-1".to_string(),
        }
    );
    assert_eq!(
        acks_by_node["node2"],
        AckMessage {
            message_id: "conn-1-msg-99".to_string(),
            message_node: "api-node-2".to_string(),
        }
    );
}

/// A message for a connection nothing has recorded is offered to every group.
///
/// A client that connected a moment ago looks exactly like this, and dropping
/// the message would lose it for a client that is there.
#[test_log::test(tokio::test)]
async fn test_a_message_for_an_unrecorded_connection_reaches_every_group() {
    let mut conn = redis_connection().await;
    let prefix = "test-pubsub-fanout";
    clear(&mut conn, prefix, &["1", "2", "7", "probe"]).await;

    let node1 = start_node(prefix, "api-node-1", 1).await;
    let mut node2 = start_node(prefix, "api-node-2", 1).await;
    assert_ne!(node1.group.id, node2.group.id);

    // Nothing records where connection 7 is, which is what a client that has
    // only just connected looks like from another node.
    node1
        .tx
        .send(Message::WebSocket(WebSocketMessage {
            connection_id: "7".to_string(),
            message_id: "m-1".to_string(),
            message_type: MessageType::Json,
            source_node: node1.name.clone(),
            message: r#"{"event":"update"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();

    let received = tokio::time::timeout(Duration::from_secs(5), node2.rx.recv())
        .await
        .expect("a message for a connection nobody claims should still be offered around")
        .unwrap();
    let Message::WebSocket(received) = received else {
        panic!("expected a websocket message, got {received:?}");
    };
    assert_eq!(received.message_id, "m-1");

    leave(&mut conn, &node1.config, &node1.group).await.unwrap();
    leave(&mut conn, &node2.config, &node2.group).await.unwrap();
}

/// An acknowledgement is published to the ack mirror of the group holding the
/// node that sent the message.
///
/// A node subscribes to both of its group's channels, so it would receive an
/// acknowledgement sent to either. Which one carries it is still part of the
/// protocol, and anything reading the mirror on its own would miss one sent to
/// the wrong channel, so this watches the channel rather than the node.
#[test_log::test(tokio::test)]
async fn test_an_acknowledgement_goes_to_the_ack_mirror_of_the_senders_group() {
    let mut conn = redis_connection().await;
    let prefix = "test-pubsub-ack-channel";
    clear(&mut conn, prefix, &["1", "2", "7", "probe"]).await;

    let sender = start_node(prefix, "api-node-1", 1).await;
    let holder = start_node(prefix, "api-node-2", 1).await;
    assert_ne!(sender.group.id, holder.group.id);

    // Watching the sender's ack mirror directly, as another implementation
    // following the protocol would.
    let (push_tx, mut push_rx) = tokio::sync::mpsc::unbounded_channel();
    let mut watcher = get_redis_connection(&redis_config(), Some(push_tx))
        .await
        .unwrap();
    watcher.subscribe(&sender.group.ack_channel).await.unwrap();

    holder
        .tx
        .send(Message::Ack(AckMessage {
            message_id: "m-1".to_string(),
            message_node: sender.name.clone(),
        }))
        .await
        .unwrap();

    // Subscribing is itself announced on this channel, so the confirmation is
    // read past rather than mistaken for the acknowledgement.
    let published = async {
        loop {
            let push = push_rx.recv().await.unwrap();
            if push.kind == redis::PushKind::Message {
                let payload: String =
                    redis::FromRedisValue::from_redis_value(&push.data[1]).unwrap();
                return payload;
            }
        }
    };
    let payload = tokio::time::timeout(Duration::from_secs(5), published)
        .await
        .expect("the acknowledgement should have been published to the ack mirror");
    assert!(
        payload.contains("m-1"),
        "the ack mirror should have carried the acknowledgement, it carried {payload}"
    );

    leave(&mut conn, &sender.config, &sender.group)
        .await
        .unwrap();
    leave(&mut conn, &holder.config, &holder.group)
        .await
        .unwrap();
}

/// A node dropped from its group and then given a place back in it before the
/// grace period is up keeps listening to it.
///
/// Giving up the old channels is scheduled when a node moves, and by the time
/// that comes around the node may be back where it started. Giving them up then
/// would leave it unable to receive messages from the group it is actually in,
/// with nothing to tell it so, since it believes it is subscribed.
#[test_log::test(tokio::test)]
async fn test_a_node_that_comes_back_to_the_group_it_left_keeps_receiveing_messages_from_it() {
    let mut conn = redis_connection().await;
    let prefix = "test-pubsub-return";
    clear(&mut conn, prefix, &["1", "2", "7", "probe"]).await;

    let sender = start_node(prefix, "api-node-1", 5).await;
    let mut mover = start_node(prefix, "api-node-2", 5).await;
    let home = mover.group.clone();

    let elsewhere = NodeGroup::new(prefix, "somewhere-else".to_string());
    conn.sadd(
        &format!("{prefix}:{{group-meta}}:node-groups"),
        &elsewhere.id,
    )
    .await
    .unwrap();

    // Away and back again, both inside the grace period the first move started.
    mover.moved.send(elsewhere.clone()).await.unwrap();
    mover.locations.set_group(elsewhere.id.clone());
    mover.moved.send(home.clone()).await.unwrap();
    mover.locations.set_group(home.id.clone());
    mover.locations.record("2").await.unwrap();

    // Long enough for the first move's grace period to have come and gone.
    tokio::time::sleep(Duration::from_millis(600)).await;

    sender
        .tx
        .send(Message::WebSocket(WebSocketMessage {
            connection_id: "2".to_string(),
            message_id: "back-where-it-started".to_string(),
            message_type: MessageType::Json,
            source_node: sender.name.clone(),
            message: r#"{"event":"back"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();
    assert_eq!(
        next_message_id(&mut mover.rx).await,
        "back-where-it-started",
        "a node back in the group it left should still be listening to it"
    );
}

/// A node that moves group hears its new channel straight away, and its old one
/// until the grace period is up.
///
/// A sender reads where a connection is and publishes a moment later, so
/// messages are always in flight against a mapping that has just changed.
#[test_log::test(tokio::test)]
async fn test_a_node_that_moves_group_hears_both_until_the_grace_is_up() {
    let mut conn = redis_connection().await;
    let prefix = "test-pubsub-migration";
    clear(&mut conn, prefix, &["1", "2", "7", "probe"]).await;

    let sender = start_node(prefix, "api-node-1", 5).await;
    let mut mover = start_node(prefix, "api-node-2", 5).await;
    let left_behind = mover.group.clone();

    // Somewhere else to be, which is what the heartbeat would have found.
    let joined = NodeGroup::new(prefix, "a-group-of-its-own".to_string());
    conn.sadd(&format!("{prefix}:{{group-meta}}:node-groups"), &joined.id)
        .await
        .unwrap();
    mover.moved.send(joined.clone()).await.unwrap();
    mover.locations.set_group(joined.id.clone());
    mover.locations.record("2").await.unwrap();
    wait_until_listening(&sender.tx, &mut mover.rx, &sender.name, &mover.locations).await;

    // Addressed to the group it moved into.
    sender
        .tx
        .send(Message::WebSocket(WebSocketMessage {
            connection_id: "2".to_string(),
            message_id: "after-the-move".to_string(),
            message_type: MessageType::Json,
            source_node: sender.name.clone(),
            message: r#"{"event":"after"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();
    assert_eq!(
        next_message_id(&mut mover.rx).await,
        "after-the-move",
        "a node should hear the group it has just joined"
    );

    // Addressed to the group it left, as a sender holding the old mapping
    // would.
    sender.locations.set_group(left_behind.id.clone());
    sender.locations.record("2").await.unwrap();
    sender
        .tx
        .send(Message::WebSocket(WebSocketMessage {
            connection_id: "2".to_string(),
            message_id: "still-in-flight".to_string(),
            message_type: MessageType::Json,
            source_node: sender.name.clone(),
            message: r#"{"event":"in flight"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();
    assert_eq!(
        next_message_id(&mut mover.rx).await,
        "still-in-flight",
        "a node should still hear the group it left while the grace period runs"
    );

    // Once the grace period is up the old channel is given up, and a message sent
    // there is nobody's.
    tokio::time::sleep(Duration::from_millis(600)).await;
    sender
        .tx
        .send(Message::WebSocket(WebSocketMessage {
            connection_id: "2".to_string(),
            message_id: "too-late".to_string(),
            message_type: MessageType::Json,
            source_node: sender.name.clone(),
            message: r#"{"event":"too late"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();
    assert!(
        tokio::time::timeout(Duration::from_millis(500), mover.rx.recv())
            .await
            .is_err(),
        "a node should have given up the group it left once the grace period was up"
    );
}

/// Waits until a node is listening to the group it has just moved into, by
/// offering it a message until one arrives.
///
/// Being told it has moved and taking up the new group's channels are separate
/// steps, and nothing says when the second has happened. Redis cannot be asked
/// either, since it answers only for the subscribers of the node asked, which
/// in a cluster is not the one holding this subscription.
///
/// Offered often, because the grace period on the group left behind is already
/// running by the time the first one lands.
async fn wait_until_listening(
    tx: &Sender<Message>,
    rx: &mut Receiver<Message>,
    source_node: &str,
    locations: &Arc<ConnectionLocations>,
) {
    locations.record("probe").await.unwrap();

    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    loop {
        tx.send(Message::WebSocket(WebSocketMessage {
            connection_id: "probe".to_string(),
            message_id: "probe".to_string(),
            message_type: MessageType::Json,
            source_node: source_node.to_string(),
            message: r#"{"event":"probe"}"#.to_string(),
            inform_clients_on_loss: None,
            caller: None,
        }))
        .await
        .unwrap();

        // The probe itself, rather than whatever else may be queued, since
        // anything else says nothing about the group just moved into.
        if let Ok(Some(Message::WebSocket(received))) =
            tokio::time::timeout(Duration::from_millis(5), rx.recv()).await
        {
            if received.connection_id == "probe" && received.message_id == "probe" {
                break;
            }
        }
        assert!(
            tokio::time::Instant::now() < deadline,
            "the node should have taken up the channels of the group it moved into"
        );
    }

    // Whatever else was offered before the one that landed.
    while tokio::time::timeout(Duration::from_millis(20), rx.recv())
        .await
        .is_ok()
    {}
    locations.forget("probe").await.unwrap();
}

async fn next_message_id(rx: &mut Receiver<Message>) -> String {
    let received = tokio::time::timeout(Duration::from_secs(5), rx.recv())
        .await
        .expect("a message should have arrived")
        .unwrap();
    match received {
        Message::WebSocket(message) => message.message_id,
        other => panic!("expected a websocket message, got {other:?}"),
    }
}

async fn send_messages_and_listen(
    dst_connection_ids: Vec<String>,
    src_client_tx: Sender<Message>,
    src_client_rx: &mut Receiver<Message>,
    other_node_name: String,
    collect_tx: Sender<Message>,
) {
    for connection_id in dst_connection_ids {
        for i in 0..100 {
            src_client_tx
                .send(Message::WebSocket(WebSocketMessage {
                    connection_id: connection_id.clone(),
                    message_id: format!("conn-{connection_id}-msg-{i}"),
                    message_type: MessageType::Json,
                    source_node: "node1".to_string(),
                    message: format!(
                        "{{\"message\": \"This is message {i} for {connection_id}\"}}"
                    ),
                    inform_clients_on_loss: None,
                    caller: None,
                }))
                .await
                .unwrap();
        }
    }

    while let Some(message) = src_client_rx.recv().await {
        if let Message::WebSocket(message) = message.clone() {
            if message.message_id.contains("msg-99") {
                let _ = src_client_tx
                    .send(Message::Ack(AckMessage {
                        message_id: message.message_id,
                        message_node: other_node_name.clone(),
                    }))
                    .await;
            }
        }
        collect_tx.send(message).await.unwrap();
    }
}

fn build_message_list(connection_id: &str, count: usize) -> Vec<String> {
    (0..count)
        .map(|i| format!("{{\"message\": \"This is message {i} for {connection_id}\"}}"))
        .collect()
}
