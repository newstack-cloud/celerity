use std::{
    collections::VecDeque,
    fmt::Debug,
    marker::PhantomData,
    sync::{Arc, Mutex},
};

use async_trait::async_trait;
use celerity_helpers::consumers::{
    Message, MessageHandler, MessageHandlerError, RoutedMessage, RoutedMessageHandler,
};
use serde_json::{json, Value};
use std::time::Instant;
use tracing::{field, info, instrument};

use crate::types::{
    ConsumerEventData, ConsumerMessage, EventData, EventDataPayload, EventResult, EventTuple,
    EventType, ScheduleEventData,
};

// ---------------------------------------------------------------------------
// ConsumerEventHandler trait (metadata-agnostic)
// ---------------------------------------------------------------------------

/// Error type for consumer/schedule event handler operations.
#[derive(Debug)]
pub enum ConsumerEventHandlerError {
    Timeout,
    HandlerFailure(String),
    MissingHandler,
    ChannelClosed,
}

impl std::fmt::Display for ConsumerEventHandlerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConsumerEventHandlerError::Timeout => write!(f, "handler timed out"),
            ConsumerEventHandlerError::HandlerFailure(msg) => write!(f, "handler failed: {msg}"),
            ConsumerEventHandlerError::MissingHandler => write!(f, "no handler registered"),
            ConsumerEventHandlerError::ChannelClosed => write!(f, "response channel closed"),
        }
    }
}

impl std::error::Error for ConsumerEventHandlerError {}

