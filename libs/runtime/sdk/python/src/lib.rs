use std::{
  collections::HashMap,
  process::abort,
  sync::{Arc, Mutex},
  thread,
};

use axum::{body::Body, http::Request, response::IntoResponse};
use pyo3::prelude::*;
use pyo3_asyncio_0_21;

use celerity_runtime_core::{
  application::Application,
  config::{
    ApiConfig, AppConfig, HttpConfig, HttpHandlerDefinition, RuntimeCallMode, RuntimeConfig,
    WebSocketConfig,
  },
};
use serde::{Deserialize, Serialize};

mod runtime;

/// Formats the sum of two numbers as string.
#[pyfunction]
fn sum_as_string(a: usize, b: usize) -> PyResult<String> {
  Ok((a + b).to_string())
}

#[pyclass]
pub struct CoreRuntimeConfig {
  #[pyo3(get)]
  blueprint_config_path: String,
  #[pyo3(get)]
  server_port: i32,
  #[pyo3(get)]
  server_loopback_only: Option<bool>,
}

#[pymethods]
impl CoreRuntimeConfig {
  #[new]
  #[pyo3(signature = (blueprint_config_path, server_port, server_loopback_only))]
  fn new(
    blueprint_config_path: String,
    server_port: i32,
    server_loopback_only: Option<bool>,
  ) -> Self {
    CoreRuntimeConfig {
      blueprint_config_path,
      server_port,
      server_loopback_only,
    }
  }
}

#[pyclass]
struct CoreRuntimeAppConfig {
  #[pyo3(get)]
  api: Option<Py<CoreApiConfig>>,
}

impl From<AppConfig> for CoreRuntimeAppConfig {
  fn from(app_config: AppConfig) -> Self {
    let api = app_config.api.map(|api_config| core_api_config(api_config));

    Self { api }
  }
}

#[pyclass]
struct CoreApiConfig {
  #[pyo3(get)]
  http: Option<Py<CoreHttpConfig>>,
  #[pyo3(get)]
  websocket: Option<Py<CoreWebSocketConfig>>,
}

fn core_api_config(api_config: ApiConfig) -> Py<CoreApiConfig> {
  Python::with_gil(|py| Py::new(py, CoreApiConfig::from(api_config)).unwrap())
}

impl From<ApiConfig> for CoreApiConfig {
  fn from(api_config: ApiConfig) -> Self {
    let http = api_config
      .http
      .map(|http_config| core_http_config(http_config));
    let websocket = api_config
      .websocket
      .map(|websocket_config| core_websocket_config(websocket_config));
    Self { http, websocket }
  }
}

#[pyclass]
struct CoreHttpConfig {
  #[pyo3(get)]
  handlers: Vec<Py<CoreHttpHandlerDefinition>>,
}

fn core_http_config(http_config: HttpConfig) -> Py<CoreHttpConfig> {
  Python::with_gil(|py| Py::new(py, CoreHttpConfig::from(http_config)).unwrap())
}

impl From<HttpConfig> for CoreHttpConfig {
  fn from(http_config: HttpConfig) -> Self {
    let handlers = http_config
      .handlers
      .into_iter()
      .map(|handler| core_http_handler_definition(handler))
      .collect::<Vec<_>>();
    Self { handlers }
  }
}

#[pyclass]
struct CoreHttpHandlerDefinition {
  #[pyo3(get)]
  path: String,
  #[pyo3(get)]
  method: String,
  #[pyo3(get)]
  location: String,
  #[pyo3(get)]
  handler: String,
}

