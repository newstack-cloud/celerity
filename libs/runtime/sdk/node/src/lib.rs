#![deny(clippy::all)]
#![allow(unexpected_cfgs)]

mod consumer;
mod invoke;
mod websocket;

use std::{collections::HashMap, str::FromStr, sync::Arc, time::Duration};

use async_trait::async_trait;
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
  auth_custom::{AuthGuardHandler, AuthGuardValidateError, AuthGuardValidateInput},
  auth_http::AuthContext,
  config::{
    ApiConfig, AppConfig, ClientIpSource, ConsumerConfig, ConsumersConfig, CustomHandlerDefinition,
    CustomHandlersConfig, EventConfig, EventHandlerDefinition, EventsConfig,
    GuardHandlerDefinition, GuardsConfig, HttpConfig, HttpHandlerDefinition, RuntimeConfig,
    ScheduleConfig, SchedulesConfig, WebSocketConfig, WebSocketHandlerDefinition,
  },
  consts::{
    DEFAULT_RESOURCE_STORE_CACHE_ENTRY_TTL, DEFAULT_RESOURCE_STORE_CLEANUP_INTERVAL,
    DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT,
  },
  request::{MatchedRoute, RequestId, ResolvedClientIp, ResolvedUserAgent},
  telemetry_utils::extract_trace_context,
};
use consumer::{ConsumerWeakTsfn, NapiConsumerEventHandlerBuilder, ScheduleWeakTsfn};
use invoke::InvokeWeakTsfn;
use napi::bindgen_prelude::*;
use napi::threadsafe_function::ThreadsafeFunction;
use napi_derive::napi;
use serde::{Deserialize, Serialize};
use tokio::time;
use tracing::Level;
use websocket::{CoreWebSocketRegistry, NapiWebSocketMessageHandler, WsMessageWeakTsfn};

const MAX_REQUEST_BODY_SIZE: usize = 10 * 1024 * 1024; // 10 MiB

/// A weak ThreadsafeFunction that does not prevent the Node.js event loop from exiting.
type WeakTsfn =
  ThreadsafeFunction<JsRequestWrapper, Promise<Response>, JsRequestWrapper, Status, true, true>;

/// A weak ThreadsafeFunction for guard handler callbacks.
type GuardWeakTsfn =
  ThreadsafeFunction<GuardInput, Promise<GuardResult>, GuardInput, Status, true, true>;

/// The platform the runtime is running on.
#[napi(string_enum)]
#[derive(Clone, Copy)]
pub enum CoreRuntimePlatform {
  #[napi(value = "aws")]
  Aws,
  #[napi(value = "azure")]
  Azure,
  #[napi(value = "gcp")]
  Gcp,
  #[napi(value = "local")]
  Local,
  #[napi(value = "other")]
  Other,
}

impl From<RuntimePlatform> for CoreRuntimePlatform {
  fn from(p: RuntimePlatform) -> Self {
    match p {
      RuntimePlatform::AWS => CoreRuntimePlatform::Aws,
      RuntimePlatform::Azure => CoreRuntimePlatform::Azure,
      RuntimePlatform::GCP => CoreRuntimePlatform::Gcp,
      RuntimePlatform::Local => CoreRuntimePlatform::Local,
      RuntimePlatform::Other => CoreRuntimePlatform::Other,
    }
  }
}

impl From<CoreRuntimePlatform> for RuntimePlatform {
  fn from(p: CoreRuntimePlatform) -> Self {
    match p {
      CoreRuntimePlatform::Aws => RuntimePlatform::AWS,
      CoreRuntimePlatform::Azure => RuntimePlatform::Azure,
      CoreRuntimePlatform::Gcp => RuntimePlatform::GCP,
      CoreRuntimePlatform::Local => RuntimePlatform::Local,
      CoreRuntimePlatform::Other => RuntimePlatform::Other,
    }
  }
}

#[napi(object)]
pub struct CoreRuntimeConfig {
  pub blueprint_config_path: String,
  pub service_name: String,
  pub server_port: i32,
  pub server_loopback_only: Option<bool>,
  pub use_custom_health_check: Option<bool>,
  pub trace_otlp_collector_endpoint: String,
  pub runtime_max_diagnostics_level: String,
  pub platform: CoreRuntimePlatform,
  pub test_mode: bool,
  pub api_resource: Option<String>,
  pub consumer_app: Option<String>,
  pub schedule_app: Option<String>,
  pub resource_store_verify_tls: bool,
  pub resource_store_cache_entry_ttl: i64,
  pub resource_store_cleanup_interval: i64,
  /// The source used to resolve client IP addresses.
  /// One of: "ConnectInfo", "CfConnectingIp", "TrueClientIp",
  /// "CloudFrontViewerAddress", "RightmostXForwardedFor", "XRealIp", "FlyClientIp".
  pub client_ip_source: Option<String>,
  /// The log output format. If not set, falls back to the `CELERITY_LOG_FORMAT`
  /// environment variable.
  pub log_format: Option<String>,
  /// Whether to enable metrics collection. Defaults to `false` when not set.
  pub metrics_enabled: Option<bool>,
  /// The trace sampling ratio (0.0–1.0). Defaults to `0.1` (10%) when not set.
  pub trace_sample_ratio: Option<f64>,
}

