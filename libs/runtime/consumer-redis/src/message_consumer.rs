use std::{collections::HashMap, fmt::Debug, sync::Arc, time::Duration};

use async_trait::async_trait;
use celerity_helpers::{
    consumers::{
        extract_context_ids, Message, MessageConsumer, MessageHandler, MessageHandlerError,
        PartialBatchFailureInfo, PinnedMessageHandlerFuture,
    },
    redis::{ConnectionWrapper, StreamTrimStrategy},
    retries::{calculate_retry_wait_time_ms, RetryConfig},
    telemetry::{CELERITY_CONTEXT_IDS_KEY, CELERITY_CONTEXT_ID_KEY},
    time::{calcuate_polling_wait_time, Clock},
};
use futures::future::join_all;
use opentelemetry::trace::SpanKind;
use redis::{streams::StreamId, RedisError, Script};
use tokio::{
    sync::broadcast,
    time::{self, timeout, Instant},
};
use tracing::{debug, error, field, info, info_span, instrument, warn, Instrument};

use crate::{
    errors::WorkerError,
    lock_durations::LockDurationExtender,
    locks::MessageLocks,
    trim_lock::TrimLock,
    types::{FromRedisStreamId, RedisMessageMetadata, ToStreamRedisArgParts},
};

/// Configuration for a Redis connection used to consume messages from a stream.
#[derive(Debug)]
pub struct RedisConsumerConfig {
    /// The name of the service that the consumer is part of, this will be
    /// used as the [hash tag](https://redis.io/docs/latest/operate/oss_and_stack/reference/cluster-spec/#hash-tags)
    /// for message locks.
    pub service_name: String,
    /// A name for the consumer that is used for debugging.
    pub name: String,
    /// The name of the stream to consume messages from.
    pub stream: String,
    /// The name of the stream that acts as a dead letter queue for messages
    /// that are not processed successfully.
    ///
    /// If not provided, messages will not be moved and will eventually be deleted from
    /// the stream based on the trim interval and or the max length of the stream.
    pub dlq_stream: Option<String>,
    /// The number of times to retry a message that is not processed
    /// successfully.
    /// The key to use to store the last message ID from the stream
    /// that was read and successfully processed from the stream.
    ///
    /// If not provided, the last message ID will be stored with
    /// the "celerity:consumer:<name>:last_message_id" key.
    pub last_message_id_key: Option<String>,
    /// The maximum time to wait for messages to be available in the stream
    /// before continuing to the next iteration of the consumer polling loop.
    ///
    /// Defaults to 30,000 milliseconds (30 seconds).
    pub block_time_ms: Option<u64>,
    /// The minimum time to wait between each call to read messages
    /// from the stream.
    ///
    /// Defaults to 10,000 milliseconds (10 seconds).
    pub polling_wait_time_ms: Option<u64>,
    /// The maximum number of messages to read in a single call to the stream.
    ///
    /// Defaults to 100 messages.
    pub batch_size: Option<usize>,
    /// The maximum time to wait for a message handler to complete
    /// in seconds.
    pub message_handler_timeout: u64,
    /// The initial lock duration for a message in the stream in milliseconds.
    /// This will be extended for long running processing of messages.
    /// This is equivalent to the visibility timeout for an Amazon SQS queue.
    ///
    /// Defaults to 30,000 milliseconds (30 seconds).
    pub lock_duration_ms: Option<u64>,
    /// The maximum number of times to retry a messages that are not processed
    /// successfully before moving to a dead letter queue.
    ///
    /// Defaults to 3 retries.
    pub max_retries: Option<i64>,
    /// The base delay in milliseconds to wait before retrying a message
    /// that has not been processed successfully.
    /// This is used as the base delay in an exponential backoff strategy.
    ///
    /// Defaults to 10,000 milliseconds (10 seconds).
    pub retry_base_delay_ms: Option<i64>,
    /// The maximum delay in seconds to wait before retrying a message
    /// that has not been processed successfully.
    ///
    /// Defaults to 60 seconds.
    pub retry_max_delay: Option<i64>,
    /// The backoff rate to use for the exponential backoff strategy.
    /// This is used to calculate the wait time between retries.
    ///
    /// Defaults to 2.0.
    pub backoff_rate: Option<f64>,
    /// The interval in seconds to trim the stream up to the last message ID.
    /// You may want to set this to a higher value if it is important
    /// to keep a record of messages that were processed for a period of days,
    /// weeks or months.
    /// Setting this to -1 will disable stream trimming.
    ///
    /// Defaults to 86,400 seconds (24 hours).
    pub trim_stream_interval: Option<i64>,
    /// The maximum length of the stream for the purpose of trimming
    /// the stream.
    /// When provided, the stream will be trimmed based on the max length
    /// instead of the last message ID.
    /// If this is not provided, the stream will be trimmed based on the last message ID.
    pub max_stream_length: Option<u64>,
    /// The maximum time that a stream trimming worker can hold a lock for.
    ///
    /// Defaults to 60,000 milliseconds (60 seconds).
    pub trim_lock_timeout_ms: Option<u64>,
    /// The number of worker tasks to use to read and process messages
    /// from the stream.
    ///
    /// Defaults to 10 workers.
    pub num_workers: Option<usize>,
}

