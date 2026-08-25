use std::{
    collections::{HashMap, HashSet},
    time::Duration,
};

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

/// How long a settled message is kept before it is swept.
///
/// A message is settled the moment its outcome is known, but something may
/// still be waiting to read that outcome, and a waiter that found nothing
/// would treat the message as lost.
const SETTLED_GRACE_MULTIPLIER: u32 = 1;

/// The default interval in milliseconds to check for the acknowledgement status of a message.
pub const ACK_WAIT_CHECK_INTERVAL_MS: u64 = 20;

/// Where a message came from, for one this node is holding the connection for.
///
/// The node that forwarded it is waiting to be told how it turned out, and it
/// is waiting against the id it sent, which doesn't need to be the id inside the
/// message that the client acknowledges by.
#[derive(Clone, Debug, PartialEq)]
pub struct MessageOrigin {
    /// The node that forwarded the message and is waiting for the outcome.
    pub node: String,
    /// The id that node is waiting against.
    pub message_id: String,
}

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
        // The node waiting to be told how this turned out, for a message
        // forwarded here rather than sent from this node.
        origin: Option<MessageOrigin>,
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
    // Carried for the same reason, so the node waiting on the outcome is still
    // known after the message has been sent to its client again.
    pub origin: Option<MessageOrigin>,
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
        // Where to report the loss, for a message forwarded here by another
        // node that is waiting on the outcome.
        origin: Option<MessageOrigin>,
    },
    // The client acknowledged a message forwarded here, which the node that
    // forwarded it is waiting to be told.
    Delivered {
        origin: MessageOrigin,
    },
    // Whether the named node is still running, asked because messages taken on
    // by it are being waited for. Raised for the node rather than for each
    // message, since one answer settles all of them.
    CheckHolder {
        holding_node: String,
    },
}

