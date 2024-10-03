use std::{
    collections::HashMap,
    fmt::Debug,
    ops::ControlFlow,
    sync::Arc,
    time::{Duration, Instant},
};

use async_trait::async_trait;
use axum::{
    extract::{
        ws::{Message, WebSocket},
        State, WebSocketUpgrade,
    },
    response::IntoResponse,
    Extension,
};
use axum_client_ip::SecureClientIp;
use axum_extra::{headers, TypedHeader};
use nanoid::nanoid;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::{sync::Mutex, time::sleep};
use tracing::{error, info, info_span, Instrument};

use crate::{
    errors::WebSocketsMessageError, request::RequestId, wsconn_registry::WebSocketConnRegistry,
};

#[derive(Clone, Debug)]
pub(crate) struct WebSocketAppState {
    pub connections: Arc<WebSocketConnRegistry>,
    pub routes: Arc<HashMap<String, Arc<dyn WebSocketMessageHandler + Send + Sync>>>,
    pub route_key: String,
}

/// A JSON message received from a WebSocket client with additional information.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct JsonMessageInfo {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "eventType")]
    pub event_type: WebSocketEventType,
    #[serde(rename = "messageId")]
    pub message_id: String,
    // Loosely typed JSON value that represents the message body parsed
    // from a text WebSocket message.
    pub body: serde_json::Value,
}

/// A binary message received from a WebSocket client with additional information.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct BinaryMessageInfo<'a> {
    // The connection ID for the client that sent the message.
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "eventType")]
    pub event_type: WebSocketEventType,
    #[serde(rename = "messageId")]
    pub message_id: String,
    // The body of the binary message after stripping routing information
    // from the beginning of the message.
    pub body: &'a [u8],
}

/// The type of event that occurred on the WebSocket connection.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub enum WebSocketEventType {
    #[serde(rename = "connect")]
    Connect,
    #[serde(rename = "message")]
    Message,
    #[serde(rename = "disconnect")]
    Disconnect,
}

#[async_trait]
pub trait WebSocketMessageHandler {
    async fn handle_json_message(
        &self,
        message: JsonMessageInfo,
    ) -> Result<(), WebSocketsMessageError>;
    async fn handle_binary_message(
        &self,
        message: BinaryMessageInfo,
    ) -> Result<(), WebSocketsMessageError>;
}

impl Debug for dyn WebSocketMessageHandler + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "WebSocketMessageHandler")
    }
}

pub(crate) async fn handler(
    ws: WebSocketUpgrade,
    user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    secure_ip: SecureClientIp,
    Extension(request_id): Extension<RequestId>,
    State(state): State<WebSocketAppState>,
) -> impl IntoResponse {
    let user_agent = match user_agent_header {
        Some(header) => header.to_string(),
        None => "Unknown User Agent".to_string(),
    };

    ws.on_upgrade(move |socket| {
        handle_socket(socket, secure_ip, request_id.0.clone(), state)
            .instrument(info_span!("websocket_connection"))
    })
}

async fn handle_socket(
    mut socket: WebSocket,
    client_ip: SecureClientIp,
    connection_id: String,
    state: WebSocketAppState,
) {
    let socket_ref = Arc::new(Mutex::new(socket));
    // todo: create span for connection.
    info!("websocket connection received: {}", connection_id);
    state
        .connections
        .add_connection(connection_id.clone(), socket_ref.clone());

    // todo: implement optional heartbeat, research into how browsers actually behave
    // when it comes to the protocol heartbeat.

    // todo: call connect handler.
    // todo: implement auth for connect (Custom WebSocket error auth code)
    // todo: add CORS checks.

    let mut connection_alive = true;
    while connection_alive {
        // Wait some time before acquiring the lock again to allow other tasks to write
        // to the socket. (i.e. a message received from another node in the cluster)
        sleep(Duration::from_millis(10)).await;
        let mut acquired_socket = socket_ref.lock().await;

        if let Some(Ok(msg)) = acquired_socket.recv().await {
            if process_message(msg, connection_id.clone(), &state)
                .await
                .is_break()
            {
                state.connections.remove_connection(connection_id.clone());
                connection_alive = false;
            }
        } else {
            // client disconnected
            state.connections.remove_connection(connection_id.clone());
            connection_alive = false;
        }
    }
}

