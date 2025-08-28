use serde::{Deserialize, Serialize};

/// The parsed data from a binary message in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/applications/resources/celerity-api#celerity-binary-message-format)
/// used for WebSocket APIs.
#[derive(Debug, PartialEq)]
pub struct BinaryMessageData {
    pub route: BinaryRoute,
    pub message_id: Option<String>,
    pub require_ack: bool,
    pub message: Vec<u8>,
}

/// The route of a binary message.
/// This can be a reserved route expected to be a single byte
/// or a custom route expected to be a utf-8 string.
#[derive(Debug, PartialEq)]
pub enum BinaryRoute {
    Reserved(u8),
    Custom(String),
}

/// The error type for parsing a binary message.
#[derive(Debug, PartialEq)]
pub enum BinaryMessageParseError {
    Malformed(String),
}

/// Parses a binary message in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/applications/resources/celerity-api#celerity-binary-message-format)
/// used for WebSocket APIs.
/// Empty payloads are allowed but all other fields are required.
pub fn parse_binary_message(
    msg_bytes: &[u8],
) -> Result<BinaryMessageData, BinaryMessageParseError> {
    if msg_bytes.len() < 4 {
        return Err(BinaryMessageParseError::Malformed(
            "message too short, must be at least 4 bytes for route \
            length, route, ack flag and message id length"
                .to_string(),
        ));
    }

    let route_length = msg_bytes[0];
    if route_length as usize + 1 > msg_bytes.len() {
        return Err(BinaryMessageParseError::Malformed(
            "route length exceeds message length".to_string(),
        ));
    }

    let route_bytes = &msg_bytes[1..=route_length as usize];
    let route = if route_bytes[0] <= 0x4 {
        // Reserved routes are single byte values from 0x0 to 0x4.
        BinaryRoute::Reserved(route_bytes[0])
    } else {
        // Custom routes are utf-8 strings.
        let route_str = String::from_utf8_lossy(route_bytes);
        BinaryRoute::Custom(route_str.to_string())
    };

    let ack_flag_index = route_length as usize + 1;
    if msg_bytes.len() < ack_flag_index + 1 {
        return Err(BinaryMessageParseError::Malformed(
            "message too short, missing bytes for ack flag and message id length".to_string(),
        ));
    }

    let require_ack = msg_bytes[ack_flag_index] == 0x1;

    let message_id_length_index = ack_flag_index + 1;
    let message_id_length = msg_bytes[message_id_length_index];
    if msg_bytes.len() < ack_flag_index + 2 + message_id_length as usize {
        return Err(BinaryMessageParseError::Malformed(
            "message too short, missing bytes for message id".to_string(),
        ));
    }

    let message_id = if message_id_length > 0 {
        let message_id_bytes = &msg_bytes
            [message_id_length_index + 1..=message_id_length_index + message_id_length as usize];
        let message_id_str = String::from_utf8_lossy(message_id_bytes);
        Some(message_id_str.to_string())
    } else {
        None
    };

    let data_start_index = message_id_length_index + 1 + message_id_length as usize;
    if data_start_index > msg_bytes.len() {
        // An empty message is allowed, for example, ping/pong messages
        // do not have a payload.
        Ok(BinaryMessageData {
            route,
            message_id,
            require_ack,
            message: Vec::new(),
        })
    } else {
        let message = &msg_bytes[data_start_index..];
        Ok(BinaryMessageData {
            route,
            message_id,
            require_ack,
            message: message.to_vec(),
        })
    }
}

/// The data for a lost message.
/// This is a notification that a message has been lost.
/// It is sent by the server to the client when a message is considered lost.
/// The client should then resend the message.
#[derive(Debug, PartialEq, Serialize, Deserialize)]
pub struct LostMessageData {
    #[serde(rename = "messageId")]
    pub message_id: String,
    pub caller: String,
}

