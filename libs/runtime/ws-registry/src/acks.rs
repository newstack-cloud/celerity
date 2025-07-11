use std::{collections::HashMap, time::Duration};

use std::sync::Arc;
use tokio::{
    sync::{mpsc::Receiver, oneshot::Sender, Mutex},
    time::Instant,
};
use tracing::{debug, error, info, info_span, Instrument};

use crate::types::AckWorkerConfig;

/// The default interval at which to check for actions based on ack statuses.
pub const DEFAULT_MESSAGE_ACTION_CHECK_INTERVAL_MS: u64 = 10000;

/// The default timeout in milliseconds for which the caller should consider re-sending
/// the message if it has not been acknowledged.
pub const DEFAULT_MESSAGE_TIMEOUT_MS: u64 = 15000;

/// The default number of times that a message should be attempted to be sent before it is
/// considered lost.
pub const DEFAULT_MAX_ATTEMPTS: u32 = 4;

/// The default interval in milliseconds to check for the acknowledgement status of a message.
pub const ACK_WAIT_CHECK_INTERVAL_MS: u64 = 20;

#[derive(Clone, Debug, PartialEq)]
pub enum AckStatus {
    // The message has been sent but no acknowledgement
    // has been received yet.
    Pending(String, Vec<String>),
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
    pub message: String,
    pub inform_clients_on_loss: Vec<String>,
}

#[derive(Clone, Debug, PartialEq)]
pub enum MessageAction {
    // The message should be re-sent by the caller along with a list of clients that should be
    // informed of the message being lost in the future.
    Resend(ResendMessageInfo),
    // The message should be considered lost and the caller should be informed that the message
    // was lost.
    Lost(String, Vec<String>),
}

pub enum AckWorkerMessage {
    AckStatus(String, AckStatus),
    AckCheck(String, Sender<AckStatus>),
    AckWait(String, Sender<AckStatus>),
}

#[derive(Clone, Debug, PartialEq)]
struct DetailedAckStatus {
    status: AckStatus,
    attempts: u32,
    last_attempt_time: Option<Instant>,
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
        Self {
            acks: Arc::new(Mutex::new(HashMap::new())),
            message_action_check_interval_ms: config
                .message_action_check_interval_ms
                .unwrap_or(DEFAULT_MESSAGE_ACTION_CHECK_INTERVAL_MS),
            message_timeout_ms: config
                .message_timeout_ms
                .unwrap_or(DEFAULT_MESSAGE_TIMEOUT_MS),
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
                        Some(AckWorkerMessage::AckStatus(message_id, ack_status)) => {
                            self.record_ack(message_id, ack_status).await;
                        }
                        Some(AckWorkerMessage::AckCheck(message_id, tx)) => {
                            let acks_guard = self.acks.lock().await;
                            let detailed_ack_status = acks_guard.get(&message_id).cloned();
                            let final_ack_status = match detailed_ack_status {
                                Some(detailed_ack_status) => detailed_ack_status.status,
                                None => AckStatus::Lost,
                            };
                            if tx.send(final_ack_status).is_err() {
                                error!(
                                    "sender dropped before receiving acknowledgement \
                                    status for message {}",
                                    message_id
                                );
                            }
                        }
                        Some(AckWorkerMessage::AckWait(message_id, tx)) => {
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

        let new_detailed_ack_status = if matches!(ack_status, AckStatus::Pending(_, _)) {
            // Only increment the attempts if the message is still pending.
            DetailedAckStatus {
                status: ack_status,
                attempts: existing_ack_status.map_or(0, |s| s.attempts) + 1,
                last_attempt_time: Some(Instant::now()),
            }
        } else {
            DetailedAckStatus {
                status: ack_status,
                attempts: existing_ack_status.as_ref().map_or(0, |s| s.attempts),
                last_attempt_time: existing_ack_status
                    .as_ref()
                    .and_then(|s| s.last_attempt_time),
            }
        };

        acks_guard.insert(message_id, new_detailed_ack_status);
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
        if let AckStatus::Pending(message, client_ids) = &detailed_ack_status.status {
            if let Some(last_attempt_time) = detailed_ack_status.last_attempt_time {
                if now.duration_since(last_attempt_time) > Duration::from_millis(message_timeout_ms)
                {
                    let action = if detailed_ack_status.attempts >= max_attempts {
                        MessageAction::Lost(message_id.clone(), client_ids.clone())
                    } else {
                        MessageAction::Resend(ResendMessageInfo {
                            client_id: message_id.clone(),
                            message_id: message_id.clone(),
                            message: message.clone(),
                            inform_clients_on_loss: client_ids.clone(),
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
            MessageAction::Resend(ResendMessageInfo {
                message_id,
                client_id: _,
                message: _,
                inform_clients_on_loss: _,
            }) => message_id.clone(),
            MessageAction::Lost(message_id, _) => message_id.clone(),
        };
        let is_lost = matches!(action, MessageAction::Lost(_, _));

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
            let is_pending = matches!(detailed_ack_status.status, AckStatus::Pending(_, _));
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
