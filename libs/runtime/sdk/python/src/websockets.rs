use std::{collections::HashMap, sync::Arc};

use async_trait::async_trait;
use celerity_runtime_core::{
  config::{WebSocketConfig, WebSocketHandlerDefinition},
  errors::WebSocketsMessageError,
  websocket::{
    BinaryMessageInfo, JsonMessageInfo, MessageRequestContext, WebSocketEventType,
    WebSocketMessageHandler,
  },
};
use celerity_ws_registry::{
  registry::{SendContext, WebSocketRegistrySend},
  types::MessageType,
};
use pyo3::prelude::*;
use tokio::sync::mpsc;

use crate::{
  errors::HandlerError, http::HttpProtocolVersion, interop::PythonCall,
  json_convert::json_value_to_python,
};

#[pyclass]
pub struct CoreWebSocketConfig {
  #[pyo3(get)]
  handlers: Vec<Py<CoreWebSocketHandlerDefinition>>,
}

fn core_websocket_handler_definition(
  websocket_handler_definition: WebSocketHandlerDefinition,
) -> Py<CoreWebSocketHandlerDefinition> {
  Python::with_gil(|py| {
    Py::new(
      py,
      CoreWebSocketHandlerDefinition::from(websocket_handler_definition),
    )
    .unwrap()
  })
}

impl From<WebSocketHandlerDefinition> for CoreWebSocketHandlerDefinition {
  fn from(handler: WebSocketHandlerDefinition) -> Self {
    Self {
      route: handler.route,
      location: handler.location,
      handler: handler.handler,
      timeout: handler.timeout,
    }
  }
}

#[pyclass]
pub struct CoreWebSocketHandlerDefinition {
  #[pyo3(get)]
  route: String,
  #[pyo3(get)]
  location: String,
  #[pyo3(get)]
  handler: String,
  #[pyo3(get)]
  timeout: i64,
}

pub fn core_websocket_config(websocket_config: WebSocketConfig) -> Py<CoreWebSocketConfig> {
  Python::with_gil(|py| Py::new(py, CoreWebSocketConfig::from(websocket_config)).unwrap())
}

impl From<WebSocketConfig> for CoreWebSocketConfig {
  fn from(websocket_config: WebSocketConfig) -> Self {
    let handlers = websocket_config
      .handlers
      .into_iter()
      .map(core_websocket_handler_definition)
      .collect::<Vec<_>>();
    Self { handlers }
  }
}

pub struct WSBindingMessageHandler {
  pub handler_id: String,
  pub py_tx: mpsc::UnboundedSender<PythonCall>,
}

pub enum WebSocketMessageInfo<'a> {
  Json(JsonMessageInfo),
  Binary(BinaryMessageInfo<'a>),
}

#[async_trait]
impl WebSocketMessageHandler for WSBindingMessageHandler {
  async fn handle_json_message(
    &self,
    message: JsonMessageInfo,
  ) -> Result<(), WebSocketsMessageError> {
    self
      .handle_message(WebSocketMessageInfo::Json(message))
      .await
  }

  async fn handle_binary_message<'a>(
    &self,
    message: BinaryMessageInfo<'a>,
  ) -> Result<(), WebSocketsMessageError> {
    self
      .handle_message(WebSocketMessageInfo::Binary(message))
      .await
  }
}

impl WSBindingMessageHandler {
  async fn handle_message<'a>(
    &self,
    message_info: WebSocketMessageInfo<'a>,
  ) -> Result<(), WebSocketsMessageError> {
    let (response_tx, response_rx) = tokio::sync::oneshot::channel();

    let request_trace_ctx = extract_trace_context_from_ws_message_info(&message_info);

    let py_message_info = Python::with_gil(move |py| match message_info {
      WebSocketMessageInfo::Json(json_message_info) => {
        WSBindingMessageInfo::from_json_message_info(json_message_info, request_trace_ctx, py)
      }
      WebSocketMessageInfo::Binary(binary_message_info) => {
        WSBindingMessageInfo::from_binary_message_info(binary_message_info, request_trace_ctx, py)
      }
    })
    .map_err(|err| HandlerError::new(err.to_string()))?;

    self
      .py_tx
      .send(PythonCall {
        handler_id: self.handler_id.clone(),
        args: vec![py_message_info.into()],
        response: response_tx,
      })
      .map_err(|_| HandlerError::new("Python worker unavailable".to_string()))?;
    let result = response_rx
      .await
      .map_err(|_| HandlerError::new("Python worker dropped".to_string()))?;
    result
      .map(|_| ())
      .map_err(|e| HandlerError::new(format!("Python error: {e}")).into())
  }
}

