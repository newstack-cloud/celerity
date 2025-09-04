use std::fmt::Debug;

use redis::{
    aio::MultiplexedConnection,
    cluster::ClusterClientBuilder,
    cluster_async::ClusterConnection,
    streams::{StreamReadOptions, StreamReadReply, StreamTrimOptions, StreamTrimmingMode},
    AsyncCommands, Client, FromRedisValue, Pipeline, PushInfo, RedisResult, ToRedisArgs,
};
use tokio::sync::mpsc::UnboundedSender;

/// Configuration for a Redis connection.
#[derive(Debug, Clone)]
pub struct ConnectionConfig {
    /// The nodes to use to connect to the Redis cluster or instance.
    pub nodes: Vec<String>,
    /// The password to use to connect to the Redis cluster or instance.
    pub password: Option<String>,
    /// Whether to use cluster mode for the Redis connection.
    pub cluster_mode: bool,
}

/// A simplified choice of strategies for the xtrim command.
#[derive(Debug, Clone)]
pub enum StreamTrimStrategy {
    MaxLen(usize),
    MinId(String),
}

/// A wrapper around a Redis connection that can be used to
/// get a connection to a Redis cluster or instance.
/// This provides a unified interface for both single node and cluster mode connections
/// for a subset of Redis commands used by the Celerity runtime.
pub enum ConnectionWrapper {
    Cluster(ClusterConnection),
    SingleNode(MultiplexedConnection),
}

impl Debug for ConnectionWrapper {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            ConnectionWrapper::Cluster(_) => {
                write!(f, "ConnectionWrapper::Cluster")
            }
            ConnectionWrapper::SingleNode(_) => write!(f, "ConnectionWrapper::SingleNode"),
        }
    }
}

impl Clone for ConnectionWrapper {
    fn clone(&self) -> Self {
        match self {
            ConnectionWrapper::Cluster(conn) => ConnectionWrapper::Cluster(conn.clone()),
            ConnectionWrapper::SingleNode(conn) => ConnectionWrapper::SingleNode(conn.clone()),
        }
    }
}

impl ConnectionWrapper {
    pub async fn get(&mut self, key: &str) -> RedisResult<String> {
        let last_message_id: Option<String> = match self {
            ConnectionWrapper::Cluster(conn) => conn.get(key).await?,
            ConnectionWrapper::SingleNode(conn) => conn.get(key).await?,
        };
        Ok(last_message_id.unwrap_or_default())
    }

    /// Set the value and expiration of a key, in milliseconds.
    /// [Redis Docs](https://redis.io/commands/SET)
    pub async fn pset_ex(&mut self, key: &str, value: &str, expire_ms: u64) -> RedisResult<bool> {
        let result: Option<String> = match self {
            ConnectionWrapper::Cluster(conn) => conn.pset_ex(key, value, expire_ms).await?,
            ConnectionWrapper::SingleNode(conn) => conn.pset_ex(key, value, expire_ms).await?,
        };

        Ok(result.is_some())
    }

    /// Set the value and expiration of a key, in milliseconds, only if the key does not exist.
    /// [Redis Docs](https://redis.io/commands/SET).
    /// This maps to the `SET` command with the `NX` and `PX` options.
    ///
    /// Returns `true` if the key was set, `false` if the key already existed.
    pub async fn pset_ex_nx(
        &mut self,
        key: &str,
        value: &str,
        expire_ms: u64,
    ) -> RedisResult<bool> {
        let mut cmd = redis::cmd("SET");
        cmd.arg(key)
            .arg(value)
            .arg("NX") // Only set if key doesn't exist
            .arg("PX") // Expire in milliseconds
            .arg(expire_ms); // Convert seconds to milliseconds

        let result: Option<String> = match self {
            ConnectionWrapper::Cluster(conn) => cmd.query_async(conn).await?,
            ConnectionWrapper::SingleNode(conn) => cmd.query_async(conn).await?,
        };

        Ok(result.is_some())
    }

    /// Evaluates a Lua script.
    /// [Redis Docs](https://redis.io/commands/EVAL)
    pub async fn eval_script<T: Default + FromRedisValue>(
        &mut self,
        script: &str,
        keys: &[&str],
        args: &[&str],
    ) -> RedisResult<T> {
        if keys.is_empty() {
            return Ok(T::default());
        }

        let script_obj = redis::Script::new(script);
        let mut script_invocation = &mut script_obj.key(keys[0]);
        for key in keys.iter().skip(1) {
            script_invocation = script_invocation.key(key);
        }

        for arg in args {
            script_invocation = script_invocation.arg(arg);
        }

        let expected: T = match self {
            ConnectionWrapper::Cluster(conn) => script_invocation.invoke_async(conn).await?,
            ConnectionWrapper::SingleNode(conn) => script_invocation.invoke_async(conn).await?,
        };

        Ok(expected)
    }

    /// Executes a pipeline of commands asynchronously.
    /// [Redis Docs](https://redis.io/docs/latest/reference/pipelining/)
    pub async fn query_pipeline_async(
        &mut self,
        pipeline: &mut Pipeline,
    ) -> RedisResult<Vec<Option<String>>> {
        match self {
            ConnectionWrapper::Cluster(conn) => pipeline.query_async(conn).await,
            ConnectionWrapper::SingleNode(conn) => pipeline.query_async(conn).await,
        }
    }

