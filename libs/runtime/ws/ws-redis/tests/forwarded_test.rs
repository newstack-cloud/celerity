use std::time::Duration;

use celerity_helpers::testing::redis_connection;
use celerity_ws_redis::forwarded::ForwardedMessages;
use celerity_ws_registry::registry::ForwardedMessageStore;

/// The first node to forward a message records it, and any node asking
/// afterwards is told it has already gone out.
#[test_log::test(tokio::test)]
async fn test_a_message_is_recognised_after_it_has_been_forwarded() {
    let mut conn = redis_connection().await;
    let prefix = "test-forwarded-recognise";
    conn.del(&format!("{prefix}:msg:m-1")).await.unwrap();

    let forwarded = ForwardedMessages::new(conn.clone(), prefix.to_string(), 10_000);

    assert!(
        !forwarded.record_and_check_forwarded("m-1").await.unwrap(),
        "a message nothing has forwarded should be sent"
    );
    assert!(
        forwarded.record_and_check_forwarded("m-1").await.unwrap(),
        "a message already forwarded should be recognised"
    );

    // Another node, which is the case that matters as a resend arrives wherever
    // the connection is now, not necessarily where it was.
    let elsewhere = ForwardedMessages::new(redis_connection().await, prefix.to_string(), 10_000);
    assert!(elsewhere.record_and_check_forwarded("m-1").await.unwrap());

    assert!(
        !forwarded.record_and_check_forwarded("m-2").await.unwrap(),
        "a different message should be its own"
    );
    conn.del(&format!("{prefix}:msg:m-1")).await.unwrap();
    conn.del(&format!("{prefix}:msg:m-2")).await.unwrap();
}

/// A message is forgotten a fixed time after it was first forwarded, and a
/// resend does not keep it alive.
///
/// An entry kept alive by the resends would outlive its usefulness and hold
/// memory for as long as the resends continued.
#[test_log::test(tokio::test)]
async fn test_a_forwarded_message_is_forgotten_on_its_own_schedule() {
    let mut conn = redis_connection().await;
    let prefix = "test-forwarded-expiry";
    conn.del(&format!("{prefix}:msg:m-1")).await.unwrap();

    let forwarded = ForwardedMessages::new(conn.clone(), prefix.to_string(), 400);
    assert!(!forwarded.record_and_check_forwarded("m-1").await.unwrap());

    tokio::time::sleep(Duration::from_millis(250)).await;
    assert!(
        forwarded.record_and_check_forwarded("m-1").await.unwrap(),
        "the message should still be recognised while its record lives"
    );

    // Past the original expiry, but only 250ms past the second call, which
    // would still be inside the window if that call had reset it.
    tokio::time::sleep(Duration::from_millis(250)).await;
    assert!(
        !forwarded.record_and_check_forwarded("m-1").await.unwrap(),
        "the record should expire on the schedule set when the message was first forwarded"
    );

    conn.del(&format!("{prefix}:msg:m-1")).await.unwrap();
}