impl From<RuntimeConfig> for CoreRuntimeConfig {
  fn from(rc: RuntimeConfig) -> Self {
    Self {
      blueprint_config_path: rc.blueprint_config_path,
      service_name: rc.service_name,
      server_port: rc.server_port,
      server_loopback_only: rc.server_loopback_only,
      use_custom_health_check: rc.use_custom_health_check,
      trace_otlp_collector_endpoint: rc.trace_otlp_collector_endpoint,
      runtime_max_diagnostics_level: rc.runtime_max_diagnostics_level.to_string(),
      platform: rc.platform.into(),
      test_mode: rc.test_mode,
      api_resource: rc.api_resource,
      consumer_app: rc.consumer_app,
      schedule_app: rc.schedule_app,
      resource_store_verify_tls: rc.resource_store_verify_tls,
      resource_store_cache_entry_ttl: rc.resource_store_cache_entry_ttl,
      resource_store_cleanup_interval: rc.resource_store_cleanup_interval,
      client_ip_source: Some(format!("{:?}", rc.client_ip_source)),
      log_format: rc.log_format,
      metrics_enabled: Some(rc.metrics_enabled),
      trace_sample_ratio: Some(rc.trace_sample_ratio),
    }
  }
}

/// Creates a `CoreRuntimeConfig` by reading from `CELERITY_*` environment variables.
///
/// This reads all `CELERITY_*` environment variables and constructs a runtime
/// configuration object. This is the recommended way to create a runtime config
/// when deploying to a managed environment.
#[napi]
pub fn runtime_config_from_env() -> CoreRuntimeConfig {
  let env = ProcessEnvVars::new();
  let runtime_config = RuntimeConfig::from_env(&env);
  CoreRuntimeConfig::from(runtime_config)
}

/// Configuration for the application running in the Celerity runtime.
///
/// Returned by `CoreRuntimeApplication.setup()` and contains the handler definitions
/// that should be used to register handlers with the application.
#[napi(object)]
pub struct CoreRuntimeAppConfig {
  /// Configuration for HTTP and WebSocket APIs, or undefined if no API is configured.
  pub api: Option<CoreApiConfig>,
  /// Configuration for event source consumers, or undefined if none are configured.
  pub consumers: Option<CoreConsumersConfig>,
  /// Configuration for event-driven consumers (datastore streams, bucket events),
  /// or undefined if none are configured.
  pub events: Option<CoreEventsConfig>,
  /// Configuration for scheduled event handlers, or undefined if none are configured.
  pub schedules: Option<CoreSchedulesConfig>,
  /// Configuration for custom handler invocations, or undefined if none are configured.
  pub custom_handlers: Option<CoreCustomHandlersConfig>,
}

impl From<AppConfig> for CoreRuntimeAppConfig {
  fn from(app_config: AppConfig) -> Self {
    let api = app_config.api.map(|api_config| api_config.into());
    let consumers = app_config.consumers.map(|c| c.into());
    let events = app_config.events.map(|e| e.into());
    let schedules = app_config.schedules.map(|s| s.into());
    let custom_handlers = app_config.custom_handlers.map(|ch| ch.into());
    Self {
      api,
      consumers,
      events,
      schedules,
      custom_handlers,
    }
  }
}

/// Configuration for HTTP and WebSocket APIs.
#[napi(object)]
pub struct CoreApiConfig {
  /// Configuration for HTTP endpoint handlers, or undefined if no HTTP handlers are configured.
  pub http: Option<CoreHttpConfig>,
  /// Configuration for WebSocket connection lifecycle and message handlers,
  /// or undefined if no WebSocket handlers are configured.
  pub websocket: Option<CoreWebsocketConfig>,
  /// Configuration for authentication guard handlers,
  /// or undefined if no guards are configured.
  pub guards: Option<CoreGuardsConfig>,
}

impl From<ApiConfig> for CoreApiConfig {
  fn from(api_config: ApiConfig) -> Self {
    let http = api_config.http.map(|http_config| http_config.into());
    let websocket = api_config
      .websocket
      .map(|websocket_config| websocket_config.into());
    let guards = api_config.guards.map(|guards_config| guards_config.into());
    Self {
      http,
      websocket,
      guards,
    }
  }
}

