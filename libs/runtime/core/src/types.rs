use std::collections::HashMap;

use celerity_helpers::runtime_types::RuntimePlatform;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::sync::oneshot;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventData {
    pub id: String,
    #[serde(rename = "eventType")]
    pub event_type: EventType,
    #[serde(rename = "handlerTag")]
    pub handler_tag: String,
    pub timestamp: u64,
    pub data: EventDataPayload,
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
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EventDataPayload {
    HttpRequestEventData(Box<HttpRequestEventData>),
    WsMessageEventData(WebSocketEventData),
    ConsumerMessageEventData(ConsumerEventData),
    ScheduleMessageEventData(ScheduleEventData),
    EventMessageEventData(EventMessageEventData),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HttpRequestEventData {
    pub method: String,
    pub path: String,
    pub route: String,
    #[serde(rename = "pathParams")]
    pub path_params: HashMap<String, String>,
    #[serde(rename = "queryParams")]
    pub query_params: HashMap<String, String>,
    #[serde(rename = "multiQueryParams")]
    pub multi_query_params: HashMap<String, Vec<String>>,
    #[serde(rename = "headers")]
    pub headers: HashMap<String, String>,
    #[serde(rename = "multiHeaders")]
    pub multi_headers: HashMap<String, Vec<String>>,
    #[serde(rename = "body")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub body: Option<String>,
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
    pub message: String,
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
    pub vendor: Value,
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
// The oneshot sender allows the caller to wait to receive the result of processing the
// event along with the original event data to carry out any further tasks using the input
// data.
pub type EventTuple = (oneshot::Sender<(EventData, EventResult)>, EventData);

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
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct HttpResponseData {
    pub status: u16,
    pub headers: HashMap<String, String>,
    pub body: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct SimpleResponseData {
    success: bool,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MessageProcessingResponseData {
    success: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    failues: Option<Vec<MessageProcessingFailure>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MessageProcessingFailure {
    #[serde(rename = "messageId")]
    message_id: String,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ScheduledEventResponseData {
    success: bool,
    #[serde(rename = "errorMessage")]
    #[serde(skip_serializing_if = "Option::is_none")]
    error_message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessages {
    pub messages: Vec<WebSocketMessage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessage {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    pub message: String,
}

// ApiAppState holds shared API application state to be used in axum
// middleware and handlers.
#[derive(Debug, Clone)]
pub struct ApiAppState {
    pub platform: RuntimePlatform,
}
