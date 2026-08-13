//! WebSocket message routing for the IPC runtime call mode.
//!
//! WebSocket messages are routed by looking the message's route up in the
//! application's route map. In the FFI call mode the SDK populates that map
//! through `Application::register_websocket_message_handler` as it binds each
//! in-process handler. In the IPC call mode there is nothing in-process to
//! bind, so without this the map stays empty and every message falls through
//! unrouted.
//!
//! This registers a handler per blueprint WebSocket handler that turns the
//! message into an event and waits for the handlers executable to process it.

use std::time::Duration;

use async_trait::async_trait;
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use tracing::{error, warn};

use crate::{
    errors::WebSocketsMessageError,
    event_queue::{admission_wait, EventQueue, EventQueueError},
    types::{
        EventData, EventDataPayload, EventOutcome, EventResultData, EventType, WebSocketEventData,
    },
    websocket::{BinaryMessageInfo, JsonMessageInfo, WebSocketMessageHandler},
};

/// Routes messages for one WebSocket route to the handlers executable.
pub struct IpcWebSocketHandler {
    event_queue: EventQueue,
    /// The tag identifying the handler that should run, as it appears on the
    /// dispatched event.
    handler_tag: String,
    /// The blueprint route this handler serves, e.g. `$connect` or a custom
    /// route value.
    route: String,
    /// The handler's configured timeout. Known when the handler is registered,
    /// so it is carried here rather than looked up per message.
    timeout: Duration,
}

impl IpcWebSocketHandler {
    pub fn new(
        event_queue: EventQueue,
        handler_tag: String,
        route: String,
        timeout: Duration,
    ) -> Self {
        IpcWebSocketHandler {
            event_queue,
            handler_tag,
            route,
            timeout,
        }
    }

    /// Puts a message on the event queue and waits for the handlers executable
    /// to report what it did with it.
    async fn dispatch(
        &self,
        connection_id: String,
        request_id: Option<String>,
        client_ip: String,
        message: String,
        is_binary: bool,
    ) -> Result<(), WebSocketsMessageError> {
        let event = EventData {
            id: nanoid::nanoid!(),
            event_type: EventType::WsMessage,
            handler_tag: self.handler_tag.clone(),
            timestamp: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs(),
            data: EventDataPayload::WsMessageEventData(WebSocketEventData {
                route: self.route.clone(),
                connection_id,
                source_ip: client_ip,
                request_id,
                message,
                is_binary,
            }),
        };

        // Waiting for queue capacity spends the same budget the handler needs,
        // so both waits are anchored to a deadline taken before admission.
        let deadline = tokio::time::Instant::now() + self.timeout;

        let result_rx = match self
            .event_queue
            .enqueue(event, admission_wait(self.timeout))
            .await
        {
            Ok(rx) => rx,
            Err(EventQueueError::QueueFull) => {
                warn!(
                    handler_tag = %self.handler_tag,
                    "shedding WebSocket message, no event queue capacity became available"
                );
                return Err(WebSocketsMessageError::UnexpectedError(
                    "the runtime is at capacity".to_string(),
                ));
            }
            Err(EventQueueError::Closed) => {
                error!(
                    handler_tag = %self.handler_tag,
                    "event queue is closed, no handlers executable is attached"
                );
                return Err(WebSocketsMessageError::UnexpectedError(
                    "handlers are not available".to_string(),
                ));
            }
        };

        // Deliberately not cancelled when the connection goes away, unlike an
        // HTTP request. A WebSocket message is closer to a queue message than
        // to a request and response as the client closing does not make the work
        // pointless, and a handler part way through persisting the message
        // should finish. The connection loop awaits this inline on its own
        // task, so a close is handled by the loop rather than dropping this.
        match tokio::time::timeout_at(deadline, result_rx).await {
            Ok(Ok(EventOutcome::Completed(_event, result))) => self.check_result(result.data),
            Ok(Ok(EventOutcome::Unservable(reason))) => {
                warn!(
                    handler_tag = %self.handler_tag,
                    %reason,
                    "shedding WebSocket message, the runtime will not dispatch it"
                );
                Err(WebSocketsMessageError::UnexpectedError(reason.to_string()))
            }
            // The sender was dropped, which happens when the cleanup task
            // removes the in-flight entry after its deadline passed, or when
            // the handlers executable went away mid-message.
            Ok(Err(_)) => Err(WebSocketsMessageError::UnexpectedError(
                "handler did not return a result".to_string(),
            )),
            Err(_) => {
                warn!(
                    handler_tag = %self.handler_tag,
                    timeout = ?self.timeout,
                    "handler did not respond within its timeout"
                );
                Err(WebSocketsMessageError::UnexpectedError(
                    "handler timed out".to_string(),
                ))
            }
        }
    }

    fn check_result(&self, data: EventResultData) -> Result<(), WebSocketsMessageError> {
        let EventResultData::WebSocketResponse(response) = data else {
            error!(
                handler_tag = %self.handler_tag,
                "handler returned a result that is not a WebSocket response"
            );
            return Err(WebSocketsMessageError::UnexpectedError(
                "handler returned an unexpected result".to_string(),
            ));
        };

        if response.success {
            return Ok(());
        }
        Err(WebSocketsMessageError::UnexpectedError(
            response
                .error_message
                .unwrap_or_else(|| "handler reported a failure".to_string()),
        ))
    }
}

#[async_trait]
impl WebSocketMessageHandler for IpcWebSocketHandler {
    async fn handle_json_message(
        &self,
        message: JsonMessageInfo,
    ) -> Result<(), WebSocketsMessageError> {
        let body = serde_json::to_string(&message.body).map_err(|err| {
            WebSocketsMessageError::UnexpectedError(format!(
                "failed to serialise the message body: {err}"
            ))
        })?;

        self.dispatch(
            message.connection_id,
            message
                .request_ctx
                .as_ref()
                .map(|ctx| ctx.request_id.clone()),
            message
                .request_ctx
                .as_ref()
                .map(|ctx| ctx.client_ip.clone())
                .unwrap_or_default(),
            body,
            false,
        )
        .await
    }

    async fn handle_binary_message<'a>(
        &self,
        message: BinaryMessageInfo<'a>,
    ) -> Result<(), WebSocketsMessageError> {
        // The event is carried as JSON, which cannot represent raw bytes, so a
        // binary frame is base64 encoded rather than being forced through a
        // lossy UTF-8 conversion that would silently corrupt it. `is_binary`
        // tells the handler to decode it. When the event moves to protobuf the
        // body becomes `bytes` and the encoding step disappears, but the flag
        // still distinguishes a binary frame from a text one.
        let body = BASE64.encode(message.body);

        self.dispatch(
            message.connection_id,
            message
                .request_ctx
                .as_ref()
                .map(|ctx| ctx.request_id.clone()),
            message
                .request_ctx
                .as_ref()
                .map(|ctx| ctx.client_ip.clone())
                .unwrap_or_default(),
            body,
            true,
        )
        .await
    }
}
