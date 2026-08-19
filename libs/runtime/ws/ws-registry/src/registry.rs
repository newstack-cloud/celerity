use std::{
    collections::HashMap,
    fmt::{Debug, Display},
    sync::{Arc, OnceLock, RwLock},
};

use async_trait::async_trait;
use axum::extract::ws::{Message, WebSocket};
use futures::{stream::SplitSink, SinkExt};
use tokio::sync::{
    mpsc::{Receiver, Sender},
    Mutex,
};
use tracing::{debug, error, info};

use crate::{
    acks::{AckStatus, AckWorkerMessage, MessageAction, Worker},
    errors::WebSocketConnError,
    message_helpers::{client_ack_request, create_message_lost_event, create_ws_message},
    types::{
        AckMessage, AckWorkerConfig, Message as RegistryMessage, MessageType, WebSocketMessage,
    },
};

/// The sending half of a connection, which is all a registry ever needs.
///
/// A connection's receiving half belongs to the task reading it, so that
/// reading never waits on a lock a sender holds. Holding only this half here
/// makes that ownership impossible to get wrong.
pub type WebSocketConnSender = SplitSink<WebSocket, Message>;

// Additional context for sending messages to a connection in a WebSocket registry.
#[derive(Default)]
pub struct SendContext {
    // The caller that is sending the message.
    // This is useful for providing context about the purpose or origin of the message.
    // If a message is considered lost, the caller will be included in the message sent
    // to the clients in the inform_clients list.
    pub caller: Option<String>,
    // Whether to wait for an acknowledgement from the node that has the connection
    // that the message was sent for, a WebSocketConnError::MessageLost error will be returned
    // for the caller to handle the case where the message was lost
    // when the wait_for_ack flag is set to true.
    // This is only used when broadcasting messages to other nodes in a cluster.
    pub wait_for_ack: bool,
    // The connection IDs of clients that should be informed if a message is lost
    // (an acknowledgement was not received after the maximum number of retries).
    // These clients will be informed regardless of the wait_for_ack flag.
    pub inform_clients: Vec<String>,
}

#[async_trait]
/// Provides a trait for sending messages to WebSocket connections.
pub trait WebSocketRegistrySend: Send + Sync + Display + Debug {
    async fn send_message(
        &self,
        connection_id: String,
        message_id: String,
        message_type: MessageType,
        message: String,
        ctx: Option<SendContext>,
    ) -> Result<(), WebSocketConnError>;
}

#[derive(Default)]
pub struct WebSocketConnRegistryConfig {
    // The configuration for the ack worker.
    pub ack_worker_config: Option<AckWorkerConfig>,
    // The name of the server node that the registry is running on.
    // This is used to identify the source node of a message when broadcasting
    // messages to other nodes in the cluster.
    pub server_node_name: String,
}

/// Provides a registry for WebSocket connections.
/// This allows for sending messages to WebSocket connections
/// in the current runtime instance and on other nodes in a cluster.
pub struct WebSocketConnRegistry {
    // The configuration for the ack worker.
    ack_worker_config: Option<AckWorkerConfig>,
    // WebSockets do not implement Sync so we need to wrap them in Arc<Mutex<...>>
    // to safely send messages to WebSocket connections from multiple threads.
    connections: Arc<RwLock<HashMap<String, Arc<Mutex<WebSocketConnSender>>>>>,
    // A channel for sending messages to the ack worker.
    ack_sender: Mutex<Option<Sender<AckWorkerMessage>>>,
    // A channel for the worker that tracks acknowledgements from clients, as
    // distinct from the one above, which tracks them from other nodes.
    //
    // Held separately because the two answer different questions. A node
    // confirming it took a message on is not a client confirming it received
    // one, and only the node holding a connection can be told the second.
    //
    // Written once, before the worker is spawned, so a send can never find it
    // empty and drop the tracking for a message that asked to be acknowledged.
    client_ack_sender: OnceLock<Sender<AckWorkerMessage>>,
    // This is called "broadcaster" because it is used to send messages to all
    // other nodes in a cluster, however, it should not be confused with a broadcast::Sender
    // for in-process broadcasting. Typically, there will be a single receiver in the same process
    // that will broadcast messages to all other nodes in the cluster via a pub/sub mechanism
    // over a network protocol.
    broadcaster: Option<Sender<RegistryMessage>>,
    // The name of the server node that the registry is running on.
    // This is used to identify the source node of a message when broadcasting
    // messages to other nodes in the cluster.
    server_node_name: String,
}

impl WebSocketConnRegistry {
    pub fn new(
        config: WebSocketConnRegistryConfig,
        broadcaster: Option<Sender<RegistryMessage>>,
    ) -> Self {
        Self {
            ack_worker_config: config.ack_worker_config,
            connections: Arc::new(RwLock::new(HashMap::new())),
            ack_sender: Mutex::new(None),
            client_ack_sender: OnceLock::new(),
            broadcaster,
            server_node_name: config.server_node_name,
        }
    }

