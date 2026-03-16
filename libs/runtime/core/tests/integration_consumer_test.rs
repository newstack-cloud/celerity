#![cfg(feature = "celerity_local_consumers")]
//! Integration tests for the consumer pipeline.
//!
//! These tests validate the full message flow:
//! RedisMessageConsumer → ConsumerHandlerBridge → ConsumerEventHandler
//!
//! Requirements:
//! - A running Valkey/Redis instance (default: redis://127.0.0.1:6379)
//! - Feature flag: `celerity_local_consumers`
//! ```

use std::{collections::HashMap, sync::Arc, time::Duration};

use async_trait::async_trait;
use celerity_consumer_redis::{
    lock_durations::{LockDurationExtender, LockDurationExtenderConfig},
    locks::MessageLocks,
    message_consumer::{RedisConsumerConfig, RedisMessageConsumer},
    types::{RedisMessageMetadata, RedisMessageType, ToStreamRedisArgParts},
};
use celerity_helpers::{
    consumers::{Message, MessageConsumer},
    redis::{get_redis_connection, ConnectionConfig},
    telemetry::CELERITY_CONTEXT_ID_KEY,
    time::DefaultClock,
};
use celerity_runtime_core::{
    consumer_handler::{ConsumerEventHandler, ConsumerEventHandlerError, ConsumerHandlerBridge},
    types::{ConsumerEventData, EventResult, EventResultData, ScheduleEventData},
};
use serde_json::json;
use tokio::sync::{broadcast, mpsc, Mutex};

// Helper: construct EventResultData with private fields via serde.
fn success_result_data() -> EventResultData {
    serde_json::from_value(json!({"success": true})).unwrap()
}

// ---------------------------------------------------------------------------
// Mock consumer event handler
// ---------------------------------------------------------------------------

struct MockConsumerEventHandler {
    tx: mpsc::Sender<ConsumerEventData>,
}

#[async_trait]
impl ConsumerEventHandler for MockConsumerEventHandler {
    async fn handle_consumer_event(
        &self,
        _handler_tag: &str,
        event_data: ConsumerEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        self.tx
            .send(event_data)
            .await
            .map_err(|_| ConsumerEventHandlerError::ChannelClosed)?;
        Ok(EventResult {
            event_id: String::new(),
            data: success_result_data(),
            context: None,
        })
    }