#[pyclass(eq, eq_int, name = "WebSocketEventType")]
#[derive(PartialEq, Clone, Debug)]
pub enum WSBindingEventType {
  #[pyo3(name = "CONNECT")]
  Connect,
  #[pyo3(name = "MESSAGE")]
  Message,
  #[pyo3(name = "DISCONNECT")]
  Disconnect,
}

impl Default for WSBindingEventType {
  fn default() -> Self {
    Self::Message
  }
}

impl From<WebSocketEventType> for WSBindingEventType {
  fn from(event_type: WebSocketEventType) -> Self {
    match event_type {
      WebSocketEventType::Connect => WSBindingEventType::Connect,
      WebSocketEventType::Message => WSBindingEventType::Message,
      WebSocketEventType::Disconnect => WSBindingEventType::Disconnect,
    }
  }
}

#[derive(Default, Debug)]
#[pyclass(name = "WebSocketMessageRequestContext")]
pub struct WSBindingMessageRequestContext {
  #[pyo3(get)]
  pub request_id: String,
  #[pyo3(get)]
  pub request_time: chrono::DateTime<chrono::Utc>,
  #[pyo3(get)]
  pub path: String,
  #[pyo3(get)]
  pub protocol_version: HttpProtocolVersion,
  #[pyo3(get)]
  pub headers: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub user_agent_header: Option<String>,
  #[pyo3(get)]
  pub client_ip: String,
  #[pyo3(get)]
  pub query: HashMap<String, Vec<String>>,
  #[pyo3(get)]
  pub cookies: HashMap<String, String>,
  #[pyo3(get)]
  pub trace_context: Option<HashMap<String, String>>,
}

impl WSBindingMessageRequestContext {
  fn from_message_request_context(
    message_request_context: MessageRequestContext,
    trace_context: Option<HashMap<String, String>>,
    py: Python,
  ) -> PyResult<Py<Self>> {
    Py::new(
      py,
      Self {
        request_id: message_request_context.request_id,
        request_time: chrono::DateTime::from_timestamp(
          message_request_context.request_time.try_into()?,
          0,
        )
        .expect("message request context request time should be a valid timestamp"),
        path: message_request_context.path,
        protocol_version: HttpProtocolVersion::from(message_request_context.protocol_version),
        headers: message_request_context.headers,
        user_agent_header: message_request_context.user_agent.map(|v| v.to_string()),
        client_ip: message_request_context.client_ip,
        query: message_request_context.query,
        cookies: message_request_context.cookies,
        trace_context,
      },
    )
  }
}

#[pyclass(name = "WebSocketMessageRequestContextBuilder")]
pub struct WSBindingMessageRequestContextBuilder {
  request_context: Option<WSBindingMessageRequestContext>,
}

#[pymethods]
impl WSBindingMessageRequestContextBuilder {
  #[new]
  fn new(
    request_id: String,
    request_time: chrono::DateTime<chrono::Utc>,
    path: String,
    protocol_version: HttpProtocolVersion,
  ) -> Self {
    Self {
      request_context: Some(WSBindingMessageRequestContext {
        request_id,
        request_time,
        path,
        protocol_version,
        headers: HashMap::new(),
        user_agent_header: None,
        client_ip: String::new(),
        query: HashMap::new(),
        cookies: HashMap::new(),
        trace_context: None,
      }),
    }
  }