    /// Starts the worker that tracks acknowledgements from clients.
    ///
    /// Started whatever the deployment, unlike the worker for acknowledgements
    /// between nodes, because a client acknowledges to the node holding its
    /// connection and a single node has clients just the same as a cluster
    /// does.
    pub fn start_client_ack_worker(self: Arc<Self>) {
        let (ack_tx, ack_rx) = tokio::sync::mpsc::channel(1024);
        let (action_tx, mut action_rx) = tokio::sync::mpsc::channel(1024);
        Worker::new(self.ack_worker_config.clone().unwrap_or_default()).start(ack_rx, action_tx);

        // Published before the worker is spawned rather than from inside it, so
        // that a send arriving between this returning and the task first being
        // polled is still tracked.
        let _ = self.client_ack_sender.set(ack_tx);

        tokio::spawn(async move {
            while let Some(action) = action_rx.recv().await {
                match action {
                    MessageAction::Resend(resend) => {
                        let Some(connection) = self.get_connection(resend.client_id.clone()) else {
                            debug!(
                                connection_id = %resend.client_id,
                                message_id = %resend.message_id,
                                "client has gone, so a message waiting on it is left to be declared lost"
                            );
                            continue;
                        };
                        let message = match create_ws_message(
                            resend.message_type.clone(),
                            resend.message.clone(),
                        ) {
                            Ok(message) => message,
                            Err(err) => {
                                error!(
                                    message_id = %resend.message_id,
                                    "could not rebuild a message to resend to a client: {err:?}"
                                );
                                continue;
                            }
                        };

                        // Noted again, which counts the attempt and restarts the
                        // clock. Without it the attempt that has just been made
                        // is never counted, so a message nobody answers is sent
                        // again on every check and never reaches the point of
                        // being declared lost. A message sent between nodes
                        // gets this from being resent through the send path,
                        // which records it on the way past, but a client's
                        // resend goes straight to the socket.
                        let client_id = resend.client_id.clone();
                        let message_id = resend.message_id.clone();
                        self.record_pending_client_ack(
                            resend.client_id,
                            resend.message_id,
                            resend.message_type,
                            resend.message,
                            resend.inform_clients_on_loss,
                            resend.caller,
                        )
                        .await;

                        let mut sender = connection.lock().await;
                        if let Err(err) = sender.send(message).await {
                            debug!(
                                connection_id = %client_id,
                                message_id = %message_id,
                                "failed to resend a message to a client: {err:?}"
                            );
                        }
                    }
                    MessageAction::Lost {
                        message_id,
                        inform_clients,
                        caller,
                    } => {
                        info!(
                            message_id = %message_id,
                            "a client did not acknowledge a message, treating it as lost"
                        );
                        self.inform_clients_of_loss(message_id, inform_clients, caller)
                            .await;
                    }
                }
            }
        });
    }

    /// Notes that a message is waiting on its client to acknowledge it.
    async fn record_pending_client_ack(
        &self,
        connection_id: String,
        message_id: String,
        message_type: MessageType,
        message: String,
        inform_clients: Vec<String>,
        caller: Option<String>,
    ) {
        if let Some(sender) = self.client_ack_sender.get() {
            let _ = sender
                .send(AckWorkerMessage::Status(
                    message_id,
                    AckStatus::Pending {
                        connection_id,
                        message,
                        message_type,
                        inform_clients,
                        caller,
                    },
                ))
                .await;
        }
    }

    /// Records that a client acknowledged a message, so it stops being tracked.
    ///
    /// Named by the connection it came from, since a message is only settled by
    /// the client it was sent to.
    pub async fn record_client_ack(&self, connection_id: String, message_id: String) {
        if let Some(sender) = self.client_ack_sender.get() {
            let _ = sender
                .send(AckWorkerMessage::ClientAck {
                    message_id,
                    connection_id,
                })
                .await;
        }
    }

    /// Tells the clients named by a send that its message may not have arrived.
    ///
    /// Only those connected to this node are reachable from here, which is the
    /// best effort the protocol asks for.
    async fn inform_clients_of_loss(
        &self,
        message_id: String,
        inform_clients: Vec<String>,
        caller: Option<String>,
    ) {
        for client_id in inform_clients {
            let Some(connection) = self.get_connection(client_id.clone()) else {
                continue;
            };
            let event = create_message_lost_event(message_id.clone(), caller.clone());
            let mut sender = connection.lock().await;
            if let Err(err) = sender.send(Message::Binary(event.into())).await {
                debug!(
                    connection_id = %client_id,
                    "failed to tell a client about a lost message: {err:?}"
                );
            }
        }
    }

    /// Starts the ack worker if a broadcaster is present in the registry.
    /// When the current node is a part of a cluster and messages will be broadcast
    /// to other nodes in the cluster, this must be called before setting up the listener
    /// and any `send_message` calls.
    pub fn start_ack_worker(self: Arc<Self>) {
        if self.broadcaster.is_some() {
            // Only start the ack worker if the broadcaster is present as the extra
            // resilience provided by the ack worker is only needed when broadcasting
            // messages to other nodes in the cluster.
            let (ack_tx, ack_rx) = tokio::sync::mpsc::channel(1024);
            let (ack_message_action_tx, mut ack_message_action_rx) =
                tokio::sync::mpsc::channel(1024);
            let ack_worker = Worker::new(self.ack_worker_config.clone().unwrap_or_default());
            ack_worker.start(ack_rx, ack_message_action_tx);

            tokio::spawn(async move {
                // Set the ack sender for the registry in the spawned future
                // as it needs to be accessed in an async context.
                {
                    let mut ack_sender = self.ack_sender.lock().await;
                    ack_sender.replace(ack_tx);
                }

                while let Some(action) = ack_message_action_rx.recv().await {
                    match action {
                        MessageAction::Resend(resend_message_info) => {
                            let result = self
                                .send_message(
                                    resend_message_info.client_id.clone(),
                                    resend_message_info.message_id.clone(),
                                    resend_message_info.message_type.clone(),
                                    resend_message_info.message.clone(),
                                    Some(SendContext {
                                        wait_for_ack: false,
                                        caller: resend_message_info.caller.clone(),
                                        inform_clients: resend_message_info.inform_clients_on_loss,
                                    }),
                                )
                                .await;
                            if let Err(error) = result {
                                debug!(
                                    client_id = %resend_message_info.client_id,
                                    message_id = %resend_message_info.message_id,
                                    "failed to resend message to client: {error:?}"
                                );
                            }
                        }
                        MessageAction::Lost {
                            message_id,
                            inform_clients,
                            caller,
                        } => {
                            self.inform_clients_of_loss(message_id, inform_clients, caller)
                                .await;
                        }
                    }
                }
            });
        }
    }

