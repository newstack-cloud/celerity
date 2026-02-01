use std::{sync::Arc, time::Duration};

use redis::RedisError;
use tokio::{
    sync::{
        oneshot::{channel, Sender},
        Mutex,
    },
    time,
};
use tracing::{debug, error, info, info_span, instrument, Instrument};

use crate::locks::MessageLocks;

/// Provides an implementation for a lock duration extender
/// which extends the duration of amessage lock that acts as the window
/// for which one or more messages are hidden from other consumers of a
/// Redis stream during processing.
#[derive(Debug)]
pub struct LockDurationExtender {
    locks: Arc<Mutex<MessageLocks>>,
    config: LockDurationExtenderConfig,
}

#[derive(Debug)]
pub struct LockDurationExtenderConfig {
    pub lock_duration_ms: u64,
    pub heartbeat_interval: u64,
}

impl LockDurationExtender {
    pub fn new(locks: Arc<Mutex<MessageLocks>>, config: LockDurationExtenderConfig) -> Self {
        Self { locks, config }
    }

    #[instrument(name = "heartbeat_initialiser", skip(self))]
    pub fn start_heartbeat(self: Arc<Self>, message_ids: Vec<String>) -> Option<Sender<()>> {
        let heartbeat_runner_task_span = info_span!("heartbeat_runner_task");
        let (send, recv) = channel::<()>();
        tokio::spawn({
            // self will be used concurrently by different tasks so we need
            // to use the Arc synchronisation primitive for sharing self.
            // https://doc.rust-lang.org/std/sync/struct.Arc.html
            let me = Arc::clone(&self);
            async move {
                tokio::select! {
                    _ = me.run_heartbeat_task(me.config.heartbeat_interval, message_ids.clone()) => {},
                    _ = recv => {}
                }
            }
        }.instrument(heartbeat_runner_task_span));
        Some(send)
    }

    async fn run_heartbeat_task(&self, heartbeat_interval: u64, message_ids: Vec<String>) {
        let mut interval = time::interval(Duration::from_secs(heartbeat_interval));
        loop {
            interval.tick().await;
            debug!(
                "{} seconds have passed, extending lock duration",
                heartbeat_interval
            );
            let result = self.extend_lock_durations(&message_ids).await;
            if result.is_err() {
                let err = result.err().unwrap();
                let err_message = err.to_string();
                error!("error extending lock durations: {}", err_message)
            } else {
                // TODO: zip message_ids and results to and report on message IDs that succeeded and failed
                info!("extended lock durations!")
            }
        }
    }

    pub async fn extend_lock_durations(
        &self,
        message_ids: &[String],
    ) -> Result<Vec<bool>, RedisError> {
        let mut message_locks = self.locks.lock().await;
        message_locks
            .extend_locks(
                &message_ids
                    .iter()
                    .map(String::as_str)
                    .collect::<Vec<&str>>(),
                self.config.lock_duration_ms,
            )
            .await
    }

    pub async fn release_locks(&self, message_ids: &[String]) -> Result<Vec<bool>, RedisError> {
        let mut message_locks = self.locks.lock().await;
        message_locks
            .release_locks(
                &message_ids
                    .iter()
                    .map(String::as_str)
                    .collect::<Vec<&str>>(),
            )
            .await
    }
}
