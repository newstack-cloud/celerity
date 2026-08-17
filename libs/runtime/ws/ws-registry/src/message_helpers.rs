use axum::extract::ws::Message;
use base64::{prelude::BASE64_STANDARD, Engine};
use celerity_helpers::websockets::{encode_reserved_message, parse_binary_message, ReservedRoute};
use serde_json::{json, Value};

use crate::{errors::WebSocketConnError, types::MessageType};

/// Creates a message lost event to be sent to a WebSocket connection.
/// This follows the Celerity Binary Message Format documented here:
/// https://celerityframework.io/docs/framework/applications/resources/celerity-api#celerity-binary-message-format
pub fn create_message_lost_event(message_id: String, caller: Option<String>) -> Vec<u8> {
    let payload = json!({
        "messageId": message_id,
        // Names the context the lost message came from, so an application can
        // tell the client what to retry rather than only that something went
        // missing. Absent when the sender named none.
        "caller": caller.unwrap_or_default(),
    })
    .to_string();

    encode_reserved_message(ReservedRoute::LostMessage, payload.as_bytes())
}

/// Converts a message type and message received by a WebSocket registry
/// into a message that can be sent to a WebSocket connection.
/// Binary messages will be base64 encoded strings that can be stored in stores
/// that back WebSocket registries.
///
/// A binary message must be in the Celerity Binary Message Format and is
/// refused if it is not. A client reads every binary frame that is not a
/// reserved one as a framed message, so there is no such thing as sending it
/// raw bytes. Unframed bytes are read as a route length, a route and an id, and
/// what reaches the application is a payload short by however many bytes that
/// invented header consumed, delivered under a route nothing is listening on.
/// Refusing here turns that silence into an error the sender can see.
pub fn create_ws_message(
    message_type: MessageType,
    message: String,
) -> Result<Message, WebSocketConnError> {
    match message_type {
        MessageType::Json => Ok(Message::Text(message.into())),
        MessageType::Binary => {
            let bytes = BASE64_STANDARD.decode(message.as_bytes())?;
            // Read from the bytes that were about to be sent, so the check
            // costs no second decode.
            if let Err(err) = parse_binary_message(&bytes) {
                return Err(WebSocketConnError::MalformedBinaryMessage(format!(
                    "a binary message must be in the Celerity Binary Message Format, {err:?}"
                )));
            }
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

    /// Unframed bytes are refused rather than sent, since a client has no way
    /// to read them as anything other than a framed message.
    #[test]
    fn test_create_ws_message_refuses_binary_that_is_not_framed() {
        // A protobuf payload, which is the shape of thing an application
        // reaches for. Field 1 as a length delimited string, then "price".
        let raw = [0x0a, 0x05, b'p', b'r', b'i', b'c', b'e'];

        let result = create_ws_message(MessageType::Binary, BASE64_STANDARD.encode(raw));

        assert!(
            matches!(result, Err(WebSocketConnError::MalformedBinaryMessage(_))),
            "unframed bytes should be refused, got {result:?}"
        );
    }

    /// A payload starting with a zero byte used to take the process down here,
    /// since the parser read it as a route of no length and then indexed the
    /// empty slice that left. Every binary message passes through this on its
    /// way out, so that was reachable from any application.
    #[test]
    fn test_create_ws_message_refuses_binary_with_a_route_of_no_length() {
        let result = create_ws_message(
            MessageType::Binary,
            BASE64_STANDARD.encode([0x00, 0x01, 0x02, 0x03]),
        );

        assert!(
            matches!(result, Err(WebSocketConnError::MalformedBinaryMessage(_))),
            "a route of no length should be refused rather than panic, got {result:?}"
        );
    }

    #[test]
    fn test_create_ws_message_passes_a_framed_binary_message_through_untouched() {
        let framed = frame(b"price.tick", 0x1, b"m-1", &[0xde, 0xad]);

        let message = create_ws_message(MessageType::Binary, framed.clone()).unwrap();

        match message {
            Message::Binary(bytes) => {
                assert_eq!(bytes[..], BASE64_STANDARD.decode(framed).unwrap()[..])
            }
            other => panic!("a binary message should stay binary, got {other:?}"),
        }
    }

    /// Text is not framed and is not checked, which is what the spec says and
    /// what every JSON message on the wire relies on.
    #[test]
    fn test_create_ws_message_leaves_text_alone() {
        let message = create_ws_message(MessageType::Json, "not json either".to_string()).unwrap();

        assert!(matches!(message, Message::Text(_)));
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
