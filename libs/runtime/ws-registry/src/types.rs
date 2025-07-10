use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessages {
    pub messages: Vec<WebSocketMessage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WebSocketMessage {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "messageId")]
    pub message_id: String,
    pub message: String,
}
