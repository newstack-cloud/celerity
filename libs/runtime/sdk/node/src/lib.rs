#![deny(clippy::all)]
#![allow(unexpected_cfgs)]

use std::{collections::HashMap, sync::Arc, time::Duration};

use axum::{
  body::Body,
  http::{request::Parts, Request, StatusCode},
  response::IntoResponse,
};
use celerity_helpers::{
  env::ProcessEnvVars,
  request::{
    cookies_from_headers, path_params_from_request_parts, query_from_uri, to_request_body,
  },
  runtime_types::{RuntimeCallMode, RuntimePlatform},
};
use celerity_runtime_core::{
  application::Application,
  auth_http::AuthContext,
  config::{
    ApiConfig, AppConfig, ClientIpSource, HttpConfig, HttpHandlerDefinition, RuntimeConfig,
    WebSocketConfig,
  },
  request::{MatchedRoute, RequestId, ResolvedClientIp, ResolvedUserAgent},
  telemetry_utils::extract_trace_context,
};
use napi::bindgen_prelude::*;
use napi::threadsafe_function::ThreadsafeFunction;
use napi_derive::napi;
use serde::{Deserialize, Serialize};
use tokio::time;

const MAX_REQUEST_BODY_SIZE: usize = 10 * 1024 * 1024; // 10 MiB

/// A weak ThreadsafeFunction that does not prevent the Node.js event loop from exiting.
type WeakTsfn =
  ThreadsafeFunction<JsRequestWrapper, Promise<Response>, JsRequestWrapper, Status, true, true>;

#[napi(object)]
pub struct CoreRuntimeConfig {
  pub blueprint_config_path: String,
  pub server_port: i32,
  pub server_loopback_only: Option<bool>,
}

#[napi(object)]
pub struct CoreRuntimeAppConfig {
  pub api: Option<CoreApiConfig>,
}

impl From<AppConfig> for CoreRuntimeAppConfig {
  fn from(app_config: AppConfig) -> Self {
    let api = app_config.api.map(|api_config| api_config.into());
    Self { api }
  }
}

#[napi(object)]
pub struct CoreApiConfig {
  pub http: Option<CoreHttpConfig>,
  pub websocket: Option<CoreWebsocketConfig>,
}

impl From<ApiConfig> for CoreApiConfig {
  fn from(api_config: ApiConfig) -> Self {
    let http = api_config.http.map(|http_config| http_config.into());
    let websocket = api_config
      .websocket
      .map(|websocket_config| websocket_config.into());
    Self { http, websocket }
  }
}

#[napi(object)]
pub struct CoreHttpConfig {
  pub handlers: Vec<CoreHttpHandlerDefinition>,
}

impl From<HttpConfig> for CoreHttpConfig {
  fn from(http_config: HttpConfig) -> Self {
    let handlers = http_config
      .handlers
      .into_iter()
      .map(|handler| handler.into())
      .collect::<Vec<_>>();
    Self { handlers }
  }
}

#[napi(object)]
pub struct CoreWebsocketConfig {}

impl From<WebSocketConfig> for CoreWebsocketConfig {
  fn from(_: WebSocketConfig) -> Self {
    Self {}
  }
}

#[napi(object)]
pub struct CoreHttpHandlerDefinition {
  pub path: String,
  pub method: String,
  pub location: String,
  pub handler: String,
  pub timeout: i64,
}

impl From<HttpHandlerDefinition> for CoreHttpHandlerDefinition {
  fn from(handler: HttpHandlerDefinition) -> Self {
    Self {
      path: handler.path,
      method: handler.method,
      location: handler.location,
      handler: handler.handler,
      timeout: handler.timeout,
    }
  }
}

#[napi(object)]
pub struct Response {
  pub status: u16,
  pub headers: Option<HashMap<String, String>>,
  pub body: Option<String>,
  pub binary_body: Option<Buffer>,
}

impl IntoResponse for Response {
  fn into_response(self) -> axum::response::Response<Body> {
    let mut builder = axum::response::Response::builder();
    for (key, value) in self.headers.unwrap_or_default() {
      builder = builder.header(key, value);
    }
    builder = builder.status(self.status);
    let body = if let Some(binary) = self.binary_body {
      Body::from(binary.to_vec())
    } else {
      Body::from(self.body.unwrap_or_default())
    };
    builder.body(body).unwrap()
  }
}

