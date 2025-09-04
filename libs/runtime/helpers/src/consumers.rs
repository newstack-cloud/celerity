use async_trait::async_trait;
use futures::future::join_all;
use serde_json::Value;
use std::{
    collections::HashMap,
    error::Error,
    fmt::{self, Debug, Display},
    future::Future,
    pin::Pin,
    sync::Arc,
};
use tokio::time::error::Elapsed;
use tracing::{debug, info_span, Instrument};

use crate::telemetry::CELERITY_CONTEXT_ID_KEY;

/// Provides a trait for a message consumer
/// that listens for messages on a queue
/// or message broker and fires registered
/// event handlers.
#[async_trait]
pub trait MessageConsumer<Metadata: Debug> {
    type Error;

    /// Registers the handler to process 1 or more messages at a time.
    /// This handler will be called when a batch of messages are returned
    /// by the message broker or queue in each request to fetch messages
    /// from the consumer.
    fn register_handler(&mut self, handler: Arc<dyn MessageHandler<Metadata> + Send + Sync>);

    /// Starts the message consumer and listens for messages on the queue
    /// or message broker.
    async fn start(&self) -> Result<(), Self::Error>;
}

/// A pinned future that can be used to handle a message or batch of messages.
pub type PinnedMessageHandlerFuture<'a> =
    Pin<Box<dyn Future<Output = Result<(), MessageHandlerError>> + Send + 'a>>;

/// A message that has been received from a message service.
#[derive(Debug, Clone)]
pub struct Message<Metadata: Debug> {
    /// A unique identifier for the message.
    pub message_id: String,
    /// The contents of the message.
    pub body: Option<String>,
    /// An MD5 digest of the message body string,
    /// can be used to verify that the original message
    /// was not corrupted.
    /// When set, this is expected to be computed by the sender
    /// or the message service (e.g. Amazon SQS)
    /// the message was received from.
    pub md5_of_body: Option<String>,
    /// Additional metadata about the message,
    /// this will often have information specific
    /// to the message service used to deliver the message.
    pub metadata: Metadata,
    /// A map of trace context values,
    /// this is used to trace messages across async
    /// message passing boundaries. (e.g. Service A -> SNS -> SQS -> Service B)
    pub trace_context: Option<HashMap<String, String>>,
}

#[async_trait]
pub trait MessageHandler<Metadata: Debug + Clone> {
    async fn handle(&self, message: &Message<Metadata>) -> Result<(), MessageHandlerError>;
    async fn handle_batch(&self, messages: &[Message<Metadata>])
        -> Result<(), MessageHandlerError>;
}

impl<Metadata: Debug> Debug for dyn MessageHandler<Message<Metadata>> + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "MessageHandler<{}>", std::any::type_name::<Metadata>())
    }
}

#[derive(Debug)]
pub struct PartialBatchFailureInfo {
    pub message_id: String,
    pub error_reason: String,
    pub retry_count: u64,
}

impl PartialBatchFailureInfo {
    pub fn new(message_id: String, error_reason: String, retry_count: u64) -> Self {
        Self {
            message_id,
            error_reason,
            retry_count,
        }
    }
}

// Provides a custom error type to be used for failures
// within message handlers.
#[derive(Debug)]
pub enum MessageHandlerError {
    MissingHandler,
    Timeout(Elapsed),
    HandlerFailure(Box<dyn Error + Send + Sync + 'static>),
    PartialBatchFailure(Vec<PartialBatchFailureInfo>),
}

impl fmt::Display for MessageHandlerError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            MessageHandlerError::MissingHandler => write!(
                f,
                "message handler failed: a handler must be registered to process messages"
            ),
            MessageHandlerError::Timeout(elapsed_error) => {
                write!(f, "message handler failed: timeout {elapsed_error}")
            }
            MessageHandlerError::HandlerFailure(handler_error) => {
                write!(f, "message handler failed: {handler_error}")
            }
            MessageHandlerError::PartialBatchFailure(partial_batch_failure_info) => {
                write!(f, "message handler failed: {partial_batch_failure_info:?}")
            }
        }
    }
}

