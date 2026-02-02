use crate::{
    errors::{is_connection_error, WorkerError},
    telemetry::SQSMessageMetadataExtractor,
    types::{FromSQSMessage, MessageHandle, SQSMessageMetadata, ToSQSMessageHandle},
    visibility_timeout::VisibilityTimeoutExtender,
};
use async_trait::async_trait;
use aws_sdk_sqs::{
    error::SdkError,
    operation::receive_message::ReceiveMessageError,
    types::{DeleteMessageBatchRequestEntry, MessageSystemAttributeName},
    Client, Error,
};
use celerity_helpers::{
    aws_telemetry::XrayTraceId,
    consumers::{
        extract_context_ids, Message, MessageConsumer, MessageHandler, MessageHandlerError,
        PinnedMessageHandlerFuture,
    },
    telemetry::{CELERITY_CONTEXT_IDS_KEY, CELERITY_CONTEXT_ID_KEY},
    time::calcuate_polling_wait_time,
};
use futures::future::join_all;
use opentelemetry::{
    global,
    trace::{SpanKind, TraceContextExt},
};
use std::{fmt::Debug, sync::Arc, time::Duration};
use tokio::time;
use tokio::time::{timeout, Instant};
use tracing::{debug, error, field, info, info_span, instrument, Instrument};
use tracing_opentelemetry::OpenTelemetrySpanExt;

/// Configuration for an SQS message consumer.
#[derive(Debug)]
pub struct SQSConsumerConfig {
    /// The URL of the SQS queue to consume messages from.
    pub queue_url: String,
    /// The minimum time to wait between each call to receive messages
    /// from the queue.
    pub polling_wait_time_ms: u64,
    /// The maximum number of messages to receive in a single call to SQS.
    /// SQS only allows a maximum of 10 messages per call.
    ///
    /// Defaults to 10 messages.
    pub batch_size: Option<i32>,
    /// The maximum time to wait for a message handler to complete.
    pub message_handler_timeout: u64,
    /// The visibility timeout to set for messages.
    ///
    /// Defaults to 30 seconds.
    pub visibility_timeout: Option<i32>,
    /// The time to wait for a message to be visible after a receive request,
    /// before the message is returned and the visibility timeout is extended.
    ///
    /// Defaults to 20 seconds.
    pub wait_time_seconds: Option<i32>,
    /// The timeout to use for authentication errors.
    pub auth_error_timeout: Option<u64>,
    /// Whether to terminate the visibility timeout when a message is deleted.
    pub terminate_visibility_timeout: bool,
    /// Whether to delete messages after they have been processed.
    pub should_delete_messages: bool,
    /// Whether to delete messages if the message handler fails.
    ///
    /// Defaults to true.
    pub delete_messages_on_handler_failure: Option<bool>,
    /// The attribute names to retrieve from the message.
    pub attribute_names: Option<Vec<MessageSystemAttributeName>>,
    /// The message attribute names to retrieve from the message.
    pub message_attribute_names: Option<Vec<String>>,
    /// The number of worker tasks to use
    /// to receive and process messages.
    /// Each worker independently polls SQS and processes messages.
    ///
    /// Defaults to 10 workers.
    pub num_workers: Option<usize>,
}

#[derive(Debug)]
struct SQSConsumerFinalisedConfig {
    queue_url: String,
    polling_wait_time_ms: u64,
    batch_size: i32,
    message_handler_timeout: u64,
    visibility_timeout: i32,
    wait_time_seconds: i32,
    auth_error_timeout: u64,
    terminate_visibility_timeout: bool,
    should_delete_messages: bool,
    delete_messages_on_handler_failure: bool,
    attribute_names: Option<Vec<MessageSystemAttributeName>>,
    message_attribute_names: Option<Vec<String>>,
    num_workers: usize,
}

/// Provides an implementation of an AWS SQS
/// message consumer that polls SQS queues
/// and fires registered event handlers.
pub struct SQSMessageConsumer {
    handler: Option<Arc<dyn MessageHandler<SQSMessageMetadata> + Send + Sync>>,
    client: Arc<Client>,
    visibility_timeout_extender: Arc<VisibilityTimeoutExtender>,
    config: Arc<SQSConsumerFinalisedConfig>,
}