/// Configuration for HTTP endpoint handlers.
#[napi(object)]
pub struct CoreHttpConfig {
  /// List of HTTP handler definitions parsed from the blueprint.
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

/// Configuration for WebSocket connection lifecycle and message handlers.
#[napi(object)]
pub struct CoreWebsocketConfig {
  /// List of WebSocket handler definitions parsed from the blueprint.
  pub handlers: Vec<CoreWebSocketHandlerDefinition>,
}

impl From<WebSocketConfig> for CoreWebsocketConfig {
  fn from(ws_config: WebSocketConfig) -> Self {
    Self {
      handlers: ws_config.handlers.into_iter().map(|h| h.into()).collect(),
    }
  }
}

/// Configuration for authentication guard handlers.
#[napi(object)]
pub struct CoreGuardsConfig {
  /// List of guard handler definitions parsed from the blueprint.
  pub handlers: Vec<CoreGuardHandlerDefinition>,
}

impl From<GuardsConfig> for CoreGuardsConfig {
  fn from(guards_config: GuardsConfig) -> Self {
    let handlers = guards_config
      .handlers
      .into_iter()
      .map(|h| h.into())
      .collect::<Vec<_>>();
    Self { handlers }
  }
}

/// Definition of a guard handler.
#[napi(object)]
pub struct CoreGuardHandlerDefinition {
  /// The name of the guard handler as defined in the blueprint.
  pub name: String,
}

impl From<GuardHandlerDefinition> for CoreGuardHandlerDefinition {
  fn from(def: GuardHandlerDefinition) -> Self {
    Self { name: def.name }
  }
}

// ---------------------------------------------------------------------------
// Consumer / Schedule / Custom Handler / WebSocket config types
// ---------------------------------------------------------------------------

/// Configuration for event source consumers.
#[napi(object)]
pub struct CoreConsumersConfig {
  /// List of consumer configurations parsed from the blueprint.
  pub consumers: Vec<CoreConsumerConfig>,
}

impl From<ConsumersConfig> for CoreConsumersConfig {
  fn from(c: ConsumersConfig) -> Self {
    Self {
      consumers: c.consumers.into_iter().map(|cc| cc.into()).collect(),
    }
  }
}

/// Configuration for an individual event source consumer.
#[napi(object)]
pub struct CoreConsumerConfig {
  /// The name of the consumer.
  pub consumer_name: String,
  /// The identifier of the event source (e.g. queue URL or stream name).
  pub source_id: String,
  /// Maximum number of messages to process in a single batch.
  pub batch_size: Option<i64>,
  /// Time in seconds before a message becomes visible again after being received.
  pub visibility_timeout: Option<i64>,
  /// Long-polling wait time in seconds.
  pub wait_time_seconds: Option<i64>,
  /// Whether to report partial batch failures.
  pub partial_failures: Option<bool>,
  /// Optional routing key for filtering messages.
  pub routing_key: Option<String>,
  /// List of event handler definitions for this consumer.
  pub handlers: Vec<CoreEventHandlerDefinition>,
}

impl From<ConsumerConfig> for CoreConsumerConfig {
  fn from(c: ConsumerConfig) -> Self {
    Self {
      consumer_name: c.consumer_name,
      source_id: c.source_id,
      batch_size: c.batch_size,
      visibility_timeout: c.visibility_timeout,
      wait_time_seconds: c.wait_time_seconds,
      partial_failures: c.partial_failures,
      routing_key: c.routing_key,
      handlers: c.handlers.into_iter().map(|h| h.into()).collect(),
    }
  }
}

/// Definition of an event handler for consumers or schedules.
#[napi(object)]
pub struct CoreEventHandlerDefinition {
  /// The name of the handler.
  pub name: String,
  /// The file location of the handler.
  pub location: String,
  /// The fully qualified handler function (e.g. "module.function").
  pub handler: String,
  /// The timeout in seconds for the handler.
  pub timeout: i64,
  /// Whether distributed tracing is enabled for the handler.
  pub tracing_enabled: bool,
  /// Optional routing key for the handler.
  pub route: Option<String>,
}

impl From<EventHandlerDefinition> for CoreEventHandlerDefinition {
  fn from(h: EventHandlerDefinition) -> Self {
    Self {
      name: h.name,
      location: h.location,
      handler: h.handler,
      timeout: h.timeout,
      tracing_enabled: h.tracing_enabled,
      route: h.route,
    }
  }
}

/// Configuration for event-driven consumers (datastore streams, bucket events).
#[napi(object)]
pub struct CoreEventsConfig {
  /// List of event consumer configurations parsed from the blueprint.
  pub events: Vec<CoreEventConsumerConfig>,
}

impl From<EventsConfig> for CoreEventsConfig {
  fn from(e: EventsConfig) -> Self {
    Self {
      events: e.events.into_iter().map(|ec| ec.into()).collect(),
    }
  }
}

/// Configuration for an individual event consumer (stream or event trigger).
#[napi(object)]
pub struct CoreEventConsumerConfig {
  /// The name of the consumer resource in the blueprint.
  pub consumer_name: String,
  /// A unique identifier for the event source (e.g. datastore or bucket resource name).
  pub source_id: String,
  /// Maximum number of messages to process in a single batch.
  pub batch_size: Option<i64>,
  /// List of event handler definitions for this event consumer.
  pub handlers: Vec<CoreEventHandlerDefinition>,
}

impl From<EventConfig> for CoreEventConsumerConfig {
  fn from(ec: EventConfig) -> Self {
    match ec {
      EventConfig::EventTrigger(cfg) => Self {
        consumer_name: cfg.consumer_name,
        source_id: cfg.queue_id,
        batch_size: cfg.batch_size,
        handlers: cfg.handlers.into_iter().map(|h| h.into()).collect(),
      },
      EventConfig::Stream(cfg) => Self {
        consumer_name: cfg.consumer_name,
        source_id: cfg.stream_id,
        batch_size: cfg.batch_size,
        handlers: cfg.handlers.into_iter().map(|h| h.into()).collect(),
      },
    }
  }
}

/// Configuration for scheduled event handlers.
#[napi(object)]
pub struct CoreSchedulesConfig {
  /// List of schedule configurations parsed from the blueprint.
  pub schedules: Vec<CoreScheduleConfig>,
}

impl From<SchedulesConfig> for CoreSchedulesConfig {
  fn from(s: SchedulesConfig) -> Self {
    Self {
      schedules: s.schedules.into_iter().map(|sc| sc.into()).collect(),
    }
  }
}

/// Configuration for an individual schedule.
#[napi(object)]
pub struct CoreScheduleConfig {
  /// The identifier of the schedule.
  pub schedule_id: String,
  /// The schedule expression (e.g. cron expression or rate).
  pub schedule_value: String,
  /// List of event handler definitions for this schedule.
  pub handlers: Vec<CoreEventHandlerDefinition>,
  /// Optional input data to pass to schedule handlers.
  pub input: Option<serde_json::Value>,
}

impl From<ScheduleConfig> for CoreScheduleConfig {
  fn from(s: ScheduleConfig) -> Self {
    Self {
      schedule_id: s.schedule_id,
      schedule_value: s.schedule_value,
      handlers: s.handlers.into_iter().map(|h| h.into()).collect(),
      input: s.input,
    }
  }
}

/// Configuration for custom handler invocations.
#[napi(object)]
pub struct CoreCustomHandlersConfig {
  /// List of custom handler definitions parsed from the blueprint.
  pub handlers: Vec<CoreCustomHandlerDefinition>,
}

impl From<CustomHandlersConfig> for CoreCustomHandlersConfig {
  fn from(ch: CustomHandlersConfig) -> Self {
    Self {
      handlers: ch.handlers.into_iter().map(|h| h.into()).collect(),
    }
  }
}

/// Definition of a custom handler that can be invoked programmatically.
#[napi(object)]
pub struct CoreCustomHandlerDefinition {
  /// The name of the custom handler.
  pub name: String,
  /// The file location of the handler.
  pub location: String,
  /// The fully qualified handler function (e.g. "module.function").
  pub handler: String,
  /// The timeout in seconds for the handler.
  pub timeout: i64,
  /// Whether distributed tracing is enabled for the handler.
  pub tracing_enabled: bool,
}

impl From<CustomHandlerDefinition> for CoreCustomHandlerDefinition {
  fn from(h: CustomHandlerDefinition) -> Self {
    Self {
      name: h.name,
      location: h.location,
      handler: h.handler,
      timeout: h.timeout,
      tracing_enabled: h.tracing_enabled,
    }
  }
}

/// Definition of a WebSocket handler.
#[napi(object)]
pub struct CoreWebSocketHandlerDefinition {
  /// The name of the WebSocket handler.
  pub name: String,
  /// The route of the handler (e.g. "$connect", "$default", "$disconnect", or a custom route).
  pub route: String,
  /// The file location of the handler.
  pub location: String,
  /// The fully qualified handler function (e.g. "module.function").
  pub handler: String,
  /// The timeout in seconds for the handler.
  pub timeout: i64,
}

impl From<WebSocketHandlerDefinition> for CoreWebSocketHandlerDefinition {
  fn from(h: WebSocketHandlerDefinition) -> Self {
    Self {
      name: h.name,
      route: h.route,
      location: h.location,
      handler: h.handler,
      timeout: h.timeout,
    }
  }
}

/// The input passed to a guard handler callback from the Rust runtime.
#[napi(object)]
pub struct GuardInput {
  /// The auth token extracted from the request by the runtime
  /// (using the configured token source and auth scheme).
  pub token: String,
  /// Request information available to the guard.
  pub request: GuardRequestInfo,
  /// Accumulated auth context from preceding guards in the chain.
  /// Keyed by guard name, e.g. `{ "jwt": { "claims": { ... } } }`.
  /// Empty for the first guard in the chain.
  pub auth: serde_json::Value,
  /// The blueprint handler name for the route being protected (e.g. "Orders").
  /// None for WebSocket connections and when no name is available.
  pub handler_name: Option<String>,
}

impl From<AuthGuardValidateInput> for GuardInput {
  fn from(input: AuthGuardValidateInput) -> Self {
    let headers = {
      let mut map: HashMap<String, Vec<String>> = HashMap::new();
      for (key, value) in input.request.headers.iter() {
        map
          .entry(key.as_str().to_string())
          .or_default()
          .push(value.to_str().unwrap_or_default().to_string());
      }
      map
    };

    let cookies = {
      let jar = input.request.cookies;
      jar
        .iter()
        .map(|c| (c.name().to_string(), c.value().to_string()))
        .collect()
    };

    Self {
      token: input.token,
      request: GuardRequestInfo {
        method: input.request.method,
        path: input.request.path,
        headers,
        query: input.request.query,
        cookies,
        body: input.request.body,
        request_id: input.request.request_id.0,
        client_ip: input.request.client_ip,
      },
      auth: serde_json::Value::Object(input.auth),
      handler_name: input.handler_name,
    }
  }
}

/// HTTP request information available to guard handlers.
#[napi(object)]
pub struct GuardRequestInfo {
  pub method: String,
  pub path: String,
  pub headers: HashMap<String, Vec<String>>,
  pub query: HashMap<String, Vec<String>>,
  pub cookies: HashMap<String, String>,
  pub body: Option<String>,
  pub request_id: String,
  pub client_ip: String,
}

/// The result returned from a guard handler callback.
#[napi(object)]
pub struct GuardResult {
  /// One of: "allowed", "unauthorised", "forbidden", "error".
  pub status: String,
  /// The auth context to store for this guard (returned on success).
  pub auth: Option<serde_json::Value>,
  /// Error message (returned on failure).
  pub message: Option<String>,
}

/// Wraps a Node.js guard callback as an `AuthGuardHandler` implementation
/// that the Rust runtime can invoke during the auth guard chain.
struct NapiAuthGuardHandler {
  tsfn: Arc<GuardWeakTsfn>,
}

impl std::fmt::Debug for NapiAuthGuardHandler {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    f.debug_struct("NapiAuthGuardHandler").finish()
  }
}