    /// Listens for messages that have been broadcast by other nodes in the cluster.
    /// This will typically be an internal receiver for a subscriber that listens
    /// to messages broadcast by other nodes in the cluster over a network protocol.
    /// The caller is responsible for closing the channel on shutdown as it is expected
    /// to hold the transmit end of the channel.
    pub fn listen(self: Arc<Self>, mut listener: Receiver<RegistryMessage>) {
        tokio::spawn(async move {
            info!("listening for messages from other nodes in the cluster");
            while let Some(message) = listener.recv().await {
                match message {
                    RegistryMessage::WebSocket(message) => {
                        debug!(connection_id = %message.connection_id, "received message from other node");
                        if self.has_received_ack(message.message_id.clone()).await {
                            info!(message_id = %message.message_id, "already received acknowledgement for message from other node, skipping duplicate message");
                            continue;
                        }

                        if let Some(connection) = self.get_connection(message.connection_id.clone())
                        {
                            debug!(
                                connection_id = %message.connection_id,
                                "acquiring lock to send message to connection: {}",
                                message.connection_id.clone()
                            );
                            let mut connection = connection.lock().await;
                            debug!(connection_id = %message.connection_id, "sending message to connection: {}", message.connection_id);
                            let ws_message =
                                match create_ws_message(message.message_type, message.message) {
                                    Ok(msg) => msg,
                                    Err(e) => {
                                        error!(
                                            connection_id = %message.connection_id,
                                            "failed to decode message for cluster relay: {e:?}",
                                        );
                                        continue;
                                    }
                                };
                            let send_result = connection.send(ws_message).await;
                            if let Err(e) = send_result {
                                error!(
                                    connection_id = %message.connection_id,
                                    "failed to send message to websocket connection: {e:?}"
                                );
                            }

                            if let Some(broadcaster) = &self.broadcaster {
                                if broadcaster
                                    .send(RegistryMessage::Ack(AckMessage {
                                        message_id: message.message_id.clone(),
                                        message_node: message.source_node.clone(),
                                    }))
                                    .await
                                    .is_err()
                                {
                                    error!(
                                        message_id = %message.message_id,
                                        "receiver dropped for broadcaster, failed to send acknowledgement for message",
                                    );
                                }
                            }
                        }
                    }
                    RegistryMessage::Ack(message) => {
                        debug!(message_id = %message.message_id, "received acknowledgement for message from other node");
                        self.record_received_ack(message.message_id.clone()).await;
                    }
                }
            }
        });
    }

    pub fn add_connection(&self, connection_id: String, ws: Arc<Mutex<WebSocketConnSender>>) {
        self.connections.write().unwrap().insert(connection_id, ws);
    }

    pub fn remove_connection(&self, connection_id: String) {
        self.connections.write().unwrap().remove(&connection_id);
    }

    fn get_connection(&self, connection_id: String) -> Option<Arc<Mutex<WebSocketConnSender>>> {
        let conn = self
            .connections
            .read()
            .unwrap()
            .get(&connection_id)
            .cloned();
        conn
    }

    /// Returns an iterable vector of connections in the registry.
    #[allow(dead_code)]
    pub fn get_connections(&self) -> Vec<(String, Arc<Mutex<WebSocketConnSender>>)> {
        self.connections
            .read()
            .unwrap()
            .iter()
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect()
    }

    async fn has_received_ack(&self, message_id: String) -> bool {
        if let Some(ack_sender) = self.ack_sender.lock().await.as_ref() {
            let (ack_tx, ack_rx) = tokio::sync::oneshot::channel();
            ack_sender
                .send(AckWorkerMessage::Check(message_id.clone(), ack_tx))
                .await
                .expect("ack worker channel unexpectedly closed");
            let ack_status = ack_rx
                .await
                .expect("oneshot channel for ack status check unexpectedly closed");
            ack_status == AckStatus::Received
        } else {
            false
        }
    }

    async fn record_received_ack(&self, message_id: String) {
        if let Some(ack_sender) = self.ack_sender.lock().await.as_ref() {
            ack_sender
                .send(AckWorkerMessage::Status(message_id, AckStatus::Received))
                .await
                .expect("ack worker channel unexpectedly closed");
        }
    }

    async fn record_pending_ack(
        &self,
        connection_id: String,
        message_id: String,
        message_type: MessageType,
        message: String,
        inform_clients: Vec<String>,
        caller: Option<String>,
    ) {
        if let Some(ack_sender) = self.ack_sender.lock().await.as_ref() {
            ack_sender
                .send(AckWorkerMessage::Status(
                    message_id,
                    AckStatus::Pending {
                        connection_id,
                        message,
                        message_type,
                        inform_clients,
                        caller,
                    },
                ))
                .await
                .expect("ack worker channel unexpectedly closed");
        }
    }

    async fn wait_for_ack(&self, message_id: String) -> Result<(), WebSocketConnError> {
        let (ack_tx, ack_rx) = tokio::sync::oneshot::channel();

        // Record a boolean so that the lock is released before waiting for the ack on
        // the oneshot channel.
        let has_ack_sender = {
            if let Some(ack_sender) = self.ack_sender.lock().await.as_ref() {
                ack_sender
                    .send(AckWorkerMessage::Wait(message_id.clone(), ack_tx))
                    .await
                    .expect("ack worker channel unexpectedly closed");
                true
            } else {
                false
            }
        };

        if has_ack_sender {
            let ack_status = ack_rx
                .await
                .expect("oneshot channel waiting for ack unexpectedly closed");
            if ack_status == AckStatus::Received {
                return Ok(());
            } else {
                return Err(WebSocketConnError::MessageLost(message_id));
            }
        }
        Ok(())
    }
}

