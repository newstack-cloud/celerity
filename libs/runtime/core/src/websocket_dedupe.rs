use std::{
    collections::HashMap,
    sync::{Arc, OnceLock, RwLock},
    time::{Duration, Instant},
};

use async_trait::async_trait;
use tracing::debug;

#[cfg(feature = "ws_clustering")]
use celerity_helpers::redis::ConnectionWrapper;
#[cfg(feature = "ws_clustering")]
use tracing::error;

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

/// The store a connection asks, which is the shared one once a cluster has been
/// joined and this node's own memory until then.
///
/// The store cannot simply be chosen on construction, because reaching a shared one
/// means connecting to it, which needs a runtime, and the API is built before
/// there is one. So a connection holds this and the answer changes underneath
/// it when the cluster is joined.
#[derive(Debug)]
pub struct SeenMessages {
    local: Arc<InMemoryMessageIdStore>,
    shared: OnceLock<Arc<dyn MessageIdStore>>,
}

impl SeenMessages {
    pub fn new(ttl_ms: u64) -> Arc<Self> {
        Arc::new(Self {
            local: Arc::new(InMemoryMessageIdStore::new(ttl_ms)),
            shared: OnceLock::new(),
        })
    }

    /// Hands over to a store the rest of the cluster can see.
    ///
    /// Refused twice, since the second would answer for messages the first has
    /// already been told about. The store that was refused comes back, so a
    /// caller can say what it was holding.
    pub fn attach_shared(
        &self,
        shared: Arc<dyn MessageIdStore>,
    ) -> Result<(), Arc<dyn MessageIdStore>> {
        self.shared.set(shared)
    }

    /// Clears out ids this node has remembered past their time.
    ///
    /// Only what this node holds. A shared store expires its own entries.
    pub fn start_eviction(self: &Arc<Self>) {
        self.local.clone().start_eviction();
    }
}

#[async_trait]
impl MessageIdStore for SeenMessages {
    async fn record_and_check_seen(&self, message_id: &str) -> bool {
        match self.shared.get() {
            Some(shared) => shared.record_and_check_seen(message_id).await,
            None => self.local.record_and_check_seen(message_id).await,
        }
    }
}

/// The store holding what every node of a cluster has already
/// been sent by its clients.
///
/// A client resends when it does not hear that its message arrived, and a
/// client that reconnected first may resend to a different node. Only a record
/// the whole cluster can read recognises that as the message it already acted
/// on.
///
/// Kept apart from the record of what the cluster has forwarded to clients,
/// which is the same idea in the other direction. An application chooses its own
/// message ids in both, so one keyspace would let a client's id collide with a
/// server's.
#[cfg(feature = "ws_clustering")]
#[derive(Debug)]
pub struct SharedMessageIdStore {
    conn: ConnectionWrapper,
    key_prefix: String,
    ttl_ms: u64,
}

#[cfg(feature = "ws_clustering")]
impl SharedMessageIdStore {
    pub fn new(conn: ConnectionWrapper, key_prefix: String, ttl_ms: u64) -> Arc<Self> {
        Arc::new(Self {
            conn,
            key_prefix,
            ttl_ms,
        })
    }

    fn key(&self, message_id: &str) -> String {
        format!("{}:client-msg:{}", self.key_prefix, message_id)
    }
}

#[cfg(feature = "ws_clustering")]
#[async_trait]
impl MessageIdStore for SharedMessageIdStore {
    /// Sets the id only where it is not already there, which answers and
    /// records in one round trip.
    ///
    /// A store that cannot answer is taken as never having seen the message, so
    /// a failed lookup costs a message handled twice rather than a message a
    /// client sent and nothing acted on.
    async fn record_and_check_seen(&self, message_id: &str) -> bool {
        match self
            .conn
            .clone()
            .pset_ex_nx(&self.key(message_id), "1", self.ttl_ms)
            .await
        {
            Ok(recorded) => !recorded,
            Err(err) => {
                error!(
                    message_id = %message_id,
                    "could not tell whether this message has been seen before, handling it \
                     rather than dropping it: {err}"
                );
                false
            }
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

    /// Until a cluster is joined a node answers from its own memory, and
    /// afterwards from the store the rest of the cluster can see.
    #[test_log::test(tokio::test)]
    async fn test_the_shared_store_takes_over_once_it_is_attached() {
        let seen = SeenMessages::new(DEFAULT_MESSAGE_ID_TTL_MS);

        assert!(!seen.record_and_check_seen("m-1").await);
        assert!(
            seen.record_and_check_seen("m-1").await,
            "this node should recognise a message it has already handled"
        );

        seen.attach_shared(Arc::new(AlwaysNew)).unwrap();
        assert!(
            !seen.record_and_check_seen("m-1").await,
            "the shared store should be answering, not the memory of this node"
        );
        assert!(
            seen.attach_shared(Arc::new(AlwaysNew)).is_err(),
            "a second shared store would answer for messages the first was told about"
        );
    }

    /// Stands in for a shared store, and answers differently from the local one
    /// so a test can tell which was asked.
    #[derive(Debug)]
    struct AlwaysNew;

    #[async_trait]
    impl MessageIdStore for AlwaysNew {
        async fn record_and_check_seen(&self, _message_id: &str) -> bool {
            false
        }
    }
}