#[derive(Debug)]
struct RedisConsumerFinalisedConfig {
    service_name: String,
    name: String,
    stream: String,
    dlq_stream: Option<String>,
    last_message_id_key: String,
    block_time_ms: u64,
    polling_wait_time_ms: u64,
    batch_size: usize,
    message_handler_timeout: u64,
    lock_duration_ms: u64,
    max_retries: i64,
    retry_base_delay_seconds: f64,
    retry_max_delay: i64,
    backoff_rate: f64,
    trim_stream_interval: i64,
    max_stream_length: Option<u64>,
    trim_lock_timeout_ms: u64,
    num_workers: usize,
}

/// Provides an implementation of a Redis message consumer
/// that consumes messages from a Redis stream.
pub struct RedisMessageConsumer {
    handler: Option<Arc<dyn MessageHandler<RedisMessageMetadata> + Send + Sync>>,
    update_last_message_id_script: Arc<Script>,
    lock_duration_extender: Arc<LockDurationExtender>,
    redis_connection: ConnectionWrapper,
    clock: Arc<dyn Clock + Send + Sync>,
    shutdown_broadcast_tx: broadcast::Sender<()>,
    config: Arc<RedisConsumerFinalisedConfig>,
}

impl Debug for RedisMessageConsumer {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "RedisMessageConsumer")
    }
}

impl Clone for RedisMessageConsumer {
    fn clone(&self) -> Self {
        RedisMessageConsumer {
            update_last_message_id_script: self.update_last_message_id_script.clone(),
            handler: self.handler.clone(),
            lock_duration_extender: self.lock_duration_extender.clone(),
            redis_connection: self.redis_connection.clone(),
            clock: self.clock.clone(),
            shutdown_broadcast_tx: self.shutdown_broadcast_tx.clone(),
            config: self.config.clone(),
        }
    }
}

#[async_trait]
impl MessageConsumer<RedisMessageMetadata> for RedisMessageConsumer {
    type Error = WorkerError;

    fn register_handler(
        &mut self,
        handler: Arc<dyn MessageHandler<RedisMessageMetadata> + Send + Sync>,
    ) {
        self.handler = Some(handler);
    }