#[async_trait]
impl WebSocketRegistrySend for WebSocketConnRegistry {
    /// Send a message to a specific connection that may be on the same instance
    /// or on another node in the cluster.
    /// This will broadcast the message to all other nodes in the cluster if the
    /// connection is not found in the local registry.
    ///
    /// When broadcasting the message, the registry will expect an acknowledgement
    /// from the node that has the connection that the message was sent for,
    /// if an acknowledgement is not received within a timeout, the message will
    /// be resent until an acknowledgement is received or a maximum number of retries
    /// is reached.
    /// If an acknowledgement was not received after the maximum number of retries,
    /// the message will be considered lost, the caller can opt-in to wait for the ack
    /// and handle the case where the message was lost and optionally, provide context
    /// about clients connected to the current node that may have been affected by
    /// the message loss so they can be informed.
    async fn send_message(
        &self,
        connection_id: String,
        message_id: String,
        message_type: MessageType,
        message: String,
        ctx: Option<SendContext>,
    ) -> Result<(), WebSocketConnError> {
        if let Some(connection) = self.get_connection(connection_id.clone()) {
            debug!(
                connection_id = %connection_id,
                "acquiring lock to send message to connection: {}",
                connection_id
            );

            let send_ctx = ctx.unwrap_or_default();
            let awaiting_ack = client_ack_request(&message_type, &message);

            if let Some(ack_id) = awaiting_ack {
                self.record_pending_client_ack(
                    connection_id.clone(),
                    ack_id,
                    message_type.clone(),
                    message.clone(),
                    send_ctx.inform_clients,
                    send_ctx.caller,
                )
                .await;
            }

            let mut connection = connection.lock().await;
            debug!(connection_id = %connection_id, "sending message to connection: {}", connection_id);
            let ws_message = create_ws_message(message_type, message)?;
            connection.send(ws_message).await?;
        } else if let Some(broadcaster) = &self.broadcaster {
            let send_ctx = ctx.unwrap_or_default();
            debug!(connection_id = %connection_id, "connection not found locally, preparing to send message to broadcaster");
            self.record_pending_ack(
                connection_id.to_string(),
                message_id.clone(),
                message_type.clone(),
                message.clone(),
                send_ctx.inform_clients.clone(),
                send_ctx.caller.clone(),
            )
            .await;

            broadcaster
                .send(RegistryMessage::WebSocket(WebSocketMessage {
                    connection_id: connection_id.to_string(),
                    source_node: self.server_node_name.clone(),
                    inform_clients_on_loss: Some(send_ctx.inform_clients),
                    caller: send_ctx.caller,
                    message_id: message_id.clone(),
                    message_type: message_type.clone(),
                    message,
                }))
                .await?;

            if send_ctx.wait_for_ack {
                self.wait_for_ack(message_id).await?;
            }
        } else {
            // If the connection is not found locally and the current deployment is not a cluster
            // (no broadcaster), then the message is lost and the provided clients connected to
            // the current node should be informed.
            let send_ctx = ctx.unwrap_or_default();
            self.inform_clients_of_loss(
                message_id.clone(),
                send_ctx.inform_clients,
                send_ctx.caller,
            )
            .await;
            return Err(WebSocketConnError::MessageLost(message_id));
        }
        Ok(())
    }
}

impl Display for WebSocketConnRegistry {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "WebSocketConnRegistry")
    }
}

impl Debug for WebSocketConnRegistry {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("WebSocketConnRegistry")
            .field("connections", &self.connections)
            .field("broadcaster", &self.broadcaster)
            .finish()
    }
}

#[cfg(test)]
mod tests {
    use std::{
        future::Future,
        net::{Ipv4Addr, SocketAddr},
        time::Duration,
    };

    use super::*;
    use axum::{
        extract::{State, WebSocketUpgrade},
        response::Response,
        routing::get,
        Router,
    };

    use base64::Engine as _;
    use futures::{FutureExt, SinkExt, StreamExt};
    use nanoid::nanoid;
    use serde::{Deserialize, Serialize};
    use tokio_tungstenite::tungstenite;

    #[derive(Clone)]
    struct ConnectionInfo {
        connection_id: Option<String>,
        other_connection_id: Option<String>,
        missing_connection_id: Option<String>,
        registry: Arc<WebSocketConnRegistry>,
    }

    #[derive(Deserialize, Debug)]
    struct MessageLostBody {
        #[serde(rename = "messageId")]
        message_id: String,
        caller: String,
    }

    #[derive(Deserialize, Debug, Serialize)]
    struct TestMessage {
        #[serde(rename = "messageId")]
        message_id: String,
        body: String,
    }

    async fn testable_handler(
        State(conn_info): State<ConnectionInfo>,
        ws: WebSocketUpgrade,
    ) -> Response {
        ws.on_upgrade(create_handle_socket(conn_info))
    }

    fn create_handle_socket(
        conn_info: ConnectionInfo,
    ) -> impl FnOnce(WebSocket) -> std::pin::Pin<Box<dyn Future<Output = ()> + Send>> {
        move |socket| {
            let registry = conn_info.registry.clone();
            let connection_id = conn_info.connection_id.clone().unwrap_or(nanoid!());
            let other_connection_id = conn_info.other_connection_id.clone();
            let missing_connection_id = conn_info.missing_connection_id.clone();
            async move {
                // Split as the runtime does, so the registry holds only the
                // sending half and reading never contends with a sender.
                let (socket_tx, mut socket_rx) = socket.split();
                let protected_socket = Arc::new(Mutex::new(socket_tx));
                let protected_socket_clone = protected_socket.clone();
                registry.add_connection(connection_id.clone(), protected_socket_clone);

                let mut connection_alive = true;
                while connection_alive {
                    let msg_wrapped = socket_rx.next().await;
                    if let Some(Ok(msg)) = msg_wrapped {
                        if let Message::Text(msg) = msg {
                            // Broadcast received message to other connection or missing connection.
                            if let Some(other_connection_id) = &other_connection_id {
                                let _ = registry
                                    .send_message(
                                        other_connection_id.clone(),
                                        nanoid!(),
                                        MessageType::Json,
                                        msg.to_string(),
                                        None,
                                    )
                                    .await;
                            } else if let Some(missing_connection_id) = &missing_connection_id {
                                let msg_payload =
                                    serde_json::from_str::<TestMessage>(&msg).unwrap();
                                let wait_result = registry
                                    .send_message(
                                        missing_connection_id.clone(),
                                        msg_payload.message_id,
                                        MessageType::Json,
                                        msg.to_string(),
                                        Some(SendContext {
                                            wait_for_ack: true,
                                            caller: Some("test-caller".to_string()),
                                            inform_clients: vec![connection_id.clone()],
                                        }),
                                    )
                                    .await;
                                if let Err(WebSocketConnError::MessageLost(message_id)) =
                                    wait_result
                                {
                                    // Message was lost, inform the client that sent the message
                                    // to get full coverage on behaviour to manually wait for the ack.
                                    protected_socket
                                        .lock()
                                        .await
                                        .send(Message::Text(
                                            format!("Custom message lost event: {message_id}")
                                                .into(),
                                        ))
                                        .await
                                        .unwrap();
                                }
                            } else {
                                // When "other connection" is not statically set,
                                // broadcast to all other connections.
                                for (id, conn) in registry.get_connections().iter() {
                                    if *id != connection_id {
                                        let mut conn = conn.lock().await;
                                        conn.send(Message::Text(msg.clone())).await.unwrap();
                                    }
                                }
                            }
                        }
                    } else {
                        connection_alive = false;
                    }
                }
                registry.remove_connection(connection_id.clone());
            }
            .boxed()
        }
    }