#[async_trait]
impl AuthGuardHandler for NapiAuthGuardHandler {
  async fn validate(
    &self,
    input: AuthGuardValidateInput,
  ) -> std::result::Result<serde_json::Value, AuthGuardValidateError> {
    let guard_input = GuardInput::from(input);
    let promise = self
      .tsfn
      .call_async(Ok(guard_input))
      .await
      .map_err(|e| AuthGuardValidateError::UnexpectedError(e.to_string()))?;
    let result: GuardResult = promise
      .await
      .map_err(|e| AuthGuardValidateError::UnexpectedError(e.to_string()))?;

    match result.status.as_str() {
      "allowed" => Ok(result.auth.unwrap_or(serde_json::Value::Null)),
      "unauthorised" => Err(AuthGuardValidateError::Unauthorised(
        result.message.unwrap_or_default(),
      )),
      "forbidden" => Err(AuthGuardValidateError::Forbidden(
        result.message.unwrap_or_default(),
      )),
      _ => Err(AuthGuardValidateError::UnexpectedError(
        result
          .message
          .unwrap_or_else(|| "Guard validation failed".to_string()),
      )),
    }
  }
}

/// Definition of an HTTP handler.
#[napi(object)]
pub struct CoreHttpHandlerDefinition {
  /// The name of the handler as defined in the blueprint.
  pub name: String,
  /// The path of the handler (e.g. "/items/{itemId}").
  pub path: String,
  /// The HTTP method of the handler (e.g. "GET", "POST").
  pub method: String,
  /// The file location of the handler.
  pub location: String,
  /// The fully qualified handler function (e.g. "module.function").
  pub handler: String,
  /// The timeout in seconds for the handler.
  pub timeout: i64,
}