    #[instrument(name = "redis_message_consumer", skip(self))]
    async fn start(&self) -> Result<(), Self::Error> {
        let consumer_arc = Arc::new(self.clone());
        let mut worker_handles = Vec::new();
        for worker_id in 0..self.config.num_workers {
            let consumer = consumer_arc.clone();
            let mut conn_for_worker = self.redis_connection.clone();
            let shutdown_broadcast_tx = self.shutdown_broadcast_tx.clone();
            let worker_handle = tokio::spawn(async move {
                consumer
                    .start_worker(worker_id, &mut conn_for_worker, &shutdown_broadcast_tx)
                    .await
            });
            worker_handles.push(worker_handle);
        }

        if self.config.trim_stream_interval >= 0 {
            let mut conn_for_trim_stream_worker = self.redis_connection.clone();
            let shutdown_broadcast_tx = self.shutdown_broadcast_tx.clone();
            let worker_handle = tokio::spawn(async move {
                consumer_arc
                    .start_stream_trimming_worker(
                        &mut conn_for_trim_stream_worker,
                        &shutdown_broadcast_tx,
                    )
                    .await
            });
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

impl RedisMessageConsumer {
    pub fn new(
        lock_duration_extender: Arc<LockDurationExtender>,
        clock: Arc<dyn Clock + Send + Sync>,
        redis_connection: ConnectionWrapper,
        shutdown_broadcast_tx: broadcast::Sender<()>,
        config: RedisConsumerConfig,
    ) -> Self {
        let update_last_message_id_script = Arc::new(Script::new(include_str!(
            "../lua-scripts/update_last_message_id.lua"
        )));

        let final_config = RedisConsumerFinalisedConfig {
            service_name: config.service_name.clone(),
            name: config.name.clone(),
            stream: config.stream.clone(),
            dlq_stream: config.dlq_stream,
            last_message_id_key: config.last_message_id_key.unwrap_or(format!(
                "celerity:consumer:{stream}:last_message_id",
                stream = config.stream
            )),
            block_time_ms: config.block_time_ms.unwrap_or(30000),
            polling_wait_time_ms: config.polling_wait_time_ms.unwrap_or(10000),
            batch_size: config.batch_size.unwrap_or(100),
            message_handler_timeout: config.message_handler_timeout,
            lock_duration_ms: config.lock_duration_ms.unwrap_or(30000),
            max_retries: config.max_retries.unwrap_or(3),
            retry_base_delay_seconds: config.retry_base_delay_ms.unwrap_or(10000) as f64 / 1000.0,
            retry_max_delay: config.retry_max_delay.unwrap_or(60),
            backoff_rate: config.backoff_rate.unwrap_or(2.0),
            trim_stream_interval: config.trim_stream_interval.unwrap_or(86400),
            max_stream_length: config.max_stream_length,
            trim_lock_timeout_ms: config.trim_lock_timeout_ms.unwrap_or(60000),
            num_workers: config.num_workers.unwrap_or(10),
        };

        RedisMessageConsumer {
            update_last_message_id_script,
            handler: None,
            lock_duration_extender,
            clock,
            shutdown_broadcast_tx,
            config: Arc::new(final_config),
            redis_connection,
        }
    }

    async fn start_worker(
        self: Arc<Self>,
        worker_id: usize,
        conn: &mut ConnectionWrapper,
        shutdown_tx: &broadcast::Sender<()>,
    ) {
        let consumer_worker_name = format!("{name}-{worker_id}", name = self.config.name);
        let mut message_locks = MessageLocks::new(
            self.config.service_name.clone(),
            consumer_worker_name,
            conn.clone(),
        );

        let mut shutdown_rx = shutdown_tx.subscribe();

        async move {
            loop {
                if let Ok(()) = shutdown_rx.try_recv() {
                    info!("received shutdown signal, stopping worker");
                    break;
                }

                let start_time = Instant::now();
                let mut current_polling_wait_time_ms = self.config.polling_wait_time_ms;

                self.receive_messages(conn, &mut message_locks).await;

                current_polling_wait_time_ms =
                    calcuate_polling_wait_time(start_time, current_polling_wait_time_ms);

                if current_polling_wait_time_ms > 0 {
                    tokio::time::sleep(Duration::from_millis(current_polling_wait_time_ms)).await;
                }
            }
        }
        .instrument(info_span!(
            "redis_message_consumer_worker",
            worker_id = worker_id
        ))
        .await
    }

    async fn receive_messages(
        &self,
        conn: &mut ConnectionWrapper,
        message_locks: &mut MessageLocks,
    ) {
        let last_message_id = match self.get_last_message_id(conn).await {
            Ok(last_message_id) => last_message_id,
            Err(err) => {
                warn!(
                    "failed to get last message ID from the stream, \
                    will read stream from the beginning: {err}"
                );
                "".to_string()
            }
        };

        let offset_ids = if last_message_id.is_empty() {
            vec!["0"]
        } else {
            vec![last_message_id.as_str()]
        };

        let read_result = conn
            .xread(
                &[&self.config.stream],
                offset_ids.as_slice(),
                self.config.batch_size,
                self.config.block_time_ms as usize,
            )
            .await;

        match read_result {
            Ok(read_result) => {
                let stream_messages = if !read_result.keys.is_empty() {
                    &read_result.keys[0].ids
                } else {
                    &vec![]
                };

                let filtered_stream_messages = self
                    .filter_stream_messages(stream_messages, message_locks)
                    .await
                    .expect("must be able to filter stream messages to honour message locks");

                let message_count = filtered_stream_messages.len();
                debug!(
                    "read {message_count} available messages from the {} stream",
                    self.config.stream
                );

                if !filtered_stream_messages.is_empty() {
                    self.process_messages(&filtered_stream_messages, conn).await;
                } else {
                    debug!("no messages available to process, moving to next iteration");
                }
            }
            Err(err) => {
                error!(
                    "failed to read messages from the {} stream: {err}",
                    self.config.stream
                );
            }
        }
    }

    async fn process_messages<'a>(
        &'a self,
        filtered_stream_messages: &'a [&'a StreamId],
        conn: &mut ConnectionWrapper,
    ) {
        let messages = filtered_stream_messages
            .iter()
            .map(|stream_id| Message::<RedisMessageMetadata>::from_redis_stream_id(stream_id))
            .collect::<Vec<Message<RedisMessageMetadata>>>();

        let last_message_id = messages
            .last()
            .map(|m| m.message_id.clone())
            .expect("must have at least one message");

        let handle_msg_future = self.derive_handler_future(messages.clone());

        let send_kill_heartbeat_opt = self
            .lock_duration_extender
            .clone()
            .start_heartbeat(messages.iter().map(|m| m.message_id.clone()).collect());

        let result = handle_msg_future.await;
        if let Some(send_kill_heartbeat) = send_kill_heartbeat_opt {
            debug!("sending kill signal to lock duration extender");
            match send_kill_heartbeat.send(()) {
                Ok(_) => (),
                Err(_) => error!("the heartbeat task receiver dropped"),
            }
        }

        let update_last_message_id_result =
            self.update_last_message_id(&last_message_id, conn).await;
        if let Err(err) = update_last_message_id_result {
            error!("failed to update last message ID: {err}");
        }

        let failures = match result {
            Ok(_) => vec![],
            Err(error) => match error {
                MessageHandlerError::Timeout(_) => {
                    let message_handler_timeout = self.config.message_handler_timeout;
                    let reason = format!(
                        "did not finish processing message(s) within {:?} seconds",
                        message_handler_timeout
                    );
                    error!(reason);
                    partial_failures_from_messages(&messages, reason, self.config.max_retries)
                }
                MessageHandlerError::MissingHandler => {
                    let reason = "message handler was not registered".to_string();
                    error!(reason);
                    partial_failures_from_messages(&messages, reason, self.config.max_retries)
                }
                MessageHandlerError::HandlerFailure(handler_error) => {
                    let reason = format!("message handler failed: {}", handler_error);
                    error!(reason);
                    partial_failures_from_messages(&messages, reason, self.config.max_retries)
                }
                MessageHandlerError::PartialBatchFailure(partial_failures) => {
                    error!("failed to process full message batch: {partial_failures:?}");
                    partial_failures
                }
            },
        };

        if !failures.is_empty() {
            if let Err(err) = self
                .move_failed_messages_to_dlq(&failures, &messages, conn)
                .await
            {
                error!("failed to move failed messages to DLQ: {err}");
            }
        }
    }

    fn derive_handler_future(
        &self,
        messages: Vec<Message<RedisMessageMetadata>>,
    ) -> PinnedMessageHandlerFuture<'_> {
        if messages.len() == 1 {
            Box::pin(self.handle_single_message_with_retries(messages[0].clone()))
        } else {
            Box::pin(self.handle_messages_with_retries(messages))
        }
    }

