use std::{collections::HashMap, sync::Arc};

use bytes::Bytes;

use celerity_helpers::runtime_types::RuntimePlatform;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::sync::oneshot;

use crate::telemetry::RuntimeMetrics;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventData {
    pub id: String,
    #[serde(rename = "eventType")]
    pub event_type: EventType,
    #[serde(rename = "handlerTag")]
    pub handler_tag: String,
    pub timestamp: u64,
    pub data: EventDataPayload,
    #[serde(rename = "traceContext", skip_serializing_if = "Option::is_none")]
    pub trace_context: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EventType {
    #[serde(rename = "httpRequest")]
    HttpRequest,
    #[serde(rename = "wsMessage")]
    WsMessage,
    #[serde(rename = "consumerMessage")]
    ConsumerMessage,
    #[serde(rename = "scheduleMessage")]
    ScheduleMessage,
    #[serde(rename = "eventMessage")]
    EventMessage,
    #[serde(rename = "customInvoke")]
    CustomInvoke,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EventDataPayload {
    HttpRequestEventData(Box<HttpRequestEventData>),
    WsMessageEventData(WebSocketEventData),
    ConsumerMessageEventData(ConsumerEventData),
    ScheduleMessageEventData(ScheduleEventData),
    EventMessageEventData(EventMessageEventData),
    CustomInvokeEventData(CustomInvokeEventData),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HttpRequestEventData {
    pub method: String,
    pub path: String,
    pub route: String,
    /// Ordered values per name. A catch-all parameter yields one value per path
    /// segment; every other kind yields exactly one.
    #[serde(rename = "pathParams")]
    pub path_params: HashMap<String, Vec<String>>,
    #[serde(rename = "queryParams")]
    pub query_params: HashMap<String, Vec<String>>,
    /// Names are lowercase. Order within a name is significant, order across
    /// names is not.
    pub headers: HashMap<String, Vec<String>>,
    /// The request body exactly as it was received. Opaque to the runtime,
    /// which never parses it.
    pub body: Bytes,
    #[serde(rename = "sourceIp")]
    pub source_ip: String,
    #[serde(rename = "requestId")]
    pub request_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct WebSocketEventData {
    pub route: String,
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "sourceIp")]
    pub source_ip: String,
    #[serde(rename = "requestId")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub request_id: Option<String>,
    /// The id the client gave this message, as distinct from `request_id`,
    /// which the runtime assigns so that every message can be traced.
    #[serde(rename = "messageId")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message_id: Option<String>,
    /// The message exactly as it arrived.
    pub message: Bytes,
    /// Whether this arrived as a binary frame rather than a text one. Bytes
    /// alone cannot express the distinction, and the WebSocket protocol treats
    /// the two frame types as distinct.
    #[serde(rename = "isBinary")]
    #[serde(default)]
    pub is_binary: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ConsumerEventData {
    pub messages: Vec<ConsumerMessage>,
    pub vendor: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ConsumerMessage {
    #[serde(rename = "messageId")]
    pub message_id: String,
    pub body: String,
    pub source: String,
    #[serde(rename = "sourceType", skip_serializing_if = "Option::is_none")]
    pub source_type: Option<String>,
    #[serde(rename = "sourceName", skip_serializing_if = "Option::is_none")]
    pub source_name: Option<String>,
    #[serde(rename = "eventType", skip_serializing_if = "Option::is_none")]
    pub event_type: Option<String>,
    /// Raw provider event name (e.g. `"s3:ObjectCreated:Put"`, `"INSERT"`).
    /// Used internally by the handler bridge to resolve `event_type` via the
    /// provider-specific body transform module.  Not serialised to the SDK.
    #[serde(skip)]
    pub event_name: Option<String>,
    #[serde(rename = "messageAttributes")]
    pub message_attributes: Value,
    pub vendor: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ScheduleEventData {
    #[serde(rename = "scheduleId")]
    pub schedule_id: String,
    #[serde(rename = "messageId")]
    pub message_id: String,
    pub schedule: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input: Option<Value>,
    pub vendor: Value,
}

/// A handler invoked by name rather than by a request, a message or a schedule.
///
/// Reaches the runtime through the local invoke endpoint, which exists so that
/// any handler can be exercised directly while developing or testing, whatever
/// normally triggers it. The input is passed through untouched, so it is the
/// caller's job to send the shape that handler expects.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CustomInvokeEventData {
    #[serde(rename = "handlerName")]
    pub handler_name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventMessageEventData {
    pub body: String,
    pub source: String,
    #[serde(rename = "messageId")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message_id: Option<String>,
    #[serde(rename = "messageAttributes")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message_attributes: Option<Value>,
    #[serde(rename = "vendor")]
    pub vendor: Value,
}

// A tuple that contains a oneshot sender and received event data.
// The purpose of this tuple is to hand off processing of an event
// to another process or task. (That takes the event from a queue asynchronously)
// The oneshot sender allows the caller to wait to receive the outcome of processing the
// event along with the original event data to carry out any further tasks using the input
// data.
pub type EventTuple = (oneshot::Sender<EventOutcome>, EventData);

/// What the runtime hands back to whoever enqueued an event.
#[derive(Debug)]
pub enum EventOutcome {
    /// A handler ran and returned this result, along with the event it was
    /// given.
    Completed(Box<EventData>, EventResult),
    /// The runtime will not have this event handled at all.
    ///
    /// Distinct from a handler failing or timing out as nothing ever ran, and
    /// nothing will. Callers should treat it as a capacity or availability
    /// signal rather than an error in the application.
    Unservable(UnservableReason),
}

/// Why the runtime will not handle an event.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum UnservableReason {
    /// No attached handler stream serves the event's handler tag, and none
    /// attached within the grace window allowed for one to connect.
    NoHandler,
    /// The runtime is shutting down and has stopped dispatching.
    ShuttingDown,
}

impl std::fmt::Display for UnservableReason {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            UnservableReason::NoHandler => write!(f, "no handler is attached for this event"),
            UnservableReason::ShuttingDown => write!(f, "the runtime is shutting down"),
        }
    }
}

/// Why the runtime is telling a handler to stop work.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CancelReason {
    /// The event's deadline passed before a result came back.
    DeadlineExceeded,
    /// The originating caller went away, so nothing is waiting for the result.
    /// Raised when an HTTP client disconnects.
    CallerGone,
    /// The runtime is shutting down.
    Shutdown,
}

/// Asks the runtime to tell whichever handler holds an event to stop.
#[derive(Debug)]
pub struct CancelRequest {
    pub event_id: String,
    pub reason: CancelReason,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventResult {
    #[serde(rename = "eventId")]
    pub event_id: String,
    pub data: EventResultData,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub context: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(untagged)]
pub enum EventResultData {
    HttpResponse(HttpResponseData),
    WebSocketResponse(SimpleResponseData),
    MessageProcessingResponse(MessageProcessingResponseData),
    ScheduledEventResponse(ScheduledEventResponseData),
    EventResponse(SimpleResponseData),
    /// Last because this enum is untagged, so variants are tried in order. Every
    /// variant above requires either `success` or the full HTTP triple, and this
    /// one requires `output`, so none of them can shadow each other.
    CustomInvokeResponse(CustomInvokeResponseData),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CustomInvokeResponseData {
    /// Whatever the handler returned, exactly as it sent it. Opaque to the
    /// runtime, which has no reason to look inside it, so it is carried rather
    /// than decoded. Whether it has to be text is a question for whoever asked
    /// for the invocation.
    pub output: Bytes,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HttpResponseData {
    pub status: u16,
    /// Ordered values per name, so a handler can emit two `Set-Cookie` headers,
    /// which RFC 9110 forbids folding into one comma-separated value.
    pub headers: HashMap<String, Vec<String>>,
    pub body: Bytes,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SimpleResponseData {
    pub success: bool,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MessageProcessingResponseData {
    pub success: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub failures: Option<Vec<MessageProcessingFailure>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MessageProcessingFailure {
    #[serde(rename = "messageId")]
    pub message_id: String,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ScheduledEventResponseData {
    pub success: bool,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_message: Option<String>,
}

// ApiAppState holds shared API application state to be used in axum
// middleware and handlers.
#[derive(Clone)]
pub struct ApiAppState {
    pub platform: RuntimePlatform,
    /// Maps (HTTP method, route path) to the blueprint handler name.
    /// Used by the tracing middleware to record handler_name in the span.
    pub handler_names: HashMap<(String, String), String>,
    /// Pre-created OTel metric instruments. `None` when metrics are disabled
    /// (`CELERITY_METRICS_ENABLED` is not set or false).
    pub metrics: Option<Arc<RuntimeMetrics>>,
}