impl From<HttpHandlerDefinition> for CoreHttpHandlerDefinition {
  fn from(handler: HttpHandlerDefinition) -> Self {
    Self {
      name: handler.name,
      path: handler.path,
      method: handler.method,
      location: handler.location,
      handler: handler.handler,
      timeout: handler.timeout,
    }
  }
}

/// HTTP response to return to the client from a handler.
#[napi(object)]
pub struct Response {
  /// HTTP status code to return to the client (e.g. 200, 404, 500).
  pub status: u16,
  /// Optional headers to return to the client.
  pub headers: Option<HashMap<String, String>>,
  /// Optional text body to return to the client.
  pub body: Option<String>,
  /// Optional binary body to return to the client as a Buffer.
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
    builder
      .body(body)
      .expect("response body construction is infallible for String and Vec<u8>")
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
    let request = builder
      .body(Body::empty())
      .expect("request body construction is infallible for Body::empty()");
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

/// Builder for manually creating core runtime configuration
/// that can be used to instantiate an application.
#[napi]
pub struct CoreRuntimeConfigBuilder {
  blueprint_config_path: String,
  service_name: String,
  server_port: i32,
  server_loopback_only: Option<bool>,
  use_custom_health_check: Option<bool>,
  trace_otlp_collector_endpoint: Option<String>,
  runtime_max_diagnostics_level: Option<String>,
  platform: Option<CoreRuntimePlatform>,
  test_mode: Option<bool>,
  api_resource: Option<String>,
  consumer_app: Option<String>,
  schedule_app: Option<String>,
  resource_store_verify_tls: Option<bool>,
  resource_store_cache_entry_ttl: Option<i64>,
  resource_store_cleanup_interval: Option<i64>,
  client_ip_source: Option<String>,
  log_format: Option<String>,
  metrics_enabled: Option<bool>,
  trace_sample_ratio: Option<f64>,
}

#[napi]
impl CoreRuntimeConfigBuilder {
  /// Creates a new builder with the required configuration elements.
  #[napi(constructor)]
  pub fn new(blueprint_config_path: String, service_name: String, server_port: i32) -> Self {
    Self {
      blueprint_config_path,
      service_name,
      server_port,
      server_loopback_only: None,
      use_custom_health_check: None,
      trace_otlp_collector_endpoint: None,
      runtime_max_diagnostics_level: None,
      platform: None,
      test_mode: None,
      api_resource: None,
      consumer_app: None,
      schedule_app: None,
      resource_store_verify_tls: None,
      resource_store_cache_entry_ttl: None,
      resource_store_cleanup_interval: None,
      client_ip_source: None,
      log_format: None,
      metrics_enabled: None,
      trace_sample_ratio: None,
    }
  }

  /// Sets whether the HTTP/WebSocket server should only be exposed on
  /// the loopback interface (127.0.0.1). Defaults to `true`.
  ///
  /// When running in an environment such as a docker container,
  /// this should be set to `false` so that the server can be accessed
  /// from outside the container.
  #[napi]
  pub fn set_server_loopback_only(&mut self, value: bool) -> &Self {
    self.server_loopback_only = Some(value);
    self
  }

  /// Sets whether the runtime should use a custom health check endpoint.
  /// Defaults to `false`.
  ///
  /// When `false`, the `GET /runtime/health/check` endpoint returns 200 OK.
  /// The default health check is not accessible under custom base paths
  /// and is only accessible from the root path.
  #[napi]
  pub fn set_use_custom_health_check(&mut self, value: bool) -> &Self {
    self.use_custom_health_check = Some(value);
    self
  }

  /// Sets the endpoint for sending trace data to an OTLP collector.
  /// Defaults to `"http://otelcollector:4317"`.
  #[napi]
  pub fn set_trace_otlp_collector_endpoint(&mut self, value: String) -> &Self {
    self.trace_otlp_collector_endpoint = Some(value);
    self
  }