    async fn handle_single_message_with_retries(
        &self,
        message: Message<RedisMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
        let mut attempt: i64 = 1;
        let mut final_result = Ok(());
        while attempt <= self.config.max_retries + 1 {
            info!("starting attempt {attempt} to process a single message");
            match self.handle_single_message(&message).await {
                Ok(_) => return Ok(()),
                Err(MessageHandlerError::MissingHandler) => {
                    error!("message handler has not been registered, will not retry processing message");
                    return Err(MessageHandlerError::MissingHandler);
                }
                Err(err) => {
                    error!("failed to process message: {err}");
                    let wait_time_ms = calculate_retry_wait_time_ms(
                        &RetryConfig {
                            jitter: Some(true),
                            max_delay: Some(self.config.retry_max_delay),
                            ..RetryConfig::default()
                        },
                        attempt - 1,
                        self.config.retry_base_delay_seconds,
                        self.config.backoff_rate,
                    );
                    info!("waiting {wait_time_ms} milliseconds before retrying");
                    tokio::time::sleep(Duration::from_millis(wait_time_ms)).await;
                    attempt += 1;
                    final_result = Err(err);
                }
            }
        }

        final_result
    }

    async fn handle_single_message(
        &self,
        message: &Message<RedisMessageMetadata>,
    ) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle(message)),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        // This span should be its own consumer node to decouple from the parent segment.
        // see https://docs.rs/tracing-opentelemetry/latest/tracing_opentelemetry/#special-fields
        // A Segment is only created for span.kind Server, even though this is a consumer,
        // we want it to have its own top-level segment to differentiate from Redis
        // (or other redis-compatible service) and the message sender.
        let span = info_span!(
            "handle_single_message",
            "otel.kind" = ?SpanKind::Server,
            // This is hardcoded instead of being a const as there's no obvious way
            // to use dynamic field names when constructing a span with the macro.
            "celerity.context-id" = field::Empty,
        );

        if let Some(trace_context) = &message.trace_context {
            if let Some(celerity_context_id) = trace_context.get(CELERITY_CONTEXT_ID_KEY) {
                span.record(CELERITY_CONTEXT_ID_KEY, celerity_context_id.to_string());
            }
        }

        self.handle_messages_future(future_result)
            .instrument(span)
            .await
    }

    async fn handle_messages_with_retries(
        &self,
        messages: Vec<Message<RedisMessageMetadata>>,
    ) -> Result<(), MessageHandlerError> {
        let mut attempt: i64 = 1;
        let mut messages_remaining = messages.clone();
        let mut final_result = Ok(());
        while attempt <= self.config.max_retries + 1 {
            info!("starting attempt {attempt} to process message batch");
            match self.handle_messages(&messages_remaining).await {
                Ok(_) => return Ok(()),
                Err(MessageHandlerError::MissingHandler) => {
                    error!("message handler has not been registered, will not retry processing message batch");
                    return Err(MessageHandlerError::MissingHandler);
                }
                Err(err) => {
                    error!("failed to process message batch: {err}");
                    let wait_time_ms = calculate_retry_wait_time_ms(
                        &RetryConfig {
                            jitter: Some(true),
                            max_delay: Some(self.config.retry_max_delay),
                            ..RetryConfig::default()
                        },
                        attempt - 1,
                        self.config.retry_base_delay_seconds,
                        self.config.backoff_rate,
                    );
                    info!("waiting {wait_time_ms} milliseconds before retrying");
                    tokio::time::sleep(Duration::from_millis(wait_time_ms)).await;

                    let final_err =
                        if let MessageHandlerError::PartialBatchFailure(partial_failures) = err {
                            keep_failed_messages(&mut messages_remaining, &partial_failures);
                            MessageHandlerError::PartialBatchFailure(
                                partial_failures_with_retries_attempted(
                                    &partial_failures,
                                    // Attempt counts will always be a low number, so we can safely
                                    // cast to u64 without worrying about truncation.
                                    attempt as u64 - 1,
                                ),
                            )
                        } else {
                            err
                        };

                    final_result = Err(final_err);
                    attempt += 1;
                }
            }
        }

        final_result
    }

    async fn handle_messages(
        &self,
        messages: &[Message<RedisMessageMetadata>],
    ) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle_batch(messages)),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        let span = info_span!(
            "handle_messages",
            "otel.kind" = ?SpanKind::Server,
            "batch_size"= messages.len(),
            // This is hardcoded instead of being a const as there's no obvious way
            // to use dynamic field names when constructing a span with the macro.
            "celerity.context-ids" = field::Empty,
        );

        let context_ids = extract_context_ids(messages);
        if !context_ids.is_empty() {
            span.record(CELERITY_CONTEXT_IDS_KEY, context_ids.join(","));
        }

        self.handle_messages_future(future_result)
            .instrument(span)
            .await
    }

    async fn handle_messages_future(
        &self,
        wrapped_future: Result<PinnedMessageHandlerFuture<'_>, MessageHandlerError>,
    ) -> Result<(), MessageHandlerError> {
        match wrapped_future {
            Err(err) => Err(err),
            Ok(future) => {
                debug!(
                    timeout = self.config.message_handler_timeout.clone(),
                    "running message handler with timeout",
                );
                match timeout(
                    Duration::from_secs(self.config.message_handler_timeout),
                    future,
                )
                .await
                {
                    Err(timeout_err) => Err(MessageHandlerError::Timeout(timeout_err)),
                    Ok(result) => result,
                }
            }
        }
    }

    async fn get_last_message_id(
        &self,
        connection: &mut ConnectionWrapper,
    ) -> Result<String, RedisError> {
        let last_message_id = connection
            .get(self.config.last_message_id_key.as_str())
            .await?;

        Ok(last_message_id)
    }

    async fn update_last_message_id(
        &self,
        message_id: &str,
        connection: &mut ConnectionWrapper,
    ) -> Result<(), RedisError> {
        let update_last_message_id_script =
            include_str!("../lua-scripts/update_last_message_id.lua");

        connection
            .eval_script::<()>(
                update_last_message_id_script,
                &[self.config.last_message_id_key.as_str()],
                &[message_id],
            )
            .await?;

        Ok(())
    }

    async fn filter_stream_messages<'a>(
        &'a self,
        stream_messages: &'a [StreamId],
        message_locks: &'a mut MessageLocks,
    ) -> Result<Vec<&'a StreamId>, WorkerError> {
        let message_ids = stream_messages
            .iter()
            .map(|m| m.id.as_str())
            .collect::<Vec<&str>>();

        let lock_results = message_locks
            .acquire_locks(&message_ids, self.config.lock_duration_ms)
            .await?;

        if lock_results.len() != stream_messages.len() {
            return Err(WorkerError::new(format!(
                "failed to acquire locks for all messages, \
                expected {} locks, got {}",
                stream_messages.len(),
                lock_results.len()
            )));
        }

        let filtered_stream_messages = stream_messages
            .iter()
            .zip(lock_results.iter())
            .filter_map(
                |(message, lock_result)| {
                    if *lock_result {
                        Some(message)
                    } else {
                        None
                    }
                },
            )
            .collect::<Vec<&StreamId>>();
        Ok(filtered_stream_messages)
    }

    async fn move_failed_messages_to_dlq(
        &self,
        partial_failures: &[PartialBatchFailureInfo],
        messages: &[Message<RedisMessageMetadata>],
        connection: &mut ConnectionWrapper,
    ) -> Result<(), RedisError> {
        if let Some(dlq_stream) = &self.config.dlq_stream {
            let partial_failure_map = partial_failures_to_map(partial_failures);

            for message in messages {
                if partial_failure_map.contains_key(&message.message_id) {
                    let message_with_failure_ctx = add_failure_context_to_message(
                        message,
                        partial_failure_map.get(&message.message_id).copied(),
                        self.clock.clone(),
                    );

                    let stream_msg_arg_parts =
                        message_with_failure_ctx.to_stream_redis_arg_parts(true);
                    connection
                        .xadd(
                            dlq_stream,
                            "*", // Use auto-generated Redis stream ID for chronological ordering
                            &stream_msg_arg_parts.fields,
                        )
                        .await?;
                }
            }
        } else {
            info!("no dead letter queue stream configured, failed messages will be lost");
        }
        Ok(())
    }

    async fn start_stream_trimming_worker(
        &self,
        conn: &mut ConnectionWrapper,
        shutdown_tx: &broadcast::Sender<()>,
    ) {
        let mut trim_lock = TrimLock::new(
            self.config.service_name.clone(),
            self.config.name.clone(),
            self.config.stream.clone(),
            conn.clone(),
        );

        let mut shutdown_rx = shutdown_tx.subscribe();
        let mut interval =
            time::interval(Duration::from_secs(self.config.trim_stream_interval as u64));

        loop {
            tokio::select! {
                _ = shutdown_rx.recv() => {
                    info!("received shutdown signal, stopping stream trimming worker");
                    break;
                }
                _ = interval.tick() => {
                    match trim_lock.acquire(self.config.trim_lock_timeout_ms).await {
                        Ok(true) => {
                            if let Err(err) = self.trim_stream(conn).await {
                                error!("failed to trim stream: {err}");
                            }
                        }
                        Ok(false) => {
                            info!("failed to acquire trim lock, will not trim stream until next interval");
                        }
                        Err(err) => {
                            error!("failed to acquire trim lock: {err}");
                        }
                    }
                }
            }
        }
    }

    async fn trim_stream(&self, conn: &mut ConnectionWrapper) -> Result<(), RedisError> {
        let trim_strategy = if let Some(max_stream_length) = self.config.max_stream_length {
            // When a max stream length is provided, we will use a more agressive
            // approach to trim the stream by ensuring that the stream is always
            // at or below the max stream length.
            StreamTrimStrategy::MaxLen(max_stream_length as usize)
        } else {
            let last_message_id = self.get_last_message_id(conn).await?;
            StreamTrimStrategy::MinId(last_message_id)
        };

        conn.xtrim(&self.config.stream, trim_strategy).await
    }
}

