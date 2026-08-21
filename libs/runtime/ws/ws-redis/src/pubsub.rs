use std::{error::Error, sync::Arc, time::Duration};

use celerity_helpers::redis::{get_redis_connection, ConnectionConfig, ConnectionWrapper};
use celerity_ws_registry::types::Message;
use redis::{FromRedisValue, PushKind};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc::{channel, unbounded_channel, Receiver, Sender};
use tracing::{debug, error, warn};

use crate::{
    locations::ConnectionLocations,
    node_group::{group_index_key, node_key, NodeGroup},
};

/// How a node reaches the rest of the cluster, and how it is named within it.
#[derive(Debug, Clone)]
pub struct PubSubConnectionConfig {
    /// Names this node, and is what other nodes address an acknowledgement to.
    pub server_node_name: String,
    /// What every key and channel is named under. Must match across the
    /// application's nodes, see
    /// [`DEFAULT_KEY_PREFIX`](crate::node_group::DEFAULT_KEY_PREFIX).
    pub key_prefix: String,
    pub nodes: Vec<String>,
    pub password: Option<String>,
    pub cluster_mode: bool,
    /// How long a node stays subscribed to the group it has left after moving
    /// to another one.
    ///
    /// A sender reads where a connection is and publishes a moment later, so
    /// there is always a message in flight against a mapping that has just
    /// changed. Holding the old subscription for longer than that gap is what
    /// stops those being missed.
    pub migration_grace_ms: u64,
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

/// Connects a node to its group's channels, returning a sender for messages
/// leaving this node and a receiver for those arriving from other nodes.
///
/// Both acknowledgements and messages travel this way. The registry is
/// responsible for what they mean; this only decides where they go.
///
/// A message is published to the group holding the connection it names, so a
/// node only hears about connections its group might be holding. An
/// acknowledgement is published to the ack channel of the group holding the
/// node that sent the original message, which is read from that node's key
/// rather than remembered per message.
///
/// `moved` carries a new group when the node has had to take a place in one,
/// and the node subscribes to it before giving up the one it left.
///
/// # Example
///
/// ```no_run
/// # use std::sync::Arc;
/// # use celerity_helpers::redis::get_redis_connection;
/// # use celerity_ws_redis::{
/// #     locations::ConnectionLocations,
/// #     node_group::{join_or_create, NodeGroupConfig, DEFAULT_KEY_PREFIX},
/// #     pubsub::{connect, PubSubConnectionConfig},
/// # };
/// # use celerity_ws_registry::registry::{WebSocketConnRegistry, WebSocketConnRegistryConfig};
/// # async fn example() -> Result<(), Box<dyn std::error::Error>> {
/// let group_config = NodeGroupConfig {
///     server_node_name: "api-node-1".to_string(),
///     capacity: 5,
///     node_ttl_ms: 30_000,
///     key_prefix: DEFAULT_KEY_PREFIX.to_string(),
/// };
/// let pubsub_config = PubSubConnectionConfig {
///     server_node_name: group_config.server_node_name.clone(),
///     key_prefix: group_config.key_prefix.clone(),
///     nodes: vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()],
///     password: None,
///     cluster_mode: false,
///     migration_grace_ms: 5_000,
/// };
///
/// let mut conn = get_redis_connection(&pubsub_config.clone().into(), None).await?;
/// let group = join_or_create(&mut conn, &group_config).await?;
/// let locations = ConnectionLocations::new(
///     conn.clone(),
///     group_config.key_prefix.clone(),
///     group.id.clone(),
///     group_config.node_ttl_ms,
/// );
///
/// let (moved_tx, moved_rx) = tokio::sync::mpsc::channel(4);
/// let (tx, rx) = connect(pubsub_config, group, locations, moved_rx).await?;
///
/// let registry = Arc::new(WebSocketConnRegistry::new(
///     WebSocketConnRegistryConfig {
///         ack_worker_config: None,
///         server_node_name: group_config.server_node_name,
///     },
///     Some(tx),
/// ));
/// registry.clone().start_ack_worker();
/// registry.listen(rx);
/// # let _ = moved_tx;
/// # Ok(())
/// # }
/// ```
pub async fn connect(
    conn_config: PubSubConnectionConfig,
    group: NodeGroup,
    locations: Arc<ConnectionLocations>,
    mut moved: Receiver<NodeGroup>,
) -> Result<(Sender<Message>, Receiver<Message>), Box<dyn Error>> {
    let (redis_tx, mut redis_rx) = unbounded_channel();

    let mut conn = get_redis_connection(&conn_config.clone().into(), Some(redis_tx)).await?;
    conn.subscribe(&group.channel).await?;
    conn.subscribe(&group.ack_channel).await?;

    // Internal channel used to forward messages to the channels that carry
    // WebSocket messages to other nodes in the cluster.
    let (caller_tx, mut internal_rx) = channel(1024);
    // Receiver from which the caller can receive messages from other nodes.
    let (internal_tx, caller_rx) = channel(1024);

    tokio::spawn(async move {
        let mut group = group;

        loop {
            tokio::select! {
                Some(push) = redis_rx.recv() => {
                    if push.kind != PushKind::Message {
                        continue;
                    }
                    let Ok(raw) = String::from_redis_value(&push.data[1]) else {
                        error!("could not read a message from a node group channel");
                        continue;
                    };
                    let wrapped: MessageWithSourceNode = match serde_json::from_str(&raw) {
                        Ok(wrapped) => wrapped,
                        Err(err) => {
                            error!("could not parse a message from a node group channel: {err}");
                            continue;
                        }
                    };

                    // A node's own messages come back to it, since it is
                    // subscribed to the channel it publishes on.
                    if wrapped.source_node == conn_config.server_node_name {
                        continue;
                    }
                    // An acknowledgement is for the node that sent the message,
                    // and the rest of the group is only overhearing it.
                    let for_this_node = match &wrapped.message {
                        Message::Ack(ack) => ack.message_node == conn_config.server_node_name,
                        Message::WebSocket(_) => true,
                    };
                    if for_this_node && internal_tx.send(wrapped.message).await.is_err() {
                        error!("receiver dropped, stopping the node group listener");
                        break;
                    }
                }
                Some(message) = internal_rx.recv() => {
                    publish(&mut conn, &conn_config, &locations, message).await;
                }
                Some(joined) = moved.recv() => {
                    // Subscribed before the old one is given up, so the node is
                    // listening to both while any sender still holds the
                    // mapping it read a moment ago.
                    if let Err(err) = subscribe_to(&mut conn, &joined).await {
                        error!(node_group = %joined.id, "failed to subscribe to a new node group: {err}");
                        continue;
                    }
                    debug!(node_group = %joined.id, "following this node into another group");
                    leave_behind(conn.clone(), group, conn_config.migration_grace_ms);
                    group = joined;
                }
                else => break,
            }
        }
    });

    Ok((caller_tx, caller_rx))
}

async fn subscribe_to(conn: &mut ConnectionWrapper, group: &NodeGroup) -> redis::RedisResult<()> {
    conn.subscribe(&group.channel).await?;
    conn.subscribe(&group.ack_channel).await
}

/// Gives up the channels of a group this node has left, after waiting out the
/// grace period.
///
/// The wait covers a sender that read the old mapping just before it changed
/// and published a moment later.
fn leave_behind(mut conn: ConnectionWrapper, group: NodeGroup, grace_ms: u64) {
    tokio::spawn(async move {
        tokio::time::sleep(Duration::from_millis(grace_ms)).await;
        for channel in [&group.channel, &group.ack_channel] {
            if let Err(err) = conn.unsubscribe(channel).await {
                error!(channel = %channel, "failed to give up a channel after moving group: {err}");
            }
        }
        debug!(node_group = %group.id, "gave up the group this node left");
    });
}

/// Sends a message to the nodes that might be able to act on it.
async fn publish(
    conn: &mut ConnectionWrapper,
    conn_config: &PubSubConnectionConfig,
    locations: &Arc<ConnectionLocations>,
    message: Message,
) {
    let channels = match &message {
        Message::WebSocket(websocket_message) => {
            match locations.group_for(&websocket_message.connection_id).await {
                Ok(Some(group_id)) => {
                    vec![NodeGroup::new(&conn_config.key_prefix, group_id).channel]
                }
                Ok(None) => {
                    // Nothing has recorded the connection, which a client that
                    // connected a moment ago looks like. Every group hears it
                    // rather than the message being dropped for a client that
                    // is there.
                    debug!(
                        connection_id = %websocket_message.connection_id,
                        "no node group holds this connection, telling all of them"
                    );
                    match every_group_channel(conn, &conn_config.key_prefix).await {
                        Ok(channels) => channels,
                        Err(err) => {
                            error!("could not read the node groups: {err}");
                            return;
                        }
                    }
                }
                Err(err) => {
                    error!("could not read where a connection is: {err}");
                    return;
                }
            }
        }
        Message::Ack(ack) => {
            let node = node_key(&conn_config.key_prefix, &ack.message_node);
            match conn.get(&node).await {
                Ok(group_id) if !group_id.is_empty() => {
                    vec![NodeGroup::new(&conn_config.key_prefix, group_id).ack_channel]
                }
                Ok(_) => {
                    // The node that sent the message has gone, so nothing is
                    // waiting to hear that its message arrived.
                    debug!(
                        node = %ack.message_node,
                        "not acknowledging to a node that has stopped running"
                    );
                    return;
                }
                Err(err) => {
                    error!("could not read which group a node belongs to: {err}");
                    return;
                }
            }
        }
    };

    let wrapped = MessageWithSourceNode {
        source_node: conn_config.server_node_name.clone(),
        message,
    };
    let body = match serde_json::to_string(&wrapped) {
        Ok(body) => body,
        Err(err) => {
            error!("could not serialise a message for another node: {err}");
            return;
        }
    };

    if channels.is_empty() {
        warn!("there are no node groups to send to, so a message has nowhere to go");
        return;
    }

    for channel in channels {
        if let Err(err) = conn.publish(&channel, body.clone()).await {
            error!(channel = %channel, "failed to send a message to a node group: {err}");
        }
    }
}

/// Every group's channel, for a message that cannot be narrowed to one of them.
async fn every_group_channel(
    conn: &mut ConnectionWrapper,
    key_prefix: &str,
) -> redis::RedisResult<Vec<String>> {
    let group_ids = conn.smembers(&group_index_key(key_prefix)).await?;
    Ok(group_ids
        .into_iter()
        .map(|group_id| NodeGroup::new(key_prefix, group_id).channel)
        .collect())
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct MessageWithSourceNode {
    source_node: String,
    message: Message,
}
