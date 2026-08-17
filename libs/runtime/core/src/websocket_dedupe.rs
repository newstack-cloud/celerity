use std::{
    collections::HashMap,
    sync::{Arc, RwLock},
    time::{Duration, Instant},
};

use async_trait::async_trait;
use tracing::debug;

/// How long a message id is remembered for, which is how far apart two copies
/// of a message can arrive and still be recognised as the same one.
pub const DEFAULT_MESSAGE_ID_TTL_MS: u64 = 300_000;

/// How often expired ids are cleaned up. Expiry is decided by the timestamp
/// rather than by the clean up, so this only bounds how long a dead entry keeps
/// its memory, not how long it counts as seen.
const EVICTION_INTERVAL_MS: u64 = 60_000;

/// Remembers the messages a client has already sent, so the same one is not
/// acted on twice.
///
/// A client resends when it does not receive an acknowledgement in time, and the
/// message that went missing may have been the acknowledgement rather than the
/// message. The second copy is a delivery the runtime already has.
#[async_trait]
pub trait MessageIdStore: Send + Sync + std::fmt::Debug {
    /// Records a message id and says whether it was already there.
    ///
    /// One call rather than a check and a write, so two copies of a message
    /// arriving together cannot both find it absent and both be processed.
    async fn record_and_check_seen(&self, message_id: &str) -> bool;
}

/// The store for a single node, which is every deployment that is not a
/// cluster.
///
/// A cluster needs a shared one, since a client that reconnects may land on a
/// different node and resend there, and what this node holds in-memory says nothing about
/// what another one has seen.
#[derive(Debug)]
pub struct InMemoryMessageIdStore {
    seen: Arc<RwLock<HashMap<String, Instant>>>,
    ttl: Duration,
}

impl InMemoryMessageIdStore {
    pub fn new(ttl_ms: u64) -> Self {
        Self {
            seen: Arc::new(RwLock::new(HashMap::new())),
            ttl: Duration::from_millis(ttl_ms),
        }
    }

    /// Starts the sweep that keeps the store from growing without limit.
    ///
    /// Spawns, so it must be called with a runtime running. Nothing depends on
    /// it for correctness, since an entry past its time is treated as absent
    /// whether or not it has been swept.
    pub fn start_eviction(self: Arc<Self>) {
        tokio::spawn(async move {
            let mut interval = tokio::time::interval(Duration::from_millis(EVICTION_INTERVAL_MS));
            loop {
                interval.tick().await;
                self.evict_expired();
            }
        });
    }

    fn evict_expired(&self) {
        let now = Instant::now();
        let mut seen = match self.seen.write() {
            Ok(seen) => seen,
            Err(poisoned) => poisoned.into_inner(),
        };
        let before = seen.len();
        seen.retain(|_, recorded_at| now.duration_since(*recorded_at) < self.ttl);
        let removed = before - seen.len();
        if removed > 0 {
            debug!("swept {removed} expired message ids, {} remain", seen.len());
        }
    }
}

#[async_trait]
impl MessageIdStore for InMemoryMessageIdStore {
    async fn record_and_check_seen(&self, message_id: &str) -> bool {
        let now = Instant::now();
        let mut seen = match self.seen.write() {
            Ok(seen) => seen,
            Err(poisoned) => poisoned.into_inner(),
        };

        match seen.insert(message_id.to_string(), now) {
            // Past its time, so this is a new message that happens to reuse an
            // id the store had not got around to sweeping.
            Some(recorded_at) => now.duration_since(recorded_at) < self.ttl,
            None => false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test_log::test(tokio::test)]
    async fn test_a_message_is_new_the_first_time_and_seen_the_second() {
        let store = InMemoryMessageIdStore::new(DEFAULT_MESSAGE_ID_TTL_MS);

        assert!(!store.record_and_check_seen("m-1").await);
        assert!(store.record_and_check_seen("m-1").await);
        assert!(store.record_and_check_seen("m-1").await);
    }

    #[test_log::test(tokio::test)]
    async fn test_messages_are_told_apart_by_their_id() {
        let store = InMemoryMessageIdStore::new(DEFAULT_MESSAGE_ID_TTL_MS);

        assert!(!store.record_and_check_seen("m-1").await);
        assert!(!store.record_and_check_seen("m-2").await);
    }

    /// An id past its time is a new message again, which is what makes the
    /// store bounded rather than a record of everything a client ever sent.
    #[test_log::test(tokio::test)]
    async fn test_an_id_is_forgotten_once_its_time_is_up() {
        let store = InMemoryMessageIdStore::new(50);

        assert!(!store.record_and_check_seen("m-1").await);
        tokio::time::sleep(Duration::from_millis(80)).await;
        assert!(
            !store.record_and_check_seen("m-1").await,
            "an id past its time should be treated as one that was never seen"
        );
    }

    /// Expiry is decided by the entry's own timestamp, so it holds whether or
    /// not the sweep has run.
    #[test_log::test(tokio::test)]
    async fn test_sweeping_removes_what_has_expired_and_keeps_what_has_not() {
        let store = InMemoryMessageIdStore::new(50);

        store.record_and_check_seen("old").await;
        tokio::time::sleep(Duration::from_millis(80)).await;
        store.record_and_check_seen("new").await;

        store.evict_expired();

        let seen = store.seen.read().unwrap();
        assert!(!seen.contains_key("old"), "an expired id should be swept");
        assert!(seen.contains_key("new"), "a live id should be kept");
    }
}
