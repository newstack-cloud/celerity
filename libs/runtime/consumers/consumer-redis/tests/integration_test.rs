use std::{collections::HashMap, env, fmt::Display, sync::Arc, time::Duration};

use async_trait::async_trait;
use celerity_consumer_redis::{
    errors::WorkerError,
    lock_durations::{LockDurationExtender, LockDurationExtenderConfig},
    locks::MessageLocks,
    message_consumer::{RedisConsumerConfig, RedisMessageConsumer},
    types::{FromRedisStreamId, RedisMessageMetadata, RedisMessageType, ToStreamRedisArgParts},
};
use celerity_helpers::{
    consumers::{
        Message, MessageConsumer, MessageHandler, MessageHandlerError, PartialBatchFailureInfo,
    },
    redis::{get_redis_connection, ConnectionConfig},
    telemetry::CELERITY_CONTEXT_ID_KEY,
    time::DefaultClock,
};
use tokio::{
    sync::{
        broadcast,
        mpsc::{self, Sender},
        Mutex,
    },
    task,
};

#[test_log::test(tokio::test)]
async fn test_consumer_single_message_processing() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(100);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        // Read 1 message at a time to test the behaviour to process
        // a single message.
        let batch_size = Some(1);
        let stream = "stream-single_message".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 0,
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-1".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(consumer_config, "stream-single_message".to_string(), 100);

    assert_messages_received(&mut rx, 100).await;
}

#[test_log::test(tokio::test)]
async fn test_consumer_batch_message_processing() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(200);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        // Read 20 messages at a time to test the behaviour to process
        // messages in batches.
        let batch_size = Some(20);
        let stream = "stream-batch_message".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 0,
            long_running_process_message_id: Some("0-1".to_string()),
            long_running_process_duration_ms: 300,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-2".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(consumer_config, "stream-batch_message".to_string(), 200);

    assert_messages_received(&mut rx, 200).await;
}

#[test_log::test(tokio::test)]
async fn test_consumer_retry_behaviour_for_single_message() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(100);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        let batch_size = Some(1);
        let stream = "stream-retry_single_message".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 2,
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-3".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(
        consumer_config,
        "stream-retry_single_message".to_string(),
        20,
    );

    assert_messages_received(&mut rx, 20).await;
}

#[test_log::test(tokio::test)]
async fn test_consumer_retry_behaviour_for_batch() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(200);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        let batch_size = Some(20);
        let stream = "stream-retry_batch_message".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 2,
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-4".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(
        consumer_config,
        "stream-retry_batch_message".to_string(),
        100,
    );

    assert_messages_received(&mut rx, 100).await;
}

#[test_log::test(tokio::test)]
async fn test_consumer_retry_behaviour_for_partial_batch_failures() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(200);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        let batch_size = Some(10);
        let stream = "stream-retry_partial_batch_failures".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 1, // Fail once, then succeed on retry
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-5".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: true, // Test partial batch failure handling
                },
            )
            .await
        }
    });

    start_message_sender(
        consumer_config,
        "stream-retry_partial_batch_failures".to_string(),
        20,
    );

    assert_messages_received(&mut rx, 20).await;
}