/// The data for an ack message.
/// This is a notification that a message has been acknowledged.
/// It is sent by the server to the client when a message has been acknowledged.
#[derive(Debug, PartialEq, Serialize, Deserialize)]
pub struct AckMessageData {
    #[serde(rename = "messageId")]
    pub message_id: String,
    pub timestamp: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn test_parse_reserved_route_ping_message() {
        let msg_bytes = &[0x1, 0x1, 0x0, 0x0];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x1),
                message_id: None,
                require_ack: false,
                message: Vec::new(),
            }
        );
    }

    #[test]
    fn test_parse_reserved_route_pong_message() {
        let msg_bytes = &[0x1, 0x2, 0x0, 0x0];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x2),
                message_id: None,
                require_ack: false,
                message: Vec::new(),
            }
        );
    }

    #[test]
    fn test_parse_reserved_route_message_lost_message() {
        let payload_bytes = json!({
            // The ID of the message that is considered lost.
            "messageId": "134578",
            "caller": "test-caller",
        })
        .to_string()
        .as_bytes()
        .to_vec();
        let mut msg_bytes: Vec<u8> = vec![0x1, 0x3, 0x0, 0x0];
        msg_bytes.extend_from_slice(&payload_bytes);
        let result = parse_binary_message(&msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x3),
                // The notification itself does not have a message ID.
                message_id: None,
                require_ack: false,
                message: payload_bytes,
            }
        );
    }

    #[test]
    fn test_parse_reserved_route_ack_message() {
        let mut msg_bytes: Vec<u8> = vec![0x1, 0x4, 0x0, 0x0];
        let payload_bytes = json!({
            // The ID of the acknowledged message.
            "messageId": "13457915",
            "timestamp": 1715769600,
        })
        .to_string()
        .as_bytes()
        .to_vec();
        msg_bytes.extend_from_slice(&payload_bytes);
        let result = parse_binary_message(&msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x4),
                message_id: None,
                require_ack: false,
                message: payload_bytes,
            }
        );
    }

    #[test]
    fn test_parse_custom_route_message_with_message_id() {
        let route = "myCustomRoute".as_bytes();
        let mut msg_bytes: Vec<u8> = vec![route.len() as u8];
        msg_bytes.extend_from_slice(route);
        // 0x0 for ack flag.
        msg_bytes.extend_from_slice(&[0x0]);
        let message_id = "13457915".as_bytes();
        msg_bytes.extend_from_slice(&[message_id.len() as u8]);
        msg_bytes.extend_from_slice(message_id);
        let payload_bytes = json!({
            "message": "Hello, this is a custom message!",
        })
        .to_string()
        .as_bytes()
        .to_vec();
        msg_bytes.extend_from_slice(&payload_bytes);

        let result = parse_binary_message(&msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Custom("myCustomRoute".to_string()),
                message_id: Some("13457915".to_string()),
                require_ack: false,
                message: payload_bytes,
            }
        );
    }

    #[test]
    fn test_parse_custom_route_message_without_message_id() {
        let route = "myCustomRoute2".as_bytes();
        let mut msg_bytes: Vec<u8> = vec![route.len() as u8];
        msg_bytes.extend_from_slice(route);
        // 0x0 for ack flag and 0x0 for message id length.
        msg_bytes.extend_from_slice(&[0x0, 0x0]);
        let payload_bytes = json!({
            "message": "Hello, this is a custom message!",
        })
        .to_string()
        .as_bytes()
        .to_vec();
        msg_bytes.extend_from_slice(&payload_bytes);

        let result = parse_binary_message(&msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Custom("myCustomRoute2".to_string()),
                message_id: None,
                require_ack: false,
                message: payload_bytes,
            }
        );
    }

    #[test]
    fn test_parse_custom_route_message_requiring_ack() {
        let route = "myCustomRoute3".as_bytes();
        let mut msg_bytes: Vec<u8> = vec![route.len() as u8];
        msg_bytes.extend_from_slice(route);
        // 0x1 for ack flag.
        msg_bytes.extend_from_slice(&[0x1]);
        let message_id = "13457915".as_bytes();
        msg_bytes.extend_from_slice(&[message_id.len() as u8]);
        msg_bytes.extend_from_slice(message_id);
        let payload_bytes = json!({
            "message": "Hello, this is a custom message!",
        })
        .to_string()
        .as_bytes()
        .to_vec();
        msg_bytes.extend_from_slice(&payload_bytes);

        let result = parse_binary_message(&msg_bytes);
        assert!(result.is_ok());
        let data = result.unwrap();
        assert_eq!(
            data,
            BinaryMessageData {
                route: BinaryRoute::Custom("myCustomRoute3".to_string()),
                message_id: Some("13457915".to_string()),
                require_ack: true,
                message: payload_bytes,
            }
        );
    }

    #[test]
    fn test_gracefully_handles_malformed_message_that_is_too_short() {
        let msg_bytes = &[0x1, 0x1, 0x0];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert_eq!(
            error,
            BinaryMessageParseError::Malformed(
                "message too short, must be at least 4 bytes for route \
            length, route, ack flag and message id length"
                    .to_string(),
            )
        );
    }

    #[test]
    fn test_gracefully_handles_malformed_message_that_has_a_route_length_that_exceeds_the_message_length(
    ) {
        let msg_bytes = &[0x5, 0x1, 0x0, 0x0];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert_eq!(
            error,
            BinaryMessageParseError::Malformed("route length exceeds message length".to_string(),)
        );
    }

    #[test]
    fn test_gracefully_handles_malformed_message_missing_ack_flag_and_message_id_length() {
        let msg_bytes = &[0x4, 0x1, 0x1, 0x0, 0x3];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert_eq!(
            error,
            BinaryMessageParseError::Malformed(
                "message too short, missing bytes for ack flag and message id length".to_string(),
            )
        );
    }

    #[test]
    fn test_gracefully_handles_malformed_message_missing_bytes_for_message_id() {
        // Route length 0x2, route [0x1 0x1], ack flag 0x0, message id length 0x3, message id [0x1 0x0]
        // when the message id length is 0x3, the message id should be 0x3 bytes long.
        let msg_bytes = &[0x2, 0x1, 0x1, 0x0, 0x3, 0x1, 0x0];
        let result = parse_binary_message(msg_bytes);
        assert!(result.is_err());
        let error = result.unwrap_err();
        assert_eq!(
            error,
            BinaryMessageParseError::Malformed(
                "message too short, missing bytes for message id".to_string(),
            )
        );
    }
}
