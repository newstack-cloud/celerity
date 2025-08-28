use axum::extract::ws::Message;
use base64::{prelude::BASE64_STANDARD, DecodeError, Engine};
use serde_json::json;

use crate::types::MessageType;

/// Creates a message lost event to be sent to a WebSocket connection.
/// This follows the Celerity Binary Message Format documented here:
/// https://www.celerityframework.io/docs/applications/resources/celerity-api#celerity-binary-message-format
pub fn create_message_lost_event(message_id: String) -> Vec<u8> {
    let route_len_byte = 0x1_u8;
    let route_byte = 0x3_u8;
    let payload = json!({
        "messageId": message_id,
    })
    .to_string();
    let payload_bytes = payload.as_bytes();
    // The message is a binary message with the route length, route and payload.
    // In this case, the route length is 1 byte, the route is 1 byte and the payload is length of
    // the serialised JSON message.
    let mut message = Vec::with_capacity(payload_bytes.len() + 2);
    message.push(route_len_byte);
    message.push(route_byte);
    message.extend_from_slice(payload_bytes);
    message
}

/// Converts a message type and message received by a WebSocket registry
/// into a message that can be sent to a WebSocket connection.
/// Binary messages will be base64 encoded strings that can be stored in stores
/// that back WebSocket registries.
pub fn create_ws_message(
    message_type: MessageType,
    message: String,
) -> Result<Message, DecodeError> {
    match message_type {
        MessageType::Json => Ok(Message::Text(message.into())),
        MessageType::Binary => {
            let bytes = BASE64_STANDARD.decode(message.as_bytes())?;
            Ok(Message::Binary(bytes.into()))
        }
    }
}
