use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessages {
    pub messages: Vec<WebSocketMessage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessage {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "sourceNode")]
    pub source_node: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "informClientsOnLoss")]
    pub inform_clients_on_loss: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "caller")]
    pub caller: Option<String>,
    #[serde(rename = "messageId")]
    pub message_id: String,
    #[serde(rename = "messageType")]
    pub message_type: MessageType,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum MessageType {
    #[serde(rename = "json")]
    Json,
    #[serde(rename = "binary")]
    Binary,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum Message {
    WebSocket(WebSocketMessage),
    Ack(AckMessage),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AckMessage {
    // The ID of the node that originally sent the message.
    pub message_node: String,
    pub message_id: String,
}

#[derive(Default, Clone)]
pub struct AckWorkerConfig {
    // The interval in milliseconds at which to check to determine whether a message
    // should be considered lost or should be re-sent by the caller.
    pub message_action_check_interval_ms: Option<u64>,
    // The timeout in milliseconds for which the caller should consider re-sending
    // the message if it has not been acknowledged.
    pub message_timeout_ms: Option<u64>,
    // The number of times that a message should be attempted to be sent before it is considered
    // lost.
    pub max_attempts: Option<u32>,
}
