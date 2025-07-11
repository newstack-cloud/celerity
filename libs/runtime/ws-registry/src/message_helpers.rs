use serde_json::json;

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