/// Metadata-agnostic handler for consumer and schedule events.
/// The SDK implements this trait — it never sees platform-specific metadata.
#[async_trait]
pub trait ConsumerEventHandler: Send + Sync {
    async fn handle_consumer_event(
        &self,
        handler_tag: &str,
        event_data: ConsumerEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError>;

    async fn handle_schedule_event(
        &self,
        handler_tag: &str,
        event_data: ScheduleEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError>;
}

// ---------------------------------------------------------------------------
// ToConsumerEventData — converts platform-specific messages
// ---------------------------------------------------------------------------

/// Converts a platform-specific `Message<M>` into consumer message data.
pub trait ToConsumerEventData {
    fn to_consumer_messages(&self, source: &str) -> Vec<ConsumerMessage>;
    fn to_vendor_json(&self) -> Value;
}

// ---------------------------------------------------------------------------
// Source parsing and event type mapping
// ---------------------------------------------------------------------------

/// Parses a `celerity:{type}:{name}` source string into `(source_type, source_name)`.
/// Returns `(None, None)` if the source does not follow the `celerity:` convention.
fn parse_source(source: &str) -> (Option<String>, Option<String>) {
    let parts: Vec<&str> = source.splitn(3, ':').collect();
    if parts.len() == 3 && parts[0] == "celerity" {
        (Some(parts[1].to_string()), Some(parts[2].to_string()))
    } else {
        (None, None)
    }
}

// Event type mapping and body transformation are delegated to the
// `body_transform` module which organises provider-specific logic by cloud
// provider (AWS, GCP, etc.).  See `crate::body_transform`.

/// Applies provider-specific event type mapping and body transformation to a
/// batch of consumer messages.
///
/// For each message that has a `source_type` and `event_name`, this resolves
/// the Celerity-standard `event_type` and transforms the raw body into the
/// normalised shape.  The original body is preserved in `vendor.originalBody`
/// when a transform is applied.
fn apply_body_transforms(messages: Vec<ConsumerMessage>, provider: &str) -> Vec<ConsumerMessage> {
    use crate::body_transform;

    messages
        .into_iter()
        .map(|mut msg| {
            let source_type = msg.source_type.as_deref();
            let event_name = msg.event_name.as_deref();

            // Map provider event name → Celerity event type.
            if let Some(en) = event_name {
                msg.event_type = body_transform::map_event_type(provider, en, source_type);
            }

            // Transform body if a provider transform exists for this source type.
            if let Some(st) = source_type {
                if msg.event_type.is_some() {
                    if let Some(transformed) =
                        body_transform::transform_body(provider, st, &msg.body)
                    {
                        if transformed != msg.body {
                            msg.vendor.as_object_mut().map(|v| {
                                v.insert(
                                    "originalBody".to_string(),
                                    Value::String(msg.body.clone()),
                                )
                            });
                            msg.body = transformed;
                        }
                    }
                }
            }

            msg
        })
        .collect()
}

#[cfg(feature = "celerity_local_consumers")]
impl ToConsumerEventData for Message<celerity_consumer_redis::types::RedisMessageMetadata> {
    fn to_consumer_messages(&self, source: &str) -> Vec<ConsumerMessage> {
        let mut attrs = serde_json::Map::new();

        if let Some(ref msg_id) = self.metadata.source_message_id {
            attrs.insert(
                "sourceMessageId".to_string(),
                json!({ "dataType": "String", "stringValue": msg_id }),
            );
        }

        if let Some(ref subject) = self.metadata.subject {
            attrs.insert(
                "subject".to_string(),
                json!({ "dataType": "String", "stringValue": subject }),
            );
        }

        if let Some(ref attributes_json) = self.metadata.attributes {
            if let Ok(user_attrs) =
                serde_json::from_str::<std::collections::HashMap<String, String>>(attributes_json)
            {
                for (k, v) in user_attrs {
                    attrs.insert(k, json!({ "dataType": "String", "stringValue": v }));
                }
            }
        }

        if let Some(ref event_name) = self.metadata.event_name {
            attrs.insert(
                "eventName".to_string(),
                json!({ "dataType": "String", "stringValue": event_name }),
            );
        }

        let (source_type, source_name) = parse_source(source);

        vec![ConsumerMessage {
            message_id: self.message_id.clone(),
            body: self.body.clone().unwrap_or_default(),
            source: source.to_string(),
            source_type,
            source_name,
            event_type: None,
            event_name: self.metadata.event_name.clone(),
            message_attributes: Value::Object(attrs),
            vendor: json!({
                "timestamp": self.metadata.timestamp,
                "messageType": format!("{:?}", self.metadata.message_type),
            }),
        }]
    }

    fn to_vendor_json(&self) -> Value {
        json!({
            "timestamp": self.metadata.timestamp,
        })
    }
}

#[cfg(feature = "aws_consumers")]
impl ToConsumerEventData for Message<celerity_consumer_sqs::types::SQSMessageMetadata> {
    fn to_consumer_messages(&self, source: &str) -> Vec<ConsumerMessage> {
        let (source_type, source_name) = parse_source(source);

        vec![ConsumerMessage {
            message_id: self.message_id.clone(),
            body: self.body.clone().unwrap_or_default(),
            source: source.to_string(),
            source_type,
            source_name,
            event_type: None,
            event_name: None,
            message_attributes: self
                .metadata
                .message_attributes
                .as_ref()
                .map(|attrs| {
                    let map: serde_json::Map<String, Value> = attrs
                        .iter()
                        .map(|(k, v)| {
                            (
                                k.clone(),
                                json!({
                                    "dataType": v.data_type(),
                                    "stringValue": v.string_value().unwrap_or_default(),
                                }),
                            )
                        })
                        .collect();
                    Value::Object(map)
                })
                .unwrap_or(json!({})),
            vendor: json!({
                "receiptHandle": self.metadata.receipt_handle,
            }),
        }]
    }

    fn to_vendor_json(&self) -> Value {
        json!({
            "receiptHandle": self.metadata.receipt_handle,
        })
    }
}

// ---------------------------------------------------------------------------
// ConsumerHandlerBridge<M> — implements MessageHandler<M> for queue consumers
// ---------------------------------------------------------------------------

/// Bridges a platform-specific `MessageConsumer<M>` to the metadata-agnostic
/// `ConsumerEventHandler`. Used for consumers without routing.
///
/// After converting platform messages via [`ToConsumerEventData`], the bridge
/// applies provider-specific body transforms and event type mapping from the
/// [`crate::body_transform`] module using the configured `provider`.
pub struct ConsumerHandlerBridge<M: Debug + Clone + Send + Sync> {
    event_handler: Arc<dyn ConsumerEventHandler>,
    handler_tag: String,
    source: String,
    provider: String,
    _metadata: PhantomData<M>,
}

impl<M: Debug + Clone + Send + Sync> ConsumerHandlerBridge<M> {
    pub fn new(
        event_handler: Arc<dyn ConsumerEventHandler>,
        handler_tag: String,
        source: String,
        provider: String,
    ) -> Self {
        Self {
            event_handler,
            handler_tag,
            source,
            provider,
            _metadata: PhantomData,
        }
    }
}

#[async_trait]
impl<M> MessageHandler<M> for ConsumerHandlerBridge<M>
where
    M: Debug + Clone + Send + Sync + 'static,
    Message<M>: ToConsumerEventData,
{
    #[instrument(
        name = "consumer_handler_bridge",
        skip(self, message),
        fields(handler_tag = %self.handler_tag, source = %self.source, otel.status_code = field::Empty)
    )]
    async fn handle(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, "consumer handler invocation started");
        let start = Instant::now();
        let messages =
            apply_body_transforms(message.to_consumer_messages(&self.source), &self.provider);
        let event_data = ConsumerEventData {
            messages,
            vendor: message.to_vendor_json(),
        };
        let result = self
            .event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, "consumer handler processed in {millis:.3} milliseconds");
        result
    }

    #[instrument(
        name = "consumer_handler_bridge_batch",
        skip(self, messages),
        fields(handler_tag = %self.handler_tag, source = %self.source, batch_size = messages.len(), otel.status_code = field::Empty)
    )]
    async fn handle_batch(&self, messages: &[Message<M>]) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, batch_size = messages.len(), "consumer batch handler invocation started");
        let start = Instant::now();
        let mut all_messages = Vec::new();
        let mut vendor = json!({});
        for msg in messages {
            all_messages.extend(msg.to_consumer_messages(&self.source));
            vendor = msg.to_vendor_json();
        }
        let all_messages = apply_body_transforms(all_messages, &self.provider);
        let event_data = ConsumerEventData {
            messages: all_messages,
            vendor,
        };
        let result = self
            .event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, "consumer batch handler processed in {millis:.3} milliseconds");
        result
    }
}

