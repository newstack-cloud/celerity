use std::{
  collections::HashMap,
  process::abort,
  str::FromStr,
  sync::{Arc, Mutex},
  thread,
};

use axum::{
  body::Body,
  http::{header::CONTENT_TYPE, Request},
};
use celerity_helpers::{
  env::ProcessEnvVars,
  request::{
    cookies_from_headers, headers_to_hashmap, path_params_from_request_parts, query_from_uri,
    to_request_body,
  },
  runtime_types::RuntimeCallMode,
};
use pyo3::prelude::*;

use celerity_runtime_core::{
  application::Application,
  config::{ApiConfig, AppConfig, RuntimeConfig},
  request::RequestId,
  telemetry_utils::extract_trace_context,
};
use tokio::sync::{mpsc, Mutex as TokioMutex};
use tracing::Level;

use crate::{
  core_runtime_config::{CoreRuntimeConfig, CoreRuntimeConfigBuilder, CoreRuntimePlatform},
  errors::HandlerError,
  http::{
    core_http_config, CoreHttpConfig, PyRequest, PyRequestBuilder, PyRequestContext, PyResponse,
    PyResponseBuilder,
  },
  interop::{python_worker, PythonCall},
  websockets::{
    core_websocket_config, CoreWebSocketConfig, CoreWebSocketHandlerDefinition, WSBindingEventType,
    WSBindingMessageHandler, WSBindingMessageInfo, WSBindingMessageInfoBuilder,
    WSBindingMessageRequestContext, WSBindingMessageRequestContextBuilder, WSBindingMessageType,
    WSBindingRegistrySend, WSBindingSendContext,
  },
};

mod core_runtime_config;
mod errors;
mod http;
mod interop;
mod json_convert;
mod runtime;
mod websockets;

#[pyclass]
struct CoreRuntimeAppConfig {
  #[pyo3(get)]
  api: Option<Py<CoreApiConfig>>,
}

