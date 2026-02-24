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

#[cfg(feature = "celerity_local_consumers")]
impl ToConsumerEventData for Message<celerity_consumer_redis::types::RedisMessageMetadata> {
    fn to_consumer_messages(&self, source: &str) -> Vec<ConsumerMessage> {
        vec![ConsumerMessage {
            message_id: self.message_id.clone(),
            body: self.body.clone().unwrap_or_default(),
            source: source.to_string(),
            message_attributes: json!({}),
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
        vec![ConsumerMessage {
            message_id: self.message_id.clone(),
            body: self.body.clone().unwrap_or_default(),
            source: source.to_string(),
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
pub struct ConsumerHandlerBridge<M: Debug + Clone + Send + Sync> {
    event_handler: Arc<dyn ConsumerEventHandler>,
    handler_tag: String,
    source: String,
    _metadata: PhantomData<M>,
}

impl<M: Debug + Clone + Send + Sync> ConsumerHandlerBridge<M> {
    pub fn new(
        event_handler: Arc<dyn ConsumerEventHandler>,
        handler_tag: String,
        source: String,
    ) -> Self {
        Self {
            event_handler,
            handler_tag,
            source,
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
    async fn handle(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        let messages = message.to_consumer_messages(&self.source);
        let event_data = ConsumerEventData {
            messages,
            vendor: message.to_vendor_json(),
        };
        self.event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
    }

    async fn handle_batch(&self, messages: &[Message<M>]) -> Result<(), MessageHandlerError> {
        let mut all_messages = Vec::new();
        let mut vendor = json!({});
        for msg in messages {
            all_messages.extend(msg.to_consumer_messages(&self.source));
            vendor = msg.to_vendor_json();
        }
        let event_data = ConsumerEventData {
            messages: all_messages,
            vendor,
        };
        self.event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
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
    _metadata: PhantomData<M>,
}

impl<M: Debug + Clone + Send + Sync> RoutedConsumerHandlerBridge<M> {
    pub fn new(
        event_handler: Arc<dyn ConsumerEventHandler>,
        handler_tag: String,
        source: String,
    ) -> Self {
        Self {
            event_handler,
            handler_tag,
            source,
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
    async fn handle(&self, message: &RoutedMessage<M>) -> Result<(), MessageHandlerError> {
        let messages = vec![ConsumerMessage {
            message_id: message.message_id.clone(),
            body: message.body.to_string(),
            source: self.source.clone(),
            message_attributes: json!({}),
            vendor: json!({}),
        }];
        let event_data = ConsumerEventData {
            messages,
            vendor: json!({}),
        };
        self.event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
    }

    async fn handle_raw_message(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        let messages = message.to_consumer_messages(&self.source);
        let event_data = ConsumerEventData {
            messages,
            vendor: message.to_vendor_json(),
        };
        self.event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
    }

    async fn handle_raw_message_batch(
        &self,
        messages: &[Message<M>],
    ) -> Result<(), MessageHandlerError> {
        let mut all_messages = Vec::new();
        let mut vendor = json!({});
        for msg in messages {
            all_messages.extend(msg.to_consumer_messages(&self.source));
            vendor = msg.to_vendor_json();
        }
        let event_data = ConsumerEventData {
            messages: all_messages,
            vendor,
        };
        self.event_handler
            .handle_consumer_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
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
    async fn handle(&self, message: &Message<M>) -> Result<(), MessageHandlerError> {
        let event_data = ScheduleEventData {
            schedule_id: self.schedule_id.clone(),
            message_id: message.message_id.clone(),
            schedule: self.schedule_value.clone(),
            input: self.input.clone(),
            vendor: message.to_vendor_json(),
        };
        self.event_handler
            .handle_schedule_event(&self.handler_tag, event_data)
            .await
            .map(|_| ())
            .map_err(map_handler_error)
    }

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
            vec![ConsumerMessage {
                message_id: self.message_id.clone(),
                body: self.body.clone().unwrap_or_default(),
                source: source.to_string(),
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
}