/// A message that has been received from a message service and has been routed
/// to a specific handler. This is for JSON messages that contain a route key
/// that can be used to route the message to a specific handler for processing
/// application-level events.
#[derive(Debug, Clone)]
pub struct RoutedMessage<Metadata: Debug + Clone> {
    /// A unique identifier for the message.
    pub message_id: String,
    /// The route value that was used to route the message to this handler.
    pub route: String,
    /// The parsed contents of the message.
    pub body: Value,
    /// Additional metadata about the message,
    /// this will often have information specific
    /// to the message service used to deliver the message.
    pub metadata: Metadata,
    /// A map of trace context values,
    /// this is used to trace messages across async
    /// message passing boundaries. (e.g. Service A -> SNS -> SQS -> Service B)
    pub trace_context: Option<HashMap<String, String>>,
}

impl<Metadata: Debug + Clone> RoutedMessage<Metadata> {
    /// Create a new routed message from an original message from a message service
    /// and the parsed JSON message body object.
    pub fn from_message_parts(message: &Message<Metadata>, route: &str, object: &Value) -> Self {
        Self {
            message_id: message.message_id.clone(),
            route: route.to_string(),
            body: object.clone(),
            metadata: message.metadata.clone(),
            trace_context: message.trace_context.clone(),
        }
    }
}

/// A message handler that can be used to handle routed messages.
/// This should be implemented by language-specific bindings to allow
/// application developers to implement handlers for routed messages
/// as well as raw messages received from a message service.
#[async_trait]
pub trait RoutedMessageHandler<Metadata: Debug + Clone> {
    /// Handle a routed message.
    async fn handle(&self, message: &RoutedMessage<Metadata>) -> Result<(), MessageHandlerError>;
    /// Handle a raw message received from a message service.
    async fn handle_raw_message(
        &self,
        message: &Message<Metadata>,
    ) -> Result<(), MessageHandlerError>;
    /// Handle a batch of raw messages received from a message service.
    async fn handle_raw_message_batch(
        &self,
        messages: &[Message<Metadata>],
    ) -> Result<(), MessageHandlerError>;
}

impl<Metadata: Debug> Debug for dyn RoutedMessageHandler<Message<Metadata>> + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(
            f,
            "RoutedMessageHandler<{}>",
            std::any::type_name::<Metadata>()
        )
    }
}

/// An implementation of a message handler that can be used with a message consumer
/// to route messages to appropriate handlers based on routing keys in the message body,
/// when the message body is a JSON payload.
/// Routing is skipped if there are no routes registered, which is the default
/// to allow this implementation to be used for passing through the original messages.
pub struct MessageHandlerWithRouter<Metadata: Debug> {
    routes: HashMap<String, Arc<dyn RoutedMessageHandler<Metadata> + Send + Sync>>,
    // The key in the JSON message body object that contains the route value.
    route_key: String,
    // The default route value to use if there is no match for the route key value
    // in the message body.
    // This is tried before sending the original message(s) to the fallback handler.
    default_route_value: Option<String>,
    // A fallback handler that will be used if no routes are registered,
    // the `handle_raw_message` or `handle_raw_message_batch`
    // method will be called with the original message.
    fallback_handler: Arc<dyn RoutedMessageHandler<Metadata> + Send + Sync>,
}

impl<Metadata: Debug + Clone> MessageHandlerWithRouter<Metadata> {
    /// Create a new message handler with a fallback handler.
    ///
    /// # Arguments
    ///
    /// * `route_key` - The key in the JSON message body object that contains the route value, defaults to `event`.
    /// * `default_route_value` - The default route value to use if there is no match for the route key value
    ///   in the message body.
    /// * `fallback_handler` - A fallback handler that will be used if no routes are registered
    ///   or a default route value is not set. The `handle_raw_message` or `handle_raw_message_batch`
    ///   method will be called with the original message.
    pub fn new(
        route_key: Option<String>,
        default_route_value: Option<String>,
        fallback_handler: Arc<dyn RoutedMessageHandler<Metadata> + Send + Sync>,
    ) -> Self {
        Self {
            routes: HashMap::new(),
            route_key: route_key.unwrap_or_else(|| "event".to_string()),
            default_route_value,
            fallback_handler,
        }
    }

    pub fn register_route(
        &mut self,
        route: String,
        handler: Arc<dyn RoutedMessageHandler<Metadata> + Send + Sync>,
    ) {
        self.routes.insert(route, handler);
    }