fn keep_failed_messages(
    messages: &mut Vec<Message<RedisMessageMetadata>>,
    partial_failures: &[PartialBatchFailureInfo],
) {
    let partial_failure_map = partial_failures_to_map(partial_failures);
    messages.retain(|m| partial_failure_map.contains_key(&m.message_id));
}

fn partial_failures_to_map(
    partial_failures: &[PartialBatchFailureInfo],
) -> HashMap<String, &PartialBatchFailureInfo> {
    partial_failures
        .iter()
        .map(|p| (p.message_id.clone(), p))
        .collect::<HashMap<String, &PartialBatchFailureInfo>>()
}

fn partial_failures_with_retries_attempted(
    partial_failures: &[PartialBatchFailureInfo],
    retries_attempted: u64,
) -> Vec<PartialBatchFailureInfo> {
    partial_failures
        .iter()
        .map(|p| {
            PartialBatchFailureInfo::new(
                p.message_id.clone(),
                p.error_reason.clone(),
                retries_attempted,
            )
        })
        .collect()
}

fn add_failure_context_to_message(
    message: &Message<RedisMessageMetadata>,
    partial_failure: Option<&PartialBatchFailureInfo>,
    clock: Arc<dyn Clock + Send + Sync>,
) -> Message<RedisMessageMetadata> {
    let mut message_with_failure_ctx = message.clone();

    let (failure_reason, retries_attempted) = match partial_failure {
        Some(partial_failure) => (
            Some(partial_failure.error_reason.clone()),
            partial_failure.retry_count,
        ),
        None => (None, 0),
    };

    message_with_failure_ctx.metadata.failure_reason = failure_reason;
    message_with_failure_ctx.metadata.retry_count =
        Some(message_with_failure_ctx.metadata.retry_count.unwrap_or(0) + retries_attempted);
    message_with_failure_ctx.metadata.failed_at = Some(clock.now());

    message_with_failure_ctx
}

fn partial_failures_from_messages(
    messages: &[Message<RedisMessageMetadata>],
    reason: String,
    retries_attempted: i64,
) -> Vec<PartialBatchFailureInfo> {
    messages
        .iter()
        .map(|m| {
            PartialBatchFailureInfo::new(
                m.message_id.clone(),
                reason.clone(),
                retries_attempted as u64,
            )
        })
        .collect()
}
