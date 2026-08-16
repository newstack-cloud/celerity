use axum::extract::ws::Message;
use base64::{prelude::BASE64_STANDARD, DecodeError, Engine};
use serde_json::{json, Value};

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

pub fn client_ack_request(message_type: &MessageType, message: &str) -> Option<String> {
    match message_type {
        MessageType::Json => {
            let value: Value = serde_json::from_str(message).ok()?;
            let object = value.as_object()?;
            if object.get("ack").and_then(Value::as_bool) != Some(true) {
                return None;
            }
            object
                .get("messageId")
                .and_then(Value::as_str)
                .map(str::to_string)
        }
        MessageType::Binary => {
            let bytes = BASE64_STANDARD.decode(message).ok()?;
            // [routeLength][route][requireAck][messageIdLength][messageId]
            let route_length = *bytes.first()? as usize;
            if *bytes.get(1 + route_length)? != 0x1 {
                return None;
            }
            let id_length = *bytes.get(route_length + 2)? as usize;
            if id_length == 0 {
                return None;
            }
            let start = route_length + 3;
            let id = bytes.get(start..start + id_length)?;
            std::str::from_utf8(id).ok().map(str::to_string)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn frame(route: &[u8], requires_ack: u8, message_id: &[u8], payload: &[u8]) -> String {
        let mut bytes = vec![route.len() as u8];
        bytes.extend_from_slice(route);
        bytes.push(requires_ack);
        bytes.push(message_id.len() as u8);
        bytes.extend_from_slice(message_id);
        bytes.extend_from_slice(payload);
        BASE64_STANDARD.encode(bytes)
    }

    #[test]
    fn test_client_ack_request_reads_the_opt_in_from_the_message() {
        assert_eq!(
            client_ack_request(
                &MessageType::Json,
                r#"{"event":"update","ack":true,"messageId":"m-1"}"#
            ),
            Some("m-1".to_string())
        );
        assert_eq!(
            client_ack_request(
                &MessageType::Binary,
                &frame(b"updates", 0x1, b"m-2", &[0xff])
            ),
            Some("m-2".to_string())
        );
    }

    #[test]
    fn test_client_ack_request_ignores_messages_that_did_not_ask() {
        // Carries an id but does not opt in.
        assert!(client_ack_request(&MessageType::Json, r#"{"messageId":"m-1"}"#).is_none());
        // Opts in with no id, so there is nothing an acknowledgement could name.
        assert!(client_ack_request(&MessageType::Json, r#"{"ack":true}"#).is_none());
        assert!(client_ack_request(
            &MessageType::Binary,
            &frame(b"updates", 0x0, b"m-2", &[0xff])
        )
        .is_none());
        assert!(
            client_ack_request(&MessageType::Binary, &frame(b"updates", 0x1, b"", &[0xff]))
                .is_none()
        );
    }
}
