//! Joining a node to the rest of the cluster.
//!
//! Every piece this wires together has tests of its own. What these cover is
//! that the wiring happened at all, which nothing else notices, for example, a store left
//! unattached leaves each piece working perfectly and the runtime behaving as
//! though it were alone.

#![cfg(feature = "ws_clustering")]

use std::{sync::Arc, time::Duration};

use celerity_helpers::{
    redis::ConnectionWrapper,
    testing::{redis_config, redis_connection},
};

use celerity_runtime_core::{
    config::WsClusterConfig,
    websocket_cluster::join_cluster,
    websocket_dedupe::{MessageIdStore, SeenMessages, SharedMessageIdStore},
};
use celerity_ws_redis::{
    forwarded::ForwardedMessages, locations::ConnectionLocations, node_group::node_key,
};
use celerity_ws_registry::{
    registry::{WebSocketConnRegistry, WebSocketConnRegistryConfig},
    types::AckWorkerConfig,
};

/// Clears what an earlier run left behind, since a failed run never reaches its
/// own cleanup.
async fn clear(conn: &mut ConnectionWrapper, prefix: &str) {
    let index_key = format!("{prefix}:{{group-meta}}:node-groups");
    for group_id in conn.smembers(&index_key).await.unwrap() {
        let members_key = format!("{prefix}:{{group-meta}}:node-group-members:{group_id}");
        for member in conn.smembers(&members_key).await.unwrap() {
            conn.del(&node_key(prefix, &member)).await.unwrap();
        }
        conn.del(&members_key).await.unwrap();
    }
    conn.del(&index_key).await.unwrap();
    conn.del(&format!("{prefix}:client-msg:m-1")).await.unwrap();
}

fn registry(node_name: &str) -> Arc<WebSocketConnRegistry> {
    Arc::new(WebSocketConnRegistry::new(
        WebSocketConnRegistryConfig {
            ack_worker_config: Some(AckWorkerConfig::default()),
            server_node_name: node_name.to_string(),
        },
        None,
    ))
}

fn cluster_config(prefix: &str) -> WsClusterConfig {
    WsClusterConfig {
        redis_nodes: redis_config().nodes,
        redis_password: redis_config().password,
        redis_cluster_mode: redis_config().cluster_mode,
        key_prefix: Some(prefix.to_string()),
        node_group_capacity: Some(2),
        node_ttl_ms: Some(30_000),
        migration_grace_ms: Some(1_000),
        forwarded_ttl_ms: Some(30_000),
        seen_ttl_ms: Some(30_000),
    }
}

/// Everything a node needs to take part is attached to it.
///
/// Each store is checked by attaching another and finding it refused, which is
/// the registry saying it already has one. A store left unattached is the
/// failure this is for, and it is silent as the node serves its own clients and
/// simply never finds anybody else's.
#[test_log::test(tokio::test)]
async fn test_joining_attaches_everything_a_node_needs() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-wiring";
    clear(&mut conn, prefix).await;

    let registry = registry("node-1");
    let seen = SeenMessages::new(30_000);
    let shutdown = join_cluster(
        registry.clone(),
        Some(seen.clone()),
        cluster_config(prefix),
        "a-service",
        "node-1",
    )
    .await
    .expect("a node should be able to join");

    let (spare_tx, _spare_rx) = tokio::sync::mpsc::channel(1);
    assert!(
        registry.set_broadcaster(spare_tx).is_err(),
        "the registry should already have somewhere to send messages between nodes"
    );
    assert!(
        registry
            .set_locations(ConnectionLocations::new(
                conn.clone(),
                prefix.to_string(),
                "a-group".to_string(),
                30_000,
            ))
            .is_err(),
        "the registry should already know where to record the connections it holds"
    );
    assert!(
        registry
            .set_forwarded_messages(ForwardedMessages::new(
                conn.clone(),
                prefix.to_string(),
                30_000
            ))
            .is_err(),
        "the registry should already know where to record what it has forwarded"
    );
    assert!(
        seen.attach_shared(SharedMessageIdStore::new(
            conn.clone(),
            prefix.to_string(),
            30_000
        ))
        .is_err(),
        "what clients have sent should already be recorded where the cluster can see it"
    );

    // And the node took a place, which is what the rest of it hangs off.
    assert!(conn.exists(&node_key(prefix, "node-1")).await.unwrap());

    drop(shutdown);
}

/// What clients have sent is recorded where every node can read it, rather than
/// in the memory of whichever node took the message.
#[test_log::test(tokio::test)]
async fn test_joining_shares_what_clients_have_sent() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-wiring-seen";
    clear(&mut conn, prefix).await;

    // Another node handled this message before this one joined.
    let elsewhere = SharedMessageIdStore::new(conn.clone(), prefix.to_string(), 30_000);
    assert!(!elsewhere.record_and_check_seen("m-1").await);

    let seen = SeenMessages::new(30_000);
    let shutdown = join_cluster(
        registry("node-1"),
        Some(seen.clone()),
        cluster_config(prefix),
        "a-service",
        "node-1",
    )
    .await
    .expect("a node should be able to join");

    assert!(
        seen.record_and_check_seen("m-1").await,
        "a joined node should recognise what another node has already handled"
    );

    drop(shutdown);
}

/// Being told to stop takes this node out of its group rather than leaving the
/// others to wait for its expiry.
#[test_log::test(tokio::test)]
async fn test_a_node_told_to_stop_leaves_its_group() {
    let mut conn = redis_connection().await;
    let prefix = "test-cluster-wiring-leave";
    clear(&mut conn, prefix).await;

    let shutdown = join_cluster(
        registry("node-1"),
        None,
        cluster_config(prefix),
        "a-service",
        "node-1",
    )
    .await
    .expect("a node should be able to join");
    assert!(conn.exists(&node_key(prefix, "node-1")).await.unwrap());

    shutdown.send(()).unwrap();

    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);
    while conn.exists(&node_key(prefix, "node-1")).await.unwrap() {
        assert!(
            tokio::time::Instant::now() < deadline,
            "a node told to stop should take its place with it"
        );
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
}

/// A Redis deployment that cannot be reached stops the runtime starting.
#[test_log::test(tokio::test)]
async fn test_a_node_that_cannot_reach_redis_refuses_to_start() {
    let mut config = cluster_config("test-cluster-wiring-unreachable");
    // Nothing listens here, and the port is outside what a test would bind.
    config.redis_nodes = vec!["redis://127.0.0.1:6399/?protocol=resp3".to_string()];

    let joined = join_cluster(registry("node-1"), None, config, "a-service", "node-1").await;
    assert!(
        joined.is_err(),
        "a node that cannot reach the others should refuse to start rather than serve alone"
    );
}