  fn set_headers(mut self_: PyRefMut<Self>, headers: HashMap<String, Vec<String>>) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.headers = headers;
    }
    self_.into()
  }

  fn set_user_agent(mut self_: PyRefMut<Self>, user_agent: String) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.user_agent_header = Some(user_agent);
    }
    self_.into()
  }

  fn set_client_ip(mut self_: PyRefMut<Self>, client_ip: String) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.client_ip = client_ip;
    }
    self_.into()
  }

  fn set_query(mut self_: PyRefMut<Self>, query: HashMap<String, Vec<String>>) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.query = query;
    }
    self_.into()
  }

  fn set_cookies(mut self_: PyRefMut<Self>, cookies: HashMap<String, String>) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.cookies = cookies;
    }
    self_.into()
  }

  fn set_trace_context(
    mut self_: PyRefMut<Self>,
    trace_context: HashMap<String, String>,
  ) -> Py<Self> {
    if let Some(request_context) = &mut self_.request_context {
      request_context.trace_context = Some(trace_context);
    }
    self_.into()
  }

  fn build(mut self_: PyRefMut<Self>, py: Python) -> PyResult<Py<WSBindingMessageRequestContext>> {
    Py::new(py, self_.request_context.take().unwrap_or_default())
  }
}

#[derive(Default, Debug)]
#[pyclass(name = "WebSocketMessageInfo")]
pub struct WSBindingMessageInfo {
  #[pyo3(get, name = "type")]
  pub message_type: WSBindingMessageType,
  #[pyo3(get)]
  pub event_type: WSBindingEventType,
  #[pyo3(get)]
  pub connection_id: String,
  #[pyo3(get)]
  pub message_id: String,
  #[pyo3(get)]
  pub json_body: Option<Py<PyAny>>,
  #[pyo3(get)]
  pub binary_body: Option<Vec<u8>>,
  #[pyo3(get)]
  pub request_context: Option<Py<WSBindingMessageRequestContext>>,
  #[pyo3(get)]
  pub trace_context: Option<HashMap<String, String>>,
}

impl WSBindingMessageInfo {
  pub fn from_json_message_info(
    message_info: JsonMessageInfo,
    trace_context: Option<HashMap<String, String>>,
    py: Python,
  ) -> PyResult<Py<Self>> {
    let request_context = message_info
      .request_ctx
      .map(|ctx| {
        WSBindingMessageRequestContext::from_message_request_context(ctx, trace_context.clone(), py)
      })
      .transpose()?;

    Py::new(
      py,
      Self {
        message_type: WSBindingMessageType::Json,
        event_type: message_info.event_type.into(),
        connection_id: message_info.connection_id,
        message_id: message_info.message_id,
        json_body: Some(json_value_to_python(&message_info.body, py)?.into()),
        binary_body: None,
        request_context,
        trace_context,
      },
    )
  }

  pub fn from_binary_message_info(
    message_info: BinaryMessageInfo,
    trace_context: Option<HashMap<String, String>>,
    py: Python,
  ) -> PyResult<Py<Self>> {
    let request_context = message_info
      .request_ctx
      .map(|ctx| {
        WSBindingMessageRequestContext::from_message_request_context(ctx, trace_context.clone(), py)
      })
      .transpose()?;

    Py::new(
      py,
      Self {
        message_type: WSBindingMessageType::Binary,
        event_type: message_info.event_type.into(),
        connection_id: message_info.connection_id,
        message_id: message_info.message_id,
        json_body: None,
        binary_body: Some(message_info.body.to_vec()),
        request_context,
        trace_context,
      },
    )
  }
}

#[pyclass(name = "WebSocketMessageInfoBuilder")]

pub struct WSBindingMessageInfoBuilder {
  message_info: Option<WSBindingMessageInfo>,
}

#[pymethods]
impl WSBindingMessageInfoBuilder {
  #[new]
  fn new(
    message_type: WSBindingMessageType,
    event_type: WSBindingEventType,
    connection_id: String,
    message_id: String,
  ) -> Self {
    Self {
      message_info: Some(WSBindingMessageInfo {
        message_type,
        event_type,
        connection_id,
        message_id,
        json_body: None,
        binary_body: None,
        request_context: None,
        trace_context: None,
      }),
    }
  }

