#![deny(clippy::all)]

use std::{collections::HashMap, time::Duration};

use axum::{
  body::{Body, Bytes},
  http::{request::Parts, Request},
  response::IntoResponse,
};
use celerity_runtime_core::{
  application::{
    ApiConfig, AppConfig, Application, HttpConfig, HttpHandlerDefinition, WebsocketConfig,
  },
  config::{RuntimeCallMode, RuntimeConfig},
};
use napi::{
  bindgen_prelude::*,
  threadsafe_function::{
    ErrorStrategy::{self},
    ThreadSafeCallContext, ThreadsafeFunction,
  },
};
use serde::{Deserialize, Serialize};
use tokio::time;

#[macro_use]
extern crate napi_derive;

#[napi]
pub fn sum(a: i32, b: i32) -> i32 {
  a + b
}

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

impl From<WebsocketConfig> for CoreWebsocketConfig {
  fn from(_: WebsocketConfig) -> Self {
    Self {}
  }
}

#[napi(object)]
pub struct CoreHttpHandlerDefinition {
  pub path: String,
  pub method: String,
  pub location: String,
  pub handler: String,
}

impl From<HttpHandlerDefinition> for CoreHttpHandlerDefinition {
  fn from(handler: HttpHandlerDefinition) -> Self {
    Self {
      path: handler.path,
      method: handler.method,
      location: handler.location,
      handler: handler.handler,
    }
  }
}

#[napi(object)]
pub struct Response {
  pub status: u16,
  pub headers: Option<HashMap<String, String>>,
  pub body: Option<String>,
}

impl IntoResponse for Response {
  fn into_response(self) -> axum::response::Response<Body> {
    let mut builder = axum::response::Response::builder();
    for (key, value) in self.headers.unwrap_or_default() {
      builder = builder.header(key, value);
    }
    builder = builder.status(self.status);
    builder
      .body(Body::from(self.body.unwrap_or_else(|| "".to_string())))
      .unwrap()
  }
}

#[derive(Debug)]
pub enum JsRequestWrapperBody {
  Bytes(Bytes),
  InnerSourceBody(Option<Body>),
  EmptyBody,
}

#[napi(js_name = "Request")]
pub struct JsRequestWrapper {
  inner_body: JsRequestWrapperBody,
  inner_parts: Parts,
}

#[napi]
impl JsRequestWrapper {
  /// Allows the creation of requests, primarily for test purposes.
  /// In normal circumstances, the request will be created by
  /// the runtime and passed to the handler.
  #[napi(constructor)]
  pub fn new(method: String, uri: String, headers: HashMap<String, String>) -> Self {
    let mut builder = Request::builder().method(method.as_str()).uri(uri);
    for (key, value) in headers {
      builder = builder.header(key, value);
    }
    let request = builder.body(Body::empty()).unwrap();
    let (parts, _) = request.into_parts();
    Self {
      inner_parts: parts,
      inner_body: JsRequestWrapperBody::EmptyBody,
    }
  }

  fn from_axum_req(req: axum::extract::Request<Body>) -> Self {
    let (parts, body) = req.into_parts();
    let content_length = parts
      .headers
      .get("content-length")
      .and_then(|value| value.to_str().ok())
      .and_then(|value| value.parse::<usize>().ok())
      .unwrap_or(0);
    Self {
      inner_parts: parts,
      inner_body: if content_length > 0 {
        JsRequestWrapperBody::InnerSourceBody(Some(body))
      } else {
        JsRequestWrapperBody::EmptyBody
      },
    }
  }

  /// The HTTP version used for the request.
  #[napi]
  pub fn http_version(&self) -> String {
    format!("{:?}", self.inner_parts.version)
  }

  /// The HTTP method of the request.
  #[napi]
  pub fn method(&self) -> String {
    self.inner_parts.method.to_string()
  }

  /// The URI of the request.
  #[napi]
  pub fn uri(&self) -> String {
    self.inner_parts.uri.to_string()
  }

  /// The headers of the request.
  #[napi]
  pub fn headers(&self) -> HashMap<String, String> {
    self
      .inner_parts
      .headers
      .iter()
      .map(|(key, value)| {
        (
          key.as_str().to_string(),
          value.to_str().unwrap().to_string(),
        )
      })
      .collect()
  }