// ---------------------------------------------------------------------------
// RoutedConsumerHandlerBridge<M> — implements RoutedMessageHandler<M>
// ---------------------------------------------------------------------------

/// Bridges a platform-specific routed message to the metadata-agnostic
/// `ConsumerEventHandler`. Used for consumers with routing.
pub struct RoutedConsumerHandlerBridge<M: Debug + Clone + Send + Sync> {
    event_handler: Arc<dyn ConsumerEventHandler>,
    handler_tag: String,
    source: String,
    provider: String,
    _metadata: PhantomData<M>,
}

impl<M: Debug + Clone + Send + Sync> RoutedConsumerHandlerBridge<M> {
    pub fn new(
        event_handler: Arc<dyn ConsumerEventHandler>,
        handler_tag: String,
        source: String,
        provider: String,
    ) -> Self {
        Self {
            event_handler,
            handler_tag,
            source,
            provider,
            _metadata: PhantomData,
        }
    }
}

#[async_trait]
impl<M> RoutedMessageHandler<M> for RoutedConsumerHandlerBridge<M>
where
    M: Debug + Clone + Send + Sync + 'static,
    Message<M>: ToConsumerEventData,
{
    #[instrument(
        name = "routed_consumer_handler_bridge",
        skip(self, message),
        fields(handler_tag = %self.handler_tag, source = %self.source, otel.status_code = field::Empty)
    )]
    async fn handle(&self, message: &RoutedMessage<M>) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, "consumer handler invocation started");
        let start = Instant::now();
        let (source_type, source_name) = parse_source(&self.source);
        let messages = vec![ConsumerMessage {
            message_id: message.message_id.clone(),
            body: message.body.to_string(),
            source: self.source.clone(),
            source_type,
            source_name,
            event_type: None,
            event_name: None,
            message_attributes: json!({}),
            vendor: json!({}),
        }];
        let event_data = ConsumerEventData {
            messages,
            vendor: json!({}),
        };
        let result = self
            .event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, "consumer handler processed in {millis:.3} milliseconds");
        result
    }

    #[instrument(
        name = "routed_consumer_handler_bridge",
        skip(self, message),
        fields(handler_tag = %self.handler_tag, source = %self.source, otel.status_code = field::Empty)
    )]
    async fn handle_raw_message(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, "consumer handler invocation started");
        let start = Instant::now();
        let messages =
            apply_body_transforms(message.to_consumer_messages(&self.source), &self.provider);
        let event_data = ConsumerEventData {
            messages,
            vendor: message.to_vendor_json(),
        };
        let result = self
            .event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, "consumer handler processed in {millis:.3} milliseconds");
        result
    }

    #[instrument(
        name = "routed_consumer_handler_bridge_batch",
        skip(self, messages),
        fields(handler_tag = %self.handler_tag, source = %self.source, batch_size = messages.len(), otel.status_code = field::Empty)
    )]
    async fn handle_raw_message_batch(
        &self,
        messages: &[Message<M>],
    ) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, batch_size = messages.len(), "consumer batch handler invocation started");
        let start = Instant::now();
        let mut all_messages = Vec::new();
        let mut vendor = json!({});
        for msg in messages {
            all_messages.extend(msg.to_consumer_messages(&self.source));
            vendor = msg.to_vendor_json();
        }
        let all_messages = apply_body_transforms(all_messages, &self.provider);
        let event_data = ConsumerEventData {
            messages: all_messages,
            vendor,
        };
        let result = self
            .event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, "consumer batch handler processed in {millis:.3} milliseconds");
        result
    }
}

// ---------------------------------------------------------------------------
// ScheduleHandlerBridge<M> — implements MessageHandler<M> for schedules
// ---------------------------------------------------------------------------

/// Bridges a platform-specific `MessageConsumer<M>` to the metadata-agnostic
/// `ConsumerEventHandler` for schedule triggers.
pub struct ScheduleHandlerBridge<M: Debug + Clone + Send + Sync> {
    event_handler: Arc<dyn ConsumerEventHandler>,
    handler_tag: String,
    schedule_id: String,
    schedule_value: String,
    input: Option<Value>,
    _metadata: PhantomData<M>,
}

impl<M: Debug + Clone + Send + Sync> ScheduleHandlerBridge<M> {
    pub fn new(
        event_handler: Arc<dyn ConsumerEventHandler>,
        handler_tag: String,
        schedule_id: String,
        schedule_value: String,
        input: Option<Value>,
    ) -> Self {
        Self {
            event_handler,
            handler_tag,
            schedule_id,
            schedule_value,
            input,
            _metadata: PhantomData,
        }
    }
}