  fn set_json_body(mut self_: PyRefMut<Self>, json_body: Py<PyAny>) -> Py<Self> {
    if let Some(message_info) = &mut self_.message_info {
      message_info.json_body = Some(json_body);
    }
    self_.into()
  }

  fn set_binary_body(mut self_: PyRefMut<Self>, binary_body: Vec<u8>) -> Py<Self> {
    if let Some(message_info) = &mut self_.message_info {
      message_info.binary_body = Some(binary_body);
    }
    self_.into()
  }

  fn set_request_context(
    mut self_: PyRefMut<Self>,
    request_context: Py<WSBindingMessageRequestContext>,
  ) -> Py<Self> {
    if let Some(message_info) = &mut self_.message_info {
      message_info.request_context = Some(request_context);
    }
    self_.into()
  }

  fn set_trace_context(
    mut self_: PyRefMut<Self>,
    trace_context: HashMap<String, String>,
  ) -> Py<Self> {
    if let Some(message_info) = &mut self_.message_info {
      message_info.trace_context = Some(trace_context);
    }
    self_.into()
  }

  fn build(mut self_: PyRefMut<Self>, py: Python) -> PyResult<Py<WSBindingMessageInfo>> {
    Py::new(py, self_.message_info.take().unwrap_or_default())
  }
}

fn extract_trace_context_from_ws_message_info(
  message_info: &WebSocketMessageInfo,
) -> Option<HashMap<String, String>> {
  match message_info {
    WebSocketMessageInfo::Json(json_message_info) => json_message_info
      .request_ctx
      .as_ref()
      .and_then(|ctx| ctx.trace_context.clone()),
    WebSocketMessageInfo::Binary(binary_message_info) => binary_message_info
      .request_ctx
      .as_ref()
      .and_then(|ctx| ctx.trace_context.clone()),
  }
}

#[pyclass(name = "SendContext")]
pub struct WSBindingSendContext {
  #[pyo3(get)]
  pub caller: Option<String>,
  #[pyo3(get)]
  pub wait_for_ack: bool,
  #[pyo3(get)]
  pub inform_clients: Vec<String>,
}

#[pymethods]
impl WSBindingSendContext {
  #[new]
  fn new(caller: Option<String>, wait_for_ack: bool, inform_clients: Vec<String>) -> Self {
    Self {
      caller,
      wait_for_ack,
      inform_clients,
    }
  }
}

#[pyclass(name = "WebSocketMessageType")]
#[derive(PartialEq, Clone, Debug)]
pub enum WSBindingMessageType {
  #[pyo3(name = "JSON")]
  Json,
  #[pyo3(name = "BINARY")]
  Binary,
}

impl Default for WSBindingMessageType {
  fn default() -> Self {
    Self::Json
  }
}

impl From<WSBindingMessageType> for MessageType {
  fn from(message_type: WSBindingMessageType) -> Self {
    match message_type {
      WSBindingMessageType::Json => MessageType::Json,
      WSBindingMessageType::Binary => MessageType::Binary,
    }
  }
}

#[pyclass(name = "WebSocketRegistry")]
pub struct WSBindingRegistrySend {
  pub inner: Arc<dyn WebSocketRegistrySend>,
}

#[pymethods]
impl WSBindingRegistrySend {
  fn send_message<'a>(
    &'a self,
    connection_id: String,
    message_id: String,
    message_type: WSBindingMessageType,
    message: String,
    ctx: Option<PyRef<WSBindingSendContext>>,
    py: Python<'a>,
  ) -> PyResult<Bound<'a, PyAny>> {
    let ctx = ctx.map(|ctx| SendContext {
      caller: ctx.caller.clone(),
      wait_for_ack: ctx.wait_for_ack,
      inform_clients: ctx.inform_clients.clone(),
    });

    let inner_registry = self.inner.clone();

    pyo3_async_runtimes::tokio::future_into_py(py, async move {
      inner_registry
        .send_message(connection_id, message_id, message_type.into(), message, ctx)
        .await
        .map_err(|e| pyo3::exceptions::PyRuntimeError::new_err(e.to_string()))?;
      Ok(())
    })
  }
}
