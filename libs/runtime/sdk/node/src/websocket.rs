use std::{collections::HashMap, sync::Arc};

use async_trait::async_trait;
use celerity_runtime_core::{
  errors::WebSocketsMessageError,
  websocket::{
    BinaryMessageInfo, JsonMessageInfo, MessageRequestContext, WebSocketEventType,
    WebSocketMessageHandler,
  },
};
use celerity_ws_registry::registry::{SendContext, WebSocketRegistrySend};
use celerity_ws_registry::types::MessageType;
use napi::bindgen_prelude::*;
use napi::threadsafe_function::ThreadsafeFunction;
use napi_derive::napi;

// ---------------------------------------------------------------------------
// ThreadsafeFunction type alias
// ---------------------------------------------------------------------------

pub(crate) type WsMessageWeakTsfn = ThreadsafeFunction<
  JsWebSocketMessageInfo,
  Promise<()>,
  JsWebSocketMessageInfo,
  Status,
  true,
  true,
>;

// ---------------------------------------------------------------------------
// JS-facing types
// ---------------------------------------------------------------------------

/// The type of event that a handler receives for a WebSocket connection.
#[napi(string_enum)]
#[derive(Debug, Clone)]
pub enum JsWebSocketEventType {
  /// A new WebSocket connection was established.
  #[napi(value = "connect")]
  Connect,
  /// A message was received on an existing WebSocket connection.
  #[napi(value = "message")]
  Message,
  /// A WebSocket connection was closed.
  #[napi(value = "disconnect")]
  Disconnect,
}

impl From<WebSocketEventType> for JsWebSocketEventType {
  fn from(event_type: WebSocketEventType) -> Self {
    match event_type {
      WebSocketEventType::Connect => JsWebSocketEventType::Connect,
      WebSocketEventType::Message => JsWebSocketEventType::Message,
      WebSocketEventType::Disconnect => JsWebSocketEventType::Disconnect,
    }
  }
}

/// Request context for a WebSocket message.
///
/// Contains information about the original HTTP request that was
/// upgraded to a WebSocket connection.
#[napi(object)]
pub struct JsMessageRequestContext {
  /// The ID of the request.
  pub request_id: String,
  /// The time the request was received as a Unix timestamp in milliseconds.
  pub request_time: i64,
  /// The path of the request including query string.
  pub path: String,
  /// The protocol version of the original HTTP request (e.g. "HTTP/1.1").
  pub protocol_version: String,
  /// A dictionary of HTTP headers, allowing for multiple values per header name.
  pub headers: HashMap<String, Vec<String>>,
  /// The value of the User-Agent header, or undefined if not present.
  pub user_agent: Option<String>,
  /// The secure IP address of the client extracted from trusted headers
  /// or directly from the socket.
  pub client_ip: String,
  /// A dictionary of query parameters, allowing for multiple values per parameter name.
  pub query: HashMap<String, Vec<String>>,
  /// A dictionary of cookies.
  pub cookies: HashMap<String, String>,
  /// Authentication context from the auth middleware, or undefined if no auth.
  pub auth: Option<serde_json::Value>,
  /// A dictionary of trace context including a W3C Trace Context string
  /// (in the traceparent format) and platform specific trace IDs.
  pub trace_context: Option<HashMap<String, String>>,
}

impl From<MessageRequestContext> for JsMessageRequestContext {
  fn from(ctx: MessageRequestContext) -> Self {
    Self {
      request_id: ctx.request_id,
      request_time: ctx.request_time as i64,
      path: ctx.path,
      protocol_version: format!("{:?}", ctx.protocol_version),
      headers: ctx.headers,
      user_agent: ctx.user_agent,
      client_ip: ctx.client_ip,
      query: ctx.query,
      cookies: ctx.cookies,
      auth: ctx.auth,
      trace_context: ctx.trace_context,
    }
  }
}

/// The handler input for when a WebSocket event occurs.
///
/// This includes the message content, event type, connection information,
/// and context from the original HTTP upgrade request.
#[napi(object)]
pub struct JsWebSocketMessageInfo {
  /// The type of the message: "json" or "binary".
  pub message_type: String,
  /// The type of event: connect, message, or disconnect.
  pub event_type: JsWebSocketEventType,
  /// The ID of the WebSocket connection.
  pub connection_id: String,
  /// The ID of the message.
  pub message_id: String,
  /// The JSON body of the message, set when message_type is "json".
  pub json_body: Option<serde_json::Value>,
  /// The binary body of the message as a Buffer, set when message_type is "binary".
  pub binary_body: Option<Buffer>,
  /// The context of the original HTTP request that upgraded to a WebSocket connection.
  pub request_context: Option<JsMessageRequestContext>,
  /// A dictionary of trace context for distributed tracing propagation.
  pub trace_context: Option<HashMap<String, String>>,
}

