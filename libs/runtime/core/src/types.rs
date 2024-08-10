use serde::{Deserialize, Serialize};
use tokio::sync::oneshot;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EventData {
    pub id: String,
}

// A tuple that contains a oneshot sender and received event data.
// The purpose of this tuple is to hand off processing of an event
// to another process or task. (That takes the event from a queue asynchronously)
// The oneshot sender allows the caller to wait to receive the result of processing the
// event along with the original event data to carry out any further tasks using the input
// data.
pub type EventTuple = (oneshot::Sender<(EventData, EventResult)>, EventData);

#[derive(Debug, Serialize, Deserialize)]
pub struct EventResult {
    #[serde(rename = "eventId")]
    pub event_id: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct WebSocketMessages {
    pub messages: Vec<WebSocketMessage>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct WebSocketMessage {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    pub message: String,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct ResponseMessage {
    pub message: String,
}