#[derive(Debug)]
pub enum JsRequestWrapperBody {
  Text(String),
  Binary(Vec<u8>),
  EmptyBody,
}

#[napi(js_name = "Request")]
pub struct JsRequestWrapper {
  inner_body: JsRequestWrapperBody,
  inner_parts: Parts,
  path_params: HashMap<String, String>,
  query: HashMap<String, Vec<String>>,
  cookies: HashMap<String, String>,
  content_type: String,
  req_path: String,
  request_id: String,
  request_time: String,
  auth_context: Option<serde_json::Value>,
  client_ip: String,
  trace_context: Option<HashMap<String, String>>,
  user_agent: String,
  matched_route: Option<String>,
}

#[napi]
impl JsRequestWrapper {
  /// Allows the creation of requests, primarily for test purposes.
  /// In normal circumstances, the request will be created by
  /// the runtime and passed to the handler.
  #[napi(constructor)]
  pub fn new(method: String, uri: String, headers: HashMap<String, String>) -> Self {
    let mut builder = Request::builder().method(method.as_str()).uri(uri.clone());
    for (key, value) in headers {
      builder = builder.header(key, value);
    }
    let request = builder.body(Body::empty()).unwrap();
    let (parts, _) = request.into_parts();
    Self {
      inner_parts: parts,
      inner_body: JsRequestWrapperBody::EmptyBody,
      path_params: HashMap::new(),
      query: HashMap::new(),
      cookies: HashMap::new(),
      content_type: String::new(),
      req_path: uri,
      request_id: String::new(),
      request_time: String::new(),
      auth_context: None,
      client_ip: String::new(),
      trace_context: None,
      user_agent: String::new(),
      matched_route: None,
    }
  }

  async fn from_axum_req(req: axum::extract::Request<Body>) -> Result<Self> {
    let (mut parts, body) = req.into_parts();

    // Extract pre-processed fields before consuming the body.
    let path_params = path_params_from_request_parts(&mut parts)
      .await
      .unwrap_or_default();
    let query = query_from_uri(&parts.uri).unwrap_or_default();
    let cookies = cookies_from_headers(&parts.headers);
    let req_path = parts.uri.path().to_string();
    let request_id = parts
      .extensions
      .get::<RequestId>()
      .map(|id| id.0.clone())
      .unwrap_or_default();
    let auth_context = parts
      .extensions
      .get::<AuthContext>()
      .and_then(|ac| ac.0.clone());
    let client_ip = parts
      .extensions
      .get::<ResolvedClientIp>()
      .map(|rci| rci.0.to_string())
      .unwrap_or_default();
    let user_agent = parts
      .extensions
      .get::<ResolvedUserAgent>()
      .map(|ua| ua.0.clone())
      .unwrap_or_default();
    let matched_route = parts
      .extensions
      .get::<MatchedRoute>()
      .map(|mr| mr.0.clone());
    let trace_context = extract_trace_context();
    let request_time = chrono::Utc::now().to_rfc3339();

    // Read and process the body.
    let content_length = parts
      .headers
      .get("content-length")
      .and_then(|value| value.to_str().ok())
      .and_then(|value| value.parse::<usize>().ok())
      .unwrap_or(0);

    let (inner_body, content_type) = if content_length > 0 {
      let bytes = axum::body::to_bytes(body, MAX_REQUEST_BODY_SIZE)
        .await
        .map_err(|err| {
          Error::new(
            Status::GenericFailure,
            format!("failed to read request body, {err}"),
          )
        })?;
      let ct_header = parts.headers.get("content-type").cloned();
      let (text_body, binary_body, content_type_str) = to_request_body(&bytes, ct_header);
      let body = if let Some(text) = text_body {
        JsRequestWrapperBody::Text(text)
      } else if let Some(_binary) = binary_body {
        JsRequestWrapperBody::Binary(bytes.to_vec())
      } else {
        JsRequestWrapperBody::EmptyBody
      };
      (body, content_type_str)
    } else {
      (
        JsRequestWrapperBody::EmptyBody,
        parts
          .headers
          .get("content-type")
          .and_then(|v| v.to_str().ok())
          .unwrap_or("")
          .to_string(),
      )
    };

    Ok(Self {
      inner_parts: parts,
      inner_body,
      path_params,
      query,
      cookies,
      content_type,
      req_path,
      request_id,
      request_time,
      auth_context,
      client_ip,
      trace_context,
      user_agent,
      matched_route,
    })
  }