  /// The body of the request, either as a string or `null` if the body is empty.
  #[napi]
  pub async unsafe fn body(&mut self) -> Result<Option<String>> {
    match &mut self.inner_body {
      JsRequestWrapperBody::Bytes(bytes) => {
        return Ok(String::from_utf8(bytes.to_vec()).map(Some).map_err(|err| {
          Error::new(
            Status::GenericFailure,
            format!("failed to parse request body, {}", err),
          )
        })?)
      }
      JsRequestWrapperBody::InnerSourceBody(body_opt) => {
        // todo: set size constraints
        let bytes = axum::body::to_bytes(
          body_opt
            .take()
            .expect("axum request source body should not have been consumed"),
          usize::MAX,
        )
        .await
        .map_err(|err| {
          Error::new(
            Status::GenericFailure,
            format!("failed to read request body, {}", err),
          )
        })?;
        self.inner_body = JsRequestWrapperBody::Bytes(bytes.clone());
        Ok(String::from_utf8(bytes.to_vec()).map(Some).map_err(|err| {
          Error::new(
            Status::GenericFailure,
            format!("failed to parse request body, {}", err),
          )
        })?)
      }
      JsRequestWrapperBody::EmptyBody => Ok(None),
    }
  }
}

#[napi]
pub struct CoreRuntimeApplication {
  inner: Application,
  tsfn_cache: Vec<ThreadsafeFunction<JsRequestWrapper, ErrorStrategy::CalleeHandled>>,
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
    };
    let inner = Application::new(native_runtime_config);
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
        format!("failed to setup core runtime, {}", err),
      )
    })?;
    Ok(app_config.into())
  }

  #[napi]
  pub fn register_http_handler(
    &mut self,
    path: String,
    method: String,
    #[napi(ts_arg_type = "(err: Error | null, request: Request) => Promise<Response>")]
    handler: JsFunction,
  ) -> Result<()> {
    let tsfn: ThreadsafeFunction<JsRequestWrapper, ErrorStrategy::CalleeHandled> = handler
      .create_threadsafe_function(0, |ctx: ThreadSafeCallContext<JsRequestWrapper>| {
        Ok(vec![ctx.value])
      })?;
    self.tsfn_cache.push(tsfn.clone());

    let handler = |req| async move {
      let js_req_wrapper = JsRequestWrapper::from_axum_req(req);
      let promise = tsfn
        .call_async::<Promise<Response>>(Ok(js_req_wrapper))
        .await
        .map_err(|err| HandlerError::new(err.to_string()))?;
      // TODO: make the timeout configurable based on handler configuration.
      let sleep = time::sleep(Duration::from_secs(30));
      tokio::select! {
        _ = sleep => {
          Err(HandlerError::new("handler timed out".to_string()))
        }
        value = promise => {
          Ok::<Response, HandlerError>(value.map_err(|err| HandlerError::new(err.to_string()))?)
        }
      }
    };
    self.inner.register_http_handler(&path, &method, handler);
    Ok(())
  }

  #[allow(clippy::missing_safety_doc)]
  #[napi]
  pub async unsafe fn run(&mut self) -> Result<()> {
    self.inner.run().await.map_err(|err| {
      Error::new(
        Status::GenericFailure,
        format!("failed to start core runtime, {}", err),
      )
    })?;
    Ok(())
  }

  #[napi]
  pub fn shutdown(&mut self, env: Env) -> Result<()> {
    self.inner.shutdown();
    for mut tsfn in self.tsfn_cache.drain(..) {
      tsfn.unref(&env)?;
    }
    Ok(())
  }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct HandlerError {
  pub message: String,
}

impl HandlerError {
  pub fn new(message: String) -> Self {
    Self { message }
  }
}

impl std::fmt::Display for HandlerError {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    write!(f, "{}", self.message)
  }
}

impl IntoResponse for HandlerError {
  fn into_response(self) -> axum::response::Response<Body> {
    axum::response::Json(self).into_response()
  }
}
