use std::{env, sync::Arc, time::Duration};

use async_trait::async_trait;
use aws_config::BehaviorVersion;
use aws_sdk_sqs::{types::SendMessageBatchRequestEntry, Client, Error};
use celerity_aws_helpers::aws_regions::RegionProvider;
use celerity_helpers::consumers::{Message, MessageConsumer, MessageHandler, MessageHandlerError};
use pretty_assertions::assert_eq;
use tokio::{
    sync::mpsc::{self, Sender},
    task,
};

use celerity_consumer_sqs::{
    errors::WorkerError,
    message_consumer::{SQSConsumerConfig, SQSMessageConsumer},
    types::SQSMessageMetadata,
    visibility_timeout::{self, VisibilityTimeoutExtenderConfig},
};

#[tokio::test]
async fn test_receive_messages_and_fire_message_handler() {
    // Enables logging the AWS SDK crates when enabling logging with
    // the RUST_LOG environment variable.
    // See https://docs.aws.amazon.com/sdk-for-rust/latest/dg/logging.html
    env_logger::init();

    let consumer_config = consumer_config_from_env();
    let (tx, mut rx) = mpsc::channel::<Message<SQSMessageMetadata>>(300);
    task::spawn({
        let tx_ref = Arc::new(tx);
        let consumer_config_for_consumer = consumer_config.clone();
        async move { start_consumer(consumer_config_for_consumer, tx_ref).await }
    });

    task::spawn({
        async move {
            // Set up a completely different SQS client for sending messages completely
            // decouple the sender from the consumer.
            let region_provider = RegionProvider::new(consumer_config.aws_region.clone());
            let config = aws_config::defaults(BehaviorVersion::v2025_01_17())
                .region(region_provider)
                .load()
                .await;
            let client = sqs_client(&config, consumer_config.sqs_endpoint.clone());

            // Batches.
            for n in 1..=20 {
                let _res = client
                    .send_message_batch()
                    .queue_url(consumer_config.sqs_queue_url.clone())
                    .set_entries(create_message_batch(n))
                    .send()
                    .await;
            }

            // Individual messages with delay.
            for n in 1..=100 {
                let _res = client
                    .send_message()
                    .queue_url(consumer_config.sqs_queue_url.clone())
                    .message_body(format!("individual message {n}"))
                    .message_group_id("TestMessageGroup")
                    .send()
                    .await;
                async_std::task::sleep(Duration::from_millis(20)).await;
            }
        }
    });

    let mut collected_messages = Vec::<Message<SQSMessageMetadata>>::new();

    let mut i = 0;
    // 200 messages in batches (20 * 10) + 100 individual messages.
    while i < 300 {
        tokio::select! {
            msg = rx.recv() => {
                if msg.is_some() {
                    let unwrapped = msg.unwrap();
                    collected_messages.push(unwrapped);
                }
            },
        }
        i += 1
    }

    assert_eq!(collected_messages.len(), 300);
    for (i, message) in collected_messages.into_iter().enumerate() {
        // The first 200 messages were sent in batches,
        // the last 100 were sent individually.
        let expected_body = if i < 200 {
            format!(
                "batch {batch} message {n}",
                batch = if i == 0 {
                    1
                } else {
                    (((i / 10) as f32).floor().trunc() as i32) + 1
                },
                n = (i % 10) + 1
            )
        } else {
            format!("individual message {n}", n = (i - 200) + 1)
        };
        assert_eq!(message.body.unwrap(), expected_body);
    }
}

fn create_message_batch(batch: i32) -> Option<Vec<SendMessageBatchRequestEntry>> {
    let mut entries = Vec::<SendMessageBatchRequestEntry>::new();
    for n in 1..=10 {
        entries.push(
            SendMessageBatchRequestEntry::builder()
                // Distinct IDs must be provided for batch entries.
                .id(format!("batch-{batch}-message-{n}"))
                .message_body(format!("batch {batch} message {n}"))
                .message_group_id("TestMessageGroup")
                .build()
                .expect("failed to build batch entry"),
        )
    }
    Some(entries)
}

async fn start_consumer(
    consumer_config: ConsumerConfig,
    tx: Arc<Sender<Message<SQSMessageMetadata>>>,
) -> Result<(), WorkerError> {
    let region_provider = RegionProvider::new(consumer_config.aws_region.clone());
    let config = aws_config::defaults(BehaviorVersion::v2025_01_17())
        .region(region_provider)
        .load()
        .await;
    let client = sqs_client(&config, consumer_config.sqs_endpoint.clone());
    let arc_client_for_vti = Arc::new(client);
    let arc_client_for_consumer = arc_client_for_vti.clone();

    let sqs_consumer_config = SQSConsumerConfig {
        queue_url: consumer_config.sqs_queue_url.clone(),
        batch_size: consumer_config.sqs_queue_message_batch_size,
        message_handler_timeout: consumer_config.sqs_message_handler_timeout_seconds,
        polling_wait_time_ms: consumer_config.sqs_polling_wait_time_ms,
        visibility_timeout: consumer_config.sqs_visibility_timeout_seconds,
        wait_time_seconds: consumer_config.sqs_wait_time_seconds,
        should_delete_messages: true,
        delete_messages_on_handler_failure: Some(true),
        auth_error_timeout: consumer_config.sqs_auth_error_timeout_seconds,
        terminate_visibility_timeout: consumer_config.sqs_terminate_visibility_timeout,
        attribute_names: None,
        message_attribute_names: None,
        num_workers: Some(3),
    };

    let visibility_timeout_extender_config = VisibilityTimeoutExtenderConfig {
        heartbeat_interval: consumer_config.sqs_heartbeat_interval_seconds,
        queue_url: consumer_config.sqs_queue_url.clone(),
        visibility_timeout: consumer_config.sqs_visibility_timeout_seconds,
    };
    let visibility_timeout_extender = visibility_timeout::VisibilityTimeoutExtender::new(
        arc_client_for_vti,
        visibility_timeout_extender_config,
    );
    let mut consumer = SQSMessageConsumer::new(
        arc_client_for_consumer,
        Arc::new(visibility_timeout_extender),
        sqs_consumer_config,
    );
    let uptime_message_handler = MockUptimeMessageHandler::new(tx);
    consumer.register_handler(Arc::new(uptime_message_handler));
    consumer.start().await?;
    Ok(())
}

