use std::error::Error;

use celerity_ws_registry::types::WebSocketMessage;
use redis::{
    aio::MultiplexedConnection, cluster::ClusterClientBuilder, cluster_async::ClusterConnection,
    AsyncCommands, Client, FromRedisValue, PushInfo, PushKind, RedisError, RedisResult,
};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc::{channel, unbounded_channel, Receiver, Sender, UnboundedSender};
use tracing::{debug, error};

/// Configuration for a Redis connection for the pubsub channel
/// used with the WebSocket connection registry for providing
/// a horizontally scalable WebSocket API that shares messages
/// between nodes in a cluster for a shared multi-client session (e.g. real-time chat).
#[derive(Debug, Clone)]
pub struct ConnectionConfig {
    // A name for the current server node,
    // primarily used to filter out messages that are not for connections
    // on the current node.
    pub server_node_name: String,
    pub channel_name: String,
    pub nodes: Vec<String>,
    pub password: Option<String>,
    pub cluster_mode: bool,
}

/// Connects to a Redis server or cluster pubsub channel and returns a
/// receiver and sender to allow for sharing messages between nodes in a cluster
/// for a WebSocket API.
///
/// This should be used for both publishing messages and subscribing to
/// messages.
///
/// When cluster mode is disabled, only the first node in the provided
/// nodes list will be used.
///
/// # Examples
///
/// **Cluster mode**
///
/// ```
/// # use celerity_ws_redis::pubsub::connect;
///
/// let (tx, rx) = connect(ConnectionConfig {
///     server_node_name: "api-node-1".to_string(),
///     channel_name: "celerity_ws_messages".to_string(),
///     nodes: vec!["redis://127.0.0.1:6379/?protocol=3".to_string()],
///     password: None,
///     cluster_mode: true,
/// })?;
/// let registry = WebSocketConnRegistry::new(Some(tx));
/// registry.listen(rx);
/// ```
///
/// **Single node mode**
/// ```
/// # use celerity_ws_redis::pubsub::connect;
///
/// let (tx, rx) = connect(ConnectionConfig {
///     server_node_name: "api-node-1".to_string(),
///     channel_name: "celerity_ws_messages".to_string(),
///     nodes: vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()],
///     password: None,
///     cluster_mode: false,
/// })?;
/// let registry = WebSocketConnRegistry::new(Some(tx));
/// registry.listen(rx);
/// ```
pub async fn connect(
    conn_config: ConnectionConfig,
) -> Result<(Sender<WebSocketMessage>, Receiver<WebSocketMessage>), Box<dyn Error>> {
    let (redis_tx, mut redis_rx) = unbounded_channel();

    let mut conn = get_redis_connection(&conn_config, redis_tx).await?;
    subscribe(&mut conn, &conn_config.channel_name).await?;

    // Internal channel used to forward messages to the Redis channel
    // that is used to send WebSocket messages to other nodes in the cluster.
    let (caller_tx, mut internal_rx) = channel(1024);
    // Receiver from which the caller can receive messages from the Redis channel.
    let (internal_tx, caller_rx) = channel(1024);

    tokio::spawn(async move {
        loop {
            tokio::select! {
                Some(message) = redis_rx.recv() => {
                    debug!("received message from redis channel {}", conn_config.channel_name);
                    if message.kind == PushKind::Message {
                        match String::from_redis_value(&message.data[1]) {
                            Ok(value) => {
                                let ws_message: WrappedWebSocketMessage = match serde_json::from_str(&value) {
                                    Ok(ws_message) => ws_message,
                                    Err(e) => {
                                        error!("error parsing message from redis channel {}: {}", conn_config.channel_name, e);
                                        continue;
                                    }
                                };
                                if ws_message.source_node != conn_config.server_node_name {
                                    // Messages will only be forwarded if they are not from the current node.
                                    if (internal_tx.send(ws_message.message).await).is_err() {
                                        error!("receiver dropped, stopping redis listener");
                                        break;
                                    }
                                }
                            }
                            Err(e) => {
                                error!("error parsing message from redis channel {}: {}", conn_config.channel_name, e);
                            }
                        }
                    }
                }
                Some(message) = internal_rx.recv() => {
                    debug!("received message to forward to channel {}", conn_config.channel_name);
                    let wrapped_message = WrappedWebSocketMessage {
                        source_node: conn_config.server_node_name.clone(),
                        message,
                    };
                    let wrapped_message_json = match serde_json::to_string(&wrapped_message) {
                        Ok(json) => json,
                        Err(e) => {
                            error!("error serializing message to json: {e:?}");
                            continue;
                        }
                    };

                    let res: Result<i32, _> = publish(&mut conn, &conn_config.channel_name, wrapped_message_json).await;
                    if let Err(e) = res {
                        error!("error publishing message to channel: {e:?}");
                    }
                }
                else => {
                    break;
                }
            }
        }
    });

    Ok((caller_tx, caller_rx))
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct WrappedWebSocketMessage {
    source_node: String,
    message: WebSocketMessage,
}

enum ConnectionWrapper {
    Cluster(ClusterConnection),
    SingleNode(MultiplexedConnection),
}

async fn get_redis_connection(
    conn_config: &ConnectionConfig,
    redis_tx: UnboundedSender<PushInfo>,
) -> RedisResult<ConnectionWrapper> {
    if !conn_config.cluster_mode {
        let client = Client::open(conn_config.nodes[0].clone())?;
        let config = redis::AsyncConnectionConfig::new().set_push_sender(redis_tx);
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
    let client = builder.push_sender(redis_tx).build()?;
    Ok(ConnectionWrapper::Cluster(
        client.get_async_connection().await?,
    ))
}

async fn subscribe(conn: &mut ConnectionWrapper, channel_name: &str) -> RedisResult<()> {
    match conn {
        ConnectionWrapper::Cluster(conn) => conn.subscribe(channel_name).await,
        ConnectionWrapper::SingleNode(conn) => conn.subscribe(channel_name).await,
    }
}

async fn publish(
    conn: &mut ConnectionWrapper,
    channel_name: &str,
    message: String,
) -> Result<i32, RedisError> {
    match conn {
        ConnectionWrapper::Cluster(conn) => conn.publish(channel_name, message).await,
        ConnectionWrapper::SingleNode(conn) => conn.publish(channel_name, message).await,
    }
}