impl Debug for SQSMessageConsumer {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "SQSMessageConsumer")
    }
}

impl Clone for SQSMessageConsumer {
    fn clone(&self) -> Self {
        SQSMessageConsumer {
            handler: self.handler.clone(),
            client: self.client.clone(),
            visibility_timeout_extender: self.visibility_timeout_extender.clone(),
            config: self.config.clone(),
        }
    }
}

#[async_trait]
impl MessageConsumer<SQSMessageMetadata> for SQSMessageConsumer {
    type Error = WorkerError;

    fn register_handler(
        &mut self,
        handler: Arc<dyn MessageHandler<SQSMessageMetadata> + Send + Sync>,
    ) {
        self.handler = Some(handler);
    }

    #[instrument(name = "sqs_message_consumer", skip(self))]
    async fn start(&self) -> Result<(), Self::Error> {
        let consumer_arc = Arc::new(self.clone());
        let mut worker_handles = Vec::new();
        for worker_id in 0..self.config.num_workers {
            let consumer = consumer_arc.clone();
            let worker_handle = tokio::spawn(async move { consumer.start_worker(worker_id).await });
            worker_handles.push(worker_handle);
        }

        let results = join_all(worker_handles).await;

        let mut errors = Vec::new();
        for (worker_id, result) in results.into_iter().enumerate() {
            match result {
                Ok(_) => info!("Worker {worker_id} finished successfully"),
                Err(err) => {
                    error!("Worker {worker_id} panicked: {err}");
                    errors.push(err.to_string());
                }
            }
        }

        if !errors.is_empty() {
            return Err(WorkerError::new(format!("Workers failed: {errors:?}")));
        }

        Ok(())
    }
}

impl SQSMessageConsumer {
    pub fn new(
        client: Arc<Client>,
        visibility_timeout_extender: Arc<VisibilityTimeoutExtender>,
        config: SQSConsumerConfig,
    ) -> SQSMessageConsumer {
        let final_config = SQSConsumerFinalisedConfig {
            queue_url: config.queue_url,
            polling_wait_time_ms: config.polling_wait_time_ms,
            batch_size: config.batch_size.unwrap_or(1),
            message_handler_timeout: config.message_handler_timeout,
            visibility_timeout: config.visibility_timeout.unwrap_or(30),
            wait_time_seconds: config.wait_time_seconds.unwrap_or(20),
            auth_error_timeout: config.auth_error_timeout.unwrap_or(10),
            terminate_visibility_timeout: config.terminate_visibility_timeout,
            should_delete_messages: config.should_delete_messages,
            delete_messages_on_handler_failure: config
                .delete_messages_on_handler_failure
                .unwrap_or(true),
            attribute_names: config.attribute_names,
            message_attribute_names: config.message_attribute_names,
            num_workers: config.num_workers.unwrap_or(10),
        };
        SQSMessageConsumer {
            handler: None,
            client,
            visibility_timeout_extender,
            config: Arc::new(final_config),
        }
    }

    async fn start_worker(self: Arc<Self>, worker_id: usize) {
        async move {
            loop {
                let start_time = Instant::now();
                let mut current_polling_wait_time_ms = self.config.polling_wait_time_ms;

                let result = self.receive_messages().await;

                current_polling_wait_time_ms =
                    calcuate_polling_wait_time(start_time, current_polling_wait_time_ms);

                if let Err(SdkError::ServiceError(service_err)) = result {
                    let source = service_err.err();
                    let raw = service_err.raw();
                    if is_connection_error(source, raw.status()) {
                        debug!("there was an authentication error. pausing before retrying.");
                        current_polling_wait_time_ms = self.config.auth_error_timeout;
                    } else {
                        error!(
                            "failed to receive and handle messages from queue: {}",
                            service_err.err()
                        )
                    }
                }

                if current_polling_wait_time_ms > 0 {
                    time::sleep(Duration::from_millis(current_polling_wait_time_ms)).await;
                }
            }
        }
        .instrument(info_span!(
            "sqs_message_consumer_worker",
            worker_id = worker_id
        ))
        .await
    }

