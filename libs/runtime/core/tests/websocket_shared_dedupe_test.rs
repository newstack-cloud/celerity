//! The record of what a client has already sent, shared across a cluster.
//!
//! A client resends when it does not hear that its message arrived, and a
//! client that reconnected first may resend to a different node. Only a record
//! every node can read recognises that as the message it already acted on.

#![cfg(feature = "ws_clustering")]

use celerity_helpers::{redis::ConnectionWrapper, testing::redis_connection};

use celerity_runtime_core::websocket_dedupe::{MessageIdStore, SeenMessages, SharedMessageIdStore};
use celerity_ws_redis::forwarded::ForwardedMessages;
use celerity_ws_registry::registry::ForwardedMessageStore;

/// Clears the records a previous run left behind.
///
/// A record outlives the run that wrote it, and a suite is usually run again
/// well inside that, so without this a second run finds its first message
/// already recorded and reads it as one it has already handled.
async fn clear(conn: &mut ConnectionWrapper, prefix: &str, message_ids: &[&str]) {
    for message_id in message_ids {
        conn.del(&format!("{prefix}:client-msg:{message_id}"))
            .await
            .unwrap();
        conn.del(&format!("{prefix}:msg:{message_id}"))
            .await
            .unwrap();
    }
}

/// A message handled by one node is recognised by another.
#[test_log::test(tokio::test)]
async fn test_a_message_handled_on_one_node_is_recognised_on_another() {
    let mut conn = redis_connection().await;
    let prefix = "test-shared-dedupe";
    clear(&mut conn, prefix, &["m-1", "m-2"]).await;

    let first_node = SharedMessageIdStore::new(conn.clone(), prefix.to_string(), 10_000);
    let second_node =
        SharedMessageIdStore::new(redis_connection().await, prefix.to_string(), 10_000);

    assert!(
        !first_node.record_and_check_seen("m-1").await,
        "a message nothing has handled should be acted on"
    );
    assert!(
        second_node.record_and_check_seen("m-1").await,
        "a client that resent to another node should be recognised there"
    );
    assert!(
        !second_node.record_and_check_seen("m-2").await,
        "a different message should be its own"
    );

    clear(&mut conn, prefix, &["m-1", "m-2"]).await;
}

/// The same message id in each direction is two different messages.
///
/// An application chooses its own ids for what it sends and what it receives,
/// so the two will collide. Sharing a keyspace would have whichever came first
/// hide the other, and the one hidden is never acted on.
#[test_log::test(tokio::test)]
async fn test_a_clients_messages_do_not_collide_with_the_servers() {
    let mut conn = redis_connection().await;
    let prefix = "test-shared-dedupe-keyspace";
    clear(&mut conn, prefix, &["shared-id"]).await;

    let from_clients = SharedMessageIdStore::new(conn.clone(), prefix.to_string(), 10_000);
    let to_clients = ForwardedMessages::new(conn.clone(), prefix.to_string(), 10_000);

    assert!(
        !from_clients.record_and_check_seen("shared-id").await,
        "a message a client sent should be acted on the first time"
    );
    assert!(
        !to_clients
            .record_and_check_forwarded("shared-id")
            .await
            .unwrap(),
        "a message the cluster is forwarding should go out, whatever id a client has used"
    );

    // And each still recognises its own.
    assert!(from_clients.record_and_check_seen("shared-id").await);
    assert!(to_clients
        .record_and_check_forwarded("shared-id")
        .await
        .unwrap());

    clear(&mut conn, prefix, &["shared-id"]).await;
}

/// A node reads the shared store once the cluster is joined, having read its
/// own memory before that.
#[test_log::test(tokio::test)]
async fn test_a_node_hands_over_to_the_shared_store() {
    let mut conn = redis_connection().await;
    let prefix = "test-shared-dedupe-handover";
    clear(&mut conn, prefix, &["m-1", "m-2"]).await;

    // Recorded by another node before this one joins.
    let elsewhere = SharedMessageIdStore::new(conn.clone(), prefix.to_string(), 10_000);
    assert!(!elsewhere.record_and_check_seen("m-1").await);

    // Asked about a different message, so that what this node remembers cannot
    // stand in for what the cluster knows once it has joined.
    let seen = SeenMessages::new(10_000);
    assert!(
        !seen.record_and_check_seen("m-2").await,
        "before joining, a node knows only what it has handled itself"
    );

    seen.attach_shared(SharedMessageIdStore::new(
        conn.clone(),
        prefix.to_string(),
        10_000,
    ))
    .unwrap();
    assert!(
        seen.record_and_check_seen("m-1").await,
        "once joined, a node should recognise what the cluster has already handled"
    );

    clear(&mut conn, prefix, &["m-1", "m-2"]).await;
}
