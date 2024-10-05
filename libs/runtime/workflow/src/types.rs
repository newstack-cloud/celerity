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
    pub data: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EventType {
    #[serde(rename = "executeStep")]
    ExecuteStep,
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
    pub data: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub context: Option<Value>,
}

// WorkflowAppState holds shared Workflow application state to be used in axum
// middleware and handlers.
#[derive(Debug, Clone)]
pub struct WorkflowAppState {
    pub platform: RuntimePlatform,
}