  /// Sets the maximum diagnostics level for logging and tracing.
  /// Defaults to `"info"`.
  #[napi]
  pub fn set_runtime_max_diagnostics_level(&mut self, value: String) -> &Self {
    self.runtime_max_diagnostics_level = Some(value);
    self
  }

  /// Sets the platform the application is running on.
  /// Defaults to `CoreRuntimePlatform.Other`.
  ///
  /// This determines which platform-specific features are available,
  /// such as AWS X-Ray trace propagation.
  #[napi]
  pub fn set_platform(&mut self, value: CoreRuntimePlatform) -> &Self {
    self.platform = Some(value);
    self
  }

  /// Sets whether the runtime is running in test mode
  /// (e.g. for integration tests). Defaults to `false`.
  #[napi]
  pub fn set_test_mode(&mut self, value: bool) -> &Self {
    self.test_mode = Some(value);
    self
  }

  /// Sets the name of the API resource in the blueprint to use as the
  /// configuration source for setting up API endpoints.
  #[napi]
  pub fn set_api_resource(&mut self, value: String) -> &Self {
    self.api_resource = Some(value);
    self
  }

  /// Sets the name of the consumer app in the blueprint to use as the
  /// configuration source for setting up event source consumers.
  ///
  /// If not set, the runtime will use the first `celerity/consumer`
  /// resource defined in the blueprint.
  #[napi]
  pub fn set_consumer_app(&mut self, value: String) -> &Self {
    self.consumer_app = Some(value);
    self
  }

  /// Sets the name of the schedule app in the blueprint to use as the
  /// configuration source for setting up scheduled event handlers.
  ///
  /// If not set, the runtime will use the first `celerity/schedule`
  /// resource defined in the blueprint.
  #[napi]
  pub fn set_schedule_app(&mut self, value: String) -> &Self {
    self.schedule_app = Some(value);
    self
  }

  /// Sets whether to verify TLS certificates when making requests
  /// to the resource store. Defaults to `true`.
  ///
  /// Must be `true` for production environments; can be `false`
  /// for development environments with self-signed certificates.
  #[napi]
  pub fn set_resource_store_verify_tls(&mut self, value: bool) -> &Self {
    self.resource_store_verify_tls = Some(value);
    self
  }

  /// Sets the TTL in seconds for cache entries in the resource store.
  /// Defaults to 600 seconds (10 minutes).
  #[napi]
  pub fn set_resource_store_cache_entry_ttl(&mut self, value: i64) -> &Self {
    self.resource_store_cache_entry_ttl = Some(value);
    self
  }

  /// Sets the interval in seconds at which the resource store cleanup
  /// task should run. Defaults to 3600 seconds (1 hour).
  #[napi]
  pub fn set_resource_store_cleanup_interval(&mut self, value: i64) -> &Self {
    self.resource_store_cleanup_interval = Some(value);
    self
  }

  /// Sets the source used to resolve client IP addresses.
  ///
  /// One of: "ConnectInfo", "CfConnectingIp", "TrueClientIp",
  /// "CloudFrontViewerAddress", "RightmostXForwardedFor", "XRealIp", "FlyClientIp".
  /// Defaults to "ConnectInfo".
  #[napi]
  pub fn set_client_ip_source(&mut self, value: String) -> &Self {
    self.client_ip_source = Some(value);
    self
  }

  /// Sets the log output format.
  /// If not set, falls back to the `CELERITY_LOG_FORMAT` environment variable.
  #[napi]
  pub fn set_log_format(&mut self, value: String) -> &Self {
    self.log_format = Some(value);
    self
  }

  /// Sets whether to enable metrics collection. Defaults to `false`.
  #[napi]
  pub fn set_metrics_enabled(&mut self, value: bool) -> &Self {
    self.metrics_enabled = Some(value);
    self
  }

  /// Sets the trace sampling ratio (0.0–1.0). Defaults to `0.1` (10%).
  #[napi]
  pub fn set_trace_sample_ratio(&mut self, value: f64) -> &Self {
    self.trace_sample_ratio = Some(value);
    self
  }