impl JsWebSocketMessageInfo {
  pub fn from_json(info: JsonMessageInfo) -> Self {
    Self {
      message_type: "json".to_string(),
      event_type: info.event_type.into(),
      connection_id: info.connection_id,
      message_id: info.message_id,
      json_body: Some(info.body),
      binary_body: None,
      request_context: info.request_ctx.map(|ctx| ctx.into()),
      trace_context: info.trace_context,
    }
  }

  pub fn from_binary(info: BinaryMessageInfo<'_>) -> Self {
    Self {
      message_type: "binary".to_string(),
      event_type: info.event_type.into(),
      connection_id: info.connection_id,
      message_id: info.message_id,
      json_body: None,
      binary_body: Some(Buffer::from(info.body.to_vec())),
      request_context: info.request_ctx.map(|ctx| ctx.into()),
      trace_context: info.trace_context,
    }
  }
}

// ---------------------------------------------------------------------------
// NapiWebSocketMessageHandler — bridges WS messages to JS via tsfn
// ---------------------------------------------------------------------------

pub struct NapiWebSocketMessageHandler {
  tsfn: Arc<WsMessageWeakTsfn>,
}

impl NapiWebSocketMessageHandler {
  pub fn new(tsfn: Arc<WsMessageWeakTsfn>) -> Self {
    Self { tsfn }
  }
}

#[async_trait]
impl WebSocketMessageHandler for NapiWebSocketMessageHandler {
  async fn handle_json_message(
    &self,
    message: JsonMessageInfo,
  ) -> std::result::Result<(), WebSocketsMessageError> {
    let js_msg = JsWebSocketMessageInfo::from_json(message);
    let promise = self
      .tsfn
      .call_async(Ok(js_msg))
      .await
      .map_err(|e| WebSocketsMessageError::UnexpectedError(e.to_string()))?;
    promise
      .await
      .map_err(|e| WebSocketsMessageError::UnexpectedError(e.to_string()))
  }

  async fn handle_binary_message<'a>(
    &self,
    message: BinaryMessageInfo<'a>,
  ) -> std::result::Result<(), WebSocketsMessageError> {
    // Copy borrowed body to owned Vec<u8> before crossing the tsfn thread boundary.
    let js_msg = JsWebSocketMessageInfo::from_binary(message);
    let promise = self
      .tsfn
      .call_async(Ok(js_msg))
      .await
      .map_err(|e| WebSocketsMessageError::UnexpectedError(e.to_string()))?;
    promise
      .await
      .map_err(|e| WebSocketsMessageError::UnexpectedError(e.to_string()))
  }
}

// ---------------------------------------------------------------------------
// CoreWebSocketRegistry — NAPI class wrapping WebSocketRegistrySend
// ---------------------------------------------------------------------------

/// The type of message to send to a WebSocket connection.
#[napi(string_enum)]
#[derive(Debug, Clone)]
pub enum JsMessageType {
  /// A JSON text message.
  #[napi(value = "json")]
  Json,
  /// A binary message (base64-encoded when passed as a string).
  #[napi(value = "binary")]
  Binary,
}

impl From<JsMessageType> for MessageType {
  fn from(mt: JsMessageType) -> Self {
    match mt {
      JsMessageType::Json => MessageType::Json,
      JsMessageType::Binary => MessageType::Binary,
    }
  }
}

/// Context for sending a message to a WebSocket connection.
#[napi(object)]
pub struct JsSendContext {
  /// The name of the caller sending the message.
  pub caller: Option<String>,
  /// Whether to wait for an acknowledgement from the client.
  pub wait_for_ack: bool,
  /// List of client IDs to inform when a message is considered lost.
  pub inform_clients: Vec<String>,
}

impl From<JsSendContext> for SendContext {
  fn from(ctx: JsSendContext) -> Self {
    SendContext {
      caller: ctx.caller,
      wait_for_ack: ctx.wait_for_ack,
      inform_clients: ctx.inform_clients,
    }
  }
}

/// Registry for sending messages to WebSocket connections.
///
/// This is thread-safe and can be used from handler functions to send messages
/// to clients connected to the current node or other nodes in a WebSocket API cluster.
#[napi]
pub struct CoreWebSocketRegistry {
  inner: Arc<dyn WebSocketRegistrySend>,
}

#[napi]
impl CoreWebSocketRegistry {
  /// Sends a message to a WebSocket connection.
  ///
  /// The connection can be on the current node or on another node in a
  /// WebSocket API cluster. The message type determines whether the
  /// message is sent as JSON text or binary data.
  #[napi]
  pub async fn send_message(
    &self,
    connection_id: String,
    message_id: String,
    message_type: JsMessageType,
    message: String,
    ctx: Option<JsSendContext>,
  ) -> Result<()> {
    self
      .inner
      .send_message(
        connection_id,
        message_id,
        message_type.into(),
        message,
        ctx.map(|c| c.into()),
      )
      .await
      .map_err(|e| Error::new(Status::GenericFailure, format!("send_message failed: {e}")))
  }
}

impl CoreWebSocketRegistry {
  pub fn new(inner: Arc<dyn WebSocketRegistrySend>) -> Self {
    Self { inner }
  }
}
