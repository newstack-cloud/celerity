use std::{collections::HashMap, time::Duration};

use std::sync::Arc;
use tokio::{
    sync::{mpsc::Receiver, oneshot::Sender, Mutex},
    time::Instant,
};
use tracing::{debug, error, info, info_span, Instrument};

use crate::types::{AckWorkerConfig, MessageType};

/// The default timeout in milliseconds for which the caller should consider re-sending
/// the message if it has not been acknowledged.
///
/// One of the suggested defaults the WebSocket runtime protocol names.
pub const DEFAULT_MESSAGE_TIMEOUT_MS: u64 = 10000;

/// The default number of times that a message should be attempted to be sent before it is
/// considered lost.
///
/// One of the suggested defaults the WebSocket runtime protocol names.
pub const DEFAULT_MAX_ATTEMPTS: u32 = 3;

/// The longest a message may sit past its timeout before the worker notices,
/// which is what the check interval buys at the cost of waking more often.
const MAX_MESSAGE_ACTION_CHECK_INTERVAL_MS: u64 = 1000;

/// The shortest the check interval may be derived as, so that a very small
/// timeout does not turn the worker into a busy loop.
const MIN_MESSAGE_ACTION_CHECK_INTERVAL_MS: u64 = 20;

/// Derives how often to look for messages that have fallen due from the timeout
/// they are due after.
///
/// The check interval is what the timeout is rounded up to, since a message
/// falling due between two checks waits for the later one. Checking every ten
/// seconds for a message due after ten would resend somewhere between ten and
/// twenty seconds, so the interval has to be a fraction of the timeout for the
/// timeout to be accurate.
fn derive_message_action_check_interval_ms(message_timeout_ms: u64) -> u64 {
    (message_timeout_ms / 10).clamp(
        MIN_MESSAGE_ACTION_CHECK_INTERVAL_MS,
        MAX_MESSAGE_ACTION_CHECK_INTERVAL_MS,
    )
}

/// How much longer than its own budget a sender waits for an outcome from the
/// node holding the connection.
///
/// That node runs the same configuration, so it has the same timeout and the
/// same attempts to spend on the client before giving up. One timeout of slack
/// covers the round trips at either end. Reaching this means the holding node
/// never responded at all, which is a node that has gone rather than a client
/// that has not responded.
const OUTCOME_TIMEOUT_SLACK: u32 = 1;

/// The default interval in milliseconds to check for the acknowledgement status of a message.
pub const ACK_WAIT_CHECK_INTERVAL_MS: u64 = 20;

#[derive(Clone, Debug, PartialEq)]
pub enum AckStatus {
    // The message has been sent but no acknowledgement
    // has been received yet.
    Pending {
        connection_id: String,
        message: String,
        message_type: MessageType,
        inform_clients: Vec<String>,
        // The context the message was sent from, carried so that a client told
        // the message was lost is told what it was for.
        caller: Option<String>,
    },
    // The message has been received by the node that
    // has the connection that the message was sent for.
    Received,
    // The message was lost and no acknowledgement was received.
    Lost,
}