#[test_log::test(tokio::test)]
async fn test_consumer_forwards_failed_messages_to_dlq() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut _rx) = mpsc::channel::<Message<RedisMessageMetadata>>(10);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        let batch_size = Some(5);
        let stream = "stream-failure_batch_message".to_string();
        let dlq_stream = Some("stream-failure_batch_message_dlq".to_string());
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 5,
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = false;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-6".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(
        consumer_config.clone(),
        "stream-failure_batch_message".to_string(),
        10,
    );

    let (dlq_stream_tx, mut dlq_stream_rx) = mpsc::channel::<Message<RedisMessageMetadata>>(10);
    task::spawn({
        let tx_ref = Arc::new(dlq_stream_tx);
        async move {
            let mut collected: Vec<String> = vec![];
            let mut redis_connection = get_redis_connection(
                &ConnectionConfig {
                    nodes: consumer_config.nodes,
                    password: consumer_config.password,
                    cluster_mode: false,
                },
                None,
            )
            .await
            .expect("must be able to connect to redis for reading from dlq stream");

            while collected.len() < 10 {
                let dlq_stream_read_reply = redis_connection
                    .xread(
                        &["stream-failure_batch_message_dlq"],
                        &["0"],
                        10,
                        consumer_config.block_time_ms.unwrap_or(1000) as usize,
                    )
                    .await
                    .expect("must be able to read messages from stream");

                let stream_messages = if !dlq_stream_read_reply.keys.is_empty() {
                    &dlq_stream_read_reply.keys[0].ids
                } else {
                    &vec![]
                };

                for stream_message in stream_messages {
                    if !collected.contains(&stream_message.id) {
                        tx_ref
                            .send(Message::<RedisMessageMetadata>::from_redis_stream_id(
                                stream_message,
                            ))
                            .await
                            .expect("must be able to send message to channel");
                        collected.push(stream_message.id.clone());
                    }
                }

                // If no new messages, wait a bit before trying again
                if stream_messages.is_empty() {
                    tokio::time::sleep(Duration::from_millis(100)).await;
                }
            }
        }
    });

    // Count messages in DLQ (they will have Redis auto-generated IDs, not original IDs)
    let mut dlq_message_count = 0;
    while let Some(_message) = dlq_stream_rx.recv().await {
        dlq_message_count += 1;
        if dlq_message_count >= 10 {
            break;
        }
    }

    assert_eq!(
        dlq_message_count, 10,
        "Expected 10 messages in DLQ, got {}",
        dlq_message_count
    );
}

#[test_log::test(tokio::test)]
async fn test_stream_trimming_behaviour() {
    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<RedisMessageMetadata>>(500);
    let shutdown_test_context = ShutdownTestContext::new();
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        let batch_size = Some(50);
        let stream = "trim_stream-messages".to_string();
        let dlq_stream = None;
        let mock_message_handler_config = MockMessageHandlerConfig {
            fail_count_per_call: 0,
            long_running_process_message_id: None,
            long_running_process_duration_ms: 0,
        };
        let enable_stream_trimming = true;
        async move {
            start_consumer(
                consumer_config_for_consumer,
                tx_ref,
                shutdown_test_context,
                TestCaseConsumerConfig {
                    service_name: "test-service-7".to_string(),
                    batch_size,
                    stream,
                    dlq_stream,
                    mock_message_handler_config,
                    enable_stream_trimming,
                    partial_batch_failures: false,
                },
            )
            .await
        }
    });

    start_message_sender(
        consumer_config.clone(),
        "trim_stream-messages".to_string(),
        500,
    );

    assert_messages_received(&mut rx, 500).await;

    assert_stream_trimmed("trim_stream-messages".to_string(), consumer_config).await;
}

async fn assert_messages_received(
    rx: &mut mpsc::Receiver<Message<RedisMessageMetadata>>,
    expected_message_count: usize,
) {
    let mut collected_messages = Vec::<Message<RedisMessageMetadata>>::new();

    let mut i = 0;
    while i < expected_message_count {
        tokio::select! {
            msg = rx.recv() => {
                if let Some(unwrapped) = msg {
                    collected_messages.push(unwrapped);
                }
            }
        }
        i += 1;
    }

    assert_eq!(collected_messages.len(), expected_message_count);

    // The order of the messages doesn't matter as concurrent workers may process the next
    // available messages before another has finished processing the previous message.
    sort_by_message_id_sequence(&mut collected_messages);

    for (i, message) in collected_messages.into_iter().enumerate() {
        let n = i + 1;
        assert_eq!(message.message_id, format!("0-{n}"));
        assert!(message.body.is_some());
        assert_eq!(message.body.unwrap(), format!("message {n}"),);
    }
}

