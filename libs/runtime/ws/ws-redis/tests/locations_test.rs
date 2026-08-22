use std::time::Duration;

use celerity_helpers::{redis::ConnectionWrapper, testing::redis_connection};

use celerity_ws_redis::{
    locations::ConnectionLocations,
    node_group::{join_or_create, node_key, spawn_heartbeat, NodeGroup, NodeGroupConfig},
};
use celerity_ws_registry::registry::ConnectionLocationStore;
use tokio::sync::mpsc::{channel, Receiver};

fn config(prefix: &str, node: &str, node_ttl_ms: u64) -> NodeGroupConfig {
    NodeGroupConfig {
        server_node_name: node.to_string(),
        capacity: 5,
        node_ttl_ms,
        key_prefix: prefix.to_string(),
    }
}

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

async fn members(conn: &mut ConnectionWrapper, prefix: &str, group: &NodeGroup) -> Vec<String> {
    conn.smembers(&format!(
        "{prefix}:{{group-meta}}:node-group-members:{}",
        group.id
    ))
    .await
    .unwrap()
}

/// A heartbeat and the channel it reports a group change on.
fn heartbeat(
    conn: ConnectionWrapper,
    node_config: NodeGroupConfig,
    group: NodeGroup,
    locations: std::sync::Arc<ConnectionLocations>,
) -> (
    tokio::sync::oneshot::Sender<()>,
    tokio::task::JoinHandle<()>,
    Receiver<NodeGroup>,
) {
    let (shutdown_tx, shutdown_rx) = tokio::sync::oneshot::channel();
    let (moved_tx, moved_rx) = channel(4);
    let handle = spawn_heartbeat(conn, node_config, group, locations, moved_tx, shutdown_rx);
    (shutdown_tx, handle, moved_rx)
}

/// A connection is recorded against the group holding it, and reading it back
/// is what tells a sender where to publish.
#[test_log::test(tokio::test)]
async fn test_a_connection_is_found_where_it_was_recorded() {
    let mut conn = redis_connection().await;
    let prefix = "test-locations-record";
    clear(&mut conn, prefix, &["conn-1"]).await;

    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        "group-1".to_string(),
        10_000,
    );

    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        None,
        "a connection nothing has recorded should not be claimed by a group"
    );

    locations.record("conn-1").await.unwrap();
    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        Some("group-1".to_string())
    );

    locations.forget("conn-1").await.unwrap();
    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        None,
        "a connection that has gone should not still be claimed"
    );
}

/// An entry nothing keeps alive expires, which is what stops a node that died
/// receiving messages for connections that died with it.
#[test_log::test(tokio::test)]
async fn test_an_entry_nothing_refreshes_expires() {
    let mut conn = redis_connection().await;
    let prefix = "test-locations-expiry";
    clear(&mut conn, prefix, &["conn-1"]).await;

    let locations =
        ConnectionLocations::new(conn.clone(), prefix.to_string(), "group-1".to_string(), 200);
    locations.record("conn-1").await.unwrap();

    tokio::time::sleep(Duration::from_millis(400)).await;
    assert_eq!(locations.group_for("conn-1").await.unwrap(), None);

    // Refreshing after the entry has gone puts it back, since this node still
    // holds the connection whatever Redis has forgotten.
    assert_eq!(locations.refresh().await.unwrap(), 1);
    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        Some("group-1".to_string())
    );
}

/// The heartbeat keeps everything this node wrote alive, past the point where
/// any of it would have expired on its own.
#[test_log::test(tokio::test)]
async fn test_the_heartbeat_keeps_a_node_and_its_connections_alive() {
    let mut conn = redis_connection().await;
    let prefix = "test-heartbeat-alive";
    clear(&mut conn, prefix, &["conn-1"]).await;

    let node_config = config(prefix, "node-1", 300);
    let group = join_or_create(&mut conn, &node_config).await.unwrap();
    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        group.id.clone(),
        node_config.node_ttl_ms,
    );
    locations.record("conn-1").await.unwrap();

    let (shutdown_tx, handle, _moved) = heartbeat(
        conn.clone(),
        node_config.clone(),
        group.clone(),
        locations.clone(),
    );

    // Comfortably past the expiry, so anything still there is there because it
    // was refreshed rather than because it had not run out yet.
    tokio::time::sleep(Duration::from_millis(900)).await;
    assert!(
        conn.exists(&node_key(prefix, "node-1")).await.unwrap(),
        "the node should still be saying it is running"
    );
    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        Some(group.id.clone()),
        "the connection should still be recorded against its group"
    );

    shutdown_tx.send(()).unwrap();
    handle.await.unwrap();
}