    /// Reads messages from the specified streams.
    /// [Redis Docs](https://redis.io/commands/XREAD)
    pub async fn xread(
        &mut self,
        streams: &[&str],
        offset_ids: &[&str],
        count: usize,
        block_time_ms: usize,
    ) -> RedisResult<StreamReadReply> {
        let options = StreamReadOptions::default()
            .count(count)
            .block(block_time_ms);

        match self {
            ConnectionWrapper::Cluster(conn) => {
                conn.xread_options(streams, offset_ids, &options).await
            }
            ConnectionWrapper::SingleNode(conn) => {
                conn.xread_options(streams, offset_ids, &options).await
            }
        }
    }

    /// Adds a message to the specified stream.
    /// [Redis Docs](https://redis.io/commands/XADD)
    pub async fn xadd<V: ToRedisArgs + Send + Sync>(
        &mut self,
        stream_name: &str,
        id: &str,
        values: &[(&str, V)],
    ) -> RedisResult<()> {
        match self {
            ConnectionWrapper::Cluster(conn) => conn.xadd(stream_name, id, values).await,
            ConnectionWrapper::SingleNode(conn) => conn.xadd(stream_name, id, values).await,
        }
    }

    /// Trims the specified stream with the specified strategy.
    /// [Redis Docs](https://redis.io/commands/XTRIM)
    pub async fn xtrim(
        &mut self,
        stream_name: &str,
        trim_strategy: StreamTrimStrategy,
    ) -> RedisResult<()> {
        let options = match trim_strategy {
            StreamTrimStrategy::MaxLen(max_length) => {
                StreamTrimOptions::maxlen(StreamTrimmingMode::Exact, max_length)
            }
            StreamTrimStrategy::MinId(min_id) => {
                StreamTrimOptions::minid(StreamTrimmingMode::Exact, min_id)
            }
        };

        match self {
            ConnectionWrapper::Cluster(conn) => conn.xtrim_options(stream_name, &options).await,
            ConnectionWrapper::SingleNode(conn) => conn.xtrim_options(stream_name, &options).await,
        }
    }

    /// Returns the number of messages in the specified stream.
    /// [Redis Docs](https://redis.io/commands/XLEN)
    pub async fn xlen(&mut self, stream_name: &str) -> RedisResult<usize> {
        match self {
            ConnectionWrapper::Cluster(conn) => conn.xlen(stream_name).await,
            ConnectionWrapper::SingleNode(conn) => conn.xlen(stream_name).await,
        }
    }

    /// Subscribes to a new channel(s).
    ///
    /// Updates from the sender will be sent on the push sender that was passed to the manager.
    /// If the manager was configured without a push sender, the connection won't be able to pass messages back to the user.
    ///
    /// This method is only available when the connection is using RESP3 protocol, and will return an error otherwise.
    /// It should be noted that the subscription will be automatically resubscribed after disconnections, so the user might
    /// receive additional pushes with [crate::PushKind::Subscribe], later after the subscription completed.
    pub async fn subscribe(&mut self, channel_name: &str) -> RedisResult<()> {
        match self {
            ConnectionWrapper::Cluster(conn) => conn.subscribe(channel_name).await,
            ConnectionWrapper::SingleNode(conn) => conn.subscribe(channel_name).await,
        }
    }

    /// Posts a message to the given channel.
    /// [Redis Docs](https://redis.io/commands/PUBLISH)
    pub async fn publish(&mut self, channel_name: &str, message: String) -> RedisResult<i32> {
        match self {
            ConnectionWrapper::Cluster(conn) => conn.publish(channel_name, message).await,
            ConnectionWrapper::SingleNode(conn) => conn.publish(channel_name, message).await,
        }
    }
}

/// Creates a connection to a Redis cluster or instance.
///
/// If a `redis_tx` is provided, the connection will be configured to
/// use the `PushInfo` sender to push messages to the Redis server.
/// This is useful for implementing a pub/sub pattern.
///
/// If a `redis_tx` is not provided, the connection will be configured
/// to use the default Redis connection configuration.
pub async fn get_redis_connection(
    conn_config: &ConnectionConfig,
    redis_tx: Option<UnboundedSender<PushInfo>>,
) -> RedisResult<ConnectionWrapper> {
    if !conn_config.cluster_mode {
        let client = Client::open(conn_config.nodes[0].clone())?;
        let mut config = redis::AsyncConnectionConfig::new();
        if let Some(redis_tx) = redis_tx {
            config = config.set_push_sender(redis_tx);
        }
        return Ok(ConnectionWrapper::SingleNode(
            client
                .get_multiplexed_async_connection_with_config(&config)
                .await?,
        ));
    }
    let mut builder = ClusterClientBuilder::new(conn_config.nodes.clone())
        .use_protocol(redis::ProtocolVersion::RESP3);

    if let Some(password) = conn_config.password.clone() {
        builder = builder.password(password);
    }

    let client = if let Some(redis_tx) = redis_tx {
        builder.push_sender(redis_tx).build()?
    } else {
        builder.build()?
    };

    Ok(ConnectionWrapper::Cluster(
        client.get_async_connection().await?,
    ))
}