    async fn handle_schedule_event(
        &self,
        _handler_tag: &str,
        _event_data: ScheduleEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        Ok(EventResult {
            event_id: String::new(),
            data: success_result_data(),
            context: None,
        })
    }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

fn redis_url() -> String {
    std::env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".to_string())
}

fn conn_config() -> ConnectionConfig {
    ConnectionConfig {
        nodes: vec![redis_url()],
        password: None,
        cluster_mode: false,
    }
}

fn create_test_message(n: u64) -> Message<RedisMessageMetadata> {
    Message {
        message_id: format!("0-{n}"),
        body: Some(format!(r#"{{"order_id": {n}}}"#)),
        md5_of_body: None,
        metadata: RedisMessageMetadata {
            timestamp: 0,
            failed_at: None,
            retry_count: None,
            failure_reason: None,
            message_type: RedisMessageType::Text,
            source_message_id: None,
            subject: None,
            attributes: None,
            event_name: None,
        },
        trace_context: Some(HashMap::from([(
            CELERITY_CONTEXT_ID_KEY.to_string(),
            format!("trace-{n}"),
        )])),
    }
}

async fn create_consumer(
    stream: &str,
    service_name: &str,
    consumer_name: &str,
    handler: Arc<dyn ConsumerEventHandler>,
) -> (RedisMessageConsumer, broadcast::Sender<()>) {
    let redis_conn = get_redis_connection(&conn_config(), None)
        .await
        .expect("must connect to Redis/Valkey");

    let message_locks = Arc::new(Mutex::new(MessageLocks::new(
        service_name.to_string(),
        consumer_name.to_string(),
        redis_conn.clone(),
    )));
    let lock_extender = Arc::new(LockDurationExtender::new(
        message_locks,
        LockDurationExtenderConfig {
            lock_duration_ms: 30_000,
            heartbeat_interval: 10,
        },
    ));
    let clock: Arc<dyn celerity_helpers::time::Clock + Send + Sync> = Arc::new(DefaultClock::new());

    let (shutdown_tx, _) = broadcast::channel(1);
    let redis_config = RedisConsumerConfig {
        service_name: service_name.to_string(),
        name: consumer_name.to_string(),
        stream: stream.to_string(),
        dlq_stream: None,
        last_message_id_key: None,
        block_time_ms: Some(500),
        polling_wait_time_ms: Some(100),
        batch_size: Some(5),
        message_handler_timeout: 30,
        lock_duration_ms: None,
        max_retries: None,
        retry_base_delay_ms: None,
        retry_max_delay: None,
        backoff_rate: None,
        trim_stream_interval: None,
        max_stream_length: None,
        trim_lock_timeout_ms: None,
        num_workers: None,
    };

    let mut consumer = RedisMessageConsumer::new(
        lock_extender,
        clock,
        redis_conn,
        conn_config(),
        shutdown_tx.clone(),
        redis_config,
    );

    let bridge = ConsumerHandlerBridge::<RedisMessageMetadata>::new(
        handler,
        format!("source::{}::TestHandler", stream),
        stream.to_string(),
        "aws".to_string(),
    );
    consumer.register_handler(Arc::new(bridge));

    (consumer, shutdown_tx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

/// Validates the full queue consumer pipeline:
/// 1. Messages published to a Valkey stream
/// 2. RedisMessageConsumer picks them up
/// 3. ConsumerHandlerBridge transforms them
/// 4. MockConsumerEventHandler receives ConsumerEventData
#[test_log::test(tokio::test)]
async fn test_consumer_e2e_queue_message_processing() {
    let stream = "celerity:queue:e2e-core-queue-test";

    let (event_tx, mut event_rx) = mpsc::channel::<ConsumerEventData>(100);
    let mock_handler: Arc<dyn ConsumerEventHandler> =
        Arc::new(MockConsumerEventHandler { tx: event_tx });

    let (consumer, shutdown_tx) =
        create_consumer(stream, "e2e-core-svc", "e2e-core-consumer", mock_handler).await;

    let consumer_handle = tokio::spawn(async move {
        consumer.start().await.ok();
    });

    // Publish messages to the stream from a separate connection.
    let message_count: u64 = 5;
    {
        let mut sender_conn = get_redis_connection(&conn_config(), None)
            .await
            .expect("must connect to Redis/Valkey for sending");

        for n in 1..=message_count {
            let msg = create_test_message(n);
            let parts = msg.to_stream_redis_arg_parts(false);
            sender_conn
                .xadd(stream, parts.id, &parts.fields)
                .await
                .expect("must be able to publish message to stream");
        }
    }

    // Collect events from the mock handler.
    let mut total_messages = 0u64;
    let deadline = tokio::time::Instant::now() + Duration::from_secs(15);
    let mut received_events = Vec::new();
    while total_messages < message_count && tokio::time::Instant::now() < deadline {
        match tokio::time::timeout(Duration::from_secs(2), event_rx.recv()).await {
            Ok(Some(event_data)) => {
                total_messages += event_data.messages.len() as u64;
                received_events.push(event_data);
            }
            Ok(None) => break,
            Err(_) => continue,
        }
    }

    let _ = shutdown_tx.send(());
    let _ = tokio::time::timeout(Duration::from_secs(5), consumer_handle).await;

    // Verify all messages were received.
    assert_eq!(
        total_messages, message_count,
        "expected {message_count} messages, received {total_messages}"
    );

    // Verify message content.
    let all_messages: Vec<_> = received_events
        .iter()
        .flat_map(|e| e.messages.iter())
        .collect();

    for msg in &all_messages {
        assert_eq!(msg.source, stream);
        assert!(!msg.body.is_empty());
    }
}

/// Validates consumer shutdown: start → shutdown signal → clean exit.
#[test_log::test(tokio::test)]
async fn test_consumer_shutdown_signal() {
    let stream = "celerity:queue:e2e-core-shutdown-test";

    let (event_tx, _event_rx) = mpsc::channel::<ConsumerEventData>(100);
    let mock_handler: Arc<dyn ConsumerEventHandler> =
        Arc::new(MockConsumerEventHandler { tx: event_tx });

    let (consumer, shutdown_tx) = create_consumer(
        stream,
        "e2e-shutdown-svc",
        "e2e-shutdown-consumer",
        mock_handler,
    )
    .await;

    let consumer_handle = tokio::spawn(async move {
        consumer.start().await.ok();
    });

    // Give the consumer time to start polling.
    tokio::time::sleep(Duration::from_millis(500)).await;

    let _ = shutdown_tx.send(());

    // Consumer should exit cleanly within a reasonable timeout.
    let result = tokio::time::timeout(Duration::from_secs(10), consumer_handle).await;
    assert!(
        result.is_ok(),
        "consumer did not shut down within 10 seconds"
    );
}
