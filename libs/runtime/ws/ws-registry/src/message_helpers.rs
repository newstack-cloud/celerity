use axum::extract::ws::Message;
use base64::{prelude::BASE64_STANDARD, DecodeError, Engine};
use serde_json::json;

use crate::types::MessageType;

/// Creates a message lost event to be sent to a WebSocket connection.
/// This follows the Celerity Binary Message Format documented here:
/// https://www.celerityframework.io/docs/applications/resources/celerity-api#celerity-binary-message-format
pub fn create_message_lost_event(message_id: String, caller: Option<String>) -> Vec<u8> {
    let payload = json!({
        "messageId": message_id,
        // Names the context the lost message came from, so an application can
        // tell the client what to retry rather than only that something went
        // missing. Absent when the sender named none.
        "caller": caller.unwrap_or_default(),
    })
    .to_string();
    let payload_bytes = payload.as_bytes();

    // `[routeLength][route][requireAck][messageIdLength]` then the payload. A
    // reserved message needs no acknowledgement of its own and carries no
    // message id of its own, which is what the two zero bytes say. Clients match
    // on all four, so a short header is not a shorter version of this message,
    // it is one they cannot recognise at all.
    let mut message = Vec::with_capacity(payload_bytes.len() + 4);
    message.extend_from_slice(&[0x1, 0x3, 0x0, 0x0]);
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