  /// Builds a new runtime configuration object from the values set on the builder.
  #[napi]
  pub fn build(&self) -> CoreRuntimeConfig {
    CoreRuntimeConfig {
      blueprint_config_path: self.blueprint_config_path.clone(),
      service_name: self.service_name.clone(),
      server_port: self.server_port,
      server_loopback_only: self.server_loopback_only,
      use_custom_health_check: self.use_custom_health_check,
      trace_otlp_collector_endpoint: self
        .trace_otlp_collector_endpoint
        .clone()
        .unwrap_or_else(|| DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT.to_string()),
      runtime_max_diagnostics_level: self
        .runtime_max_diagnostics_level
        .clone()
        .unwrap_or_else(|| "info".to_string()),
      platform: self.platform.unwrap_or(CoreRuntimePlatform::Other),
      test_mode: self.test_mode.unwrap_or(false),
      api_resource: self.api_resource.clone(),
      consumer_app: self.consumer_app.clone(),
      schedule_app: self.schedule_app.clone(),
      resource_store_verify_tls: self.resource_store_verify_tls.unwrap_or(true),
      resource_store_cache_entry_ttl: self
        .resource_store_cache_entry_ttl
        .unwrap_or(DEFAULT_RESOURCE_STORE_CACHE_ENTRY_TTL),
      resource_store_cleanup_interval: self
        .resource_store_cleanup_interval
        .unwrap_or(DEFAULT_RESOURCE_STORE_CLEANUP_INTERVAL),
      client_ip_source: self.client_ip_source.clone(),
      log_format: self.log_format.clone(),
      metrics_enabled: self.metrics_enabled,
      trace_sample_ratio: self.trace_sample_ratio,
    }
  }
}

/// An application to run in the Celerity runtime.
///
/// Depending on the blueprint configuration file, this can run HTTP APIs,
/// WebSocket APIs and event source consumers for the current environment
/// (e.g. SQS queue consumers when deployed to AWS).
///
/// This class is not thread-safe — handlers run on the Rust async runtime
/// and should not access the application instance directly.
/// Use `websocketRegistry()` to get a thread-safe registry for sending
/// WebSocket messages from handlers.
#[napi]
pub struct CoreRuntimeApplication {
  inner: Application,
  tsfn_cache: Vec<Arc<WeakTsfn>>,
  guard_tsfn_cache: Vec<Arc<GuardWeakTsfn>>,
  consumer_handler_builder: NapiConsumerEventHandlerBuilder,
  consumer_tsfn_cache: Vec<Arc<ConsumerWeakTsfn>>,
  schedule_tsfn_cache: Vec<Arc<ScheduleWeakTsfn>>,
  ws_tsfn_cache: Vec<Arc<WsMessageWeakTsfn>>,
  invoke_tsfn_cache: Vec<Arc<InvokeWeakTsfn>>,
}

#[napi]
impl CoreRuntimeApplication {
  /// Creates a new application with the given runtime configuration.
  #[napi(constructor)]
  pub fn new(runtime_config: CoreRuntimeConfig) -> Self {
    let platform: RuntimePlatform = runtime_config.platform.into();

    let diagnostics_level =
      Level::from_str(&runtime_config.runtime_max_diagnostics_level).unwrap_or(Level::INFO);

    let client_ip_source = runtime_config
      .client_ip_source
      .as_deref()
      .unwrap_or("ConnectInfo")
      .parse::<ClientIpSource>()
      .unwrap_or(ClientIpSource::ConnectInfo);

    let log_format = runtime_config
      .log_format
      .or_else(|| std::env::var("CELERITY_LOG_FORMAT").ok());

    let native_runtime_config = RuntimeConfig {
      blueprint_config_path: runtime_config.blueprint_config_path,
      runtime_call_mode: RuntimeCallMode::Ffi,
      service_name: runtime_config.service_name,
      server_port: runtime_config.server_port,
      server_loopback_only: runtime_config.server_loopback_only,
      // Local API port is not used for the Node runtime
      // as the runtime mode for interaction with application handlers
      // is FFI.
      local_api_port: 0,
      use_custom_health_check: runtime_config.use_custom_health_check,
      trace_otlp_collector_endpoint: runtime_config.trace_otlp_collector_endpoint,
      runtime_max_diagnostics_level: diagnostics_level,
      platform,
      test_mode: runtime_config.test_mode,
      api_resource: runtime_config.api_resource,
      consumer_app: runtime_config.consumer_app,
      schedule_app: runtime_config.schedule_app,
      resource_store_verify_tls: runtime_config.resource_store_verify_tls,
      resource_store_cache_entry_ttl: runtime_config.resource_store_cache_entry_ttl,
      resource_store_cleanup_interval: runtime_config.resource_store_cleanup_interval,
      client_ip_source,
      log_format,
      metrics_enabled: runtime_config.metrics_enabled.unwrap_or(false),
      trace_sample_ratio: runtime_config.trace_sample_ratio.unwrap_or(0.1),
      deploy_target: std::env::var("CELERITY_DEPLOY_TARGET").ok(),
    };
    let inner = Application::new(native_runtime_config, Box::new(ProcessEnvVars::new()));
    CoreRuntimeApplication {
      inner,
      tsfn_cache: vec![],
      guard_tsfn_cache: vec![],
      consumer_handler_builder: NapiConsumerEventHandlerBuilder::new(),
      consumer_tsfn_cache: vec![],
      schedule_tsfn_cache: vec![],
      ws_tsfn_cache: vec![],
      invoke_tsfn_cache: vec![],
    }
  }

  /// Sets up the application based on the configuration the application was
  /// instantiated with. Returns configuration such as HTTP handler definitions
  /// that should be used to register handlers.
  ///
  /// This must be called before `registerHttpHandler`, `registerWebsocketHandler`,
  /// or any other registration methods.
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

  /// Registers a new HTTP handler with the application.
  ///
  /// The handler must be an async function that takes a `Request` object
  /// and returns a `Response` object. An optional timeout in seconds can be
  /// specified (defaults to 60s).
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

  /// Registers a custom authentication guard handler with the application.
  ///
  /// The handler receives a `GuardInput` and must return a `GuardResult`
  /// with status "allowed", "unauthorised", or "forbidden".
  #[allow(clippy::missing_safety_doc)]
  #[napi]
  pub async unsafe fn register_guard_handler(
    &mut self,
    name: String,
    #[napi(ts_arg_type = "(err: Error | null, input: GuardInput) => Promise<GuardResult>")]
    handler: GuardWeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.guard_tsfn_cache.push(tsfn.clone());
    let guard_handler = NapiAuthGuardHandler { tsfn };
    self
      .inner
      .register_custom_auth_guard(&name, guard_handler)
      .await;
    Ok(())
  }

