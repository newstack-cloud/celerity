use std::{
    collections::HashMap,
    fmt::{Debug, Display},
    sync::{Arc, RwLock},
};

use async_trait::async_trait;
use axum::extract::ws::{Message, WebSocket};
use tokio::sync::{
    mpsc::{Receiver, Sender},
    Mutex,
};
use tracing::{debug, error, info};

use crate::{
    acks::{AckStatus, AckWorkerMessage, MessageAction, Worker},
    errors::WebSocketConnError,
    message_helpers::create_message_lost_event,
    types::{AckMessage, AckWorkerConfig, Message as RegistryMessage, WebSocketMessage},
};

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
    connections: Arc<RwLock<HashMap<String, Arc<Mutex<WebSocket>>>>>,
    // A channel for sending messages to the ack worker.
    ack_sender: Mutex<Option<Sender<AckWorkerMessage>>>,
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
            broadcaster,
            server_node_name: config.server_node_name,
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
                                    resend_message_info.message.clone(),
                                    Some(SendContext {
                                        wait_for_ack: false,
                                        caller: None,
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
                        MessageAction::Lost(message_id, inform_clients) => {
                            for client_id in inform_clients {
                                // Only inform clients that are connected to the current node.
                                if let Some(connection) = self.get_connection(client_id.clone()) {
                                    debug!(
                                        connection_id = %client_id,
                                        "acquiring lock to send message lost event to connection: {}",
                                        client_id.clone()
                                    );
                                    let mut conn_lock = connection.lock().await;
                                    debug!(connection_id = %client_id, "sending message lost event to connection: {}", client_id);
                                    conn_lock
                                        .send(Message::Binary(create_message_lost_event(
                                            message_id.clone(),
                                        )))
                                        .await
                                        .unwrap();
                                }
                            }
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
    #[allow(dead_code)]
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
                            let send_result = connection.send(Message::Text(message.message)).await;
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

    pub fn add_connection(&self, connection_id: String, ws: Arc<Mutex<WebSocket>>) {
        self.connections.write().unwrap().insert(connection_id, ws);
    }

    pub fn remove_connection(&self, connection_id: String) {
        self.connections.write().unwrap().remove(&connection_id);
    }

    fn get_connection(&self, connection_id: String) -> Option<Arc<Mutex<WebSocket>>> {
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
    pub fn get_connections(&self) -> Vec<(String, Arc<Mutex<WebSocket>>)> {
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
        message_id: String,
        message: String,
        inform_clients: Vec<String>,
    ) {
        if let Some(ack_sender) = self.ack_sender.lock().await.as_ref() {
            ack_sender
                .send(AckWorkerMessage::Status(
                    message_id,
                    AckStatus::Pending(message, inform_clients),
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
        message: String,
        ctx: Option<SendContext>,
    ) -> Result<(), WebSocketConnError> {
        if let Some(connection) = self.get_connection(connection_id.clone()) {
            debug!(
                connection_id = %connection_id,
                "acquiring lock to send message to connection: {}",
                connection_id
            );
            let mut connection = connection.lock().await;
            debug!(connection_id = %connection_id, "sending message to connection: {}", connection_id);
            connection.send(Message::Text(message)).await?;
        } else if let Some(broadcaster) = &self.broadcaster {
            let send_ctx = ctx.unwrap_or_default();
            debug!(connection_id = %connection_id, "connection not found locally, preparing to send message to broadcaster");
            self.record_pending_ack(
                message_id.clone(),
                message.clone(),
                send_ctx.inform_clients.clone(),
            )
            .await;

            broadcaster
                .send(RegistryMessage::WebSocket(WebSocketMessage {
                    connection_id: connection_id.to_string(),
                    source_node: self.server_node_name.clone(),
                    inform_clients_on_loss: Some(send_ctx.inform_clients),
                    caller: send_ctx.caller,
                    message_id: message_id.clone(),
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
            for client_id in send_ctx.inform_clients {
                if let Some(connection) = self.get_connection(client_id.clone()) {
                    connection
                        .lock()
                        .await
                        .send(Message::Binary(create_message_lost_event(
                            message_id.clone(),
                        )))
                        .await?;
                }
            }
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

    use futures::{FutureExt, SinkExt, StreamExt};
    use nanoid::nanoid;
    use serde::{Deserialize, Serialize};
    use tokio::time::sleep;
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
                let protected_socket = Arc::new(Mutex::new(socket));
                let protected_socket_clone = protected_socket.clone();
                registry.add_connection(connection_id.clone(), protected_socket_clone);

                let mut connection_alive = true;
                while connection_alive {
                    // Wait some time before acquiring the lock again to allow other tasks to write
                    // to the socket. (i.e. a message received from another node in the cluster)
                    sleep(Duration::from_millis(10)).await;
                    let mut socket_lock = protected_socket.lock().await;
                    tokio::select! {
                        msg_wrapped = socket_lock.recv() => {
                            if let Some(Ok(msg)) = msg_wrapped {
                                if let Message::Text(msg) = msg {
                                    // Broadcast received message to other connection or missing connection.
                                    if let Some(other_connection_id) = &other_connection_id {
                                        let _ = registry
                                            .send_message(other_connection_id.clone(), nanoid!(), msg, None)
                                            .await;
                                    } else if let Some(missing_connection_id) = &missing_connection_id {
                                        let msg_payload = serde_json::from_str::<TestMessage>(&msg).unwrap();
                                        let wait_result = registry
                                            .send_message(
                                                missing_connection_id.clone(),
                                                msg_payload.message_id,
                                                msg,
                                                Some(SendContext {
                                                    wait_for_ack: true,
                                                    caller: None,
                                                    inform_clients: vec![connection_id.clone()],
                                                }),
                                            )
                                            .await;
                                        if let Err(WebSocketConnError::MessageLost(message_id)) =
                                            wait_result
                                        {
                                            // Message was lost, inform the client that sent the message
                                            // to get full coverage on behaviour to manually wait for the ack.
                                            socket_lock.send(Message::Text(format!("Custom message lost event: {message_id}"))).await.unwrap();
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
                        // Timeout to allow other tasks to write to the socket
                        // at an interval.
                        _ = sleep(Duration::from_secs(5)) => {
                            // Skip to next iteration of the loop to release the lock.
                        },
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

        // The custom message lost event should be received first as is called by the test handler
        // while it holds a lock on the socket, when this is released, the protocol-level message lost
        // message will be sent to the socket by the registry in response to ack worker events.
        let node1_manual_wait_for_ack_msg_received = match socket1.next().await.unwrap().unwrap() {
            tungstenite::Message::Text(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };

        assert_eq!(
            node1_manual_wait_for_ack_msg_received,
            "Custom message lost event: test-message-1"
        );

        let node1_msg_received = match socket1.next().await.unwrap().unwrap() {
            tungstenite::Message::Binary(msg) => msg,
            other => panic!("Unexpected message but got {other:?}"),
        };

        // After a number of retry attempts, the message will be considered lost
        // and client should be informed with a message lost event.
        assert_eq!(node1_msg_received[0], 0x1); // Route length should be 1
        assert_eq!(node1_msg_received[1], 0x3); // Route should be 0x3 (message lost)
        let json_msg = String::from_utf8(node1_msg_received[2..].to_vec()).unwrap();
        let message_lost_event: MessageLostBody = serde_json::from_str(&json_msg).unwrap();
        assert_eq!(message_lost_event.message_id, "test-message-1");
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
}
