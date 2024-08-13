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
use tracing::{debug, info};

use crate::{errors::WebSocketConnError, types::WebSocketMessage};

#[async_trait]
/// Provides a trait for sending messages to WebSocket connections.
pub trait WebSocketRegistrySend: Send + Sync + Display + Debug {
    async fn send_message(
        &self,
        connection_id: String,
        message: String,
    ) -> Result<(), WebSocketConnError>;
}

#[derive(Clone)]
/// Provides a registry for WebSocket connections.
/// This allows for sending messages to WebSocket connections
/// in the current runtime instance and on other nodes in a cluster.
pub struct WebSocketConnRegistry {
    // WebSockets do not implement Sync so we need to wrap them in Arc<Mutex<...>>
    // to safely send messages to WebSocket connections from multiple threads.
    connections: Arc<RwLock<HashMap<String, Arc<Mutex<WebSocket>>>>>,
    // This is called "broadcaster" because it is used to send messages to all
    // other nodes in a cluster, however, it should not be confused with a broadcast::Sender
    // for in-process broadcasting. Typically, there will be a single receiver in the same process
    // that will broadcast messages to all other nodes in the cluster via a pub/sub mechanism
    // over a network protocol.
    broadcaster: Option<Sender<WebSocketMessage>>,
}

impl WebSocketConnRegistry {
    pub fn new(broadcaster: Option<Sender<WebSocketMessage>>) -> Self {
        WebSocketConnRegistry {
            connections: Arc::new(RwLock::new(HashMap::new())),
            broadcaster,
        }
    }

    /// Listens for messages that have been broadcasted by other nodes in the cluster.
    /// This will typically be an internal receiver for a subscriber that listens
    /// to messages broadcast by other nodes in the cluster over a network protocol.
    /// The caller is responsible for closing the channel on shutdown as it is expected
    /// to hold the transmit end of the channel.
    pub fn listen(self: Arc<Self>, mut listener: Receiver<WebSocketMessage>) {
        tokio::spawn(async move {
            info!("listening for messages from other nodes in the cluster");
            while let Some(message) = listener.recv().await {
                debug!(connection_id = %message.connection_id, "Received message from other node");
                if let Some(connection) = self.get_connection(message.connection_id.clone()) {
                    debug!(
                        connection_id = %message.connection_id,
                        "acquiring lock to send message to connection: {}",
                        message.connection_id.clone()
                    );
                    let mut connection = connection.lock().await;
                    debug!(connection_id = %message.connection_id, "sending message to connection: {}", message.connection_id);
                    connection
                        .send(Message::Text(message.message))
                        .await
                        .unwrap();
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
    pub fn get_connections(&self) -> Vec<(String, Arc<Mutex<WebSocket>>)> {
        self.connections
            .read()
            .unwrap()
            .iter()
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect()
    }
}

#[async_trait]
impl WebSocketRegistrySend for WebSocketConnRegistry {
    /// Send a message to a specific connection that may be on the same instance
    /// or on another node in the cluster.
    /// This will broadcast the message to all other nodes in the cluster if the
    /// connection is not found in the local registry.
    async fn send_message(
        &self,
        connection_id: String,
        message: String,
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
            debug!(connection_id = %connection_id, "connection not found locally, sending message to broadcaster");
            broadcaster
                .send(WebSocketMessage {
                    connection_id: connection_id.to_string(),
                    message,
                })
                .await?;
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
    use tokio::time::sleep;
    use tokio_tungstenite::tungstenite;

    #[derive(Clone)]
    struct ConnectionInfo {
        connection_id: Option<String>,
        other_connection_id: Option<String>,
        registry: Arc<WebSocketConnRegistry>,
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
                                    // Broadcast received message to other connection.
                                    if let Some(other_connection_id) = &other_connection_id {
                                        let _ = registry
                                            .send_message(other_connection_id.clone(), msg)
                                            .await;
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
        let node1_registry = Arc::new(WebSocketConnRegistry::new(Some(node2_tx)));
        node1_registry.clone().listen(node1_rx);

        // Node 2 broadcasts to node 1, listens with node 2 receiver.
        let node2_registry = Arc::new(WebSocketConnRegistry::new(Some(node1_tx)));
        node2_registry.clone().listen(node2_rx);

        let app1: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: Some("node1".to_string()),
                other_connection_id: Some("node2".to_string()),
                registry: node1_registry,
            });

        let app2: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                connection_id: Some("node2".to_string()),
                other_connection_id: Some("node1".to_string()),
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
            other => panic!("Unexpected message but got {:?}", other),
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
            other => panic!("Unexpected message but got {:?}", other),
        };

        assert_eq!(node1_msg_received, "Hello, forward this to Node 1!");
    }

    #[test_log::test(tokio::test)]
    async fn test_ws_conn_registry_sends_messages_to_connection_on_same_instance() {
        let (tx, _) = tokio::sync::mpsc::channel(1024);
        let registry = Arc::new(WebSocketConnRegistry::new(Some(tx)));

        let app: Router = Router::new()
            .route("/ws", get(testable_handler))
            .with_state(ConnectionInfo {
                // Allow dynamic IDs to be assigned to connections.
                connection_id: None,
                other_connection_id: None,
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
            other => panic!("Unexpected message but got {:?}", other),
        };

        assert_eq!(socket2_msg_received, "Hello, forward this to Connection 2!");
    }
}