#[derive(Clone, Debug, PartialEq)]
pub struct ResendMessageInfo {
    pub client_id: String,
    pub message_id: String,
    pub message_type: MessageType,
    pub message: String,
    pub inform_clients_on_loss: Vec<String>,
    // Carried through the resend so that it is still there if the message is
    // eventually declared lost, rather than being dropped on the first retry.
    pub caller: Option<String>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum MessageAction {
    // The message should be re-sent by the caller along with a list of clients that should be
    // informed of the message being lost in the future.
    Resend(ResendMessageInfo),
    // The message should be considered lost and the caller should be informed that the message
    // was lost.
    Lost {
        message_id: String,
        inform_clients: Vec<String>,
        caller: Option<String>,
    },
}

pub enum AckWorkerMessage {
    Status(String, AckStatus),
    /// The node holding the connection has the message and is handling it.
    TakenOn {
        message_id: String,
        holding_node: Option<String>,
    },
    ClientAck {
        message_id: String,
        connection_id: String,
    },
    Wait(String, Sender<AckStatus>),
}

/// What the node holding a connection reported when it took a message on.
///
/// Kept beside the status rather than replacing it, because the message is
/// still pending until that node reports how it turned out, and the details a
/// loss would need are still in the status.
#[derive(Clone, Debug, PartialEq)]
struct TakenOn {
    /// When it was taken on, which is where the wait for an outcome starts
    /// rather than where the last forward was sent.
    at: Instant,
    /// Whose liveness settles the message if no outcome ever arrives.
    holding_node: Option<String>,
}

#[derive(Clone, Debug, PartialEq)]
struct DetailedAckStatus {
    status: AckStatus,
    attempts: u32,
    last_attempt_time: Option<Instant>,
    /// Set once another node has the message. It stops being forwarded and
    /// waits on that node from then on.
    taken_on: Option<TakenOn>,
}

/// A worker that manages acknowledgements for messages sent between
/// nodes in a WebSocket API cluster.
pub struct Worker {
    // A map of message IDs to their acknowledgement status from other nodes in a cluster.
    acks: Arc<Mutex<HashMap<String, DetailedAckStatus>>>,
    // The interval at which to check for actions based on ack statuses.
    message_action_check_interval_ms: u64,
    // The timeout in milliseconds for which the caller should consider re-sending
    // the message if it has not been acknowledged.
    message_timeout_ms: u64,
    // The number of times that a message should be attempted to be sent before it is considered
    // lost.
    max_attempts: u32,
}

impl Worker {
    pub fn new(config: AckWorkerConfig) -> Self {
        let message_timeout_ms = config
            .message_timeout_ms
            .unwrap_or(DEFAULT_MESSAGE_TIMEOUT_MS);
        Self {
            acks: Arc::new(Mutex::new(HashMap::new())),
            message_action_check_interval_ms: config
                .message_action_check_interval_ms
                .unwrap_or_else(|| derive_message_action_check_interval_ms(message_timeout_ms)),
            message_timeout_ms,
            max_attempts: config.max_attempts.unwrap_or(DEFAULT_MAX_ATTEMPTS),
        }
    }

    /// Start the ack worker that will manage acknowledgements for messages sent to other nodes in
    /// the cluster.
    /// The worker will periodically check for whether a message should be re-sent by the caller
    /// or considered lost. When the message should be re-sent, the worker will send a message to
    /// the caller with the message ID and when the message should be considered lost, the worker
    /// will send a message to the caller with the message ID and a list of client IDs that should
    /// be informed that the message was lost.
    pub fn start(
        mut self,
        mut ack_rx: Receiver<AckWorkerMessage>,
        message_action_tx: tokio::sync::mpsc::Sender<MessageAction>,
    ) {
        tokio::spawn(
            async move {
                info!(
                    "starting ack worker for managing acknowledgements for \
                messages sent to other nodes in the cluster",
                );

                // Spawn a separate task for periodic action checking
                let acks = Arc::clone(&self.acks);
                let message_action_tx_clone = message_action_tx.clone();
                let message_action_check_interval_ms = self.message_action_check_interval_ms;
                let message_timeout_ms = self.message_timeout_ms;
                let max_attempts = self.max_attempts;

                tokio::spawn(async move {
                    let mut interval = tokio::time::interval(Duration::from_millis(
                        message_action_check_interval_ms,
                    ));

                    loop {
                        interval.tick().await;
                        check_for_actions_periodic(
                            &acks,
                            &message_action_tx_clone,
                            message_timeout_ms,
                            max_attempts,
                        )
                        .await;
                    }
                });

                // Main loop only handles incoming messages
                loop {
                    match ack_rx.recv().await {
                        Some(AckWorkerMessage::Status(message_id, ack_status)) => {
                            self.record_ack(message_id, ack_status).await;
                        }
                        Some(AckWorkerMessage::TakenOn {
                            message_id,
                            holding_node,
                        }) => {
                            self.record_taken_on(message_id, holding_node).await;
                        }
                        Some(AckWorkerMessage::ClientAck {
                            message_id,
                            connection_id,
                        }) => {
                            self.record_client_ack(message_id, connection_id).await;
                        }
                        Some(AckWorkerMessage::Wait(message_id, tx)) => {
                            // Spawn a separate task to handle the ack wait without blocking
                            // the main worker loop.
                            let acks = Arc::clone(&self.acks);

                            tokio::spawn(handle_ack_wait(
                                message_id,
                                tx,
                                acks,
                                ACK_WAIT_CHECK_INTERVAL_MS,
                            ));
                        }
                        None => {
                            // Make sure we break out of the worker loop when the channel is closed
                            break;
                        }
                    }
                }
            }
            .instrument(info_span!("ack_worker")),
        );
    }