  /// Registers a consumer event handler with the application.
  ///
  /// The handler receives a `JsConsumerEventInput` containing a batch of messages
  /// and must return a `JsEventResult`. An optional timeout in seconds can be
  /// specified (defaults to 60s).
  #[napi]
  pub fn register_consumer_handler(
    &mut self,
    handler_tag: String,
    timeout_seconds: Option<i64>,
    #[napi(
      ts_arg_type = "(err: Error | null, input: JsConsumerEventInput) => Promise<JsEventResult>"
    )]
    handler: ConsumerWeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.consumer_tsfn_cache.push(tsfn.clone());
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    self
      .consumer_handler_builder
      .add_consumer_handler(handler_tag, timeout_secs, tsfn);
    Ok(())
  }

  /// Registers a schedule event handler with the application.
  ///
  /// The handler receives a `JsScheduleEventInput` when a schedule fires
  /// and must return a `JsEventResult`. An optional timeout in seconds can be
  /// specified (defaults to 60s).
  #[napi]
  pub fn register_schedule_handler(
    &mut self,
    handler_tag: String,
    timeout_seconds: Option<i64>,
    #[napi(
      ts_arg_type = "(err: Error | null, input: JsScheduleEventInput) => Promise<JsEventResult>"
    )]
    handler: ScheduleWeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.schedule_tsfn_cache.push(tsfn.clone());
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    self
      .consumer_handler_builder
      .add_schedule_handler(handler_tag, timeout_secs, tsfn);
    Ok(())
  }

  /// Registers a WebSocket handler with the application for the given route.
  ///
  /// The handler receives a `JsWebSocketMessageInfo` object for each WebSocket event
  /// ($connect, $disconnect, or a message matching the route).
  #[napi]
  pub fn register_websocket_handler(
    &mut self,
    route: String,
    #[napi(ts_arg_type = "(err: Error | null, message: JsWebSocketMessageInfo) => Promise<void>")]
    handler: WsMessageWeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.ws_tsfn_cache.push(tsfn.clone());
    let ws_handler = NapiWebSocketMessageHandler::new(tsfn);
    self
      .inner
      .register_websocket_message_handler(&route, ws_handler);
    Ok(())
  }

  /// Retrieves the WebSocket registry for the application.
  ///
  /// The registry allows sending messages to specific WebSocket connections
  /// that are either connected to the current node or other nodes in a
  /// WebSocket API cluster. The returned registry is thread-safe and can
  /// be used from handler functions.
  #[napi]
  pub fn websocket_registry(&self) -> CoreWebSocketRegistry {
    CoreWebSocketRegistry::new(self.inner.websocket_registry())
  }

  /// Registers a custom handler that can be invoked programmatically
  /// via `invokeHandler()`.
  ///
  /// An optional timeout in seconds can be specified (defaults to 60s).
  #[napi]
  pub fn register_custom_handler(
    &mut self,
    handler_name: String,
    timeout_seconds: Option<i64>,
    #[napi(ts_arg_type = "(err: Error | null, payload: any) => Promise<any>")]
    handler: InvokeWeakTsfn,
  ) -> Result<()> {
    let tsfn = Arc::new(handler);
    self.invoke_tsfn_cache.push(tsfn.clone());
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    let invoker = invoke::NapiHandlerInvoker::new(tsfn, timeout_secs);
    self
      .inner
      .register_handler_invoker(handler_name, Arc::new(invoker));
    Ok(())
  }

  /// Invokes a registered custom handler by name with the given payload.
  ///
  /// Returns the result from the handler. Throws if the handler is not
  /// registered or the application is not running.
  #[napi]
  pub async fn invoke_handler(
    &self,
    handler_name: String,
    payload: serde_json::Value,
  ) -> Result<serde_json::Value> {
    let registry = self.inner.handler_invoke_registry();
    let guard = registry.lock().await;
    let invoker = guard.get(&handler_name).cloned().ok_or_else(|| {
      Error::new(
        Status::GenericFailure,
        format!("handler '{}' not found", handler_name),
      )
    })?;
    drop(guard);
    invoker
      .invoke(payload)
      .await
      .map_err(|e| Error::new(Status::GenericFailure, e.to_string()))
  }

  /// Runs the application including an HTTP/WebSocket server and event source consumers
  /// based on the configuration the application was instantiated with.
  ///
  /// When `block` is `true`, the method blocks until the application is stopped.
  /// When `block` is `false`, the application runs in a background thread and
  /// returns immediately — call `shutdown()` to stop it.
  #[allow(clippy::missing_safety_doc)]
  #[napi]
  pub async unsafe fn run(&mut self, block: bool) -> Result<()> {
    // Finalize the consumer/schedule handler builder before starting.
    if !self.consumer_handler_builder.is_empty() {
      let builder = std::mem::take(&mut self.consumer_handler_builder);
      let handler = builder.build();
      self.inner.register_consumer_handler(Arc::new(handler));
    }

    self.inner.run(block).await.map_err(|err| {
      Error::new(
        Status::GenericFailure,
        format!("failed to start core runtime, {err}"),
      )
    })?;
    Ok(())
  }

  /// Shuts down the application and cleans up resources.
  ///
  /// This should be called when the application is running in non-blocking mode
  /// (i.e. `run(false)`) to gracefully stop the server and consumers.
  #[napi]
  pub fn shutdown(&mut self) -> Result<()> {
    self.inner.shutdown();
    self.tsfn_cache.clear();
    self.guard_tsfn_cache.clear();
    self.consumer_tsfn_cache.clear();
    self.schedule_tsfn_cache.clear();
    self.ws_tsfn_cache.clear();
    self.invoke_tsfn_cache.clear();
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