/// A node dropped from its group while it was slow takes a place again on the
/// next beat, rather than running on as a member of nothing.
///
/// The group it left still has room, so it lands back in the same one and its
/// subscriptions go on being the right ones.
#[test_log::test(tokio::test)]
async fn test_a_node_dropped_from_its_group_rejoins_it() {
    let mut conn = redis_connection().await;
    let prefix = "test-heartbeat-rejoin";
    clear(&mut conn, prefix, &["conn-1"]).await;

    let node_config = config(prefix, "node-1", 300);
    let group = join_or_create(&mut conn, &node_config).await.unwrap();
    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        group.id.clone(),
        node_config.node_ttl_ms,
    );

    let (shutdown_tx, handle, _moved) = heartbeat(
        conn.clone(),
        node_config.clone(),
        group.clone(),
        locations.clone(),
    );

    // What another node would have done on finding this one's liveness key
    // expired.
    conn.srem(
        &format!("{prefix}:{{group-meta}}:node-group-members:{}", group.id),
        "node-1",
    )
    .await
    .unwrap();

    tokio::time::sleep(Duration::from_millis(500)).await;
    assert_eq!(
        members(&mut conn, prefix, &group).await,
        vec!["node-1".to_string()],
        "the node should have taken a place again, in the group it was already using"
    );

    shutdown_tx.send(()).unwrap();
    handle.await.unwrap();
}

/// Shutting down takes the node's membership and its connections away at once,
/// rather than leaving the group looking fuller than it is until the expiry
/// catches up.
#[test_log::test(tokio::test)]
async fn test_shutting_down_takes_everything_the_node_wrote_with_it() {
    let mut conn = redis_connection().await;
    let prefix = "test-heartbeat-shutdown";
    clear(&mut conn, prefix, &["conn-1"]).await;

    let node_config = config(prefix, "node-1", 10_000);
    let group = join_or_create(&mut conn, &node_config).await.unwrap();
    let locations = ConnectionLocations::new(
        conn.clone(),
        prefix.to_string(),
        group.id.clone(),
        node_config.node_ttl_ms,
    );
    locations.record("conn-1").await.unwrap();

    let (shutdown_tx, handle, _moved) = heartbeat(
        conn.clone(),
        node_config.clone(),
        group.clone(),
        locations.clone(),
    );

    shutdown_tx.send(()).unwrap();
    handle.await.unwrap();

    assert!(!conn.exists(&node_key(prefix, "node-1")).await.unwrap());
    assert!(members(&mut conn, prefix, &group).await.is_empty());
    assert_eq!(
        locations.group_for("conn-1").await.unwrap(),
        None,
        "a node that shut down should not leave its connections claimed by a group it has left"
    );
}

/// A connection whose entry could not be written still has to be one the node
/// keeps alive, since the refresh is the only thing that will ever write it.
/// Dropping it there would leave the connection unreachable from any other node
/// for as long as it lasted.
///
/// The write is made to fail by asking for an expiry of nothing, which Redis
/// refuses.
#[test_log::test(tokio::test)]
async fn test_a_connection_is_kept_alive_even_where_its_entry_could_not_be_written() {
    let conn = redis_connection().await;
    let prefix = "test-locations-unwritable";

    let locations = ConnectionLocations::new(conn, prefix.to_string(), "group-1".to_string(), 0);

    assert!(
        locations.record("conn-1").await.is_err(),
        "an expiry of nothing should be refused, otherwise this test proves nothing"
    );

    assert_eq!(
        locations.forget_all().await.unwrap(),
        1,
        "the connection should be one of this node's to keep alive and to take away"
    );
}