    async fn record_ack(&mut self, message_id: String, ack_status: AckStatus) {
        let mut acks_guard = self.acks.lock().await;
        let existing_ack_status = acks_guard.get(&message_id).cloned();

        let new_detailed_ack_status = if matches!(ack_status, AckStatus::Pending { .. }) {
            // Only increment the attempts if the message is still pending.
            DetailedAckStatus {
                status: ack_status,
                attempts: existing_ack_status.as_ref().map_or(0, |s| s.attempts) + 1,
                last_attempt_time: Some(Instant::now()),
                taken_on: existing_ack_status.and_then(|s| s.taken_on),
            }
        } else {
            DetailedAckStatus {
                status: ack_status,
                attempts: existing_ack_status.as_ref().map_or(0, |s| s.attempts),
                last_attempt_time: existing_ack_status
                    .as_ref()
                    .and_then(|s| s.last_attempt_time),
                taken_on: existing_ack_status.and_then(|s| s.taken_on),
            }
        };

        acks_guard.insert(message_id, new_detailed_ack_status);
    }

    /// Notes that another node has the message and is handling it.
    ///
    /// The message stays pending, since what its client makes of it is still to
    /// come, but it stops being forwarded and the wait for an outcome starts
    /// here rather than where the last forward was sent.
    async fn record_taken_on(&mut self, message_id: String, holding_node: Option<String>) {
        let mut acks_guard = self.acks.lock().await;
        let Some(existing) = acks_guard.get_mut(&message_id) else {
            debug!(
                message_id = %message_id,
                "a node took on a message nothing is waiting on, ignoring it"
            );
            return;
        };

        // Settled already, so there is nothing left to take on. A duplicate
        // forward responded to late produces this.
        if !matches!(existing.status, AckStatus::Pending { .. }) {
            return;
        }

        existing.taken_on = Some(TakenOn {
            at: Instant::now(),
            holding_node,
        });
    }