#[async_trait]
impl<M> MessageHandler<M> for ScheduleHandlerBridge<M>
where
    M: Debug + Clone + Send + Sync + 'static,
    Message<M>: ToConsumerEventData,
{
    #[instrument(
        name = "schedule_handler_bridge",
        skip(self, message),
        fields(
            handler_tag = %self.handler_tag,
            schedule_id = %self.schedule_id,
            schedule_value = %self.schedule_value,
        )
    )]
    async fn handle(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        info!(handler_tag = %self.handler_tag, schedule_id = %self.schedule_id, "schedule handler invocation started");
        let start = Instant::now();
        let event_data = ScheduleEventData {
            schedule_id: self.schedule_id.clone(),
            message_id: message.message_id.clone(),
            schedule: self.schedule_value.clone(),
            input: self.input.clone(),
            vendor: message.to_vendor_json(),
        };
        let result = self
            .event_handler
            .handle_schedule_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error);
        let millis = start.elapsed().as_micros() as f64 / 1000.0;
        info!(handler_tag = %self.handler_tag, schedule_id = %self.schedule_id, "schedule handler processed in {millis:.3} milliseconds");
        result
    }

    #[instrument(
        name = "schedule_handler_bridge_batch",
        skip(self, messages),
        fields(
            handler_tag = %self.handler_tag,
            schedule_id = %self.schedule_id,
            schedule_value = %self.schedule_value,
            batch_size = messages.len(),
        )
    )]
    async fn handle_batch(&self, messages: &[Message<M>]) -> Result<(), MessageHandlerError> {
        for message in messages {
            self.handle(message).await?;
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// EventQueueConsumerEventHandler (HTTP call mode)
// ---------------------------------------------------------------------------

/// Implements `ConsumerEventHandler` for HTTP call mode by pushing events
/// onto the shared event queue and awaiting results via oneshot channels.
pub struct EventQueueConsumerEventHandler {
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
}

impl EventQueueConsumerEventHandler {
    pub fn new(event_queue: Arc<Mutex<VecDeque<EventTuple>>>) -> Self {
        Self { event_queue }
    }
}

#[async_trait]
impl ConsumerEventHandler for EventQueueConsumerEventHandler {
    #[instrument(
        name = "event_queue_consumer_handler",
        skip(self, event_data),
        fields(handler_tag = %handler_tag, event_type = "consumer")
    )]
    async fn handle_consumer_event(
        &self,
        handler_tag: &str,
        event_data: ConsumerEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        let event = EventData {
            id: nanoid::nanoid!(),
            event_type: EventType::ConsumerMessage,
            handler_tag: handler_tag.to_string(),
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs(),
            data: EventDataPayload::ConsumerMessageEventData(event_data),
        };
        enqueue_and_await(self.event_queue.clone(), event).await
    }

    #[instrument(
        name = "event_queue_schedule_handler",
        skip(self, event_data),
        fields(handler_tag = %handler_tag, event_type = "schedule")
    )]
    async fn handle_schedule_event(
        &self,
        handler_tag: &str,
        event_data: ScheduleEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        let event = EventData {
            id: nanoid::nanoid!(),
            event_type: EventType::ScheduleMessage,
            handler_tag: handler_tag.to_string(),
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs(),
            data: EventDataPayload::ScheduleMessageEventData(event_data),
        };
        enqueue_and_await(self.event_queue.clone(), event).await
    }
}

async fn enqueue_and_await(
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    event: EventData,
) -> Result<EventResult, ConsumerEventHandlerError> {
    let (tx, rx) = tokio::sync::oneshot::channel();
    {
        let mut queue = event_queue.lock().unwrap();
        queue.push_back((tx, event));
    }
    match rx.await {
        Ok((_event_data, result)) => Ok(result),
        Err(_) => Err(ConsumerEventHandlerError::ChannelClosed),
    }
}

// ---------------------------------------------------------------------------
// SharedConsumerEventHandler (FFI call mode late-binding)
// ---------------------------------------------------------------------------

/// Wraps an optional inner handler behind a `RwLock`, allowing the SDK
/// to register the handler after `setup()` but before `run()`.
pub struct SharedConsumerEventHandler {
    inner: std::sync::RwLock<Option<Arc<dyn ConsumerEventHandler>>>,
}

impl Default for SharedConsumerEventHandler {
    fn default() -> Self {
        Self::new()
    }
}

impl SharedConsumerEventHandler {
    pub fn new() -> Self {
        Self {
            inner: std::sync::RwLock::new(None),
        }
    }

    pub fn set(&self, handler: Arc<dyn ConsumerEventHandler>) {
        let mut inner = self.inner.write().unwrap();
        *inner = Some(handler);
    }
}

#[async_trait]
impl ConsumerEventHandler for SharedConsumerEventHandler {
    async fn handle_consumer_event(
        &self,
        handler_tag: &str,
        event_data: ConsumerEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        let handler = {
            let inner = self.inner.read().unwrap();
            inner.clone()
        };
        match handler {
            Some(h) => h.handle_consumer_event(handler_tag, event_data).await,
            None => Err(ConsumerEventHandlerError::MissingHandler),
        }
    }

