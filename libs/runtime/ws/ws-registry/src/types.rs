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

/// How far a message has got, as recorded by the node holding its client.
///
/// A message crossing the cluster is settled in two steps that mean different
/// things. The node holding the connection first records that it has the
/// message and is handling it, which stops the sender forwarding it again but
/// carries no information about the client. It then records how it turned out.
///
/// A message that requires no acknowledgement from its client has only the
/// second, since a write to the socket is the whole of what can be known about
/// it.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub enum AckStage {
    /// The holding node has the message and is handling it.
    #[serde(rename = "takenOn")]
    TakenOn,
    /// The client acknowledged it, or it required no acknowledgement and
    /// reached the socket.
    #[serde(rename = "delivered")]
    Delivered,
    /// The holding node exhausted its attempts on it.
    #[serde(rename = "lost")]
    Lost,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AckMessage {
    // The ID of the node that originally sent the message.
    pub message_node: String,
    pub message_id: String,
    /// How far the message has got.
    ///
    /// Required, so that an acknowledgement which does not record what it
    /// means is refused rather than read as the message having been delivered.
    pub stage: AckStage,
    /// The node that has taken the message on, carried by
    /// [`AckStage::TakenOn`] so the sender can resolve whose liveness settles
    /// it if nothing further arrives.
    ///
    /// Absent from the stages that settle a message, which is why it defaults
    /// rather than being required.
    #[serde(default)]
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "holdingNode")]
    pub holding_node: Option<String>,
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

#[cfg(test)]
mod tests {
    use super::*;

    /// An acknowledgement that does not record what it means is refused.
    /// Reading a missing stage as any particular one would settle a message on
    /// that was never sent, and the obvious guess, delivered, is the one that
    /// quietly reports to a sender that its message arrived.
    #[test]
    fn test_an_acknowledgement_without_a_stage_is_refused() {
        let refused =
            serde_json::from_str::<AckMessage>(r#"{"message_node":"node-1","message_id":"m-1"}"#);

        assert!(refused.is_err(), "got {refused:?}");
    }

    /// Only the stage that hands a message over names a node, so the rest are
    /// written without one and have to read back without one.
    #[test]
    fn test_a_settling_acknowledgement_carries_no_holding_node() {
        let written = serde_json::to_string(&AckMessage {
            message_node: "node-1".to_string(),
            message_id: "m-1".to_string(),
            stage: AckStage::Delivered,
            holding_node: None,
        })
        .unwrap();

        assert!(!written.contains("holdingNode"), "written as {written}");
        assert_eq!(
            serde_json::from_str::<AckMessage>(&written).unwrap().stage,
            AckStage::Delivered
        );
    }

    #[test]
    fn test_a_stage_survives_the_round_trip() {
        let taken_on = AckMessage {
            message_node: "node-1".to_string(),
            message_id: "m-1".to_string(),
            stage: AckStage::TakenOn,
            holding_node: Some("node-2".to_string()),
        };

        let written = serde_json::to_string(&taken_on).unwrap();

        assert_eq!(
            serde_json::from_str::<AckMessage>(&written).unwrap(),
            taken_on
        );
    }
}