    /// Settles a message on the word of the client it was sent to.
    ///
    /// An acknowledgement naming a message that is waiting on a different
    /// connection is ignored, so one client cannot call off the delivery
    /// guarantees another is relying on.
    async fn record_client_ack(&mut self, message_id: String, connection_id: String) {
        let mut acks_guard = self.acks.lock().await;
        let Some(existing) = acks_guard.get_mut(&message_id) else {
            debug!(
                message_id = %message_id,
                connection_id = %connection_id,
                "a client acknowledged a message nothing is waiting on, ignoring it"
            );
            return;
        };

        let owed_by = match &existing.status {
            AckStatus::Pending {
                connection_id: pending_connection_id,
                ..
            } => pending_connection_id,
            // Already settled one way or the other, so there is nothing left to
            // say about it.
            _ => return,
        };

        if *owed_by != connection_id {
            debug!(
                message_id = %message_id,
                connection_id = %connection_id,
                "a client acknowledged a message owed by another connection, ignoring it"
            );
            return;
        }

        existing.status = AckStatus::Received;
    }
}

async fn check_for_actions_periodic(
    acks: &Arc<Mutex<HashMap<String, DetailedAckStatus>>>,
    message_action_tx: &tokio::sync::mpsc::Sender<MessageAction>,
    message_timeout_ms: u64,
    max_attempts: u32,
) {
    debug!("checking for actions based on ack statuses");
    let now = Instant::now();
    let mut actions = Vec::new();

    let mut acks_guard = acks.lock().await;

    for (message_id, detailed_ack_status) in acks_guard.iter_mut() {
        if let AckStatus::Pending {
            connection_id,
            message,
            message_type,
            inform_clients: client_ids,
            caller,
        } = &detailed_ack_status.status
        {
            // Taken on by another node, so it is that node's to handle and
            // there is nothing here to send again. All that is watched for is
            // an outcome that never arrives, which means the node holding it
            // has gone rather than a client that has not responded.
            if let Some(taken_on) = &detailed_ack_status.taken_on {
                let outcome_budget = Duration::from_millis(
                    message_timeout_ms * u64::from(max_attempts + OUTCOME_TIMEOUT_SLACK),
                );
                if now.duration_since(taken_on.at) > outcome_budget {
                    actions.push(MessageAction::Lost {
                        message_id: message_id.clone(),
                        inform_clients: client_ids.clone(),
                        caller: caller.clone(),
                    });
                }
                continue;
            }

            if let Some(last_attempt_time) = detailed_ack_status.last_attempt_time {
                if now.duration_since(last_attempt_time) > Duration::from_millis(message_timeout_ms)
                {
                    let action = if detailed_ack_status.attempts >= max_attempts {
                        MessageAction::Lost {
                            message_id: message_id.clone(),
                            inform_clients: client_ids.clone(),
                            caller: caller.clone(),
                        }
                    } else {
                        MessageAction::Resend(ResendMessageInfo {
                            client_id: connection_id.clone(),
                            message_id: message_id.clone(),
                            message_type: message_type.clone(),
                            message: message.clone(),
                            inform_clients_on_loss: client_ids.clone(),
                            caller: caller.clone(),
                        })
                    };
                    actions.push(action);
                }
            }
        }
    }

    // Release the lock before sending actions
    drop(acks_guard);

    for action in actions {
        let message_id = match &action {
            MessageAction::Resend(ResendMessageInfo { message_id, .. }) => message_id.clone(),
            MessageAction::Lost { message_id, .. } => message_id.clone(),
        };
        let is_lost = matches!(action, MessageAction::Lost { .. });

        if message_action_tx.send(action).await.is_err() {
            error!(
                "sender dropped before sending message action for message {}",
                message_id
            );
        }

        if is_lost {
            let mut acks_guard = acks.lock().await;
            acks_guard.remove(&message_id);
        }
    }
}

async fn handle_ack_wait(
    message_id: String,
    tx: Sender<AckStatus>,
    acks: Arc<Mutex<HashMap<String, DetailedAckStatus>>>,
    check_interval_ms: u64,
) {
    let mut check_interval = tokio::time::interval(Duration::from_millis(check_interval_ms));

    loop {
        check_interval.tick().await;

        // Check if we have a status for this message
        let acks_guard = acks.lock().await;
        if let Some(detailed_ack_status) = acks_guard.get(&message_id) {
            let is_pending = matches!(detailed_ack_status.status, AckStatus::Pending { .. });
            if !is_pending {
                // Message is no longer pending, send the final status
                if tx.send(detailed_ack_status.status.clone()).is_err() {
                    error!(
                        "sender dropped before sending final ack status for message {}",
                        message_id
                    );
                }
                break;
            }
        } else {
            // Message not found, consider it lost
            if tx.send(AckStatus::Lost).is_err() {
                error!(
                    "sender dropped before sending final ack status for message {}",
                    message_id
                );
            }
            break;
        }
        // Release the lock before the next iteration
        drop(acks_guard);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// A resend has to go to the connection the message was for.
    ///
    /// Pending acknowledgements are keyed by message id, which says nothing
    /// about where the message was headed, so the connection it was for is held
    /// alongside. Sending a resend to the message id instead finds no
    /// connection anywhere in the cluster and the message reaches nobody, while
    /// still counting as an attempt until it is declared lost.
    #[test_log::test(tokio::test)]
    async fn test_resend_is_addressed_to_the_connection_not_the_message() {
        let acks = Arc::new(Mutex::new(HashMap::from([(
            "message-1".to_string(),
            DetailedAckStatus {
                status: AckStatus::Pending {
                    connection_id: "connection-1".to_string(),
                    message: "{}".to_string(),
                    message_type: MessageType::Json,
                    inform_clients: vec![],
                    caller: None,
                },
                attempts: 0,
                // Long enough ago that it is due to be retried.
                last_attempt_time: Some(Instant::now() - Duration::from_secs(60)),
                taken_on: None,
            },
        )])));

        let (action_tx, mut action_rx) = tokio::sync::mpsc::channel(4);
        check_for_actions_periodic(&acks, &action_tx, 1_000, 3).await;

        let action = action_rx.recv().await.expect("a resend should be due");
        let MessageAction::Resend(resend) = action else {
            panic!("expected a resend, got {action:?}");
        };
        assert_eq!(resend.client_id, "connection-1");
        assert_eq!(resend.message_id, "message-1");
    }

    /// A timeout only means what it says if the worker looks often enough to
    /// notice, so the interval is derived from it rather than set beside it.
    #[test]
    fn test_the_check_interval_is_a_fraction_of_the_timeout_it_is_derived_from() {
        assert_eq!(
            derive_message_action_check_interval_ms(DEFAULT_MESSAGE_TIMEOUT_MS),
            1_000
        );
        assert_eq!(derive_message_action_check_interval_ms(2_000), 200);
    }

    /// Bounded at both ends. A long timeout does not buy a proportionally
    /// coarse check, and a very short one does not turn the worker into a busy
    /// loop.
    #[test]
    fn test_the_derived_check_interval_stays_within_its_bounds() {
        assert_eq!(derive_message_action_check_interval_ms(600_000), 1_000);
        assert_eq!(derive_message_action_check_interval_ms(100), 20);
        assert_eq!(derive_message_action_check_interval_ms(0), 20);
    }

    /// An interval given explicitly is used as given, which is what lets a test
    /// watch a resend without waiting on the production timings.
    #[test]
    fn test_an_explicit_check_interval_is_kept() {
        let worker = Worker::new(AckWorkerConfig {
            message_action_check_interval_ms: Some(50),
            message_timeout_ms: Some(100),
            max_attempts: Some(2),
        });

        assert_eq!(worker.message_action_check_interval_ms, 50);
        assert_eq!(worker.message_timeout_ms, 100);
        assert_eq!(worker.max_attempts, 2);
    }

    /// The good defaults the WebSocket runtime protocol names, which a
    /// deployment gets by configuring nothing.
    #[test]
    fn test_the_defaults_are_the_ones_the_protocol_names() {
        let worker = Worker::new(AckWorkerConfig::default());

        assert_eq!(worker.message_timeout_ms, 10_000);
        assert_eq!(worker.max_attempts, 3);
        assert_eq!(worker.message_action_check_interval_ms, 1_000);
    }

    #[test_log::test(tokio::test)]
    async fn test_a_message_taken_on_by_another_node_is_not_forwarded_again() {
        let acks = pending_since(Duration::from_secs(60), Some(taken_on_now()));
        let (action_tx, mut action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 3).await;

        assert!(
            action_rx.try_recv().is_err(),
            "a message another node is handling should be left alone"
        );
    }

    /// A node that took a message on and then recorded nothing has gone, and
    /// the message it was holding goes with it. The deadline is what keeps an
    /// application from waiting for an outcome that never arrives.
    #[test_log::test(tokio::test)]
    async fn test_a_message_taken_on_and_never_answered_for_is_declared_lost() {
        let taken_on = Some(TakenOn {
            // Past the timeout multiplied by the attempts allowed, and the
            // slack on top of it.
            at: Instant::now() - Duration::from_millis(100 * (3 + 1) + 50),
            holding_node: Some("node-2".to_string()),
        });
        let acks = pending_since(Duration::from_secs(60), taken_on);
        let (action_tx, mut action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 3).await;

        assert!(
            matches!(
                action_rx.try_recv(),
                Ok(MessageAction::Lost { ref message_id, .. }) if message_id == "message-1"
            ),
            "a message whose holding node never answered should be declared lost"
        );
    }

    /// Taking a message on records that the node has it, not that the client
    /// has received it, so an application waiting for the outcome keeps
    /// waiting.
    #[test_log::test(tokio::test)]
    async fn test_taking_a_message_on_does_not_settle_it() {
        let mut worker = Worker::new(AckWorkerConfig::default());
        worker
            .record_ack("message-1".to_string(), pending_status())
            .await;

        worker
            .record_taken_on("message-1".to_string(), Some("node-2".to_string()))
            .await;

        let acks = worker.acks.lock().await;
        let held = acks.get("message-1").unwrap();
        assert!(
            matches!(held.status, AckStatus::Pending { .. }),
            "a message taken on is still waiting on its client"
        );
        assert_eq!(
            held.taken_on.as_ref().unwrap().holding_node,
            Some("node-2".to_string())
        );
    }

    /// A message already settled is not reopened by a forward answered late,
    /// which a sender retrying just before the outcome arrived produces.
    #[test_log::test(tokio::test)]
    async fn test_a_settled_message_is_not_taken_on_after_the_fact() {
        let mut worker = Worker::new(AckWorkerConfig::default());
        worker
            .record_ack("message-1".to_string(), pending_status())
            .await;
        worker
            .record_ack("message-1".to_string(), AckStatus::Received)
            .await;

        worker
            .record_taken_on("message-1".to_string(), Some("node-2".to_string()))
            .await;

        let acks = worker.acks.lock().await;
        assert!(acks.get("message-1").unwrap().taken_on.is_none());
    }

    fn pending_status() -> AckStatus {
        AckStatus::Pending {
            connection_id: "connection-1".to_string(),
            message: "{}".to_string(),
            message_type: MessageType::Json,
            inform_clients: vec![],
            caller: None,
        }
    }

    fn taken_on_now() -> TakenOn {
        TakenOn {
            at: Instant::now(),
            holding_node: Some("node-2".to_string()),
        }
    }

    /// One message waiting, last tried however long ago, and taken on or not.
    fn pending_since(
        since: Duration,
        taken_on: Option<TakenOn>,
    ) -> Arc<Mutex<HashMap<String, DetailedAckStatus>>> {
        Arc::new(Mutex::new(HashMap::from([(
            "message-1".to_string(),
            DetailedAckStatus {
                status: pending_status(),
                attempts: 0,
                last_attempt_time: Some(Instant::now() - since),
                taken_on,
            },
        )])))
    }
}
