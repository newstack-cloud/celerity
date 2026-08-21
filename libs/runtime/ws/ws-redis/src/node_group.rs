use std::{sync::Arc, time::Duration};

use celerity_helpers::redis::ConnectionWrapper;
use nanoid::nanoid;
use redis::RedisResult;
use tokio::{sync::mpsc::Sender, sync::oneshot, task::JoinHandle};
use tracing::{debug, error, info};

use crate::locations::ConnectionLocations;

/// The prefix the protocol documents, and the fallback for a caller that names
/// nothing of its own.
///
/// It separates nothing by itself, since it is what any Celerity application
/// would use. Nodes sharing a prefix are one cluster, so two applications on one
/// Redis deployment need prefixes of their own or each will publish messages to
/// the other's nodes for connections they have never heard of. A prefix also has to be the
/// same on every node of an application, which is why it is derived from the
/// application rather than from anything belonging to a node.
pub const DEFAULT_KEY_PREFIX: &str = "celerity";

/// The shortest gap between heartbeats, whatever the expiry works out to.
///
/// A beat is a third of the expiry, so a very short expiry would otherwise ask
/// for a round trip every few milliseconds. Only a test sets one that short, and
/// what it is asking for is a node that looks dead quickly rather than one that
/// spends itself on round trips.
const MIN_HEARTBEAT_MS: u64 = 20;

/// Decides a group and takes a place in it, without another node being able to
/// count the group in between.
///
/// Reading the members, deciding, and adding have to happen together. Nodes
/// start together far more often than they start alone, a rolling deploy or a
/// scale-out being the ordinary case, and every one of them reading a group as
/// having room before any of them has taken it would put them all in the same
/// group.
///
/// A node already in a group stays there, which makes this the heartbeat as
/// well as the join. A node that lapsed and was dropped runs the same path as a
/// node starting, so it takes whatever place is free rather than a place that
/// has to be kept for it.
const JOIN_SCRIPT: &str = include_str!("scripts/join_node_group.lua");

/// Gives up a place and takes the group with it where it was the last one in
/// it, without another node being able to join in between.
///
/// Counting the members and taking an empty group out of the index have to
/// happen together. A node joining between the two would be left holding a
/// group that has just been taken out of the index, which nothing else can find
/// and nothing prunes, so its membership would outlive it.
const LEAVE_SCRIPT: &str = include_str!("scripts/leave_node_group.lua");

/// What a node needs to decide which group to join and to stay a member of it.
#[derive(Debug, Clone)]
pub struct NodeGroupConfig {
    /// Names this node among the others, and is what an acknowledgement is
    /// addressed to.
    pub server_node_name: String,
    /// How many nodes a group holds before a new one is started. Shared across
    /// the cluster, since a group filled to one node's idea of capacity and not
    /// another's is not really bounded.
    pub capacity: usize,
    /// How long this node's liveness key outlives its last refresh. A node that
    /// stops refreshing stops counting towards its group's capacity once this
    /// runs out, and is dropped from the member set by the next node to look.
    pub node_ttl_ms: u64,
    /// What every key and channel is named under. Nodes sharing a prefix are
    /// one cluster, so this must name the application and must match across its
    /// nodes. See [`DEFAULT_KEY_PREFIX`].
    pub key_prefix: String,
}

/// A node group, and the two channels that carry its traffic.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NodeGroup {
    pub id: String,
    /// Carries messages for connections held by the group's nodes.
    pub channel: String,
    /// Mirrors the group's channel and carries only acknowledgements, so a node
    /// waiting to hear that a message arrived is not reading every message the
    /// group is handling.
    pub ack_channel: String,
}

impl NodeGroup {
    /// Names a group's channels from its id, for a caller holding an id and
    /// nothing else.
    pub fn new(prefix: &str, id: String) -> Self {
        Self {
            channel: format!("{prefix}:node-group:{id}"),
            ack_channel: format!("{prefix}:node-group-ack:{id}"),
            id,
        }
    }
}

/// What the keys holding group membership are named under.
///
/// The brace is a Redis Cluster hash tag, which puts every key the join and
/// leave scripts touch in one slot. A script reaching across slots is refused,
/// and these keys are few and small enough that one shard holding them does
/// not have a significant cost.
/// Connection entries are deliberately outside it, since there is one per client
/// and they belong spread across the cluster.
fn meta_prefix(prefix: &str) -> String {
    format!("{prefix}:{{group-meta}}")
}

/// The set holding every node group's id.
pub fn group_index_key(prefix: &str) -> String {
    format!("{}:node-groups", meta_prefix(prefix))
}

/// The key whose presence says a node is still running, and whose value is the
/// group it belongs to.
pub fn node_key(prefix: &str, server_node_name: &str) -> String {
    format!("{}:node:{}", meta_prefix(prefix), server_node_name)
}

