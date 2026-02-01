use celerity_helpers::redis::ConnectionWrapper;
use redis::RedisError;

/// Provides a mechanism to acquire and release locks for messages
/// that are being processed by a consumer.
/// This is useful for ensuring that each message is processed by only one
/// instance of a consumer at a time.
/// This is designed to act like a visibility timeout for a service such as SQS.
#[derive(Debug)]
pub struct MessageLocks {
    service_name: String,
    consumer_name: String,
    connection: ConnectionWrapper,
}

impl MessageLocks {
    /// Creates a new instance of the MessageLocks struct.
    ///
    /// # Arguments
    ///
    /// * `service_name` - The name of the service that is using the message locks, this should be used for all workers
    ///   in the same node and across all nodes in a cluster of nodes for the same service.
    /// * `consumer_name` - The name of the consumer that is using the message locks.
    /// * `redis_conn` - The Redis connection to use to store the message locks.
    pub fn new(service_name: String, consumer_name: String, redis_conn: ConnectionWrapper) -> Self {
        Self {
            service_name,
            consumer_name,
            connection: redis_conn,
        }
    }

    fn get_lock_key(&self, message_id: &str) -> String {
        // The service name is used as a hash tag to ensure that all message locks
        // for the same service are allocated in the same hash slot,
        // allowing for efficiently acquiring, extending and releasing locks
        // in batches for large batches of messages received by a consumer.
        format!(
            "celerity:consumer:{{{service_name}}}:lock:{message_id}",
            service_name = self.service_name
        )
    }

    /// Acquires locks for a given set of messages.
    ///
    /// This method will set a key in Redis with the message ID as the key and the current timestamp
    /// as the value for each message.
    /// The keys will expire after the lock duration.
    ///
    /// A new message lock will be acquired only if the key does not exist.
    ///
    /// This will return a list of booleans in the same order as the provided
    /// message IDs to indicate which message locks were acquired.
    pub async fn acquire_locks(
        &mut self,
        message_ids: &[&str],
        lock_duration_ms: u64,
    ) -> Result<Vec<bool>, RedisError> {
        if message_ids.is_empty() {
            return Ok(vec![]);
        }

        let mut pipeline = redis::pipe();

        for message_id in message_ids {
            pipeline
                .cmd("SET")
                .arg(self.get_lock_key(message_id))
                .arg(&self.consumer_name)
                .arg("NX")
                .arg("PX")
                .arg(lock_duration_ms);
        }

        let results: Vec<Option<String>> =
            self.connection.query_pipeline_async(&mut pipeline).await?;

        Ok(results.iter().map(|result| result.is_some()).collect())
    }

    /// Extends the locks for a given set of messages if the current consumer
    /// owns the current lock.
    pub async fn extend_locks(
        &mut self,
        message_ids: &[&str],
        lock_duration_ms: u64,
    ) -> Result<Vec<bool>, RedisError> {
        let extend_lock_script = include_str!("../lua-scripts/extend_locks.lua");

        let keys: Vec<String> = message_ids.iter().map(|id| self.get_lock_key(id)).collect();

        let extended: Vec<i32> = self
            .connection
            .eval_script(
                extend_lock_script,
                &keys.iter().map(String::as_ref).collect::<Vec<&str>>(),
                &[&self.consumer_name, &lock_duration_ms.to_string()],
            )
            .await?;

        Ok(extended.into_iter().map(|result| result == 1).collect())
    }

    /// Releases the locks for a given set of messages if the current consumer
    /// owns the current lock.
    pub async fn release_locks(&mut self, message_ids: &[&str]) -> Result<Vec<bool>, RedisError> {
        let release_lock_script = include_str!("../lua-scripts/release_locks.lua");

        let keys: Vec<String> = message_ids.iter().map(|id| self.get_lock_key(id)).collect();

        let released: Vec<i32> = self
            .connection
            .eval_script(
                release_lock_script,
                &keys.iter().map(String::as_ref).collect::<Vec<&str>>(),
                &[&self.consumer_name],
            )
            .await?;

        Ok(released.into_iter().map(|result| result == 1).collect())
    }
}