    async fn handle_schedule_event(
        &self,
        handler_tag: &str,
        event_data: ScheduleEventData,
    ) -> Result<EventResult, ConsumerEventHandlerError> {
        let handler = {
            let inner = self.inner.read().unwrap();
            inner.clone()
        };
        match handler {
            Some(h) => h.handle_schedule_event(handler_tag, event_data).await,
            None => Err(ConsumerEventHandlerError::MissingHandler),
        }
    }
}

// ---------------------------------------------------------------------------
// ManagedConsumer trait (type erasure)
// ---------------------------------------------------------------------------

/// Type-erased consumer that can be spawned as a task.
#[async_trait]
pub(crate) trait ManagedConsumer: Send {
    async fn start(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>>;
}

#[cfg(feature = "celerity_local_consumers")]
pub(crate) struct ManagedRedisConsumer(
    pub celerity_consumer_redis::message_consumer::RedisMessageConsumer,
);

#[cfg(feature = "celerity_local_consumers")]
#[async_trait]
impl ManagedConsumer for ManagedRedisConsumer {
    async fn start(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        use celerity_helpers::consumers::MessageConsumer;
        self.0.start().await.map_err(|e| e.to_string().into())
    }
}

#[cfg(feature = "aws_consumers")]
pub(crate) struct ManagedSqsConsumer(
    pub celerity_consumer_sqs::message_consumer::SQSMessageConsumer,
);

#[cfg(feature = "aws_consumers")]
#[async_trait]
impl ManagedConsumer for ManagedSqsConsumer {
    async fn start(&self) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
        use celerity_helpers::consumers::MessageConsumer;
        self.0.start().await.map_err(|e| e.to_string().into())
    }
}

