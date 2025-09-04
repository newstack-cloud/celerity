use std::error::Error;

use celerity_helpers::redis::{get_redis_connection, ConnectionConfig};
use celerity_ws_registry::types::Message;
use redis::{FromRedisValue, PushKind};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc::{channel, unbounded_channel, Receiver, Sender};
use tracing::{debug, error};

/// Configuration for a Redis connection for the pubsub channel
/// used with the WebSocket connection registry for providing
/// a horizontally scalable WebSocket API that shares messages
/// between nodes in a cluster for a shared multi-client session (e.g. real-time chat).
#[derive(Debug, Clone)]
pub struct PubSubConnectionConfig {
    // A name for the current server node,
    // primarily used to filter out messages that are not for connections
    // on the current node.
    pub server_node_name: String,
    pub channel_name: String,
    pub nodes: Vec<String>,
    pub password: Option<String>,
    pub cluster_mode: bool,
}

impl From<PubSubConnectionConfig> for ConnectionConfig {
    fn from(config: PubSubConnectionConfig) -> Self {
        Self {
            nodes: config.nodes,
            password: config.password,
            cluster_mode: config.cluster_mode,
        }
    }
}

/// Connects to a Redis server or cluster pubsub channel and returns a
/// receiver and sender to allow for sharing messages between nodes in a cluster
/// for a WebSocket API.
///
/// This should be used for both publishing messages and subscribing to
/// messages. This allows for both messages and acknowledgements to be sent
/// to the same channel, the caller is responsible for handling acknowledgements
/// and providing a layer of resiliency for message sharing between nodes.
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
/// let registry = Arc::new(WebSocketConnRegistry::new(
///     WebSocketConnRegistryConfig {
///         ack_worker_config: None,
///     },
///     Some(tx),
/// ));
/// registry.start_ack_worker();
/// registry.listen(rx);
///
/// /// Do something with the registry ...
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
/// let registry = Arc::new(WebSocketConnRegistry::new(
///     WebSocketConnRegistryConfig {
///         ack_worker_config: None,
///     },
///     Some(tx),
/// ));
/// registry.start_ack_worker();
/// registry.listen(rx);
/// ```
pub async fn connect(
    conn_config: PubSubConnectionConfig,
) -> Result<(Sender<Message>, Receiver<Message>), Box<dyn Error>> {
    let (redis_tx, mut redis_rx) = unbounded_channel();

    let mut conn = get_redis_connection(&conn_config.clone().into(), Some(redis_tx)).await?;
    conn.subscribe(&conn_config.channel_name).await?;

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
                                let wrapped_message: MessageWithSourceNode = match serde_json::from_str(&value) {
                                    Ok(wrapped_message) => wrapped_message,
                                    Err(e) => {
                                        error!("error parsing message from redis channel {}: {}", conn_config.channel_name, e);
                                        continue;
                                    }
                                };
                                if wrapped_message.source_node != conn_config.server_node_name {
                                    // Messages will only be forwarded if they are not from the current node.
                                    // Acknowledgements for messages that were not sent by the current node
                                    // are ignored.
                                    let should_send = match wrapped_message.message.clone() {
                                        Message::Ack(ack_message) => {
                                            ack_message.message_node == conn_config.server_node_name
                                        }
                                        Message::WebSocket(_) => true,
                                    };
                                    if should_send
                                        && (internal_tx.send(wrapped_message.message).await).is_err() {
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
                    let wrapped_message = MessageWithSourceNode {
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

                    let res: Result<i32, _> = conn.publish(&conn_config.channel_name, wrapped_message_json).await;
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
struct MessageWithSourceNode {
    source_node: String,
    message: Message,
}