async fn process_message(
    msg: Message,
    connection_id: String,
    state: &WebSocketAppState,
) -> ControlFlow<(), ()> {
    match msg {
        Message::Text(text) => {
            let resolved = resolve_route(text, connection_id.clone(), state.route_key.clone())?;
            if let Some((route, data)) = resolved {
                if let Some(handler) = state.routes.get(&route) {
                    handle_json_message(
                        handler.clone(),
                        connection_id.clone(),
                        route.clone(),
                        data,
                    )
                    .await;
                } else {
                    error!(
                        "no handler found for route `{}` in WebSocket text message from client {}",
                        route, connection_id
                    );
                }
            }
        }
        Message::Binary(bytes) => {
            let resolved = resolve_binary_route(&bytes, connection_id.clone())?;
            if let Some((route, bytes_stripped)) = resolved {
                if let Some(handler) = state.routes.get(&route) {
                    handle_binary_message(
                        handler.clone(),
                        connection_id.clone(),
                        route.clone(),
                        bytes_stripped,
                    )
                    .await;
                } else {
                    error!(
                        "no handler found for route `{}` in WebSocket binary message from client {}",
                        route, connection_id
                    );
                }
            }
        }
        Message::Close(close) => {
            if let Some(close_frame) = close {
                info!(
                    "connection closed, client {} sent close with code {} and reason `{}`",
                    connection_id, close_frame.code, close_frame.reason,
                );
                // process_close(connection_id.clone()).await;
            } else {
                info!(
                    "connection closed, client {} sent close without close frame",
                    connection_id
                );
                // process_close(connection_id.clone()).await;
            }
            return ControlFlow::Break(());
        }
        _ => {}
    }
    ControlFlow::Continue(())
}

async fn handle_json_message(
    handler: Arc<dyn WebSocketMessageHandler + Send + Sync>,
    connection_id: String,
    route: String,
    data: Value,
) {
    let message_id = nanoid!();
    async {
        info!("JSON websocket message received");
        let start = Instant::now();
        let result = handler
            .handle_json_message(JsonMessageInfo {
                connection_id: connection_id.clone(),
                event_type: WebSocketEventType::Message,
                message_id: message_id.clone(),
                body: data,
            })
            .await;

        let success = result.is_ok();
        if let Err(e) = result {
            error!(
                "failed to handle websocket message from client {}: {}",
                connection_id, e
            );
        }
        log_message_processing_finished(start.elapsed(), success);
    }
    .instrument(info_span!(
        "websocket_json_message",
        message_id = %message_id,
        route = %route,
    ))
    .await;
}

async fn handle_binary_message(
    handler: Arc<dyn WebSocketMessageHandler + Send + Sync>,
    connection_id: String,
    route: String,
    data: &[u8],
) {
    let message_id = nanoid!();
    async {
        info!("binary websocket message received");
        let start = Instant::now();
        let result = handler
            .handle_binary_message(BinaryMessageInfo {
                connection_id: connection_id.clone(),
                event_type: WebSocketEventType::Message,
                message_id: message_id.clone(),
                body: data,
            })
            .await;

        let success = result.is_ok();
        if let Err(e) = result {
            error!(
                "failed to handle websocket message from client {}: {}",
                connection_id, e
            );
        }
        log_message_processing_finished(start.elapsed(), success);
    }
    .instrument(info_span!(
        "websocket_binary_message",
        message_id = %message_id,
        route = %route,
    ))
    .await;
}

fn resolve_route(
    msg_text: String,
    connection_id: String,
    route_key: String,
) -> ControlFlow<(), Option<(String, Value)>> {
    let data: Value = match serde_json::from_str(&msg_text) {
        Ok(data) => data,
        Err(e) => {
            error!(
                "failed to parse JSON message from client {}: {}",
                connection_id, e
            );
            return ControlFlow::Continue(None);
        }
    };
    let data_obj = match &data {
        Value::Object(obj) => obj,
        _ => {
            error!(
                "invalid JSON message from client {}, expected object",
                connection_id
            );
            return ControlFlow::Continue(None);
        }
    };
    let route_opt = data_obj.get(&route_key);
    if let Some(route_val) = route_opt {
        if let Value::String(route) = route_val {
            ControlFlow::Continue(Some((route.clone(), data)))
        } else {
            error!(
                "invalid JSON message from client {}, expected route value to be a string",
                connection_id
            );
            ControlFlow::Continue(None)
        }
    } else {
        error!(
            "invalid JSON message from client {}, missing route key",
            connection_id
        );
        ControlFlow::Continue(None)
    }
}

fn resolve_binary_route<'a>(
    msg_bytes: &'a Vec<u8>,
    connection_id: String,
) -> ControlFlow<(), Option<(String, &'a [u8])>> {
    let route_length = msg_bytes[0];
    if route_length as usize > msg_bytes.len() - 1 {
        error!(
            "invalid binary message from client {}, route length exceeds message length",
            connection_id
        );
        return ControlFlow::Continue(None);
    }

    let route_result = std::str::from_utf8(&msg_bytes[1..=route_length as usize]);
    let route = match route_result {
        Ok(route) => route,
        Err(e) => {
            error!(
                "invalid binary message from client {}, failed to parse route: {}",
                connection_id, e
            );
            return ControlFlow::Continue(None);
        }
    };
    ControlFlow::Continue(Some((
        route.to_string(),
        &msg_bytes[route_length as usize + 1..],
    )))
}

fn log_message_processing_finished(elapsed: Duration, success: bool) {
    let millis_precise = elapsed.as_micros() as f64 / 1000.0;

    if success {
        info!(
            "websocket message processing finished in {} milliseconds",
            millis_precise
        );
    } else {
        error!(
            "websocket message processing failed after {} milliseconds",
            millis_precise
        );
    }
}