fn core_http_handler_definition(
  http_handler_definition: HttpHandlerDefinition,
) -> Py<CoreHttpHandlerDefinition> {
  Python::with_gil(|py| {
    Py::new(py, CoreHttpHandlerDefinition::from(http_handler_definition)).unwrap()
  })
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

#[pyclass]
struct CoreWebSocketConfig {}

fn core_websocket_config(websocket_config: WebSocketConfig) -> Py<CoreWebSocketConfig> {
  Python::with_gil(|py| Py::new(py, CoreWebSocketConfig::from(websocket_config)).unwrap())
}

impl From<WebSocketConfig> for CoreWebSocketConfig {
  fn from(_: WebSocketConfig) -> Self {
    Self {}
  }
}

#[derive(Debug, Clone, FromPyObject)]
struct Response {
  status: u16,
  headers: Option<HashMap<String, String>>,
  body: Option<String>,
}

impl IntoResponse for Response {
  fn into_response(self) -> axum::response::Response<Body> {
    let mut builder = axum::response::Response::builder();
    for (key, value) in self.headers.unwrap_or_default() {
      builder = builder.header(key, value);
    }
    builder = builder.status(self.status);
    builder
      .body(Body::from(self.body.unwrap_or_default()))
      .unwrap()
  }
}

#[pyclass(name = "Response")]
#[derive(Debug, Clone)]
pub struct PyResponse {
  #[pyo3(get)]
  status: u16,
  #[pyo3(get)]
  headers: Option<HashMap<String, String>>,
  #[pyo3(get)]
  body: Option<String>,
}

#[pymethods]
impl PyResponse {
  #[new]
  #[pyo3(signature = (status, headers, body))]
  fn new(status: u16, headers: Option<HashMap<String, String>>, body: Option<String>) -> Self {
    PyResponse {
      status,
      headers,
      body,
    }
  }
}

#[pyclass]
struct CoreRuntimeApplication {
  inner: Arc<Mutex<Application>>,
  task_locals: Option<pyo3_asyncio_0_21::TaskLocals>,
}

#[pymethods]
impl CoreRuntimeApplication {
  #[new]
  fn new(runtime_config: PyRef<CoreRuntimeConfig>) -> Self {
    let native_runtime_config = RuntimeConfig {
      blueprint_config_path: runtime_config.blueprint_config_path.clone(),
      runtime_call_mode: RuntimeCallMode::Ffi,
      server_loopback_only: runtime_config.server_loopback_only,
      server_port: runtime_config.server_port,
      local_api_port: 8259,
      use_custom_health_check: None,
    };
    print!(
      "Creating CoreRuntimeApplication with config: {:?}\n",
      native_runtime_config
    );
    let inner = Application::new(native_runtime_config);
    CoreRuntimeApplication {
      inner: Arc::new(Mutex::new(inner)),
      task_locals: None,
    }
  }

  fn setup(&mut self, py: Python) -> PyResult<CoreRuntimeAppConfig> {
    // Set up the asyncio event loop
    let asyncio = py.import_bound("asyncio")?;
    let event_loop = asyncio.call_method0("new_event_loop")?;
    asyncio.call_method1("set_event_loop", (event_loop.clone(),))?;

    let task_locals = pyo3_asyncio_0_21::TaskLocals::new(event_loop).copy_context(py)?;
    self.task_locals = Some(task_locals);

    let app_config = self
      .inner
      .lock()
      .map_err(|err| {
        PyErr::new::<pyo3::exceptions::PyException, _>(format!(
          "failed to obtain lock to application, {}",
          err
        ))
      })?
      .setup()
      .map_err(|err| {
        PyErr::new::<pyo3::exceptions::PyException, _>(format!(
          "failed to setup core runtime, {}",
          err
        ))
      })?;
    Ok(app_config.into())
  }

  fn register_http_handler(
    &mut self,
    path: String,
    method: String,
    handler: Py<PyAny>,
  ) -> PyResult<()> {
    let task_locals_copy = self.task_locals.clone().ok_or_else(|| {
      PyErr::new::<pyo3::exceptions::PyException, _>(
        "async event loop not initialised, call setup() first",
      )
    })?;

    let final_handler = |_req: Request<Body>| async move {
      let result = pyo3_asyncio_0_21::tokio::scope(task_locals_copy, async move {
        let output = Python::with_gil(|py| {
          let internal_handler = handler.bind(py);
          let coro = internal_handler.call0()?;

          // Convert the coroutine into a Rust future using the `tokio` runtime.
          pyo3_asyncio_0_21::tokio::into_future(coro)
        })
        .map_err(|err| HandlerError::new(err.to_string()))?
        .await
        .map_err(|err| HandlerError::new(err.to_string()))?;

        Python::with_gil(|py| -> PyResult<Response> { output.extract(py) })
          .map_err(|err| HandlerError::new(err.to_string()))
      })
      .await;
      result
    };
    self
      .inner
      .lock()
      .unwrap()
      .register_http_handler(&path, &method, final_handler);
    Ok(())
  }

  fn run(&mut self, py: Python) -> PyResult<()> {
    let inner = self.inner.clone();
    thread::spawn(move || {
      let rt = runtime::new_tokio_multi_thread().expect("failed to create tokio runtime");
      let _ = rt.block_on(inner.lock().unwrap().run(true));
    });

    let event_loop = self.task_locals.clone().unwrap().event_loop(py);
    let run_forever_res = event_loop.call_method0("run_forever");
    if run_forever_res.is_err() {
      println!("Ctrl C pressed, shutting down...");
      abort();
    }
    Ok(())
  }
}

/// The Celerity Runtime SDK module implemented in Rust.
#[pymodule]
fn _celerity_runtime_sdk(m: &Bound<'_, PyModule>) -> PyResult<()> {
  m.add_function(wrap_pyfunction!(sum_as_string, m)?)?;
  m.add_class::<CoreRuntimeConfig>()?;
  m.add_class::<CoreRuntimeApplication>()?;
  m.add_class::<PyResponse>()?;
  Ok(())
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