  /// The HTTP version used for the request.
  #[napi(getter)]
  pub fn http_version(&self) -> String {
    format!("{:?}", self.inner_parts.version)
  }

  /// The HTTP method of the request.
  #[napi(getter)]
  pub fn method(&self) -> String {
    self.inner_parts.method.to_string()
  }

  /// The URI of the request.
  #[napi(getter)]
  pub fn uri(&self) -> String {
    self.inner_parts.uri.to_string()
  }

  /// The headers of the request as a map of header name to list of values.
  #[napi(getter)]
  pub fn headers(&self) -> HashMap<String, Vec<String>> {
    let mut map: HashMap<String, Vec<String>> = HashMap::new();
    for (key, value) in self.inner_parts.headers.iter() {
      map
        .entry(key.as_str().to_string())
        .or_default()
        .push(value.to_str().unwrap_or_default().to_string());
    }
    map
  }

  /// The path of the request (e.g. "/orders/123").
  #[napi(getter)]
  pub fn path(&self) -> String {
    self.req_path.clone()
  }

  /// Path parameters extracted from the URL (e.g. { "orderId": "123" }).
  #[napi(getter)]
  pub fn path_params(&self) -> HashMap<String, String> {
    self.path_params.clone()
  }

  /// Query parameters, supporting multiple values per key.
  #[napi(getter)]
  pub fn query(&self) -> HashMap<String, Vec<String>> {
    self.query.clone()
  }

  /// Cookies from the request.
  #[napi(getter)]
  pub fn cookies(&self) -> HashMap<String, String> {
    self.cookies.clone()
  }

  /// The content type of the request body.
  #[napi(getter)]
  pub fn content_type(&self) -> String {
    self.content_type.clone()
  }

  /// The request ID (from x-request-id header or auto-generated).
  #[napi(getter)]
  pub fn request_id(&self) -> String {
    self.request_id.clone()
  }

  /// The request time as an ISO 8601 string.
  #[napi(getter)]
  pub fn request_time(&self) -> String {
    self.request_time.clone()
  }

  /// Authentication context from the auth middleware, or null if no auth.
  /// Claims are namespaced by guard name: `{ "guardName": claims }`.
  /// When multiple guards are configured in a chain, each guard's claims
  /// appear under its own key.
  #[napi(getter)]
  pub fn auth(&self) -> Option<serde_json::Value> {
    self.auth_context.clone()
  }

  /// The client IP address resolved by the runtime.
  #[napi(getter)]
  pub fn client_ip(&self) -> String {
    self.client_ip.clone()
  }

  /// The trace context for distributed tracing propagation.
  /// Contains "traceparent" (W3C) and optionally "xray_trace_id" (AWS).
  #[napi(getter)]
  pub fn trace_context(&self) -> Option<HashMap<String, String>> {
    self.trace_context.clone()
  }

  /// The user-agent string from the request.
  #[napi(getter)]
  pub fn user_agent(&self) -> String {
    self.user_agent.clone()
  }

  /// The matched route pattern (e.g. "/orders/{orderId}"), or null if unavailable.
  #[napi(getter)]
  pub fn matched_route(&self) -> Option<String> {
    self.matched_route.clone()
  }

  /// The text body of the request, or null if the body is empty or binary.
  #[napi(getter)]
  pub fn text_body(&self) -> Option<String> {
    match &self.inner_body {
      JsRequestWrapperBody::Text(text) => Some(text.clone()),
      _ => None,
    }
  }

  /// The binary body of the request as a Buffer, or null if the body is empty or text.
  #[napi(getter)]
  pub fn binary_body(&self) -> Option<Buffer> {
    match &self.inner_body {
      JsRequestWrapperBody::Binary(bytes) => Some(Buffer::from(bytes.clone())),
      _ => None,
    }
  }
}

