use serde::{Deserialize, Serialize};

/// The parsed data from a binary message in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/framework/applications/resources/celerity-api#celerity-binary-message-format)
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

/// The routes the protocol keeps for itself.
///
/// An application's route is a name; these are a single byte, which is what
/// makes them reserved rather than reachable.
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum ReservedRoute {
    Ping = 0x1,
    Pong = 0x2,
    LostMessage = 0x3,
    Ack = 0x4,
    Capabilities = 0x5,
}

impl ReservedRoute {
    /// Reads a route byte as one of the protocol's own, if it is one.
    ///
    /// The used by the parser and encoder for a single source of truth on
    /// reserved routes.
    pub fn from_byte(byte: u8) -> Option<Self> {
        match byte {
            0x1 => Some(ReservedRoute::Ping),
            0x2 => Some(ReservedRoute::Pong),
            0x3 => Some(ReservedRoute::LostMessage),
            0x4 => Some(ReservedRoute::Ack),
            0x5 => Some(ReservedRoute::Capabilities),
            _ => None,
        }
    }
}

/// Frames one of the protocol's own messages.
///
/// Not a different format from an application's message, the same one with a
/// route that is a single byte, asking for no acknowledgement and carrying no
/// id of its own. Writing that header out by hand at each place one of these is
/// built is how it came to be written with two bytes in one of them, leaving a
/// message no client could recognise.
pub fn encode_reserved_message(route: ReservedRoute, payload: &[u8]) -> Vec<u8> {
    let mut framed = Vec::with_capacity(4 + payload.len());
    framed.extend_from_slice(&[0x1, route as u8, 0x0, 0x0]);
    framed.extend_from_slice(payload);
    framed
}

/// The error type for parsing a binary message.
#[derive(Debug, PartialEq)]
pub enum BinaryMessageParseError {
    Malformed(String),
}

/// Parses a binary message in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/framework/applications/resources/celerity-api#celerity-binary-message-format)
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
    if route_length == 0 {
        return Err(BinaryMessageParseError::Malformed(
            "a message must name the route it is for, and this one has a route \
            of no length"
                .to_string(),
        ));
    }
    if route_length as usize + 1 > msg_bytes.len() {
        return Err(BinaryMessageParseError::Malformed(
            "route length exceeds message length".to_string(),
        ));
    }

    let route_bytes = &msg_bytes[1..=route_length as usize];
    let route = if ReservedRoute::from_byte(route_bytes[0]).is_some() {
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

/// The parts of a binary message to be framed in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/framework/applications/resources/celerity-api#celerity-binary-message-format).
pub struct BinaryMessageParts<'a> {
    /// The route the message is for, which a client uses to hand it to the
    /// right handler. Reserved single byte routes are not built through here,
    /// as those frames are fixed and are written out directly.
    pub route: &'a str,
    /// The id the message is known by, which is what an acknowledgement names,
    /// what deduplication keys on and what a loss notification refers to.
    pub message_id: Option<&'a str>,
    /// Whether the receiver is being asked to acknowledge this message, which
    /// only means anything alongside an id.
    pub require_ack: bool,
    /// The payload, carried without being read.
    pub message: &'a [u8],
}

/// The error type for framing a binary message.
#[derive(Debug, PartialEq)]
pub enum BinaryMessageEncodeError {
    Invalid(String),
}