    async fn handle_single_message(
        &self,
        message: Message<SQSMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle(&message)),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        let parent_context = global::get_text_map_propagator(|propagator| {
            let extractor = SQSMessageMetadataExtractor::new(&message.metadata);
            propagator.extract(&extractor)
        });

        // Grab the X-Ray trace ID from the parent opentelemetry context to tie span events
        // that are exported as logs to the trace context via fields.
        // This allows for searching and filtering events and spans by X-Ray trace ID without
        // having to convert an OTel trace ID to an X-Ray trace ID in external
        // monitoring systems.
        let xray_trace_id = XrayTraceId::from(parent_context.span().span_context().trace_id());

        // This span should be its own consumer node to decouple from the parent segment.
        // see https://docs.rs/tracing-opentelemetry/latest/tracing_opentelemetry/#special-fields
        // A Segment is only created for span.kind Server, even though this is a consumer,
        // we want it to have its own top-level segment to differentiate from SQS and the message sender.
        let span = info_span!(
            "handle_single_message",
            "otel.kind" = ?SpanKind::Server,
            "xray_trace_id" = xray_trace_id.to_string(),
            // This is hardcoded instead of being a const as there's no obvious way
            // to use dynamic field names when constructing a span with the macro.
            "celerity.context-id" = field::Empty,
        );
        span.set_parent(parent_context);

        if let Some(trace_context) = &message.trace_context {
            if let Some(celerity_context_id) = trace_context.get(CELERITY_CONTEXT_ID_KEY) {
                span.record(CELERITY_CONTEXT_ID_KEY, celerity_context_id.to_string());
            }
        }