async fn assert_stream_trimmed(stream: String, consumer_config: ConsumerConfig) {
    let mut redis_connection = get_redis_connection(
        &ConnectionConfig {
            nodes: consumer_config.nodes,
            password: consumer_config.password,
            cluster_mode: false,
        },
        None,
    )
    .await
    .expect("must be able to connect to redis for reading from stream");

    let start_time = std::time::Instant::now();
    let timeout = Duration::from_secs(30);

    loop {
        let stream_length = redis_connection
            .xlen(&stream)
            .await
            .expect("must be able to get stream length");

        // Allow for approximate trimming with 10% leeway
        let leeway_percentage = 0.1;
        let max_allowed_length = (consumer_config.max_stream_length.unwrap_or(200) as f64
            * (1.0 + leeway_percentage)) as u64;

        if stream_length <= max_allowed_length as usize {
            println!(
                "Stream trimmed successfully: length={}, max_allowed={}",
                stream_length, max_allowed_length
            );
            return;
        }

        if start_time.elapsed() > timeout {
            panic!(
                "Stream was not trimmed within timeout. Current length: {}, max_allowed: {}",
                stream_length, max_allowed_length
            );
        }

        tokio::time::sleep(Duration::from_millis(100)).await;
    }
}

struct TestCaseConsumerConfig {
    service_name: String,
    batch_size: Option<usize>,
    stream: String,
    dlq_stream: Option<String>,
    mock_message_handler_config: MockMessageHandlerConfig,
    enable_stream_trimming: bool,
    partial_batch_failures: bool,
}

async fn start_consumer(
    consumer_config: ConsumerConfig,
    tx: Arc<Sender<Message<RedisMessageMetadata>>>,
    shutdown_test_context: ShutdownTestContext,
    test_case_config: TestCaseConsumerConfig,
) -> Result<(), WorkerError> {
    let redis_consumer_config = RedisConsumerConfig {
        service_name: test_case_config.service_name.clone(),
        name: consumer_config.name.clone(),
        stream: test_case_config.stream,
        dlq_stream: test_case_config.dlq_stream,
        last_message_id_key: None,
        block_time_ms: consumer_config.block_time_ms,
        polling_wait_time_ms: consumer_config.polling_wait_time_ms,
        batch_size: test_case_config.batch_size,
        message_handler_timeout: consumer_config.message_handler_timeout,
        lock_duration_ms: consumer_config.lock_duration_ms,
        max_retries: Some(3),
        retry_max_delay: consumer_config.retry_max_delay,
        retry_base_delay_ms: consumer_config.retry_base_delay_ms,
        backoff_rate: consumer_config.backoff_rate,
        trim_stream_interval: if test_case_config.enable_stream_trimming {
            consumer_config.trim_stream_interval
        } else {
            // an interval of -1 will disable stream trimming
            Some(-1)
        },
        max_stream_length: consumer_config.max_stream_length,
        trim_lock_timeout_ms: consumer_config.trim_lock_timeout_ms,
        num_workers: consumer_config.num_workers,
    };

    let redis_connection = get_redis_connection(
        &ConnectionConfig {
            nodes: consumer_config.nodes,
            password: consumer_config.password,
            cluster_mode: false,
        },
        None,
    )
    .await?;

    let message_locks = MessageLocks::new(
        test_case_config.service_name,
        consumer_config.name,
        redis_connection.clone(),
    );
    let lock_duration_extender = LockDurationExtender::new(
        Arc::new(tokio::sync::Mutex::new(message_locks)),
        LockDurationExtenderConfig {
            lock_duration_ms: consumer_config.lock_duration_ms.unwrap_or(30000),
            heartbeat_interval: 10,
        },
    );

    let mut consumer = RedisMessageConsumer::new(
        Arc::new(lock_duration_extender),
        Arc::new(DefaultClock::new()),
        redis_connection,
        shutdown_test_context.shutdown_broadcast_tx.clone(),
        redis_consumer_config,
    );
    let message_handler: Arc<dyn MessageHandler<RedisMessageMetadata> + Send + Sync> =
        if test_case_config.partial_batch_failures {
            // Use a specialized handler for partial batch failure testing
            Arc::new(PartialBatchFailureMockHandler::new(
                tx,
                test_case_config.mock_message_handler_config,
            ))
        } else {
            Arc::new(MockMessageHandler::new(
                tx,
                test_case_config.mock_message_handler_config,
            ))
        };
    consumer.register_handler(message_handler);
    consumer.start().await?;
    Ok(())
}

