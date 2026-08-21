//! Joining a node to the others serving the same WebSocket API.

use std::sync::Arc;

use celerity_ws_registry::registry::WebSocketConnRegistry;
use tokio::sync::oneshot;

use crate::{
    config::WsClusterConfig,
    consts::{
        DEFAULT_WS_CLUSTER_MIGRATION_GRACE_MS, DEFAULT_WS_CLUSTER_NODE_TTL_MS,
        DEFAULT_WS_NODE_GROUP_CAPACITY,
    },
    errors::ApplicationStartError,
    websocket_dedupe::{SeenMessages, SharedMessageIdStore, DEFAULT_MESSAGE_ID_TTL_MS},
};

/// One shape for every way joining a cluster can fail, so a message names the
/// step rather than only the error.
fn cluster_setup_error<E: std::fmt::Debug>(message: &str, err: E) -> ApplicationStartError {
    ApplicationStartError::WebSocketClusterSetup(format!("{message}: {err:?}"))
}

/// Joins this node to the others serving the same WebSocket API, and returns
/// what to tell when it should leave.
///
/// A failure here should stop the runtime starting. A node that carried on
/// would serve its own clients while every message for a connection held
/// elsewhere went nowhere, which is worse than not starting.
pub async fn join_cluster(
    registry: Arc<WebSocketConnRegistry>,
    seen_messages: Option<Arc<SeenMessages>>,
    cluster_config: WsClusterConfig,
    service_name: &str,
    server_node_name: &str,
) -> Result<oneshot::Sender<()>, ApplicationStartError> {
    use celerity_helpers::redis::{get_redis_connection, ConnectionConfig};
    use celerity_ws_redis::{
        forwarded::{ForwardedMessages, DEFAULT_FORWARDED_TTL_MS},
        locations::ConnectionLocations,
        node_group::{join_or_create, spawn_heartbeat, NodeGroupConfig},
        pubsub::{connect, PubSubConnectionConfig},
    };

    // Fallback to the service name as an application's nodes share it and two
    // separate applications do not.
    let key_prefix = cluster_config
        .key_prefix
        .clone()
        .unwrap_or_else(|| service_name.to_string());
    let group_config = NodeGroupConfig {
        server_node_name: server_node_name.to_string(),
        capacity: cluster_config
            .node_group_capacity
            .unwrap_or(DEFAULT_WS_NODE_GROUP_CAPACITY),
        node_ttl_ms: cluster_config
            .node_ttl_ms
            .unwrap_or(DEFAULT_WS_CLUSTER_NODE_TTL_MS),
        key_prefix: key_prefix.clone(),
    };

    let mut conn = get_redis_connection(
        &ConnectionConfig {
            nodes: cluster_config.redis_nodes.clone(),
            password: cluster_config.redis_password.clone(),
            cluster_mode: cluster_config.redis_cluster_mode,
        },
        None,
    )
    .await
    .map_err(|err| {
        cluster_setup_error(
            "could not reach the redis deployment the nodes cluster over",
            err,
        )
    })?;

    let group = join_or_create(&mut conn, &group_config)
        .await
        .map_err(|err| cluster_setup_error("could not join a node group", err))?;
    let locations = ConnectionLocations::new(
        conn.clone(),
        key_prefix.clone(),
        group.id.clone(),
        group_config.node_ttl_ms,
    );
    registry
        .set_locations(locations.clone())
        .map_err(|err| cluster_setup_error("could not attach the connection store", err))?;

    // The other direction of the same idea: a client resending to whichever
    // node it reconnected to, rather than a node relaying to a client.
    if let Some(seen_messages) = seen_messages {
        seen_messages
            .attach_shared(SharedMessageIdStore::new(
                conn.clone(),
                key_prefix.clone(),
                cluster_config
                    .seen_ttl_ms
                    .unwrap_or(DEFAULT_MESSAGE_ID_TTL_MS),
            ))
            .map_err(|_| {
                ApplicationStartError::WebSocketClusterSetup(
                    "the store for messages seen from clients was already shared".to_string(),
                )
            })?;
    }

    // Shared, because a message resent after its acknowledgement went
    // missing arrives at whichever node holds the connection now, which may
    // not be the node that delivered it the first time.
    registry
        .set_forwarded_messages(ForwardedMessages::new(
            conn.clone(),
            key_prefix.clone(),
            cluster_config
                .forwarded_ttl_ms
                .unwrap_or(DEFAULT_FORWARDED_TTL_MS),
        ))
        .map_err(|err| cluster_setup_error("could not attach the forwarded message store", err))?;

    let (moved_tx, moved_rx) = tokio::sync::mpsc::channel(4);
    let (broadcaster, from_other_nodes) = connect(
        PubSubConnectionConfig {
            server_node_name: group_config.server_node_name.clone(),
            key_prefix,
            nodes: cluster_config.redis_nodes,
            password: cluster_config.redis_password,
            cluster_mode: cluster_config.redis_cluster_mode,
            migration_grace_ms: cluster_config
                .migration_grace_ms
                .unwrap_or(DEFAULT_WS_CLUSTER_MIGRATION_GRACE_MS),
        },
        group.clone(),
        locations.clone(),
        moved_rx,
    )
    .await
    .map_err(|err| cluster_setup_error("could not subscribe to the node group", err))?;

    // We need to initialise the registry in this order because the ack worker
    // only starts where there is a broadcaster to resend through,
    // and listening before either would take in a message with nowhere to put it.
    registry
        .set_broadcaster(broadcaster)
        .map_err(|err| cluster_setup_error("could not attach the broadcaster", err))?;
    registry.clone().start_ack_worker();
    registry.listen(from_other_nodes);

    let (shutdown_tx, shutdown_rx) = tokio::sync::oneshot::channel();
    spawn_heartbeat(conn, group_config, group, locations, moved_tx, shutdown_rx);

    Ok(shutdown_tx)
}