pub enum AckWorkerMessage {
    Status(String, AckStatus),
    /// The node holding the connection has the message and is handling it.
    TakenOn {
        message_id: String,
        holding_node: Option<String>,
    },
    /// The node holding messages taken on from here has gone, so there is
    /// nothing left to wait for on any of them.
    HolderGone {
        holding_node: String,
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
    /// When the outcome became known, which is when the entry is no longer
    /// needed but is still available to read for a short time.
    settled_at: Option<Instant>,
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
                        Some(AckWorkerMessage::HolderGone { holding_node }) => {
                            for lost in self.record_holder_gone(&holding_node).await {
                                if message_action_tx.send(lost).await.is_err() {
                                    error!(
                                        "receiver dropped before reporting messages held by a \
                                         node that has gone"
                                    );
                                }
                            }
                        }
                        Some(AckWorkerMessage::ClientAck {
                            message_id,
                            connection_id,
                        }) => {
                            // A message forwarded here leaves another node
                            // waiting on the outcome, which is only known once
                            // the client has answered.
                            if let Some(origin) =
                                self.record_client_ack(message_id, connection_id).await
                            {
                                if message_action_tx
                                    .send(MessageAction::Delivered { origin })
                                    .await
                                    .is_err()
                                {
                                    error!(
                                        "receiver dropped before reporting a message as delivered"
                                    );
                                }
                            }
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
                settled_at: None,
            }
        } else {
            DetailedAckStatus {
                status: ack_status,
                attempts: existing_ack_status.as_ref().map_or(0, |s| s.attempts),
                last_attempt_time: existing_ack_status
                    .as_ref()
                    .and_then(|s| s.last_attempt_time),
                taken_on: existing_ack_status.and_then(|s| s.taken_on),
                settled_at: Some(Instant::now()),
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

    /// Settles every message taken on by a node that has gone.
    ///
    /// Its connections went with it, so no acknowledgement can arrive for any
    /// of them and there is nothing to gain by waiting the deadline out.
    /// Answers with what to report for each, and takes them out of the map the
    /// same way the deadline would.
    async fn record_holder_gone(&mut self, holding_node: &str) -> Vec<MessageAction> {
        let mut acks_guard = self.acks.lock().await;
        let mut lost = Vec::new();

        acks_guard.retain(|message_id, held| {
            let held_by_gone_node = held
                .taken_on
                .as_ref()
                .and_then(|taken_on| taken_on.holding_node.as_deref())
                == Some(holding_node);
            let AckStatus::Pending {
                inform_clients,
                caller,
                origin,
                ..
            } = &held.status
            else {
                return true;
            };
            if !held_by_gone_node {
                return true;
            }

            lost.push(MessageAction::Lost {
                message_id: message_id.clone(),
                inform_clients: inform_clients.clone(),
                caller: caller.clone(),
                origin: origin.clone(),
            });
            false
        });

        lost
    }

    /// Settles a message on the word of the client it was sent to.
    ///
    /// An acknowledgement naming a message that is waiting on a different
    /// connection is ignored, so one client cannot call off the delivery
    /// guarantees another is relying on.
    ///
    /// Answers with the node owed the outcome, where the message was forwarded
    /// here by one.
    async fn record_client_ack(
        &mut self,
        message_id: String,
        connection_id: String,
    ) -> Option<MessageOrigin> {
        let mut acks_guard = self.acks.lock().await;
        let Some(existing) = acks_guard.get_mut(&message_id) else {
            debug!(
                message_id = %message_id,
                connection_id = %connection_id,
                "a client acknowledged a message nothing is waiting on, ignoring it"
            );
            return None;
        };

        let (owed_by, origin) = match &existing.status {
            AckStatus::Pending {
                connection_id: pending_connection_id,
                origin,
                ..
            } => (pending_connection_id, origin.clone()),
            // Already settled one way or the other, so there is nothing left to
            // report about it.
            _ => return None,
        };

        if *owed_by != connection_id {
            debug!(
                message_id = %message_id,
                connection_id = %connection_id,
                "a client acknowledged a message owed by another connection, ignoring it"
            );
            return None;
        }

        existing.status = AckStatus::Received;
        existing.settled_at = Some(Instant::now());
        origin
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
    let mut holders_to_check = HashSet::new();

    let mut acks_guard = acks.lock().await;

    for (message_id, detailed_ack_status) in acks_guard.iter_mut() {
        if let AckStatus::Pending {
            connection_id,
            message,
            message_type,
            inform_clients: client_ids,
            caller,
            origin,
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
                        origin: origin.clone(),
                    });
                    continue;
                }

                // The deadline is the backstop. Where the node holding it has
                // gone, its connections went with it and no acknowledgement can
                // ever arrive, so there is no reason to wait the rest out. One
                // name is collected however many messages it holds, since one
                // answer settles all of them.
                if let Some(holding_node) = &taken_on.holding_node {
                    if now.duration_since(taken_on.at) > Duration::from_millis(message_timeout_ms) {
                        holders_to_check.insert(holding_node.clone());
                    }
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
                            origin: origin.clone(),
                        }
                    } else {
                        MessageAction::Resend(ResendMessageInfo {
                            client_id: connection_id.clone(),
                            message_id: message_id.clone(),
                            message_type: message_type.clone(),
                            message: message.clone(),
                            inform_clients_on_loss: client_ids.clone(),
                            caller: caller.clone(),
                            origin: origin.clone(),
                        })
                    };
                    actions.push(action);
                }
            }
        }
    }

    // Anything settled long enough ago that nothing can still be waiting to
    // read it. Without this the map keeps an entry for every message that was
    // ever acknowledged, since only a message that was lost is taken away.
    let settled_grace =
        Duration::from_millis(message_timeout_ms * u64::from(SETTLED_GRACE_MULTIPLIER));
    acks_guard.retain(|_, status| match status.settled_at {
        Some(settled_at) => now.duration_since(settled_at) <= settled_grace,
        None => true,
    });

    // Release the lock before sending actions
    drop(acks_guard);

    for holding_node in holders_to_check {
        if message_action_tx
            .send(MessageAction::CheckHolder { holding_node })
            .await
            .is_err()
        {
            error!("receiver dropped before asking whether a node is still running");
        }
    }

    for action in actions {
        let message_id = match &action {
            MessageAction::Resend(ResendMessageInfo { message_id, .. }) => message_id.clone(),
            MessageAction::Lost { message_id, .. } => message_id.clone(),
            MessageAction::Delivered { origin } => origin.message_id.clone(),
            // Asked about a node rather than about one message, and answered
            // by the caller rather than settling anything here.
            MessageAction::CheckHolder { .. } => String::new(),
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
                    origin: None,
                },
                attempts: 0,
                // Long enough ago that it is due to be retried.
                last_attempt_time: Some(Instant::now() - Duration::from_secs(60)),
                taken_on: None,
                settled_at: None,
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
            origin: None,
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
                settled_at: None,
            },
        )])))
    }

    /// Only a lost message used to be taken out of the map, so every message
    /// that was acknowledged left an entry behind for as long as the node ran.
    #[test_log::test(tokio::test)]
    async fn test_a_settled_message_is_swept_once_nothing_can_be_waiting_on_it() {
        let acks = settled_since(Duration::from_millis(500), AckStatus::Received);
        let (action_tx, _action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 3).await;

        assert!(
            acks.lock().await.is_empty(),
            "a message settled long ago should not be kept"
        );
    }

    /// Swept too soon and a waiter finds nothing, which it can only read as the
    /// message having been lost. So it is kept for as long as a message may go
    /// unanswered, which is far longer than reading an outcome takes.
    #[test_log::test(tokio::test)]
    async fn test_a_message_just_settled_is_kept_for_whatever_is_still_reading_it() {
        let acks = settled_since(Duration::from_millis(0), AckStatus::Received);
        let (action_tx, _action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 3).await;

        assert_eq!(
            acks.lock().await.get("message-1").unwrap().status,
            AckStatus::Received,
            "a message only just settled should still be readable"
        );
    }

    /// A message waiting for its client is not swept however long it waits, or
    /// the wait would end by the entry going missing rather than by an
    /// outcome.
    #[test_log::test(tokio::test)]
    async fn test_a_message_still_waiting_is_never_swept() {
        let acks = pending_since(Duration::from_secs(60), Some(taken_on_now()));
        let (action_tx, _action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 3).await;

        assert!(acks.lock().await.contains_key("message-1"));
    }

    /// Declaring a message lost settles it rather than taking it away, so the
    /// check that follows must not declare it lost all over again.
    #[test_log::test(tokio::test)]
    async fn test_a_message_already_declared_lost_is_not_declared_lost_again() {
        let acks = pending_since(Duration::from_secs(60), None);
        let (action_tx, mut action_rx) = tokio::sync::mpsc::channel(8);

        check_for_actions_periodic(&acks, &action_tx, 100, 0).await;
        assert!(matches!(
            action_rx.try_recv(),
            Ok(MessageAction::Lost { .. })
        ));

        check_for_actions_periodic(&acks, &action_tx, 100, 0).await;

        assert!(action_rx.try_recv().is_err(), "a message is only lost once");
    }

    /// One message settled however long ago
    /// as determined by `since`.
    fn settled_since(
        since: Duration,
        status: AckStatus,
    ) -> Arc<Mutex<HashMap<String, DetailedAckStatus>>> {
        Arc::new(Mutex::new(HashMap::from([(
            "message-1".to_string(),
            DetailedAckStatus {
                status,
                attempts: 1,
                last_attempt_time: Some(Instant::now() - since),
                taken_on: None,
                settled_at: Some(Instant::now() - since),
            },
        )])))
    }

    #[test_log::test(tokio::test)]
    async fn test_settling_a_message_was_recorded_when_it_happened() {
        let mut worker = Worker::new(AckWorkerConfig::default());
        worker
            .record_ack("message-1".to_string(), pending_status())
            .await;
        assert!(
            worker.acks.lock().await["message-1"].settled_at.is_none(),
            "a message still waiting has not settled"
        );

        worker
            .record_ack("message-1".to_string(), AckStatus::Received)
            .await;

        assert!(worker.acks.lock().await["message-1"].settled_at.is_some());
    }

    #[test_log::test(tokio::test)]
    async fn test_a_client_settling_a_message_records_when_it_happened() {
        let mut worker = Worker::new(AckWorkerConfig::default());
        worker
            .record_ack("message-1".to_string(), pending_status())
            .await;

        worker
            .record_client_ack("message-1".to_string(), "connection-1".to_string())
            .await;

        let acks = worker.acks.lock().await;
        assert_eq!(acks["message-1"].status, AckStatus::Received);
        assert!(acks["message-1"].settled_at.is_some());
    }
}