fn map_handler_error(err: ConsumerEventHandlerError) -> MessageHandlerError {
    let span = tracing::Span::current();
    span.record("otel.status_code", "ERROR");

    match err {
        ConsumerEventHandlerError::Timeout => MessageHandlerError::HandlerFailure(Box::new(err)),
        ConsumerEventHandlerError::HandlerFailure(_) => {
            MessageHandlerError::HandlerFailure(Box::new(err))
        }
        ConsumerEventHandlerError::MissingHandler => MessageHandlerError::MissingHandler,
        ConsumerEventHandlerError::ChannelClosed => {
            MessageHandlerError::HandlerFailure(Box::new(err))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::EventResultData;
    use std::sync::atomic::{AtomicBool, Ordering};

    fn success_event_result(event_id: &str) -> EventResult {
        // EventResultData uses #[serde(untagged)], so deserialize the inner struct directly.
        let data: EventResultData = serde_json::from_value(json!({"success": true})).unwrap();
        EventResult {
            event_id: event_id.to_string(),
            data,
            context: None,
        }
    }

    struct MockConsumerEventHandler {
        consumer_called: Arc<AtomicBool>,
        schedule_called: Arc<AtomicBool>,
    }

    impl MockConsumerEventHandler {
        fn new() -> (Self, Arc<AtomicBool>, Arc<AtomicBool>) {
            let consumer_called = Arc::new(AtomicBool::new(false));
            let schedule_called = Arc::new(AtomicBool::new(false));
            (
                Self {
                    consumer_called: consumer_called.clone(),
                    schedule_called: schedule_called.clone(),
                },
                consumer_called,
                schedule_called,
            )
        }
    }

    #[async_trait]
    impl ConsumerEventHandler for MockConsumerEventHandler {
        async fn handle_consumer_event(
            &self,
            _handler_tag: &str,
            _event_data: ConsumerEventData,
        ) -> Result<EventResult, ConsumerEventHandlerError> {
            self.consumer_called.store(true, Ordering::SeqCst);
            Ok(success_event_result("test"))
        }

        async fn handle_schedule_event(
            &self,
            _handler_tag: &str,
            _event_data: ScheduleEventData,
        ) -> Result<EventResult, ConsumerEventHandlerError> {
            self.schedule_called.store(true, Ordering::SeqCst);
            Ok(success_event_result("test"))
        }
    }

    // A simple metadata type for testing (avoids needing feature flags).
    #[derive(Debug, Clone, Default)]
    struct TestMetadata {
        timestamp: u64,
    }

    impl ToConsumerEventData for Message<TestMetadata> {
        fn to_consumer_messages(&self, source: &str) -> Vec<ConsumerMessage> {
            let (source_type, source_name) = parse_source(source);
            vec![ConsumerMessage {
                message_id: self.message_id.clone(),
                body: self.body.clone().unwrap_or_default(),
                source: source.to_string(),
                source_type,
                source_name,
                event_type: None,
                event_name: None,
                message_attributes: json!({}),
                vendor: json!({ "timestamp": self.metadata.timestamp }),
            }]
        }

        fn to_vendor_json(&self) -> Value {
            json!({ "timestamp": self.metadata.timestamp })
        }
    }

    fn test_message() -> Message<TestMetadata> {
        Message {
            message_id: "msg-1".to_string(),
            body: Some(r#"{"data":"test"}"#.to_string()),
            md5_of_body: None,
            metadata: TestMetadata { timestamp: 1000 },
            trace_context: None,
        }
    }

    #[tokio::test]
    async fn test_bridge_single_message() {
        let (handler, consumer_called, _schedule_called) = MockConsumerEventHandler::new();
        let bridge = ConsumerHandlerBridge::<TestMetadata>::new(
            Arc::new(handler),
            "source::queue1::handler1".to_string(),
            "queue1".to_string(),
            "aws".to_string(),
        );

        let msg = test_message();
        let result = bridge.handle(&msg).await;
        assert!(result.is_ok());
        assert!(consumer_called.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_bridge_batch() {
        let (handler, consumer_called, _) = MockConsumerEventHandler::new();
        let bridge = ConsumerHandlerBridge::<TestMetadata>::new(
            Arc::new(handler),
            "source::queue1::handler1".to_string(),
            "queue1".to_string(),
            "aws".to_string(),
        );

        let messages = vec![test_message(), test_message()];
        let result = bridge.handle_batch(&messages).await;
        assert!(result.is_ok());
        assert!(consumer_called.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_schedule_bridge() {
        let (handler, _consumer_called, schedule_called) = MockConsumerEventHandler::new();
        let bridge = ScheduleHandlerBridge::<TestMetadata>::new(
            Arc::new(handler),
            "source::sched1::handler1".to_string(),
            "sched1".to_string(),
            "rate(5 minutes)".to_string(),
            Some(json!({"task": "cleanup"})),
        );

        let msg = test_message();
        let result = bridge.handle(&msg).await;
        assert!(result.is_ok());
        assert!(schedule_called.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_schedule_bridge_calls_handle_schedule_event() {
        let (handler, consumer_called, schedule_called) = MockConsumerEventHandler::new();
        let bridge = ScheduleHandlerBridge::<TestMetadata>::new(
            Arc::new(handler),
            "source::sched1::handler1".to_string(),
            "sched1".to_string(),
            "rate(1 hour)".to_string(),
            None,
        );

        let msg = test_message();
        bridge.handle(&msg).await.unwrap();

        assert!(schedule_called.load(Ordering::SeqCst));
        assert!(!consumer_called.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_event_queue_handler() {
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let handler = EventQueueConsumerEventHandler::new(event_queue.clone());

        let event_data = ConsumerEventData {
            messages: vec![ConsumerMessage {
                message_id: "msg-1".to_string(),
                body: "test body".to_string(),
                source: "queue1".to_string(),
                source_type: None,
                source_name: None,
                event_type: None,
                event_name: None,
                message_attributes: json!({}),
                vendor: json!({}),
            }],
            vendor: json!({}),
        };

        // Spawn the handler call and then send a result from the "processing" side.
        let handle = tokio::spawn(async move {
            handler
                .handle_consumer_event("source::queue1::handler1", event_data)
                .await
        });

        // Wait for the event to appear in the queue.
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        let (tx, event) = {
            let mut queue = event_queue.lock().unwrap();
            queue.pop_front().expect("event should be in queue")
        };
        let result = success_event_result(&event.id);
        let result_event_id = result.event_id.clone();
        tx.send((event, result)).unwrap();

        let handler_result = handle.await.unwrap();
        assert!(handler_result.is_ok());
        assert_eq!(handler_result.unwrap().event_id, result_event_id);
    }

    #[tokio::test]
    async fn test_event_queue_handler_schedule() {
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let handler = EventQueueConsumerEventHandler::new(event_queue.clone());

        let event_data = ScheduleEventData {
            schedule_id: "sched1".to_string(),
            message_id: "msg-1".to_string(),
            schedule: "rate(5 minutes)".to_string(),
            input: Some(json!({"task": "cleanup"})),
            vendor: json!({}),
        };

        let handle = tokio::spawn(async move {
            handler
                .handle_schedule_event("source::sched1::handler1", event_data)
                .await
        });

        tokio::time::sleep(std::time::Duration::from_millis(50)).await;
        let (tx, event) = {
            let mut queue = event_queue.lock().unwrap();
            queue.pop_front().expect("event should be in queue")
        };
        assert_eq!(event.event_type, EventType::ScheduleMessage);

        let result = success_event_result(&event.id);
        tx.send((event, result)).unwrap();

        let handler_result = handle.await.unwrap();
        assert!(handler_result.is_ok());
    }

    #[tokio::test]
    async fn test_shared_handler_missing() {
        let shared = SharedConsumerEventHandler::new();
        let result = shared
            .handle_consumer_event(
                "tag",
                ConsumerEventData {
                    messages: vec![],
                    vendor: json!({}),
                },
            )
            .await;
        assert!(matches!(
            result,
            Err(ConsumerEventHandlerError::MissingHandler)
        ));
    }

    #[tokio::test]
    async fn test_shared_handler_set() {
        let shared = SharedConsumerEventHandler::new();
        let (handler, consumer_called, _) = MockConsumerEventHandler::new();
        shared.set(Arc::new(handler));

        let result = shared
            .handle_consumer_event(
                "tag",
                ConsumerEventData {
                    messages: vec![],
                    vendor: json!({}),
                },
            )
            .await;
        assert!(result.is_ok());
        assert!(consumer_called.load(Ordering::SeqCst));
    }

    #[cfg(feature = "celerity_local_consumers")]
    mod redis_consumer_event_data_tests {
        use super::*;
        use celerity_consumer_redis::types::{RedisMessageMetadata, RedisMessageType};

        #[test]
        fn test_topic_envelope_fields_populate_message_attributes() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "1709740800123-0".to_string(),
                body: Some(r#"{"orderId":"123"}"#.to_string()),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1709740800,
                    message_type: RedisMessageType::Text,
                    source_message_id: Some("550e8400-e29b-41d4-a716-446655440000".to_string()),
                    subject: Some("OrderCreated".to_string()),
                    attributes: Some(r#"{"env":"prod","region":"us-east-1"}"#.to_string()),
                    ..Default::default()
                },
                trace_context: None,
            };

            let consumer_messages = msg.to_consumer_messages("orders-stream");
            assert_eq!(consumer_messages.len(), 1);

            let cm = &consumer_messages[0];
            // messageId remains the Redis stream ID
            assert_eq!(cm.message_id, "1709740800123-0");
            assert_eq!(cm.body, r#"{"orderId":"123"}"#);
            assert_eq!(cm.source, "orders-stream");

            // sourceMessageId from topic envelope
            assert_eq!(
                cm.message_attributes["sourceMessageId"]["stringValue"],
                "550e8400-e29b-41d4-a716-446655440000"
            );
            assert_eq!(
                cm.message_attributes["sourceMessageId"]["dataType"],
                "String"
            );

            // subject from topic envelope
            assert_eq!(
                cm.message_attributes["subject"]["stringValue"],
                "OrderCreated"
            );

            // user-defined attributes from topic envelope
            assert_eq!(cm.message_attributes["env"]["stringValue"], "prod");
            assert_eq!(cm.message_attributes["region"]["stringValue"], "us-east-1");
        }

        #[test]
        fn test_plain_message_has_empty_attributes() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "0-1".to_string(),
                body: Some("hello".to_string()),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1000,
                    message_type: RedisMessageType::Text,
                    ..Default::default()
                },
                trace_context: None,
            };

            let consumer_messages = msg.to_consumer_messages("queue1");
            let cm = &consumer_messages[0];
            assert_eq!(cm.message_attributes, json!({}));
        }

        #[test]
        fn test_datastore_event_sets_event_type_and_transforms_body() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "1709740800456-0".to_string(),
                body: Some(
                    r#"{"Keys":{"id":{"S":"123"}},"NewImage":{"id":{"S":"123"},"name":{"S":"John"},"age":{"N":"30"}}}"#
                        .to_string(),
                ),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1709740800,
                    message_type: RedisMessageType::Text,
                    event_name: Some("INSERT".to_string()),
                    ..Default::default()
                },
                trace_context: None,
            };

            let raw = msg.to_consumer_messages("celerity:datastore:orders");
            let consumer_messages = apply_body_transforms(raw, "aws");
            assert_eq!(consumer_messages.len(), 1);

            let cm = &consumer_messages[0];
            assert_eq!(cm.source_type.as_deref(), Some("datastore"));
            assert_eq!(cm.source_name.as_deref(), Some("orders"));
            assert_eq!(cm.event_type.as_deref(), Some("inserted"));

            // Body should be Celerity-standard shape with unmarshalled attributes
            let body: Value = serde_json::from_str(&cm.body).unwrap();
            assert_eq!(body["keys"]["id"], "123");
            assert_eq!(body["newItem"]["name"], "John");
            assert_eq!(body["newItem"]["age"], 30);

            // Original body preserved in vendor
            assert!(cm.vendor.get("originalBody").is_some());

            // eventName still in message attributes
            assert_eq!(cm.message_attributes["eventName"]["stringValue"], "INSERT");
        }

        #[test]
        fn test_bucket_event_sets_event_type_and_transforms_body() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "1709740800789-0".to_string(),
                body: Some(
                    r#"{"Records":[{"s3":{"bucket":{"name":"uploads"},"object":{"key":"photo.jpg","size":1024,"eTag":"abc123"}}}]}"#
                        .to_string(),
                ),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1709740800,
                    message_type: RedisMessageType::Text,
                    event_name: Some("s3:ObjectCreated:Put".to_string()),
                    ..Default::default()
                },
                trace_context: None,
            };

            let raw = msg.to_consumer_messages("celerity:bucket:uploads");
            let consumer_messages = apply_body_transforms(raw, "aws");
            assert_eq!(consumer_messages.len(), 1);

            let cm = &consumer_messages[0];
            assert_eq!(cm.source_type.as_deref(), Some("bucket"));
            assert_eq!(cm.source_name.as_deref(), Some("uploads"));
            assert_eq!(cm.event_type.as_deref(), Some("created"));

            // Body should be Celerity-standard shape
            let body: Value = serde_json::from_str(&cm.body).unwrap();
            assert_eq!(body["key"], "photo.jpg");
            assert_eq!(body["size"], 1024);
            assert_eq!(body["eTag"], "abc123");

            // Original body preserved in vendor
            assert!(cm.vendor.get("originalBody").is_some());

            // eventName still in message attributes
            assert_eq!(
                cm.message_attributes["eventName"]["stringValue"],
                "s3:ObjectCreated:Put"
            );
        }

        #[test]
        fn test_event_name_absent_when_not_set() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "0-1".to_string(),
                body: Some("hello".to_string()),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1000,
                    message_type: RedisMessageType::Text,
                    ..Default::default()
                },
                trace_context: None,
            };

            let consumer_messages = msg.to_consumer_messages("queue1");
            let cm = &consumer_messages[0];
            assert!(cm.message_attributes.get("eventName").is_none());
            assert!(cm.source_type.is_none());
            assert!(cm.source_name.is_none());
            assert!(cm.event_type.is_none());
        }

        #[test]
        fn test_plain_celerity_source_sets_source_type_and_name() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "0-1".to_string(),
                body: Some("hello".to_string()),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1000,
                    message_type: RedisMessageType::Text,
                    ..Default::default()
                },
                trace_context: None,
            };

            let consumer_messages = msg.to_consumer_messages("celerity:queue:my-queue");
            let cm = &consumer_messages[0];
            assert_eq!(cm.source_type.as_deref(), Some("queue"));
            assert_eq!(cm.source_name.as_deref(), Some("my-queue"));
            assert!(cm.event_type.is_none());
            // Body unchanged for queue sources
            assert_eq!(cm.body, "hello");
        }

        #[test]
        fn test_datastore_modify_event_with_old_and_new_image() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "0-1".to_string(),
                body: Some(
                    r#"{"Keys":{"userId":{"S":"u1"}},"OldImage":{"userId":{"S":"u1"},"age":{"N":"29"}},"NewImage":{"userId":{"S":"u1"},"age":{"N":"30"}}}"#
                        .to_string(),
                ),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1000,
                    message_type: RedisMessageType::Text,
                    event_name: Some("MODIFY".to_string()),
                    ..Default::default()
                },
                trace_context: None,
            };

            let raw = msg.to_consumer_messages("celerity:datastore:users");
            let consumer_messages = apply_body_transforms(raw, "aws");
            let cm = &consumer_messages[0];
            assert_eq!(cm.event_type.as_deref(), Some("modified"));

            let body: Value = serde_json::from_str(&cm.body).unwrap();
            assert_eq!(body["keys"]["userId"], "u1");
            assert_eq!(body["oldItem"]["age"], 29);
            assert_eq!(body["newItem"]["age"], 30);
        }

        #[test]
        fn test_bucket_delete_event() {
            let msg: Message<RedisMessageMetadata> = Message {
                message_id: "0-1".to_string(),
                body: Some(
                    r#"{"Records":[{"s3":{"bucket":{"name":"uploads"},"object":{"key":"photo.jpg"}}}]}"#
                        .to_string(),
                ),
                md5_of_body: None,
                metadata: RedisMessageMetadata {
                    timestamp: 1000,
                    message_type: RedisMessageType::Text,
                    event_name: Some("s3:ObjectRemoved:Delete".to_string()),
                    ..Default::default()
                },
                trace_context: None,
            };

            let raw = msg.to_consumer_messages("celerity:bucket:uploads");
            let consumer_messages = apply_body_transforms(raw, "aws");
            let cm = &consumer_messages[0];
            assert_eq!(cm.event_type.as_deref(), Some("deleted"));

            let body: Value = serde_json::from_str(&cm.body).unwrap();
            assert_eq!(body["key"], "photo.jpg");
            assert!(body.get("size").is_none());
            assert!(body.get("eTag").is_none());
        }
    }

    mod helper_function_tests {
        use super::*;

        #[test]
        fn test_parse_source_celerity_format() {
            let (st, sn) = parse_source("celerity:bucket:uploads");
            assert_eq!(st.as_deref(), Some("bucket"));
            assert_eq!(sn.as_deref(), Some("uploads"));
        }

        #[test]
        fn test_parse_source_with_colons_in_name() {
            let (st, sn) = parse_source("celerity:datastore:my:special:table");
            assert_eq!(st.as_deref(), Some("datastore"));
            assert_eq!(sn.as_deref(), Some("my:special:table"));
        }

        #[test]
        fn test_parse_source_non_celerity() {
            let (st, sn) = parse_source("my-queue");
            assert!(st.is_none());
            assert!(sn.is_none());
        }

        #[test]
        fn test_parse_source_wrong_prefix() {
            let (st, sn) = parse_source("other:bucket:uploads");
            assert!(st.is_none());
            assert!(sn.is_none());
        }

        // Transform, event type mapping, and unmarshalling tests are in
        // `crate::body_transform::aws_s3` and `crate::body_transform::aws_dynamodb`.
    }
}