/// Joins the emptiest group with room in it, or starts a new one, or refreshes
/// the place this node already holds.
///
/// A group's size is its member set, filtered to the members still saying they
/// are running, which reads the same from any Redis node and tells a node that
/// died apart from one that was never there.
pub async fn join_or_create(
    conn: &mut ConnectionWrapper,
    config: &NodeGroupConfig,
) -> RedisResult<NodeGroup> {
    let index_key = group_index_key(&config.key_prefix);
    let meta = meta_prefix(&config.key_prefix);
    let capacity = config.capacity.to_string();
    let ttl_ms = config.node_ttl_ms.to_string();
    let new_group_id = nanoid!();

    let result: Vec<String> = conn
        .eval_script(
            JOIN_SCRIPT,
            &[&index_key],
            &[
                &meta,
                &config.server_node_name,
                &capacity,
                &ttl_ms,
                &new_group_id,
            ],
        )
        .await?;

    let group_id = result
        .first()
        .cloned()
        .unwrap_or_else(|| new_group_id.clone());
    match result.get(1).map(String::as_str) {
        Some("held") => debug!(node_group = %group_id, "still holding a place in this node group"),
        _ if group_id == new_group_id => {
            info!(node_group = %group_id, "no node group had room, started a new one")
        }
        _ => info!(node_group = %group_id, "joined a node group"),
    }

    Ok(NodeGroup::new(&config.key_prefix, group_id))
}

/// Leaves the group, taking this node's membership and liveness key with it.
///
/// The alternative is waiting out the TTL, during which the group looks fuller
/// than it is and messages for connections this node no longer holds are still
/// published to it.
pub async fn leave(
    conn: &mut ConnectionWrapper,
    config: &NodeGroupConfig,
    group: &NodeGroup,
) -> RedisResult<()> {
    // An empty group is taken out of the index by the same script, since it
    // exists only as somewhere for nodes to be.
    let removed: i64 = conn
        .eval_script(
            LEAVE_SCRIPT,
            &[&group_index_key(&config.key_prefix)],
            &[
                &meta_prefix(&config.key_prefix),
                &config.server_node_name,
                &group.id,
            ],
        )
        .await?;

    if removed == 1 {
        debug!(node_group = %group.id, "last node left the group, removing it");
    }

    Ok(())
}

/// Keeps this node's place and its connection entries alive until it is asked
/// to stop, and leaves the group tidily when it is.
///
/// Every beat runs the join, so a node dropped from its group while it was slow
/// takes a place again without anything having to keep one for it. The place it
/// gets may be in another group, which is reported on `moved` for the caller to
/// follow with its subscriptions.
pub fn spawn_heartbeat(
    mut conn: ConnectionWrapper,
    config: NodeGroupConfig,
    group: NodeGroup,
    locations: Arc<ConnectionLocations>,
    moved: Sender<NodeGroup>,
    mut shutdown: oneshot::Receiver<()>,
) -> JoinHandle<()> {
    // Beating three times inside the expiry means two beats can be lost, to a
    // stalled task or a slow round trip, before anything else decides this node
    // has gone.
    let interval = Duration::from_millis((config.node_ttl_ms / 3).max(MIN_HEARTBEAT_MS));

    tokio::spawn(async move {
        let mut ticker = tokio::time::interval(interval);
        // The first tick is immediate, and the keys were just written by
        // joining, so it is skipped rather than beating twice at startup.
        ticker.tick().await;
        let mut group = group;

        loop {
            tokio::select! {
                _ = ticker.tick() => {
                    group = beat(&mut conn, &config, group, &locations, &moved).await;
                }
                _ = &mut shutdown => {
                    if let Err(err) = leave(&mut conn, &config, &group).await {
                        error!("failed to leave the node group on shutdown: {err:?}");
                    }
                    if let Err(err) = locations.forget_all().await {
                        error!(
                            "failed to take away this node's connection entries on shutdown: \
                             {err:?}"
                        );
                    }
                    info!(node_group = %group.id, "left the node group");
                    return;
                }
            }
        }
    })
}

/// One beat, returning the group this node holds a place in afterwards.
///
/// A failure is logged and left to the next beat, since a round trip that
/// failed says nothing about whether the next one will.
async fn beat(
    conn: &mut ConnectionWrapper,
    config: &NodeGroupConfig,
    group: NodeGroup,
    locations: &Arc<ConnectionLocations>,
    moved: &Sender<NodeGroup>,
) -> NodeGroup {
    let group = match join_or_create(conn, config).await {
        Ok(held) => held,
        Err(err) => {
            error!("failed to say this node is still running: {err:?}");
            group
        }
    };

    if group.id != locations.group() {
        info!(
            node_group = %group.id,
            "this node was dropped from its group and took a place in another one"
        );
        // Told before the entries move, so the caller is listening to the new
        // group's channel before anything is sent there for this node.
        if moved.send(group.clone()).await.is_err() {
            error!("nothing is following this node's group any more");
        }
        locations.set_group(group.id.clone());
    }

    if let Err(err) = locations.refresh().await {
        error!("failed to keep this node's connection entries alive: {err:?}");
    }

    group
}
