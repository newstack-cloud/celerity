use std::sync::Arc;

use async_trait::async_trait;
use celerity_helpers::redis::ConnectionWrapper;
use celerity_ws_registry::{errors::WebSocketConnError, registry::ForwardedMessageStore};

/// How long a message id is remembered for, which is how far apart two copies
/// of a message can arrive and still be recognised as the same one.
///
/// Has to outlast a message's whole life as a sender sees it, which is the
/// acknowledgement timeout multiplied by the attempts allowed. Five minutes
/// leaves room for both to be raised well beyond their defaults.
pub const DEFAULT_FORWARDED_TTL_MS: u64 = 300_000;

/// What the cluster has already forwarded to its clients.
///
/// A sender resends a message whose acknowledgement did not arrive, and what
/// went missing may have been the acknowledgement rather than the message.
/// Recording what has been forwarded is how the second copy is recognised.
#[derive(Debug)]
pub struct ForwardedMessages {
    conn: ConnectionWrapper,
    key_prefix: String,
    ttl_ms: u64,
}

impl ForwardedMessages {
    pub fn new(conn: ConnectionWrapper, key_prefix: String, ttl_ms: u64) -> Arc<Self> {
        Arc::new(Self {
            conn,
            key_prefix,
            ttl_ms,
        })
    }

    fn key(&self, message_id: &str) -> String {
        format!("{}:msg:{}", self.key_prefix, message_id)
    }
}

#[async_trait]
impl ForwardedMessageStore for ForwardedMessages {
    /// Sets the id only where it is not already there, which answers and records
    /// in one round trip.
    ///
    /// A second copy leaves the expiry where it was, so an id is forgotten a
    /// fixed time after the message was first forwarded rather than being kept
    /// alive by the resends.
    async fn record_and_check_forwarded(
        &self,
        message_id: &str,
    ) -> Result<bool, WebSocketConnError> {
        let recorded = self
            .conn
            .clone()
            .pset_ex_nx(&self.key(message_id), "1", self.ttl_ms)
            .await
            .map_err(|err| {
                WebSocketConnError::ForwardedMessageError(format!(
                    "could not record a message as forwarded: {err}"
                ))
            })?;

        Ok(!recorded)
    }
}
