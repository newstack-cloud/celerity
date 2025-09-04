use std::fmt::Debug;
use std::sync::Arc;
use std::time::Duration;

use aws_sdk_sqs::{
    error::SdkError,
    operation::change_message_visibility_batch::{
        ChangeMessageVisibilityBatchError, ChangeMessageVisibilityBatchOutput,
    },
    types::ChangeMessageVisibilityBatchRequestEntry,
    Client,
};
use tokio::sync::oneshot::{channel, Sender};
use tokio::time;
use tracing::{debug, error, info, info_span, instrument, Instrument};

use crate::types::MessageHandle;

/// Provides an implementation for a visibility timeout extender
/// which extends the window for which one or more messages
/// are hidden from other consumers of an SQS queue during
/// processing.
#[derive(Debug)]
pub struct VisibilityTimeoutExtender {
    client: Arc<Client>,
    config: VisibilityTimeoutExtenderConfig,
}

#[derive(Debug)]
pub struct VisibilityTimeoutExtenderConfig {
    pub queue_url: String,
    pub visibility_timeout: Option<i32>,
    pub heartbeat_interval: Option<u64>,
}

impl VisibilityTimeoutExtender {
    pub fn new(
        client: Arc<Client>,
        config: VisibilityTimeoutExtenderConfig,
    ) -> VisibilityTimeoutExtender {
        VisibilityTimeoutExtender { client, config }
    }

    #[instrument(name = "heartbeat_initialiser", skip(self, messages))]
    pub fn start_heartbeat(self: Arc<Self>, messages: Vec<MessageHandle>) -> Option<Sender<()>> {
        let heartbeat_runner_task_span = info_span!("heartbeat_runner_task");
        let (send, recv) = channel::<()>();
        tokio::spawn({
            // self will be used concurrently by different tasks so we need
            // to use the Arc synchronisation primitive for sharing self.
            // https://doc.rust-lang.org/std/sync/struct.Arc.html
            let me = Arc::clone(&self);
            async move {
                tokio::select! {
                    _ = me.run_heartbeat_task(me.config.heartbeat_interval.unwrap(), messages.clone()) => {},
                    _ = recv => {}
                }
            }
        }.instrument(heartbeat_runner_task_span));
        Some(send)
    }

    async fn run_heartbeat_task(&self, heartbeat_interval: u64, messages: Vec<MessageHandle>) {
        let mut interval = time::interval(Duration::from_secs(heartbeat_interval));
        loop {
            interval.tick().await;
            debug!(
                "{} seconds have passed, extending visibility timeout",
                heartbeat_interval
            );
            // Extend the visibility timeout on an interval as per the AWS SQS
            // Working with messages guidance:
            // https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/working-with-messages.html
            let result = self.change_visibility_timeout(&messages, None).await;
            if result.is_err() {
                let err = result.err().unwrap();
                let err_message = err.to_string();
                error!("error changing visibility timeout: {}", err_message)
            } else {
                info!("changed visibility timeout!")
            }
        }
    }

    pub async fn change_visibility_timeout(
        &self,
        messages: &[MessageHandle],
        visibility_timeout: Option<i32>,
    ) -> Result<ChangeMessageVisibilityBatchOutput, SdkError<ChangeMessageVisibilityBatchError>>
    {
        let default_visibility_timeout = self.config.visibility_timeout.unwrap_or(30);
        let final_visibility_timeout = visibility_timeout.unwrap_or(default_visibility_timeout);
        self.client
            .change_message_visibility_batch()
            .queue_url(self.config.queue_url.clone())
            .set_entries(Some(
                messages
                    .iter()
                    .map(|message| {
                        ChangeMessageVisibilityBatchRequestEntry::builder()
                            .visibility_timeout(final_visibility_timeout)
                            .set_id(message.message_id.clone())
                            .set_receipt_handle(message.receipt_handle.clone())
                            .build()
                            .unwrap()
                    })
                    .collect(),
            ))
            .send()
            .await
    }
}