    fn match_route(
        &self,
        message: &Message<Metadata>,
        object: Value,
        route_key: &str,
    ) -> Option<(
        Arc<dyn RoutedMessageHandler<Metadata> + Send + Sync>,
        RoutedMessage<Metadata>,
    )> {
        if let Some(Value::String(route)) = object.get(route_key) {
            if let Some(handler) = self.routes.get(route) {
                debug!("matched on route \"{route_key}={route}\"");
                let routed_message = RoutedMessage::from_message_parts(message, route, &object);
                return Some((handler.clone(), routed_message));
            }
        }

        if let Some(default_route_value) = &self.default_route_value {
            if let Some(handler) = self.routes.get(default_route_value) {
                debug!("matched on default route \"{route_key}={default_route_value}\"");
                let routed_message =
                    RoutedMessage::from_message_parts(message, default_route_value, &object);
                return Some((handler.clone(), routed_message));
            } else {
                debug!("no handler found for default route \"{route_key}={default_route_value}\"");
                return None;
            }
        }

        debug!(
            "route key \"{route_key}\" not found in message JSON object \
            and there is no default route"
        );
        None
    }
}

#[async_trait]
impl<Metadata: Debug + Clone + Send + Sync> MessageHandler<Metadata>
    for MessageHandlerWithRouter<Metadata>
{
    async fn handle(&self, message: &Message<Metadata>) -> Result<(), MessageHandlerError> {
        if self.routes.is_empty() {
            return self.fallback_handler.handle_raw_message(message).await;
        }

        let route_key = &self.route_key;
        let match_result_opt = match serde_json::from_str::<Value>(
            message.body.as_deref().unwrap_or_default(),
        ) {
            Ok(Value::Object(object)) => {
                self.match_route(message, Value::Object(object), route_key)
            }
            Ok(_) => {
                debug!("message body is not a JSON object, original message will be passed to raw message handler");
                None
            }
            Err(e) => {
                debug!("failed to parse message body as JSON, original message will be passed to raw message handler: {e}");
                None
            }
        };

        if let Some((handler, routed_message)) = match_result_opt {
            handler
                .handle(&routed_message)
                .instrument(info_span!(
                    "routed_message_handler",
                    route = routed_message.route,
                    message_id = routed_message.message_id,
                ))
                .await
        } else {
            self.fallback_handler.handle_raw_message(message).await
        }
    }

    async fn handle_batch(
        &self,
        messages: &[Message<Metadata>],
    ) -> Result<(), MessageHandlerError> {
        if self.routes.is_empty() {
            return self
                .fallback_handler
                .handle_raw_message_batch(messages)
                .await;
        }

        let mut futures = Vec::new();
        for message in messages {
            // When routing is enabled, handle each message in the batch
            // individually to route each message to the appropriate handler.
            futures.push(self.handle(message));
        }

        let results = join_all(futures).await;

        let mut errors = Vec::new();
        for result in results.into_iter() {
            match result {
                Ok(()) => debug!("message handler finished successfully"),
                Err(err) => {
                    debug!("message handler failed: {err}");
                    errors.push(err.to_string());
                }
            }
        }

        if !errors.is_empty() {
            return Err(MessageHandlerError::HandlerFailure(Box::new(
                RoutedMessageHandlerBatchError::new(format!("message handlers failed: {errors:?}")),
            )));
        }

        Ok(())
    }
}

/// Extracts the context IDs from the trace context of the messages.
pub fn extract_context_ids<Metadata: Debug>(messages: &[Message<Metadata>]) -> Vec<String> {
    messages
        .iter()
        .filter_map(|m| {
            m.trace_context
                .as_ref()
                .and_then(|tc| tc.get(CELERITY_CONTEXT_ID_KEY).cloned())
        })
        .collect()
}

#[derive(Debug)]
pub struct RoutedMessageHandlerBatchError {
    message: String,
}

impl RoutedMessageHandlerBatchError {
    pub fn new(message: String) -> Self {
        Self { message }
    }
}

impl Display for RoutedMessageHandlerBatchError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Routed message handler batch error: {}", self.message)
    }
}