impl From<AppConfig> for CoreRuntimeAppConfig {
  fn from(app_config: AppConfig) -> Self {
    let api = app_config.api.map(core_api_config);

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
    let http = api_config.http.map(core_http_config);
    let websocket = api_config.websocket.map(core_websocket_config);
    Self { http, websocket }
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
      // Local API port is not used for the Python runtime
      // as the runtime mode for interaction with application handlers
      // is FFI.
      local_api_port: 0,
      use_custom_health_check: runtime_config.use_custom_health_check,
      service_name: runtime_config.service_name.clone(),
      platform: runtime_config.platform.clone().into(),
      trace_otlp_collector_endpoint: runtime_config.trace_otlp_collector_endpoint.clone(),
      runtime_max_diagnostics_level: Level::from_str(&runtime_config.runtime_max_diagnostics_level)
        .expect("runtime_max_diagnostics_level should be a valid tracing level"),
      test_mode: runtime_config.test_mode,
      api_resource: runtime_config.api_resource.clone(),
      consumer_app: runtime_config.consumer_app.clone(),
      schedule_app: runtime_config.schedule_app.clone(),
      resource_store_verify_tls: runtime_config.resource_store_verify_tls,
      resource_store_cache_entry_ttl: runtime_config.resource_store_cache_entry_ttl,
      resource_store_cleanup_interval: runtime_config.resource_store_cleanup_interval,
    };
    println!("Creating CoreRuntimeApplication with config: {native_runtime_config:?}");
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
          "failed to obtain lock to application, {err}",
        ))
      })?
      .setup()
      .map_err(|err| {
        PyErr::new::<pyo3::exceptions::PyException, _>(format!(
          "failed to setup core runtime, {err}",
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
    let handler_id = format!("{path}::{method}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }

    let py_tx = self.py_tx.as_ref().unwrap().clone();
    let final_handler = move |req: Request<Body>| {
      let py_tx = py_tx.clone();
      let handler_id = handler_id.clone();

      async move {
        let (response_tx, response_rx) = tokio::sync::oneshot::channel();
        let request_id = req
          .extensions()
          .get::<RequestId>()
          .unwrap_or(&RequestId("".to_string()))
          .0
          .clone();

        let (mut parts, body) = req.into_parts();
        let body_bytes = axum::body::to_bytes(body, usize::MAX)
          .await
          .map_err(|err| HandlerError::new(err.to_string()))?;

        let (text_body, binary_body, content_type) =
          to_request_body(&body_bytes, parts.headers.get(CONTENT_TYPE).cloned());

        let query = query_from_uri(&parts.uri).map_err(|err| HandlerError::new(err.to_string()))?;
        let cookies = cookies_from_headers(&parts.headers);
        let path_params = path_params_from_request_parts(&mut parts)
          .await
          .map_err(|err| HandlerError::new(err.to_string()))?;

        let py_req = Python::with_gil(|py| {
          Py::new(
            py,
            PyRequest {
              text_body,
              binary_body,
              content_type,
              headers: headers_to_hashmap(&parts.headers),
              query,
              cookies,
              method: parts.method.to_string(),
              path: parts.uri.path().to_string(),
              path_params,
              protocol_version: parts.version.into(),
            },
          )
        })
        .map_err(|err| HandlerError::new(err.to_string()))?;

        let py_req_ctx = Python::with_gil(|py| {
          Py::new(
            py,
            PyRequestContext {
              request_id,
              request_time: chrono::Utc::now(),
              auth: Python::None(py),
              trace_context: extract_trace_context(),
            },
          )
        })
        .map_err(|err| HandlerError::new(err.to_string()))?;

        py_tx
          .send(PythonCall {
            handler_id,
            args: vec![py_req.into(), py_req_ctx.into()],
            response: response_tx,
          })
          .map_err(|_| HandlerError::new("Python worker unavailable".to_string()))?;
        let result = response_rx
          .await
          .map_err(|_| HandlerError::new("Python worker dropped".to_string()))?;
        result.map_err(|e| HandlerError::new(format!("Python error: {e}")))
      }
    };

    self
      .inner
      .lock()
      .expect("should be able to obtain inner application lock")
      .register_http_handler(&path, &method, final_handler);

    Ok(())
  }

  fn register_websocket_handler(&mut self, route: String, handler: Py<PyAny>) -> PyResult<()> {
    let handler_id = format!("websocket::{route}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }

    let py_tx = self.py_tx.as_ref().unwrap().clone();
    let final_handler = WSBindingMessageHandler { handler_id, py_tx };

    self
      .inner
      .lock()
      .expect("should be able to obtain inner application lock")
      .register_websocket_message_handler(&route, final_handler);

    Ok(())
  }

  fn websocket_registry(&self, py: Python) -> PyResult<Py<WSBindingRegistrySend>> {
    Py::new(
      py,
      WSBindingRegistrySend {
        inner: self
          .inner
          .lock()
          .expect("should be able to obtain inner application lock")
          .websocket_registry(),
      },
    )
  }

  // SAFETY: run can hold a std mutex lock across an await boundary as there will be no other
  // threads/tasks trying to obtain a lock on the application for the duration of the await
  // that runs the application.
  // Locks are only held on the inner core runtime application
  // for setup and handler registration which must always be called before run.
  #[allow(clippy::await_holding_lock)]
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
      rt.block_on(async move {
        tokio::spawn(pyo3_async_runtimes::tokio::scope(
          task_locals,
          python_worker(py_rx, handler_registry),
        ));

        match inner.lock().unwrap().run(true).await {
          Ok(_) => {}
          Err(err) => {
            println!("Error running core runtime: {err}");
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
  m.add_class::<CoreRuntimeConfig>()?;
  m.add_class::<CoreRuntimeConfigBuilder>()?;
  m.add_class::<CoreRuntimePlatform>()?;
  m.add_class::<CoreRuntimeApplication>()?;
  m.add_class::<PyResponse>()?;
  m.add_class::<PyResponseBuilder>()?;
  m.add_class::<WSBindingMessageInfo>()?;
  m.add_class::<WSBindingMessageInfoBuilder>()?;
  m.add_class::<WSBindingMessageRequestContext>()?;
  m.add_class::<WSBindingMessageRequestContextBuilder>()?;
  m.add_class::<WSBindingEventType>()?;
  m.add_class::<WSBindingMessageType>()?;
  m.add_class::<CoreWebSocketConfig>()?;
  m.add_class::<CoreWebSocketHandlerDefinition>()?;
  m.add_class::<PyRequest>()?;
  m.add_class::<PyRequestBuilder>()?;
  m.add_class::<PyRequestContext>()?;
  m.add_class::<WSBindingSendContext>()?;
  m.add_class::<WSBindingRegistrySend>()?;
  Ok(())
}
