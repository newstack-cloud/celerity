use std::{
    collections::HashSet,
    sync::{Arc, RwLock},
};

use async_trait::async_trait;
use celerity_helpers::redis::ConnectionWrapper;
use celerity_ws_registry::{errors::WebSocketConnError, registry::ConnectionLocationStore};
use futures::{stream, StreamExt};
use redis::RedisResult;
use tracing::debug;

/// Where each connection is, so a message can be published to the one node
/// group holding its client instead of to every node in the cluster.
///
/// Entries carry an expiry and are refreshed while this node runs, so a node
/// that dies without tidying up leaves nothing behind that would go on
/// attracting messages for connections that died with it.
/// How many of a node's entries are written at once.
const AT_ONCE: usize = 32;

#[derive(Debug)]
pub struct ConnectionLocations {
    conn: ConnectionWrapper,
    key_prefix: String,
    /// The group this node currently belongs to, which is what its connections
    /// are recorded against.
    group_id: RwLock<String>,
    ttl_ms: u64,
    /// The connections this node recorded, so it knows which entries are its to
    /// keep alive and to take away.
    recorded: RwLock<HashSet<String>>,
}

impl ConnectionLocations {
    pub fn new(
        conn: ConnectionWrapper,
        key_prefix: String,
        group_id: String,
        ttl_ms: u64,
    ) -> Arc<Self> {
        Arc::new(Self {
            conn,
            key_prefix,
            group_id: RwLock::new(group_id),
            ttl_ms,
            recorded: RwLock::new(HashSet::new()),
        })
    }

    /// Reads the group holding a connection, or `None` where nothing has been
    /// recorded for it.
    pub async fn group_for(&self, connection_id: &str) -> RedisResult<Option<String>> {
        let group_id = self.connection().get(&self.key(connection_id)).await?;
        Ok(Some(group_id).filter(|group_id| !group_id.is_empty()))
    }

    /// Pushes every entry this node holds out to a fresh expiry.
    ///
    /// One command per entry rather than one request carrying all of them,
    /// because connection entries are spread across a cluster's slots by design
    /// and a request spanning slots is refused. Several are in flight at once,
    /// so the cost is bounded by that rather than by how many entries this node
    /// holds.
    ///
    /// Written again rather than expired again, since the two cost the same and
    /// writing also repairs an entry that expired while the node was too busy
    /// to say otherwise.
    pub async fn refresh(&self) -> RedisResult<usize> {
        let recorded: Vec<String> = self.recorded.read().unwrap().iter().cloned().collect();
        if recorded.is_empty() {
            return Ok(0);
        }

        let group_id = self.group();
        let ttl_ms = self.ttl_ms;
        let conn = self.conn.clone();
        let results: Vec<RedisResult<()>> = stream::iter(
            recorded
                .iter()
                .map(|connection_id| self.key(connection_id))
                .collect::<Vec<String>>(),
        )
        .map(|key| {
            let mut conn = conn.clone();
            let group_id = group_id.clone();
            async move { conn.pset_ex(&key, &group_id, ttl_ms).await.map(|_| ()) }
        })
        .buffer_unordered(AT_ONCE)
        .collect()
        .await;

        let refreshed = settle(results)?;
        debug!(
            connections = refreshed,
            "refreshed this node's connection entries"
        );
        Ok(refreshed)
    }

    /// Takes away every entry this node holds, for a node that is shutting down
    /// rather than dying.
    pub async fn forget_all(&self) -> RedisResult<usize> {
        let recorded: Vec<String> = self.recorded.write().unwrap().drain().collect();
        if recorded.is_empty() {
            return Ok(0);
        }

        let conn = self.conn.clone();
        let results: Vec<RedisResult<()>> = stream::iter(
            recorded
                .iter()
                .map(|connection_id| self.key(connection_id))
                .collect::<Vec<String>>(),
        )
        .map(|key| {
            let mut conn = conn.clone();
            async move { conn.del(&key).await.map(|_| ()) }
        })
        .buffer_unordered(AT_ONCE)
        .collect()
        .await;

        settle(results)
    }

    /// Points this node's connections at a different group, for a node that had
    /// to rejoin.
    pub fn set_group(&self, group_id: String) {
        *self.group_id.write().unwrap() = group_id;
    }

    /// The group this node's connections are currently recorded against.
    pub fn group(&self) -> String {
        self.group_id.read().unwrap().clone()
    }

    fn key(&self, connection_id: &str) -> String {
        format!("{}:conn:{}", self.key_prefix, connection_id)
    }

    /// A connection of this task's own. Cloning a multiplexed connection shares
    /// the one socket, so this costs nothing and saves every lookup queueing
    /// behind a lock held by whoever asked first.
    fn connection(&self) -> ConnectionWrapper {
        self.conn.clone()
    }
}

#[async_trait]
impl ConnectionLocationStore for ConnectionLocations {
    async fn record(&self, connection_id: &str) -> Result<(), WebSocketConnError> {
        // Taken as this node's before the write, so a write that fails is
        // repaired by the next refresh rather than leaving a connection no
        // other node can reach for as long as it lasts.
        self.recorded
            .write()
            .unwrap()
            .insert(connection_id.to_string());
        let group_id = self.group();
        self.connection()
            .pset_ex(&self.key(connection_id), &group_id, self.ttl_ms)
            .await
            .map_err(location_error)?;
        Ok(())
    }

    async fn forget(&self, connection_id: &str) -> Result<(), WebSocketConnError> {
        // Forgotten here first, so a refresh that overlaps this cannot write
        // the entry back after it has been taken away.
        self.recorded.write().unwrap().remove(connection_id);
        self.connection()
            .del(&self.key(connection_id))
            .await
            .map_err(location_error)?;
        Ok(())
    }
}

fn location_error(err: redis::RedisError) -> WebSocketConnError {
    WebSocketConnError::ConnectionLocationError(err.to_string())
}

/// Counts what succeeded, and returns the first failure once everything has
/// been tried.
///
/// Each entry stands alone, so one failing is no reason to leave the rest
/// unwritten.
fn settle(results: Vec<RedisResult<()>>) -> RedisResult<usize> {
    let done = results.iter().filter(|result| result.is_ok()).count();
    for result in results {
        result?;
    }
    Ok(done)
}