impl Error for RoutedMessageHandlerBatchError {}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use async_trait::async_trait;
    use tokio::{select, sync::mpsc};

    use super::*;

    const ERROR_ROUTE: &str = "error_route";
    const ERROR_MESSAGE_ID: &str = "error_message_id";

    #[derive(Debug)]
    struct TestRouteError {
        message: String,
    }

    impl TestRouteError {
        pub fn new(message: String) -> Self {
            Self { message }
        }
    }

    impl Display for TestRouteError {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            write!(f, "Test route error: {}", self.message)
        }
    }

    impl Error for TestRouteError {}

    struct TestMessageHandler {
        sender: mpsc::Sender<String>,
    }

    impl TestMessageHandler {
        pub fn new(sender: mpsc::Sender<String>) -> Self {
            Self { sender }
        }
    }

    #[async_trait]
    impl RoutedMessageHandler<()> for TestMessageHandler {
        async fn handle(&self, message: &RoutedMessage<()>) -> Result<(), MessageHandlerError> {
            if message.route == ERROR_ROUTE {
                return Err(MessageHandlerError::HandlerFailure(Box::new(
                    TestRouteError::new("an unexpected error occurred".to_string()),
                )));
            }

            self.sender
                .send(message.message_id.clone())
                .await
                .map_err(|err| MessageHandlerError::HandlerFailure(Box::new(err)))?;

            Ok(())
        }

        async fn handle_raw_message(
            &self,
            message: &Message<()>,
        ) -> Result<(), MessageHandlerError> {
            if message.message_id == ERROR_MESSAGE_ID {
                return Err(MessageHandlerError::HandlerFailure(Box::new(
                    TestRouteError::new("an unexpected error occurred".to_string()),
                )));
            }

            // "raw:" prefix is used to indicate that the message was passed through
            // the fallback handler and not routed to a specific handler.
            let prefixed_message_id = format!("raw:{}", message.message_id);
            self.sender
                .send(prefixed_message_id)
                .await
                .map_err(|err| MessageHandlerError::HandlerFailure(Box::new(err)))?;

            Ok(())
        }

        async fn handle_raw_message_batch(
            &self,
            messages: &[Message<()>],
        ) -> Result<(), MessageHandlerError> {
            if messages.iter().any(|m| m.message_id == ERROR_MESSAGE_ID) {
                return Err(MessageHandlerError::HandlerFailure(Box::new(
                    TestRouteError::new("an unexpected error occurred".to_string()),
                )));
            }

            for message in messages {
                let prefixed_message_id = format!("raw:{}", message.message_id);
                self.sender
                    .send(prefixed_message_id)
                    .await
                    .map_err(|err| MessageHandlerError::HandlerFailure(Box::new(err)))?;
            }

            Ok(())
        }
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_for_a_message_with_a_route() {
        let (tx, mut rx) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let message = Message {
            message_id: "test-message-1".to_string(),
            body: Some("{\"event\": \"test_route\"}".to_string()),
            md5_of_body: None,
            metadata: (),
            trace_context: None,
        };

        let mut router =
            MessageHandlerWithRouter::new(Some("event".to_string()), None, handler.clone());
        router.register_route("test_route".to_string(), handler);

        router.handle(&message).await.unwrap();

        let result = select! {
            result = rx.recv() => result,
            _ = tokio::time::sleep(Duration::from_secs(1)) => panic!("timeout waiting for message"),
        };

        assert_eq!(result, Some("test-message-1".to_string()));
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_for_a_batch_of_messages_with_routes() {
        let (tx, mut rx) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let mut messages = Vec::new();
        for i in 0..5 {
            messages.push(Message {
                message_id: format!("test-message-{i}"),
                body: Some("{\"event\": \"test_route\"}".to_string()),
                md5_of_body: None,
                metadata: (),
                trace_context: None,
            });
        }

        let mut router =
            MessageHandlerWithRouter::new(Some("event".to_string()), None, handler.clone());
        router.register_route("test_route".to_string(), handler);

        router.handle_batch(&messages).await.unwrap();

        let mut collected = Vec::new();
        for _ in 0..5 {
            let result = select! {
            result = rx.recv() => result,
                _ = tokio::time::sleep(Duration::from_secs(1)) => panic!("timeout waiting for message"),
            };
            collected.push(result);
        }

        assert_eq!(
            collected,
            messages
                .iter()
                .map(|m| Some(m.message_id.clone()))
                .collect::<Vec<_>>()
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_uses_default_route() {
        let (tx, mut rx) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let message = Message {
            message_id: "test-message-1".to_string(),
            body: Some("{\"event\": \"other_route\"}".to_string()),
            md5_of_body: None,
            metadata: (),
            trace_context: None,
        };

        let mut router = MessageHandlerWithRouter::new(
            Some("event".to_string()),
            Some("default_route".to_string()),
            handler.clone(),
        );
        router.register_route("default_route".to_string(), handler.clone());

        router.handle(&message).await.unwrap();

        let result = select! {
            result = rx.recv() => result,
            _ = tokio::time::sleep(Duration::from_secs(1)) => panic!("timeout waiting for message"),
        };

        assert_eq!(result, Some("test-message-1".to_string()));
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_uses_fallback_handler() {
        let (tx, mut rx) = mpsc::channel(10);
        let handler = TestMessageHandler::new(tx);

        let message = Message {
            message_id: "test-message-1".to_string(),
            // A message that does not support routing.
            body: Some("{\"id\": \"30492\"}".to_string()),
            md5_of_body: None,
            metadata: (),
            trace_context: None,
        };

        let router =
            MessageHandlerWithRouter::new(Some("event".to_string()), None, Arc::new(handler));

        router.handle(&message).await.unwrap();

        let result = select! {
            result = rx.recv() => result,
            _ = tokio::time::sleep(Duration::from_secs(1)) => panic!("timeout waiting for message"),
        };

        assert_eq!(result, Some("raw:test-message-1".to_string()));
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_returns_expected_error_for_failed_route_handler() {
        let (tx, _) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let message = Message {
            message_id: "test-message-1".to_string(),
            body: Some(format!("{{ \"event\": \"{ERROR_ROUTE}\" }}",)),
            md5_of_body: None,
            metadata: (),
            trace_context: None,
        };

        let mut router =
            MessageHandlerWithRouter::new(Some("event".to_string()), None, handler.clone());
        router.register_route(ERROR_ROUTE.to_string(), handler);

        let result = router.handle(&message).await;

        assert!(result.is_err());
        assert!(matches!(
            result,
            Err(MessageHandlerError::HandlerFailure(_))
        ));
        assert_eq!(
            result.unwrap_err().to_string(),
            "message handler failed: Test route error: \
                an unexpected error occurred"
                .to_string(),
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_returns_expected_error_for_failed_fallback_handler() {
        let (tx, _) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let message = Message {
            message_id: ERROR_MESSAGE_ID.to_string(),
            // A message that does not support routing.
            body: Some("{\"id\": \"30492\"}".to_string()),
            md5_of_body: None,
            metadata: (),
            trace_context: None,
        };

        let router = MessageHandlerWithRouter::new(Some("event".to_string()), None, handler);

        let result = router.handle(&message).await;

        assert!(result.is_err());
        assert!(matches!(
            result,
            Err(MessageHandlerError::HandlerFailure(_))
        ));
        assert_eq!(
            result.unwrap_err().to_string(),
            "message handler failed: Test route error: \
                an unexpected error occurred"
                .to_string(),
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_message_handler_with_router_returns_expected_error_for_batch_with_failed_fallback_handler(
    ) {
        let (tx, _) = mpsc::channel(10);
        let handler = Arc::new(TestMessageHandler::new(tx));

        let mut messages = Vec::new();
        for i in 0..5 {
            let message_id = if i == 0 {
                ERROR_MESSAGE_ID.to_string()
            } else {
                format!("test-message-{i}")
            };
            messages.push(Message {
                message_id,
                body: Some("{\"id\": \"30492\"}".to_string()),
                md5_of_body: None,
                metadata: (),
                trace_context: None,
            });
        }

        let router = MessageHandlerWithRouter::new(Some("event".to_string()), None, handler);

        let result = router.handle_batch(&messages).await;

        assert!(result.is_err());
        assert!(matches!(
            result,
            Err(MessageHandlerError::HandlerFailure(_))
        ));
        assert_eq!(
            result.unwrap_err().to_string(),
            "message handler failed: Test route error: \
                an unexpected error occurred"
                .to_string(),
        );
    }
}