#[napi]
pub struct CoreRuntimeApplication {
  inner: Application,
  tsfn_cache: Vec<Arc<WeakTsfn>>,
}

#[napi]
impl CoreRuntimeApplication {
  #[napi(constructor)]
  pub fn new(runtime_config: CoreRuntimeConfig) -> Self {
    let native_runtime_config = RuntimeConfig {
      blueprint_config_path: runtime_config.blueprint_config_path,
      runtime_call_mode: RuntimeCallMode::Ffi,
      server_loopback_only: runtime_config.server_loopback_only,
      server_port: runtime_config.server_port,
      local_api_port: 8259,
      use_custom_health_check: None,
      service_name: "CelerityTestService".to_string(),
      platform: RuntimePlatform::Local,
      trace_otlp_collector_endpoint: "http://localhost:4317".to_string(),
      runtime_max_diagnostics_level: tracing::Level::INFO,
      test_mode: false,
      api_resource: None,
      consumer_app: None,
      schedule_app: None,
      resource_store_verify_tls: true,
      resource_store_cache_entry_ttl: 600,
      resource_store_cleanup_interval: 3600,
      client_ip_source: ClientIpSource::ConnectInfo,
    };
    let inner = Application::new(native_runtime_config, Box::new(ProcessEnvVars::new()));
    CoreRuntimeApplication {
      inner,
      tsfn_cache: vec![],
    }
  }

  #[napi]
  pub fn setup(&mut self) -> Result<CoreRuntimeAppConfig> {
    let app_config = self.inner.setup().map_err(|err| {
      Error::new(
        Status::GenericFailure,
        format!("failed to setup core runtime, {err}"),
      )
    })?;
    Ok(app_config.into())
  }

  #[napi]
  pub fn register_http_handler(
    &mut self,
    path: String,
    method: String,
    timeout_seconds: Option<i64>,
    #[napi(ts_arg_type = "(err: Error | null, request: Request) => Promise<Response>")]
    handler: WeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.tsfn_cache.push(tsfn.clone());
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;

    let handler = move |req| {
      let tsfn = tsfn.clone();
      async move {
        let js_req_wrapper = JsRequestWrapper::from_axum_req(req)
          .await
          .map_err(|err| HandlerError::new(err.to_string()))?;
        let promise = tsfn
          .call_async(Ok(js_req_wrapper))
          .await
          .map_err(|err| HandlerError::new(err.to_string()))?;
        let sleep = time::sleep(Duration::from_secs(timeout_secs));
        tokio::select! {
          _ = sleep => {
            Err(HandlerError::timeout())
          }
          value = promise => {
            Ok::<Response, HandlerError>(value.map_err(|err| HandlerError::new(err.to_string()))?)
          }
        }
      }
    };
    self.inner.register_http_handler(&path, &method, handler);
    Ok(())
  }

  #[allow(clippy::missing_safety_doc)]
  #[napi]
  pub async unsafe fn run(&mut self, block: bool) -> Result<()> {
    self.inner.run(block).await.map_err(|err| {
      Error::new(
        Status::GenericFailure,
        format!("failed to start core runtime, {err}"),
      )
    })?;
    Ok(())
  }

  #[napi]
  pub fn shutdown(&mut self) -> Result<()> {
    self.inner.shutdown();
    self.tsfn_cache.clear();
    Ok(())
  }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct HandlerError {
  pub message: String,
  #[serde(skip)]
  pub is_timeout: bool,
}

impl HandlerError {
  pub fn new(message: String) -> Self {
    Self {
      message,
      is_timeout: false,
    }
  }

  pub fn timeout() -> Self {
    Self {
      message: "handler timed out".to_string(),
      is_timeout: true,
    }
  }
}

impl std::fmt::Display for HandlerError {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    write!(f, "{}", self.message)
  }
}

impl IntoResponse for HandlerError {
  fn into_response(self) -> axum::response::Response<Body> {
    let status = if self.is_timeout {
      StatusCode::GATEWAY_TIMEOUT
    } else {
      StatusCode::INTERNAL_SERVER_ERROR
    };
    (status, axum::response::Json(self)).into_response()
  }
}