fn start_message_sender(consumer_config: ConsumerConfig, stream: String, message_count: u64) {
    task::spawn({
        async move {
            // Set up a separate Redis connection for sending messages to the consumer
            // to decouple the sender from the consumer.
            let mut redis_connection = get_redis_connection(
                &ConnectionConfig {
                    nodes: consumer_config.nodes,
                    password: consumer_config.password,
                    cluster_mode: false,
                },
                None,
            )
            .await
            .expect("must be able to connect to redis for sending messages to stream");

            for n in 1..=message_count {
                let message = create_test_message(n);
                let parts = message.to_stream_redis_arg_parts(false);
                redis_connection
                    .xadd(&stream, parts.id, &parts.fields)
                    .await
                    .expect("must be able to send message to stream");
            }
        }
    });
}

/// Provide configuration from the envrionment that is required
/// for the context of the Redis consumer only.
#[derive(Debug, Clone)]
pub struct ConsumerConfig {
    name: String,
    nodes: Vec<String>,
    password: Option<String>,
    block_time_ms: Option<u64>,
    polling_wait_time_ms: Option<u64>,
    message_handler_timeout: u64,
    lock_duration_ms: Option<u64>,
    retry_base_delay_ms: Option<i64>,
    retry_max_delay: Option<i64>,
    backoff_rate: Option<f64>,
    trim_stream_interval: Option<i64>,
    max_stream_length: Option<u64>,
    trim_lock_timeout_ms: Option<u64>,
    num_workers: Option<usize>,
}