    #[test_log::test(tokio::test)]
    async fn test_ws_conn_registry_broadcast_messages_to_other_nodes() {
        let (node1_tx, node1_rx) = tokio::sync::mpsc::channel(1024);
        let (node2_tx, node2_rx) = tokio::sync::mpsc::channel(1024);

        // Node 1 broadcasts to node 2, listens with node 1 receiver.
        let node1_registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: None,
                server_node_name: "node1".to_string(),
            },
            Some(node2_tx),
        ));
        node1_registry.clone().start_ack_worker();
        node1_registry.clone().listen(node1_rx);

        // Node 2 broadcasts to node 1, listens with node 2 receiver.
        let node2_registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: None,
                server_node_name: "node2".to_string(),
            },
            Some(node1_tx),
        ));
        node2_registry.clone().start_ack_worker();
        node2_registry.clone().listen(node2_rx);

        let app1: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: Some("node1".to_string()),
                other_connection_id: Some("node2".to_string()),
                missing_connection_id: None,
                registry: node1_registry,
            });

        let app2: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: Some("node2".to_string()),
                other_connection_id: Some("node1".to_string()),
                missing_connection_id: None,
                registry: node2_registry,
            });

        let listener1 = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr1 = listener1.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener1, app1).await.unwrap();
        });

        let listener2 = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr2 = listener2.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener2, app2).await.unwrap();
        });

        let (mut socket1, _response) = tokio_tungstenite::connect_async(format!("ws://{addr1}/ws"))
            .await
            .unwrap();

        let (mut socket2, _response) = tokio_tungstenite::connect_async(format!("ws://{addr2}/ws"))
            .await
            .unwrap();

        socket1
            .send(tungstenite::Message::Text(
                "Hello, forward this to Node 2!".to_string(),
            ))
            .await
            .unwrap();

        let node2_msg_received = match socket2.next().await.unwrap().unwrap() {
            tungstenite::Message::Text(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };

        assert_eq!(node2_msg_received, "Hello, forward this to Node 2!");

        socket2
            .send(tungstenite::Message::Text(
                "Hello, forward this to Node 1!".to_string(),
            ))
            .await
            .unwrap();

        let node1_msg_received = match socket1.next().await.unwrap().unwrap() {
            tungstenite::Message::Text(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };

        assert_eq!(node1_msg_received, "Hello, forward this to Node 1!");
    }

    #[test_log::test(tokio::test)]
    async fn test_ws_conn_registry_handles_message_broadcast_to_missing_connection() {
        // Node 1 broadcasts a message to a missing connection and after
        // a maximum number of retries, the message will be considered lost
        // and the client that sent the message will be informed
        let (_, node1_rx) = tokio::sync::mpsc::channel(1024);

        // Broadcaster is used to send messages to all other nodes in the cluster.
        let (broadcaster_tx, mut broadcaster_rx) = tokio::sync::mpsc::channel(1024);
        tokio::spawn(async move {
            // We need to manually receive messages from the broadcaster to act like a real
            // intermediary receiver that would forward messages to a network protocol broadcaster
            // to avoid blocking when waiting for messages to be sent to the broadcaster.
            while let Some(msg) = broadcaster_rx.recv().await {
                println!("broadcaster received message: {msg:?}");
            }
        });

        // Node 1 broadcasts to node 2, listens with node 1 receiver.
        let node1_registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: Some(AckWorkerConfig {
                    message_action_check_interval_ms: Some(10),
                    message_timeout_ms: Some(50),
                    max_attempts: Some(3),
                }),
                server_node_name: "node1".to_string(),
            },
            Some(broadcaster_tx),
        ));
        node1_registry.clone().start_ack_worker();
        node1_registry.clone().listen(node1_rx);

        let app1: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: Some("node1".to_string()),
                other_connection_id: None,
                missing_connection_id: Some("node3".to_string()),
                registry: node1_registry,
            });

        let listener1 = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr1 = listener1.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener1, app1).await.unwrap();
        });

        let (mut socket1, _response) = tokio_tungstenite::connect_async(format!("ws://{addr1}/ws"))
            .await
            .unwrap();

        socket1
            .send(tungstenite::Message::Text(
                serde_json::to_string(&TestMessage {
                    message_id: "test-message-1".to_string(),
                    body: "Hello, forward this to Node 3!".to_string(),
                })
                .unwrap(),
            ))
            .await
            .unwrap();

        // Both notifications arrive, one from the handler waiting on the ack
        // itself and one from the registry acting on the ack worker's event.
        // Their order is not guaranteed. They come from separate tasks, and
        // nothing sequences them now that reading a connection no longer holds
        // a lock the senders have to wait behind.
        let mut manual_msg = None;
        let mut lost_event = None;
        for _ in 0..2 {
            match socket1.next().await.unwrap().unwrap() {
                tungstenite::Message::Text(msg) => manual_msg = Some(msg),
                tungstenite::Message::Binary(msg) => lost_event = Some(msg),
                other => panic!("Unexpected message but got {other:?}"),
            }
        }

        assert_eq!(
            manual_msg.expect("the handler should report the lost message itself"),
            "Custom message lost event: test-message-1"
        );

        // After a number of retry attempts, the message will be considered lost
        // and client should be informed with a message lost event.
        let node1_msg_received =
            lost_event.expect("the registry should send a protocol-level lost message event");
        // `[routeLength][route][requireAck][messageIdLength]`, all four of which
        // a client matches on before it will treat this as a lost message.
        assert_eq!(node1_msg_received[..4], [0x1, 0x3, 0x0, 0x0]);
        let json_msg = String::from_utf8(node1_msg_received[4..].to_vec()).unwrap();
        let message_lost_event: MessageLostBody = serde_json::from_str(&json_msg).unwrap();
        assert_eq!(message_lost_event.message_id, "test-message-1");
        // Carried through from the send, so the client is told what the lost
        // message was for and not only that one went missing.
        assert_eq!(message_lost_event.caller, "test-caller");
    }

    #[test_log::test(tokio::test)]
    async fn test_ws_conn_registry_sends_messages_to_connection_on_same_instance() {
        let (tx, _) = tokio::sync::mpsc::channel(1024);
        let registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: None,
                server_node_name: "node1".to_string(),
            },
            Some(tx),
        ));

        let app: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                // Allow dynamic IDs to be assigned to connections.
                connection_id: None,
                other_connection_id: None,
                missing_connection_id: None,
                registry,
            });

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        let (mut socket1, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
            .await
            .unwrap();

        let (mut socket2, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
            .await
            .unwrap();

        socket1
            .send(tungstenite::Message::Text(
                "Hello, forward this to Connection 2!".to_string(),
            ))
            .await
            .unwrap();

        let socket2_msg_received = match socket2.next().await.unwrap().unwrap() {
            tungstenite::Message::Text(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };

        assert_eq!(socket2_msg_received, "Hello, forward this to Connection 2!");
    }

    /// A connection that is registered and then removed leaves nothing behind.
    ///
    /// The registry holds a sender for every connection in it, and callers
    /// broadcast by walking that set, so an entry that outlives its connection
    /// is a sender nothing can deliver through and a connection that still
    /// counts as present. Every path that ends a connection has to reach
    /// `remove_connection`, including the ones that refuse a connection after
    /// registering it, which is why this covers the primitive they all rely on.
    #[test_log::test(tokio::test)]
    async fn test_ws_conn_registry_removes_connections() {
        let (tx, _) = tokio::sync::mpsc::channel(1024);
        let registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: None,
                server_node_name: "node1".to_string(),
            },
            Some(tx),
        ));

        let app: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: None,
                other_connection_id: None,
                missing_connection_id: None,
                registry: registry.clone(),
            });

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        // Connected one at a time, because the handler names each connection
        // itself and reading the registry between the two is the only way to
        // tell which id belongs to which socket.
        let (mut socket1, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
            .await
            .unwrap();
        wait_for_connection_count(&registry, 1).await;
        let first_id = registry.get_connections()[0].0.clone();

        let (_socket2, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
            .await
            .unwrap();
        wait_for_connection_count(&registry, 2).await;
        let second_id = registry
            .get_connections()
            .into_iter()
            .map(|(id, _)| id)
            .find(|id| *id != first_id)
            .expect("the second connection should have an id of its own");

        socket1.close(None).await.unwrap();

        wait_for_connection_count(&registry, 1).await;

        // Named rather than counted. A registry that dropped the wrong
        // connection would hold one either way.
        let remaining: Vec<String> = registry
            .get_connections()
            .into_iter()
            .map(|(id, _)| id)
            .collect();
        assert_eq!(
            remaining,
            vec![second_id],
            "closing one connection should remove that one and leave the other"
        );
    }

    /// Sets up a registry serving one client, with acknowledgement timings
    /// short enough for a test to watch a resend happen.
    async fn registry_with_one_client() -> (
        Arc<WebSocketConnRegistry>,
        String,
        tokio_tungstenite::WebSocketStream<
            tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>,
        >,
    ) {
        let (tx, _) = tokio::sync::mpsc::channel(1024);
        let registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: Some(AckWorkerConfig {
                    message_action_check_interval_ms: Some(50),
                    message_timeout_ms: Some(100),
                    max_attempts: Some(3),
                }),
                server_node_name: "node1".to_string(),
            },
            Some(tx),
        ));
        registry.clone().start_client_ack_worker();

        let app: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: None,
                other_connection_id: None,
                missing_connection_id: None,
                registry: registry.clone(),
            });

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        let (socket, _response) = tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
            .await
            .unwrap();
        wait_for_connection_count(&registry, 1).await;
        let connection_id = registry.get_connections()[0].0.clone();

        (registry, connection_id, socket)
    }

    /// A message that asks to be acknowledged is chased until the client
    /// answers, and stops once it does.
    #[test_log::test(tokio::test)]
    async fn test_a_message_asking_to_be_acknowledged_is_settled_by_the_client() {
        let (registry, connection_id, mut socket) = registry_with_one_client().await;

        registry
            .send_message(
                connection_id.clone(),
                "m-1".to_string(),
                MessageType::Json,
                r#"{"event":"update","data":{"value":1},"messageId":"m-1","ack":true}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await
            .unwrap();

        let received = match socket.next().await.unwrap().unwrap() {
            tungstenite::Message::Text(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };
        let received: serde_json::Value = serde_json::from_str(&received).unwrap();
        // Delivered as composed, opt in and all.
        assert_eq!(received["data"]["value"], 1);
        assert_eq!(received["messageId"], "m-1");
        assert_eq!(received["ack"], true);

        registry
            .record_client_ack(connection_id, "m-1".to_string())
            .await;

        // Long enough for several rounds of the worker's checks, so a message
        // still being chased would show up here.
        let resent = tokio::time::timeout(Duration::from_millis(400), socket.next()).await;
        assert!(
            resent.is_err(),
            "an acknowledged message should stop being resent, got {resent:?}"
        );
    }

    /// Binary messages ask to be acknowledged the same way JSON ones do, by
    /// saying so in the frame the application composed.
    #[test_log::test(tokio::test)]
    async fn test_a_binary_message_can_ask_to_be_acknowledged() {
        let (registry, connection_id, mut socket) = registry_with_one_client().await;

        // [routeLength][route][requireAck][messageIdLength][messageId][payload]
        let mut original = vec![7u8];
        original.extend_from_slice(b"updates");
        original.push(0x1);
        original.push(5);
        original.extend_from_slice(b"m-bin");
        original.extend_from_slice(&[0xde, 0xad, 0xbe, 0xef]);

        registry
            .send_message(
                connection_id.clone(),
                "m-bin".to_string(),
                MessageType::Binary,
                base64::prelude::BASE64_STANDARD.encode(&original),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await
            .unwrap();

        let received = match socket.next().await.unwrap().unwrap() {
            tungstenite::Message::Binary(bytes) => bytes,
            other => panic!("Unexpected message but got {other:?}"),
        };

        // Delivered exactly as the application framed it. The runtime reads the
        // opt in, it does not add or change one.
        assert_eq!(&received[..], original.as_slice());

        // Sent again while it goes unanswered, which is what says the opt in was
        // read at all. Without waiting for this the test passes just as well
        // against a runtime that never reads the byte, since a message nothing
        // is waiting on is also a message that is never resent.
        let resent = tokio::time::timeout(Duration::from_millis(400), socket.next())
            .await
            .expect("a binary message asking to be acknowledged should be sent again")
            .unwrap()
            .unwrap();
        match resent {
            tungstenite::Message::Binary(bytes) => assert_eq!(&bytes[..], original.as_slice()),
            other => panic!("Unexpected message but got {other:?}"),
        }

        registry
            .record_client_ack(connection_id, "m-bin".to_string())
            .await;
        let after_ack = tokio::time::timeout(Duration::from_millis(400), socket.next()).await;
        assert!(
            after_ack.is_err(),
            "an acknowledged message should not be resent, got {after_ack:?}"
        );
    }

    /// A message nobody ever acknowledges is eventually given up on.
    ///
    /// Each resend has to count as an attempt, or the message is sent again on
    /// every check for as long as the connection lives and the clients waiting
    /// to hear that it went missing never do.
    #[test_log::test(tokio::test)]
    async fn test_a_message_nobody_acknowledges_is_declared_lost() {
        let (registry, connection_id, mut socket) = registry_with_one_client().await;

        registry
            .send_message(
                connection_id.clone(),
                "m-4".to_string(),
                MessageType::Json,
                r#"{"event":"update","data":{"value":4},"messageId":"m-4","ack":true}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: Some("caller-route".to_string()),
                    // Told to itself, so the loss event lands somewhere this
                    // test can watch.
                    inform_clients: vec![connection_id],
                }),
            )
            .await
            .unwrap();

        // Three attempts at 100ms apart, checked every 50ms, so a second or so
        // covers the escalation with room for a loaded machine.
        let lost = tokio::time::timeout(Duration::from_secs(3), async {
            while let Some(Ok(message)) = socket.next().await {
                if let tungstenite::Message::Binary(bytes) = message {
                    if bytes.starts_with(&[0x1, 0x3, 0x0, 0x0]) {
                        let payload: serde_json::Value =
                            serde_json::from_slice(&bytes[4..]).unwrap();
                        return Some(payload);
                    }
                }
            }
            None
        })
        .await
        .expect("a message nobody acknowledges should be declared lost rather than resent forever");

        let payload = lost.expect("the connection closed before the loss event arrived");
        assert_eq!(payload["messageId"], "m-4");
        assert_eq!(payload["caller"], "caller-route");
    }

    /// A message is only settled by the client it was sent to.
    ///
    /// A message id is chosen by the application, so it says nothing about who
    /// the message went to, and the same id may be in flight to several clients
    /// at once. An acknowledgement from anyone else has to count for nothing,
    /// or one client can call off the resend and the loss event another is
    /// relying on.
    #[test_log::test(tokio::test)]
    async fn test_a_message_is_not_settled_by_a_different_connection() {
        let (registry, connection_id, mut socket) = registry_with_one_client().await;

        registry
            .send_message(
                connection_id,
                "m-3".to_string(),
                MessageType::Json,
                r#"{"event":"update","data":{"value":3},"messageId":"m-3","ack":true}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await
            .unwrap();

        registry
            .record_client_ack("another-connection".to_string(), "m-3".to_string())
            .await;

        let mut deliveries = 0;
        while let Ok(Some(Ok(message))) =
            tokio::time::timeout(Duration::from_millis(400), socket.next()).await
        {
            if let tungstenite::Message::Text(text) = message {
                let value: serde_json::Value = serde_json::from_str(&text).unwrap();
                if value["messageId"] == "m-3" {
                    deliveries += 1;
                }
            }
            if deliveries > 1 {
                break;
            }
        }

        assert!(
            deliveries > 1,
            "an acknowledgement from another connection should settle nothing, \
             but the message stopped being resent after {deliveries} delivery(s)"
        );
    }

    /// A message the client never acknowledges is sent again.
    #[test_log::test(tokio::test)]
    async fn test_a_message_asking_to_be_acknowledged_is_resent_while_unanswered() {
        let (registry, connection_id, mut socket) = registry_with_one_client().await;

        registry
            .send_message(
                connection_id,
                "m-2".to_string(),
                MessageType::Json,
                r#"{"event":"update","data":{"value":2},"messageId":"m-2","ack":true}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: None,
                    inform_clients: vec![],
                }),
            )
            .await
            .unwrap();

        let mut deliveries = 0;
        while let Ok(Some(Ok(message))) =
            tokio::time::timeout(Duration::from_millis(400), socket.next()).await
        {
            if let tungstenite::Message::Text(text) = message {
                let value: serde_json::Value = serde_json::from_str(&text).unwrap();
                if value["messageId"] == "m-2" {
                    deliveries += 1;
                }
            }
            if deliveries > 1 {
                break;
            }
        }

        assert!(
            deliveries > 1,
            "a message nobody acknowledged should be sent again, saw it {deliveries} time(s)"
        );
    }

    /// Waits for the registry to hold the given number of connections.
    ///
    /// Registration and removal happen on the connection's own task, so a test
    /// that looks straight after connecting or closing races that task.
    async fn wait_for_connection_count(registry: &Arc<WebSocketConnRegistry>, expected: usize) {
        let deadline = tokio::time::Instant::now() + std::time::Duration::from_secs(5);
        while registry.get_connections().len() != expected {
            assert!(
                tokio::time::Instant::now() < deadline,
                "expected {expected} connections, registry has {}",
                registry.get_connections().len()
            );
            tokio::time::sleep(std::time::Duration::from_millis(20)).await;
        }
    }

    /// Holds a connection open in the state a lost message send has to survive,
    /// where the client has gone but the registry does not know it yet.
    ///
    /// A connection is only taken out of the registry by its own read loop, and
    /// that loop only ends once it sees the client leave. Between the client
    /// going and the loop noticing, the registry hands out a connection whose
    /// every write fails. Reading nothing here holds that window open for as
    /// long as a test needs it, rather than leaving the test racing the loop.
    ///
    /// Returning closes nothing, since the registry holds the sending half of
    /// the split socket and the socket lives while either half does.
    fn create_unwatched_socket(
        conn_info: ConnectionInfo,
    ) -> impl FnOnce(WebSocket) -> std::pin::Pin<Box<dyn Future<Output = ()> + Send>> {
        move |socket| {
            let registry = conn_info.registry.clone();
            async move {
                let (socket_tx, _socket_rx) = socket.split();
                registry.add_connection(nanoid!(), Arc::new(Mutex::new(socket_tx)));
            }
            .boxed()
        }
    }

    async fn unwatched_connection_handler(
        State(conn_info): State<ConnectionInfo>,
        ws: WebSocketUpgrade,
    ) -> Response {
        ws.on_upgrade(create_unwatched_socket(conn_info))
    }

    /// A client that has gone does not cost the others their notification.
    ///
    /// Naming a client that can no longer be written to is ordinary, since a
    /// connection can go at any point and a message being declared lost is
    /// itself a sign that something is wrong with the connections involved.
    #[test_log::test(tokio::test)]
    async fn test_a_client_that_has_gone_does_not_stop_the_others_being_told() {
        let registry = Arc::new(WebSocketConnRegistry::new(
            WebSocketConnRegistryConfig {
                ack_worker_config: None,
                server_node_name: "node1".to_string(),
            },
            None,
        ));

        let app: Router = Router::new()
            .route("/ws", get(unwatched_connection_handler))
            .with_state(ConnectionInfo {
                connection_id: None,
                other_connection_id: None,
                missing_connection_id: None,
                registry: registry.clone(),
            });

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        // Connected by hand, and one at a time so each id can be told apart.
        // By hand because the socket has to be made to reset rather than close,
        // which is what makes a later write fail rather than succeed into a
        // buffer nothing will ever read.
        let gone_stream = tokio::net::TcpStream::connect(addr).await.unwrap();
        // Deprecated because lingering can block a thread on drop. A zero
        // timeout has nothing to wait for, it only asks for a reset.
        #[allow(deprecated)]
        gone_stream
            .set_linger(Some(std::time::Duration::ZERO))
            .unwrap();
        let (gone_socket, _response) =
            tokio_tungstenite::client_async(format!("ws://{addr}/ws"), gone_stream)
                .await
                .unwrap();
        wait_for_connection_count(&registry, 1).await;
        let gone_id = registry.get_connections()[0].0.clone();

        let (mut live_socket, _response) =
            tokio_tungstenite::connect_async(format!("ws://{addr}/ws"))
                .await
                .unwrap();
        wait_for_connection_count(&registry, 2).await;
        let live_id = registry
            .get_connections()
            .into_iter()
            .map(|(id, _)| id)
            .find(|id| *id != gone_id)
            .expect("the second connection should have an id of its own");

        drop(gone_socket);
        tokio::time::sleep(std::time::Duration::from_millis(100)).await;

        // Named first, so a send that gives up on the first failure never
        // reaches the client that is still there.
        let result = registry
            .send_message(
                "a-connection-that-is-not-here".to_string(),
                "m-1".to_string(),
                MessageType::Json,
                r#"{"event":"update"}"#.to_string(),
                Some(SendContext {
                    wait_for_ack: false,
                    caller: Some("test-caller".to_string()),
                    inform_clients: vec![gone_id, live_id],
                }),
            )
            .await;

        assert!(
            matches!(result, Err(WebSocketConnError::MessageLost(ref id)) if id == "m-1"),
            "a message with nowhere to go should be reported lost, got {result:?}"
        );

        let received = tokio::time::timeout(std::time::Duration::from_secs(5), live_socket.next())
            .await
            .expect("the client still connected should have been told about the lost message")
            .expect("the connection closed before the lost message arrived")
            .unwrap();
        let tungstenite::Message::Binary(bytes) = received else {
            panic!("expected a binary lost message event, got {received:?}");
        };
        assert_eq!(bytes[..4], [0x1, 0x3, 0x0, 0x0]);
        let body: MessageLostBody = serde_json::from_slice(&bytes[4..]).unwrap();
        assert_eq!(body.message_id, "m-1");
    }
}