fn sqs_client(conf: &aws_types::SdkConfig, endpoint_opt: Option<String>) -> aws_sdk_sqs::Client {
    let mut sqs_config_builder = aws_sdk_sqs::config::Builder::from(conf);
    if let Some(endpoint) = endpoint_opt {
        sqs_config_builder = sqs_config_builder.endpoint_url(endpoint)
    }
    Client::from_conf(sqs_config_builder.build())
}

#[derive(Debug)]
pub struct MockUptimeMessageHandler {
    tx: Arc<Sender<Message<SQSMessageMetadata>>>,
}

impl MockUptimeMessageHandler {
    fn new(tx: Arc<Sender<Message<SQSMessageMetadata>>>) -> Self {
        MockUptimeMessageHandler { tx }
    }
}

#[async_trait]
impl MessageHandler<SQSMessageMetadata> for MockUptimeMessageHandler {
    async fn handle(
        &self,
        message: &Message<SQSMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
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
        messages: &[Message<SQSMessageMetadata>],
    ) -> Result<(), MessageHandlerError> {
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

/// Provide configuration from the environment that is required
/// for the context of the SQS consumer only.
#[derive(Debug, Clone)]
pub struct ConsumerConfig {
    pub sqs_queue_url: String,
    pub sqs_endpoint: Option<String>,
    pub sqs_polling_wait_time_ms: u64,
    pub sqs_queue_message_batch_size: Option<i32>,
    pub sqs_visibility_timeout_seconds: Option<i32>,
    pub sqs_wait_time_seconds: Option<i32>,
    pub sqs_auth_error_timeout_seconds: Option<u64>,
    pub sqs_terminate_visibility_timeout: bool,
    pub sqs_heartbeat_interval_seconds: Option<u64>,
    pub sqs_message_handler_timeout_seconds: u64,
    pub aws_region: String,
}

fn consumer_config_from_env() -> ConsumerConfig {
    let sqs_queue_url = env::var("CELERITY_SQS_CONSUMER_SQS_QUEUE_URL").unwrap();

    let sqs_polling_wait_time_ms = env::var("CELERITY_SQS_CONSUMER_SQS_POLLING_WAIT_TIME_MS")
        .unwrap()
        .parse::<u64>()
        .unwrap();

    let sqs_queue_message_batch_size =
        env::var("CELERITY_SQS_CONSUMER_SQS_QUEUE_MESSAGE_BATCH_SIZE")
            .ok()
            .unwrap_or_else(|| String::from(""))
            .parse::<i32>()
            .ok();

    let sqs_visibility_timeout_seconds =
        env::var("CELERITY_SQS_CONSUMER_SQS_VISIBLITY_TIMEOUT_SECONDS")
            .ok()
            .unwrap_or_else(|| String::from(""))
            .parse::<i32>()
            .ok();

    let sqs_wait_time_seconds = env::var("CELERITY_SQS_CONSUMER_SQS_WAIT_TIME_SECONDS")
        .ok()
        .unwrap_or_else(|| String::from(""))
        .parse::<i32>()
        .ok();

    let sqs_auth_error_timeout_seconds =
        env::var("CELERITY_SQS_CONSUMER_SQS_AUTH_ERROR_TIMEOUT_SECONDS")
            .ok()
            .unwrap_or_else(|| String::from(""))
            .parse::<u64>()
            .ok();

    let aws_region = env::var("CELERITY_SQS_CONSUMER_AWS_REGION")
        .ok()
        .unwrap_or_else(|| String::from(""));

    let sqs_terminate_visibility_timeout =
        env::var("CELERITY_SQS_CONSUMER_SQS_TERMINATE_VISIBILITY_TIMEOUT")
            .unwrap()
            .parse::<bool>()
            .unwrap();

    let sqs_heartbeat_interval_seconds =
        env::var("CELERITY_SQS_CONSUMER_SQS_HEARTBEAT_INTERVAL_SECONDS")
            .ok()
            .unwrap_or_else(|| String::from(""))
            .parse::<u64>()
            .ok();

    let sqs_message_handler_timeout_seconds =
        env::var("CELERITY_SQS_CONSUMER_SQS_MESSAGE_HANDLER_TIMEOUT_SECONDS")
            .ok()
            .unwrap_or_else(|| String::from(""))
            .parse::<u64>()
            .unwrap();

    let sqs_endpoint = env::var("CELERITY_SQS_CONSUMER_SQS_ENDPOINT").ok();

    ConsumerConfig {
        sqs_queue_url,
        sqs_endpoint,
        sqs_queue_message_batch_size,
        sqs_polling_wait_time_ms,
        sqs_visibility_timeout_seconds,
        sqs_wait_time_seconds,
        sqs_auth_error_timeout_seconds,
        sqs_terminate_visibility_timeout,
        sqs_heartbeat_interval_seconds,
        sqs_message_handler_timeout_seconds,
        aws_region,
    }
}