fn consumer_config_from_env() -> ConsumerConfig {
    let name =
        env::var("CELERITY_REDIS_CONSUMER_NAME").expect("CELERITY_REDIS_CONSUMER_NAME must be set");

    let nodes = env::var("CELERITY_REDIS_CONSUMER_NODES")
        .expect("CELERITY_REDIS_CONSUMER_NODES must be set")
        .split(',')
        .map(|s| s.to_string())
        .collect();

    let password = env::var("CELERITY_REDIS_CONSUMER_PASSWORD").ok();

    let block_time_ms = env::var("CELERITY_REDIS_CONSUMER_BLOCK_TIME_MS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .ok();

    let polling_wait_time_ms = env::var("CELERITY_REDIS_CONSUMER_POLLING_WAIT_TIME_MS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .ok();

    let message_handler_timeout = env::var("CELERITY_REDIS_CONSUMER_MESSAGE_HANDLER_TIMEOUT")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .expect("CELERITY_REDIS_CONSUMER_MESSAGE_HANDLER_TIMEOUT must be set");

    let lock_duration_ms = env::var("CELERITY_REDIS_CONSUMER_LOCK_DURATION_MS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .ok();

    let retry_base_delay_ms = env::var("CELERITY_REDIS_CONSUMER_RETRY_BASE_DELAY_MS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<i64>()
        .ok();

    let retry_max_delay = env::var("CELERITY_REDIS_CONSUMER_RETRY_MAX_DELAY")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<i64>()
        .ok();

    let backoff_rate = env::var("CELERITY_REDIS_CONSUMER_BACKOFF_RATE")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<f64>()
        .ok();

    let trim_stream_interval = env::var("CELERITY_REDIS_CONSUMER_TRIM_STREAM_INTERVAL")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<i64>()
        .ok();

    let max_stream_length = env::var("CELERITY_REDIS_CONSUMER_MAX_STREAM_LENGTH")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .ok();

    let trim_lock_timeout_ms = env::var("CELERITY_REDIS_CONSUMER_TRIM_LOCK_TIMEOUT_MS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<u64>()
        .ok();

    let num_workers = env::var("CELERITY_REDIS_CONSUMER_NUM_WORKERS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<usize>()
        .ok();

    ConsumerConfig {
        name,
        nodes,
        password,
        block_time_ms,
        polling_wait_time_ms,
        message_handler_timeout,
        lock_duration_ms,
        retry_base_delay_ms,
        retry_max_delay,
        backoff_rate,
        trim_stream_interval,
        max_stream_length,
        trim_lock_timeout_ms,
        num_workers,
    }
}

/// Context for shutdown that will gracefully shutdown the consumer
/// when it is dropped.
struct ShutdownTestContext {
    shutdown_broadcast_tx: broadcast::Sender<()>,
}

impl ShutdownTestContext {
    fn new() -> Self {
        Self {
            shutdown_broadcast_tx: broadcast::Sender::new(10),
        }
    }
}

impl Drop for ShutdownTestContext {
    fn drop(&mut self) {
        let _res = self.shutdown_broadcast_tx.send(());
    }
}

fn create_test_message(n: u64) -> Message<RedisMessageMetadata> {
    Message {
        message_id: format!("0-{n}"),
        body: Some(format!("message {n}")),
        md5_of_body: None,
        metadata: RedisMessageMetadata {
            timestamp: 0,
            failed_at: None,
            retry_count: None,
            failure_reason: None,
            message_type: RedisMessageType::Text,
        },
        trace_context: Some(HashMap::from([(
            CELERITY_CONTEXT_ID_KEY.to_string(),
            "1234567890".to_string(),
        )])),
    }
}

#[derive(Debug)]
struct TestMessageHandlerError {
    message: String,
}

impl TestMessageHandlerError {
    fn new(message: String) -> Self {
        Self { message }
    }
}

impl Display for TestMessageHandlerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Test message handler error: {}", self.message)
    }
}

impl std::error::Error for TestMessageHandlerError {}

#[derive(Debug)]
struct MockMessageHandlerConfig {
    // The number of times to fail the message handler
    // for each message or batch received.
    fail_count_per_call: u64,
    // The message ID that will be used to determine if the message handler
    // should sleep for the long running process duration.
    long_running_process_message_id: Option<String>,
    // The duration in milliseconds to sleep for when processing a message
    // that matches the long running message ID.
    long_running_process_duration_ms: u64,
}

#[derive(Debug)]
struct MockMessageHandler {
    tx: Arc<Sender<Message<RedisMessageMetadata>>>,
    config: MockMessageHandlerConfig,
    fail_count_map: Arc<Mutex<HashMap<String, u64>>>,
}

impl MockMessageHandler {
    fn new(
        tx: Arc<Sender<Message<RedisMessageMetadata>>>,
        config: MockMessageHandlerConfig,
    ) -> Self {
        MockMessageHandler {
            tx,
            config,
            fail_count_map: Arc::new(Mutex::new(HashMap::new())),
        }
    }
}

#[async_trait]
impl MessageHandler<RedisMessageMetadata> for MockMessageHandler {
    async fn handle(
        &self,
        message: &Message<RedisMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
        let mut fail_count_map = self.fail_count_map.lock().await;
        let fail_count = *fail_count_map.get(&message.message_id).unwrap_or(&0);
        if fail_count < self.config.fail_count_per_call {
            fail_count_map.insert(message.message_id.clone(), fail_count + 1);
            return Err(MessageHandlerError::HandlerFailure(Box::new(
                TestMessageHandlerError::new("an unexpected error occurred".to_string()),
            )));
        }

        if let Some(long_running_process_message_id) = &self.config.long_running_process_message_id
        {
            if message.message_id == *long_running_process_message_id {
                tokio::time::sleep(Duration::from_millis(
                    self.config.long_running_process_duration_ms,
                ))
                .await;
            }
        }

        let message_owned = message.clone();
        tokio::spawn({
            let tx = self.tx.clone();
            async move {
                let _res = tx.send(message_owned).await;
            }
        });
        Ok(())
    }

    async fn handle_batch(
        &self,
        messages: &[Message<RedisMessageMetadata>],
    ) -> Result<(), MessageHandlerError> {
        let key = messages
            .iter()
            .map(|m| m.message_id.clone())
            .collect::<Vec<String>>()
            .join(",");

        let mut fail_count_map = self.fail_count_map.lock().await;
        let fail_count = *fail_count_map.get(&key).unwrap_or(&0);
        if fail_count < self.config.fail_count_per_call {
            fail_count_map.insert(key, fail_count + 1);

            return Err(MessageHandlerError::HandlerFailure(Box::new(
                TestMessageHandlerError::new("an unexpected error occurred".to_string()),
            )));
        }

        if let Some(long_running_process_message_id) = &self.config.long_running_process_message_id
        {
            if messages
                .iter()
                .any(|m| m.message_id == *long_running_process_message_id)
            {
                tokio::time::sleep(Duration::from_millis(
                    self.config.long_running_process_duration_ms,
                ))
                .await;
            }
        }

        let messages_owned = messages.to_vec();
        tokio::spawn({
            let tx = self.tx.clone();
            async move {
                for message in messages_owned {
                    let _res = tx.send(message).await;
                }
            }
        });
        Ok(())
    }
}

#[derive(Debug)]
struct PartialBatchFailureMockHandler {
    tx: Arc<Sender<Message<RedisMessageMetadata>>>,
    config: MockMessageHandlerConfig,
    fail_count_map: Arc<Mutex<HashMap<String, u64>>>,
}

impl PartialBatchFailureMockHandler {
    fn new(
        tx: Arc<Sender<Message<RedisMessageMetadata>>>,
        config: MockMessageHandlerConfig,
    ) -> Self {
        Self {
            tx,
            config,
            fail_count_map: Arc::new(Mutex::new(HashMap::new())),
        }
    }
}

#[async_trait]
impl MessageHandler<RedisMessageMetadata> for PartialBatchFailureMockHandler {
    async fn handle(
        &self,
        message: &Message<RedisMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
        if let Some(long_running_process_message_id) = &self.config.long_running_process_message_id
        {
            if message.message_id == *long_running_process_message_id {
                tokio::time::sleep(Duration::from_millis(
                    self.config.long_running_process_duration_ms,
                ))
                .await;
            }
        }

        let message_owned = message.clone();
        tokio::spawn({
            let tx = self.tx.clone();
            async move {
                let _res = tx.send(message_owned).await;
            }
        });
        Ok(())
    }

    async fn handle_batch(
        &self,
        messages: &[Message<RedisMessageMetadata>],
    ) -> Result<(), MessageHandlerError> {
        let mut fail_count_map = self.fail_count_map.lock().await;

        // Check if any message in the batch should fail based on individual message fail counts
        let mut should_fail = false;
        for message in messages {
            let message_key = message.message_id.clone();
            let fail_count = *fail_count_map.get(&message_key).unwrap_or(&0);
            if fail_count < self.config.fail_count_per_call {
                should_fail = true;
                break;
            }
        }

        if should_fail {
            // Increment fail count for all messages in the batch
            for message in messages {
                let message_key = message.message_id.clone();
                let fail_count = *fail_count_map.get(&message_key).unwrap_or(&0);
                fail_count_map.insert(message_key, fail_count + 1);
            }

            // Forward successful messages to the channel before returning partial batch failure.
            let successful_messages = if messages.len() > 1 {
                &messages[0..1] // First message is successful
            } else {
                &[]
            };

            for message in successful_messages {
                let message_owned = message.clone();
                let tx = self.tx.clone();
                tokio::spawn(async move {
                    let _res = tx.send(message_owned).await;
                });
            }

            return Err(MessageHandlerError::PartialBatchFailure(
                create_test_partial_batch_failures(if messages.len() > 1 {
                    &messages[1..]
                } else {
                    messages
                }),
            ));
        }

        if let Some(long_running_process_message_id) = &self.config.long_running_process_message_id
        {
            if messages
                .iter()
                .any(|m| m.message_id == *long_running_process_message_id)
            {
                tokio::time::sleep(Duration::from_millis(
                    self.config.long_running_process_duration_ms,
                ))
                .await;
            }
        }

        let messages_owned = messages.to_vec();
        tokio::spawn({
            let tx = self.tx.clone();
            async move {
                for message in messages_owned {
                    let _res = tx.send(message).await;
                }
            }
        });
        Ok(())
    }
}

fn sort_by_message_id_sequence(messages: &mut [Message<RedisMessageMetadata>]) {
    messages.sort_by_key(|m| {
        m.message_id
            .clone()
            .split('-')
            .nth(1)
            .unwrap()
            .parse::<u64>()
            .unwrap()
    });
}

fn create_test_partial_batch_failures(
    messages: &[Message<RedisMessageMetadata>],
) -> Vec<PartialBatchFailureInfo> {
    messages
        .iter()
        .map(|m| {
            PartialBatchFailureInfo::new(
                m.message_id.clone(),
                "an unexpected error occurred".to_string(),
                0,
            )
        })
        .collect()
}