/// Frames a binary message in the
/// [Celerity Binary Message Format](https://celerityframework.io/docs/framework/applications/resources/celerity-api#celerity-binary-message-format).
///
/// The mirror of [`parse_binary_message`], and the reason a caller should not
/// be laying out these bytes itself. Every field that cannot be represented is
/// refused rather than truncated into a frame that would be read as something
/// other than what was meant.
pub fn encode_binary_message(
    parts: BinaryMessageParts<'_>,
) -> Result<Vec<u8>, BinaryMessageEncodeError> {
    let route_bytes = parts.route.as_bytes();
    if route_bytes.is_empty() {
        return Err(BinaryMessageEncodeError::Invalid(
            "a route is required, as it is what the message is delivered by".to_string(),
        ));
    }
    if route_bytes.len() > u8::MAX as usize {
        return Err(BinaryMessageEncodeError::Invalid(format!(
            "a route can not be longer than {} bytes, this one is {}",
            u8::MAX,
            route_bytes.len()
        )));
    }
    // A route is read as reserved when its first byte is one of the reserved
    // values, so a custom route starting with one would come back as a ping or
    // an acknowledgement rather than as itself.
    if ReservedRoute::from_byte(route_bytes[0]).is_some() {
        return Err(BinaryMessageEncodeError::Invalid(format!(
            "a route can not begin with the byte {:#x}, which is reserved",
            route_bytes[0]
        )));
    }

    let message_id_bytes = parts.message_id.unwrap_or_default().as_bytes();
    if message_id_bytes.len() > u8::MAX as usize {
        return Err(BinaryMessageEncodeError::Invalid(format!(
            "a message id can not be longer than {} bytes, this one is {}",
            u8::MAX,
            message_id_bytes.len()
        )));
    }
    // Asking to be acknowledged without an id is refused rather than quietly
    // dropped, since the sender would otherwise wait for an answer that has
    // nothing to name and can never come.
    if parts.require_ack && message_id_bytes.is_empty() {
        return Err(BinaryMessageEncodeError::Invalid(
            "a message asking to be acknowledged needs an id for the acknowledgement to name"
                .to_string(),
        ));
    }

    let mut framed =
        Vec::with_capacity(3 + route_bytes.len() + message_id_bytes.len() + parts.message.len());
    framed.push(route_bytes.len() as u8);
    framed.extend_from_slice(route_bytes);
    framed.push(if parts.require_ack { 0x1 } else { 0x0 });
    framed.push(message_id_bytes.len() as u8);
    framed.extend_from_slice(message_id_bytes);
    framed.extend_from_slice(parts.message);

    Ok(framed)
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

    /// What is framed is what comes back out, which is the property every other
    /// use of this depends on.
    #[test]
    fn test_encode_binary_message_round_trips_through_the_parser() {
        let framed = encode_binary_message(BinaryMessageParts {
            route: "price.tick",
            message_id: Some("m-1"),
            require_ack: true,
            message: &[0xde, 0xad, 0xbe, 0xef],
        })
        .unwrap();

        assert_eq!(
            parse_binary_message(&framed).unwrap(),
            BinaryMessageData {
                route: BinaryRoute::Custom("price.tick".to_string()),
                message_id: Some("m-1".to_string()),
                require_ack: true,
                message: vec![0xde, 0xad, 0xbe, 0xef],
            }
        );
    }

    #[test]
    fn test_encode_binary_message_lays_the_fields_out_in_order() {
        let framed = encode_binary_message(BinaryMessageParts {
            route: "up",
            message_id: Some("id"),
            require_ack: false,
            message: &[0xff],
        })
        .unwrap();

        // [routeLength][route][requireAck][messageIdLength][messageId][message]
        assert_eq!(framed, vec![0x2, b'u', b'p', 0x0, 0x2, b'i', b'd', 0xff]);
    }

    /// A message with nothing to say is still a message, and a message with no
    /// id is the ordinary case for one that does not ask to be acknowledged.
    #[test]
    fn test_encode_binary_message_allows_an_empty_payload_and_no_id() {
        let framed = encode_binary_message(BinaryMessageParts {
            route: "up",
            message_id: None,
            require_ack: false,
            message: &[],
        })
        .unwrap();

        assert_eq!(framed, vec![0x2, b'u', b'p', 0x0, 0x0]);
        assert_eq!(
            parse_binary_message(&framed).unwrap(),
            BinaryMessageData {
                route: BinaryRoute::Custom("up".to_string()),
                message_id: None,
                require_ack: false,
                message: Vec::new(),
            }
        );
    }

    #[test]
    fn test_encode_binary_message_refuses_a_route_it_cannot_represent() {
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: "",
                message_id: None,
                require_ack: false,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));

        let too_long = "r".repeat(256);
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: &too_long,
                message_id: None,
                require_ack: false,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));
    }

    /// A custom route beginning with a reserved byte would be read back as a
    /// ping or an acknowledgement rather than as itself.
    #[test]
    fn test_encode_binary_message_refuses_a_route_that_would_read_as_reserved() {
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: "\u{4}route",
                message_id: None,
                require_ack: false,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));
    }

    #[test]
    fn test_encode_binary_message_refuses_an_id_it_cannot_represent() {
        let too_long = "i".repeat(256);
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: "up",
                message_id: Some(&too_long),
                require_ack: false,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));
    }

    /// A reserved message is read back as the route it names, with the payload
    /// whole. The parser is the client's side of this, so a round trip is what
    /// says the two agree.
    #[test]
    fn test_encode_reserved_message_round_trips_through_the_parser() {
        let framed = encode_reserved_message(ReservedRoute::LostMessage, br#"{"messageId":"m-1"}"#);

        assert_eq!(
            parse_binary_message(&framed).unwrap(),
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x3),
                message_id: None,
                require_ack: false,
                message: br#"{"messageId":"m-1"}"#.to_vec(),
            }
        );
    }

    /// The header is four bytes, not two. A short one is not a shorter version
    /// of the message, it is one no client can recognise, which is what a
    /// hand written header here produced before.
    #[test]
    fn test_encode_reserved_message_writes_the_whole_header() {
        assert_eq!(
            encode_reserved_message(ReservedRoute::Pong, &[]),
            vec![0x1, 0x2, 0x0, 0x0]
        );
        assert_eq!(
            encode_reserved_message(ReservedRoute::Capabilities, &[]),
            vec![0x1, 0x5, 0x0, 0x0]
        );
        assert_eq!(
            encode_reserved_message(ReservedRoute::Ack, &[0xff]),
            vec![0x1, 0x4, 0x0, 0x0, 0xff]
        );
    }

    /// A route of no length left an empty slice that was then indexed, which
    /// took the process down rather than reporting a malformed message. Every
    /// binary message sent to a client is read by this on its way out, so a
    /// payload beginning with a zero byte was enough to do it.
    #[test]
    fn test_parse_binary_message_refuses_a_route_of_no_length() {
        let result = parse_binary_message(&[0x00, 0x01, 0x02, 0x03]);

        assert!(
            matches!(result, Err(BinaryMessageParseError::Malformed(_))),
            "a route of no length should be refused, got {result:?}"
        );
    }

    /// The capabilities signal is one of the protocol's own routes, so it has
    /// to be read back as one. Read as a custom route it would be handed to an
    /// application as a message on a route named by a control byte.
    #[test]
    fn test_the_capabilities_signal_round_trips_as_a_reserved_route() {
        let framed = encode_reserved_message(ReservedRoute::Capabilities, &[]);

        assert_eq!(
            parse_binary_message(&framed).unwrap(),
            BinaryMessageData {
                route: BinaryRoute::Reserved(0x5),
                message_id: None,
                require_ack: false,
                message: Vec::new(),
            }
        );
    }

    /// Every reserved route, so none of them can be left out of the parser's
    /// reckoning the way the capabilities one was.
    #[test]
    fn test_every_reserved_route_round_trips_as_reserved() {
        for route in [
            ReservedRoute::Ping,
            ReservedRoute::Pong,
            ReservedRoute::LostMessage,
            ReservedRoute::Ack,
            ReservedRoute::Capabilities,
        ] {
            let framed = encode_reserved_message(route, &[]);
            let parsed = parse_binary_message(&framed).unwrap();

            assert_eq!(
                parsed.route,
                BinaryRoute::Reserved(route as u8),
                "{route:?} should be read back as the reserved route it is"
            );
        }
    }

    /// An application must not be able to compose a route that would be read as
    /// the capabilities signal, for the same reason as the other reserved ones.
    #[test]
    fn test_encode_binary_message_refuses_a_route_beginning_with_the_capabilities_byte() {
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: "\u{5}route",
                message_id: None,
                require_ack: false,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));
    }

    /// Asking for an acknowledgement with no id is refused rather than framed
    /// and ignored, since the sender would wait for an answer that has nothing
    /// to name.
    #[test]
    fn test_encode_binary_message_refuses_an_ack_request_with_no_id() {
        assert!(matches!(
            encode_binary_message(BinaryMessageParts {
                route: "up",
                message_id: None,
                require_ack: true,
                message: &[],
            }),
            Err(BinaryMessageEncodeError::Invalid(_))
        ));
    }
}
