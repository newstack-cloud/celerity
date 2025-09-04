use celerity_helpers::redis::ConnectionWrapper;
use redis::RedisError;

/// Provides a mechanism to acquire and release a lock for
/// trimming a stream for a consumer.
/// This is useful for ensuring that only one consumer attempts to
/// trim the stream at a time.
/// This is a best effort lock mechanism as duplicate stream trims or
/// occassional failure to trim the stream are non-critical issues.
#[derive(Debug)]
pub struct TrimLock {
    service_name: String,
    consumer_name: String,
    stream: String,
    connection: ConnectionWrapper,
}

impl TrimLock {
    pub fn new(
        service_name: String,
        consumer_name: String,
        stream: String,
        connection: ConnectionWrapper,
    ) -> Self {
        Self {
            service_name,
            consumer_name,
            stream,
            connection,
        }
    }

    fn get_lock_key(&self) -> String {
        format!("celerity:consumer:{stream}:trim_lock", stream = self.stream)
    }

    fn get_lock_value(&self) -> String {
        format!(
            "{service_name}:{consumer_name}",
            service_name = self.service_name,
            consumer_name = self.consumer_name
        )
    }

    /// Acquires the lock for the stream that the TrimLock instance is associated with.
    /// This will return true if the lock was acquired, false if the lock already exists.
    pub async fn acquire(&mut self, lock_timeout_ms: u64) -> Result<bool, RedisError> {
        self.connection
            .pset_ex_nx(
                &self.get_lock_key(),
                &self.get_lock_value(),
                lock_timeout_ms,
            )
            .await
    }

    /// Releases the lock for the stream that the TrimLock instance is associated with.
    /// This will only succeed if the current consumer is the owner of the lock.
    /// This will return true if the lock was released, false if the lock does not exist
    /// or is owned by a different consumer.
    pub async fn release(&mut self) -> Result<bool, RedisError> {
        let release_lock_script = include_str!("../lua-scripts/release_stream_trim_lock.lua");

        let result: i32 = self
            .connection
            .eval_script(
                release_lock_script,
                &[&self.get_lock_key()],
                &[&self.get_lock_value()],
            )
            .await?;

        Ok(result == 1)
    }
}