        self.handle_messages_future(future_result)
            .instrument(span)
            .await
    }

    async fn handle_messages(
        &self,
        messages: Vec<Message<SQSMessageMetadata>>,
    ) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle_batch(&messages)),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        let xray_trace_id = XrayTraceId::from(
            opentelemetry::Context::current()
                .span()
                .span_context()
                .trace_id(),
        );

        let span = info_span!(
            "handle_messages",
            "otel.kind" = ?SpanKind::Server,
            "batch_size"= messages.len(),
            "xray_trace_id" = xray_trace_id.to_string(),
            // This is hardcoded instead of being a const as there's no obvious way
            // to use dynamic field names when constructing a span with the macro.
            "celerity.context-ids" = field::Empty,
        );

        let context_ids = extract_context_ids(&messages);
        if !context_ids.is_empty() {
            span.record(CELERITY_CONTEXT_IDS_KEY, context_ids.join(","));
        }

        self.handle_messages_future(future_result)
            .instrument(span)
            .await
    }

    async fn handle_messages_future(
        &self,
        future_result: Result<PinnedMessageHandlerFuture<'_>, MessageHandlerError>,
    ) -> Result<(), MessageHandlerError> {
        if future_result.is_err() {
            return Err(future_result.err().unwrap());
        }

        debug!(
            timeout = self.config.message_handler_timeout.clone(),
            "running message handler with timeout",
        );
        match timeout(
            Duration::from_secs(self.config.message_handler_timeout),
            future_result.unwrap(),
        )
        .await
        {
            Err(timeout_err) => Err(MessageHandlerError::Timeout(timeout_err)),
            Ok(result) => result,
        }
    }

    fn derive_handler_future(
        &self,
        messages: Vec<Message<SQSMessageMetadata>>,
    ) -> PinnedMessageHandlerFuture<'_> {
        if messages.len() == 1 {
            Box::pin(self.handle_single_message(messages[0].clone()))
        } else {
            Box::pin(self.handle_messages(messages))
        }
    }

    async fn terminate_visibility_timeout(&self, messages: &[MessageHandle]) -> Result<(), Error> {
        if !self.config.terminate_visibility_timeout {
            debug!("sqs consumer not configured to terminate visibility timeout, moving on");
            return Ok(());
        }

        let result = self
            .visibility_timeout_extender
            .change_visibility_timeout(messages, Some(0))
            .await;

        if result.is_err() {
            let err = result.err().unwrap();
            error!("failed to terminate visibility timeout: {}", err);
        }
        Ok(())
    }

    async fn delete_messages(
        &self,
        messages: &[MessageHandle],
        handler_failed: bool,
    ) -> Result<(), Error> {
        if !self.config.should_delete_messages {
            debug!("skipping message deletion as should_delete_messages is set to false");
            return Ok(());
        }

        if handler_failed && !self.config.delete_messages_on_handler_failure {
            debug!(concat!(
                "skipping message deletion as handler failed and ",
                "delete_messages_on_handler_failure is set to false"
            ));
            return Ok(());
        }

        if messages.is_empty() {
            debug!("skipping message deletion as there are no messages to delete");
            return Ok(());
        }

        debug!("deleting handled message batch");
        let result = self
            .client
            .delete_message_batch()
            .queue_url(self.config.queue_url.clone())
            .set_entries(Some(
                messages
                    .iter()
                    .map(|message| {
                        DeleteMessageBatchRequestEntry::builder()
                            .set_id(message.message_id.clone())
                            .set_receipt_handle(message.receipt_handle.clone())
                            .build()
                            .unwrap()
                    })
                    .collect(),
            ))
            .send()
            .await;

        if result.is_err() {
            let err = result.err().unwrap();
            error!("failed to delete messages from queue: {}", err);
        }
        Ok(())
    }

    #[instrument(skip(self))]
    async fn receive_messages(&self) -> Result<(), SdkError<ReceiveMessageError>> {
        let rcv_message_output = self
            .client
            .receive_message()
            .queue_url(self.config.queue_url.clone())
            .set_wait_time_seconds(Some(self.config.wait_time_seconds))
            .set_max_number_of_messages(Some(self.config.batch_size))
            .set_visibility_timeout(Some(self.config.visibility_timeout))
            .set_message_system_attribute_names(self.config.attribute_names.clone())
            .set_message_attribute_names(self.config.message_attribute_names.clone())
            .send()
            .await?;

        let messages = rcv_message_output.messages.unwrap_or_default();
        let handler_messages: Vec<Message<SQSMessageMetadata>> = messages
            .iter()
            .map(|msg| Message::<SQSMessageMetadata>::from_sqs_message(msg.clone()))
            .collect();
        let message_handles: Vec<MessageHandle> = handler_messages
            .iter()
            .map(|m| m.to_sqs_message_handle())
            .collect();

        let handle_msg_future = self.derive_handler_future(handler_messages.clone());

        // May or may not start a heartbeat for the visibility timeout extender,
        // it's at the discretion of the visibility timeout extender.
        let send_kill_heartbeat_opt = self
            .visibility_timeout_extender
            .clone()
            .start_heartbeat(messages.into_iter().map(|m| m.into()).collect());

        let result = handle_msg_future.await;
        if let Some(send_kill_heartbeat) = send_kill_heartbeat_opt {
            debug!("sending kill signal to visibility timeout extender");
            match send_kill_heartbeat.send(()) {
                Ok(_) => (),
                Err(_) => error!("the heartbeat task receiver dropped"),
            }
        }
        let delete_result = self
            .delete_messages(&message_handles, result.is_err())
            .await;
        if let Err(err) = delete_result {
            error!("failed to delete messages from queue: {}", err);
        }

        match result {
            Ok(_) => (),
            Err(error) => {
                let _res = self.terminate_visibility_timeout(&message_handles).await;

                match error {
                    MessageHandlerError::Timeout(_) => {
                        let message_handler_timeout = self.config.message_handler_timeout;
                        error!(
                            "did not finish processing message(s) within {:?} seconds",
                            message_handler_timeout
                        );
                    }
                    MessageHandlerError::MissingHandler => {
                        error!("message handler was not registered")
                    }
                    MessageHandlerError::HandlerFailure(handler_error) => {
                        error!("message handler failed: {}", handler_error)
                    }
                    _ => {}
                }
            }
        }
        Ok(())
    }
}
