use std::{
  collections::HashMap,
  process::abort,
  sync::{Arc, Mutex},
  thread,
};

use axum::{body::Body, http::Request, response::IntoResponse};
use celerity_helpers::{
  env::ProcessEnvVars,
  runtime_types::{RuntimeCallMode, RuntimePlatform},
};
use pyo3::prelude::*;
use pyo3_async_runtimes;

use celerity_runtime_core::{
  application::Application,
  config::{
    ApiConfig, AppConfig, HttpConfig, HttpHandlerDefinition, RuntimeConfig, WebSocketConfig,
  },
};
use serde::{Deserialize, Serialize};
use tokio::sync::{mpsc, oneshot, Mutex as TokioMutex};

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

// Message sent to the Python worker
struct PythonCall {
  handler_id: String,
  args: Vec<PyObject>, // or whatever your handler expects
  response: oneshot::Sender<Result<Response, HandlerError>>,
}

async fn python_worker(
  mut rx: mpsc::UnboundedReceiver<PythonCall>,
  handlers: Arc<TokioMutex<HashMap<String, Py<PyAny>>>>,
) {
  while let Some(PythonCall {
    handler_id,
    args,
    response,
  }) = rx.recv().await
  {
    let handlers_lock = handlers.lock().await;
    let handler = handlers_lock.get(&handler_id);
    let wrapped_future = Python::with_gil(|py| {
      if let Some(handler) = handler {
        let internal_handler = handler.bind(py);
        let coro = internal_handler.call0()?;
        pyo3_async_runtimes::tokio::into_future(coro)
      } else {
        Err(PyErr::new::<pyo3::exceptions::PyException, _>(format!(
          "Handler not found: {}",
          handler_id
        )))
      }
    });

    let run_result = match wrapped_future {
      Ok(future) => future
        .await
        .map_err(|err| HandlerError::new(err.to_string())),
      Err(err) => Err(HandlerError::new(err.to_string())),
    };

    let result = match run_result {
      Ok(output) => Python::with_gil(|py| -> PyResult<Response> { output.extract(py) })
        .map_err(|err| HandlerError::new(err.to_string())),
      Err(err) => Err(HandlerError::new(err.to_string())),
    };

    let _ = response.send(result);
  }
}

#[pyclass]
struct CoreRuntimeApplication {
  inner: Arc<Mutex<Application>>,
  task_locals: Option<pyo3_async_runtimes::TaskLocals>,
  py_rx: Option<mpsc::UnboundedReceiver<PythonCall>>,
  py_tx: Option<mpsc::UnboundedSender<PythonCall>>,
  handler_registry: Arc<TokioMutex<HashMap<String, Py<PyAny>>>>,
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
      service_name: "CelerityTestService".to_string(),
      platform: RuntimePlatform::Local,
      trace_otlp_collector_endpoint: "http://localhost:4317".to_string(),
      runtime_max_diagnostics_level: tracing::Level::INFO,
      test_mode: false,
      api_resource: None,
      consumer_app: None,
      schedule_app: None,
    };
    print!(
      "Creating CoreRuntimeApplication with config: {:?}\n",
      native_runtime_config
    );
    let inner = Application::new(native_runtime_config, Box::new(ProcessEnvVars::new()));
    CoreRuntimeApplication {
      inner: Arc::new(Mutex::new(inner)),
      task_locals: None,
      py_rx: None,
      py_tx: None,
      handler_registry: Arc::new(TokioMutex::new(HashMap::new())),
    }
  }

  fn setup(&mut self, py: Python) -> PyResult<CoreRuntimeAppConfig> {
    // Set up the asyncio event loop
    let asyncio = py.import("asyncio")?;
    let event_loop = asyncio.call_method0("new_event_loop")?;
    asyncio.call_method1("set_event_loop", (event_loop.clone(),))?;

    let task_locals = pyo3_async_runtimes::TaskLocals::new(event_loop).copy_context(py)?;
    self.task_locals = Some(task_locals);

    let (py_tx, py_rx) = mpsc::unbounded_channel();
    self.py_tx = Some(py_tx);
    self.py_rx = Some(py_rx);

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
    let handler_id = format!("{}::{}", path, method);
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }

    let py_tx = self.py_tx.as_ref().unwrap().clone();
    let final_handler = move |_req: Request<Body>| {
      let py_tx = py_tx.clone();
      let handler_id = handler_id.clone();
      async move {
        let (response_tx, response_rx) = tokio::sync::oneshot::channel();
        py_tx
          .send(PythonCall {
            handler_id,
            args: vec![],
            response: response_tx,
          })
          .map_err(|_| HandlerError::new("Python worker unavailable".to_string()))?;
        let result = response_rx
          .await
          .map_err(|_| HandlerError::new("Python worker dropped".to_string()))?;
        result.map_err(|e| HandlerError::new(format!("Python error: {e}")))
      }
    };

    // 4. Register the HTTP handler with the Rust runtime (synchronously)
    self
      .inner
      .lock()
      .unwrap()
      .register_http_handler(&path, &method, final_handler);

    Ok(())
  }

  fn run(&mut self, py: Python) -> PyResult<()> {
    let inner = self.inner.clone();
    let handler_registry = self.handler_registry.clone();
    let py_rx = self
      .py_rx
      .take()
      .expect("run should be called before setup and should only be called once");
    let task_locals = self
      .task_locals
      .as_ref()
      .expect("run should be called before setup")
      .clone_ref(py);

    thread::spawn(move || {
      let rt = runtime::new_tokio_multi_thread().expect("failed to create tokio runtime");
      let _ = rt.block_on(async move {
        tokio::spawn(pyo3_async_runtimes::tokio::scope(
          task_locals,
          python_worker(py_rx, handler_registry),
        ));

        match inner.lock().unwrap().run(true).await {
          Ok(_) => {}
          Err(err) => {
            println!("Error running core runtime: {}", err);
            abort();
          }
        }
      });
    });

    let event_loop = self.task_locals.as_ref().unwrap().event_loop(py);
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
