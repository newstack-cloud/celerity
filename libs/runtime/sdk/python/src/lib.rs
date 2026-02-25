use std::{
  collections::HashMap,
  process::abort,
  str::FromStr,
  sync::{Arc, Mutex},
  thread,
  time::Duration,
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
  auth_http::AuthContext,
  config::{
    ApiConfig, AppConfig, ClientIpSource, ConsumerConfig, ConsumersConfig, CustomHandlerDefinition,
    CustomHandlersConfig, EventHandlerDefinition, GuardHandlerDefinition, GuardsConfig,
    RuntimeConfig, ScheduleConfig, SchedulesConfig,
  },
  request::{MatchedRoute, RequestId, ResolvedClientIp, ResolvedUserAgent},
  telemetry_utils::extract_trace_context,
};
use pythonize::pythonize;
use tokio::sync::{mpsc, Mutex as TokioMutex};
use tokio::time;
use tracing::Level;

const MAX_REQUEST_BODY_SIZE: usize = 10 * 1024 * 1024; // 10 MiB

use crate::{
  core_runtime_config::{CoreRuntimeConfig, CoreRuntimeConfigBuilder, CoreRuntimePlatform},
  errors::HandlerError,
  http::{
    core_http_config, CoreHttpConfig, CoreHttpHandlerDefinition, HttpProtocolVersion, PyRequest,
    PyRequestBuilder, PyRequestContext, PyResponse, PyResponseBuilder,
  },
  interop::{python_worker, PythonCall},
  websockets::{
    core_websocket_config, CoreWebSocketConfig, CoreWebSocketHandlerDefinition, WSBindingEventType,
    WSBindingMessageHandler, WSBindingMessageInfo, WSBindingMessageInfoBuilder,
    WSBindingMessageRequestContext, WSBindingMessageRequestContextBuilder, WSBindingMessageType,
    WSBindingRegistrySend, WSBindingSendContext,
  },
};

mod consumer;
mod core_runtime_config;
mod errors;
mod guard;
mod http;
mod interop;
mod invoke;
mod json_convert;
mod runtime;
mod websockets;

#[pyclass]
struct CoreRuntimeAppConfig {
  #[pyo3(get)]
  api: Option<Py<CoreApiConfig>>,
  #[pyo3(get)]
  consumers: Option<Py<CoreConsumersConfig>>,
  #[pyo3(get)]
  schedules: Option<Py<CoreSchedulesConfig>>,
  #[pyo3(get)]
  custom_handlers: Option<Py<CoreCustomHandlersConfig>>,
}

impl CoreRuntimeAppConfig {
  fn try_from_core(app_config: AppConfig) -> PyResult<Self> {
    Python::with_gil(|py| {
      let api = app_config.api.map(|a| core_api_config(a, py)).transpose()?;
      let consumers = app_config
        .consumers
        .map(|c| Py::new(py, CoreConsumersConfig::try_from_core(c, py)?))
        .transpose()?;
      let schedules = app_config
        .schedules
        .map(|s| Py::new(py, CoreSchedulesConfig::try_from_core(s, py)?))
        .transpose()?;
      let custom_handlers = app_config
        .custom_handlers
        .map(|ch| Py::new(py, CoreCustomHandlersConfig::try_from_core(ch, py)?))
        .transpose()?;

      Ok(Self {
        api,
        consumers,
        schedules,
        custom_handlers,
      })
    })
  }
}

#[pyclass]
struct CoreApiConfig {
  #[pyo3(get)]
  http: Option<Py<CoreHttpConfig>>,
  #[pyo3(get)]
  websocket: Option<Py<CoreWebSocketConfig>>,
  #[pyo3(get)]
  guards: Option<Py<CoreGuardsConfig>>,
}

fn core_api_config(api_config: ApiConfig, py: Python) -> PyResult<Py<CoreApiConfig>> {
  Py::new(py, CoreApiConfig::try_from_core(api_config, py)?)
}

impl CoreApiConfig {
  fn try_from_core(api_config: ApiConfig, py: Python) -> PyResult<Self> {
    let http = api_config
      .http
      .map(|h| core_http_config(h, py))
      .transpose()?;
    let websocket = api_config
      .websocket
      .map(|w| core_websocket_config(w, py))
      .transpose()?;
    let guards = api_config
      .guards
      .map(|g| Py::new(py, CoreGuardsConfig::try_from_core(g, py)?))
      .transpose()?;
    Ok(Self {
      http,
      websocket,
      guards,
    })
  }
}

// ---------------------------------------------------------------------------
// Guards config types
// ---------------------------------------------------------------------------

#[pyclass]
pub struct CoreGuardsConfig {
  #[pyo3(get)]
  handlers: Vec<Py<CoreGuardHandlerDefinition>>,
}

impl CoreGuardsConfig {
  fn try_from_core(guards_config: GuardsConfig, py: Python) -> PyResult<Self> {
    let handlers = guards_config
      .handlers
      .into_iter()
      .map(|h| Py::new(py, CoreGuardHandlerDefinition::from(h)))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self { handlers })
  }
}

#[pyclass]
pub struct CoreGuardHandlerDefinition {
  #[pyo3(get)]
  name: String,
}

impl From<GuardHandlerDefinition> for CoreGuardHandlerDefinition {
  fn from(def: GuardHandlerDefinition) -> Self {
    Self { name: def.name }
  }
}

// ---------------------------------------------------------------------------
// Consumer config types
// ---------------------------------------------------------------------------

#[pyclass]
pub struct CoreConsumersConfig {
  #[pyo3(get)]
  consumers: Vec<Py<CoreConsumerConfig>>,
}

impl CoreConsumersConfig {
  fn try_from_core(c: ConsumersConfig, py: Python) -> PyResult<Self> {
    let consumers = c
      .consumers
      .into_iter()
      .map(|cc| Py::new(py, CoreConsumerConfig::try_from_core(cc, py)?))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self { consumers })
  }
}

#[pyclass]
pub struct CoreConsumerConfig {
  #[pyo3(get)]
  consumer_name: String,
  #[pyo3(get)]
  source_id: String,
  #[pyo3(get)]
  batch_size: Option<i64>,
  #[pyo3(get)]
  visibility_timeout: Option<i64>,
  #[pyo3(get)]
  wait_time_seconds: Option<i64>,
  #[pyo3(get)]
  partial_failures: Option<bool>,
  #[pyo3(get)]
  routing_key: Option<String>,
  #[pyo3(get)]
  handlers: Vec<Py<CoreEventHandlerDefinition>>,
}

impl CoreConsumerConfig {
  fn try_from_core(c: ConsumerConfig, py: Python) -> PyResult<Self> {
    let handlers = c
      .handlers
      .into_iter()
      .map(|h| Py::new(py, CoreEventHandlerDefinition::from(h)))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self {
      consumer_name: c.consumer_name,
      source_id: c.source_id,
      batch_size: c.batch_size,
      visibility_timeout: c.visibility_timeout,
      wait_time_seconds: c.wait_time_seconds,
      partial_failures: c.partial_failures,
      routing_key: c.routing_key,
      handlers,
    })
  }
}

#[pyclass]
pub struct CoreEventHandlerDefinition {
  #[pyo3(get)]
  name: String,
  #[pyo3(get)]
  location: String,
  #[pyo3(get)]
  handler: String,
  #[pyo3(get)]
  timeout: i64,
  #[pyo3(get)]
  tracing_enabled: bool,
  #[pyo3(get)]
  route: Option<String>,
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

// ---------------------------------------------------------------------------
// Schedule config types
// ---------------------------------------------------------------------------

#[pyclass]
pub struct CoreSchedulesConfig {
  #[pyo3(get)]
  schedules: Vec<Py<CoreScheduleConfig>>,
}

impl CoreSchedulesConfig {
  fn try_from_core(s: SchedulesConfig, py: Python) -> PyResult<Self> {
    let schedules = s
      .schedules
      .into_iter()
      .map(|sc| Py::new(py, CoreScheduleConfig::try_from_core(sc, py)?))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self { schedules })
  }
}

#[pyclass]
pub struct CoreScheduleConfig {
  #[pyo3(get)]
  schedule_id: String,
  #[pyo3(get)]
  schedule_value: String,
  #[pyo3(get)]
  handlers: Vec<Py<CoreEventHandlerDefinition>>,
  #[pyo3(get)]
  input: Option<Py<PyAny>>,
}

impl CoreScheduleConfig {
  fn try_from_core(s: ScheduleConfig, py: Python) -> PyResult<Self> {
    let input = s
      .input
      .map(|v| pythonize(py, &v).map(|bound| bound.unbind()))
      .transpose()?;
    let handlers = s
      .handlers
      .into_iter()
      .map(|h| Py::new(py, CoreEventHandlerDefinition::from(h)))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self {
      schedule_id: s.schedule_id,
      schedule_value: s.schedule_value,
      handlers,
      input,
    })
  }
}

// ---------------------------------------------------------------------------
// Custom handler config types
// ---------------------------------------------------------------------------

#[pyclass]
pub struct CoreCustomHandlersConfig {
  #[pyo3(get)]
  handlers: Vec<Py<CoreCustomHandlerDefinition>>,
}

impl CoreCustomHandlersConfig {
  fn try_from_core(ch: CustomHandlersConfig, py: Python) -> PyResult<Self> {
    let handlers = ch
      .handlers
      .into_iter()
      .map(|h| Py::new(py, CoreCustomHandlerDefinition::from(h)))
      .collect::<PyResult<Vec<_>>>()?;
    Ok(Self { handlers })
  }
}

#[pyclass]
pub struct CoreCustomHandlerDefinition {
  #[pyo3(get)]
  name: String,
  #[pyo3(get)]
  location: String,
  #[pyo3(get)]
  handler: String,
  #[pyo3(get)]
  timeout: i64,
  #[pyo3(get)]
  tracing_enabled: bool,
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

#[pyclass]
struct CoreRuntimeApplication {
  inner: Arc<Mutex<Application>>,
  task_locals: Option<pyo3_async_runtimes::TaskLocals>,
  py_rx: Option<mpsc::UnboundedReceiver<PythonCall>>,
  py_tx: Option<mpsc::UnboundedSender<PythonCall>>,
  handler_registry: Arc<TokioMutex<HashMap<String, Py<PyAny>>>>,
  consumer_handler_builder: consumer::PyConsumerEventHandlerBuilder,
  pending_guards: Vec<(String, guard::PyAuthGuardHandler)>,
}

impl CoreRuntimeApplication {
  fn lock_inner(&self) -> PyResult<std::sync::MutexGuard<'_, Application>> {
    self.inner.lock().map_err(|e| {
      PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(format!(
        "failed to acquire application lock: {e}"
      ))
    })
  }
}

#[pymethods]
impl CoreRuntimeApplication {
  #[new]
  fn new(runtime_config: PyRef<CoreRuntimeConfig>) -> PyResult<Self> {
    let diagnostics_level = Level::from_str(&runtime_config.runtime_max_diagnostics_level)
      .map_err(|_| {
        PyErr::new::<pyo3::exceptions::PyValueError, _>(format!(
          "invalid tracing level '{}'",
          runtime_config.runtime_max_diagnostics_level
        ))
      })?;

    let client_ip_source = runtime_config
      .client_ip_source
      .as_deref()
      .unwrap_or("ConnectInfo")
      .parse::<ClientIpSource>()
      .unwrap_or(ClientIpSource::ConnectInfo);

    let log_format = runtime_config
      .log_format
      .clone()
      .or_else(|| std::env::var("CELERITY_LOG_FORMAT").ok());

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
      runtime_max_diagnostics_level: diagnostics_level,
      test_mode: runtime_config.test_mode,
      api_resource: runtime_config.api_resource.clone(),
      consumer_app: runtime_config.consumer_app.clone(),
      schedule_app: runtime_config.schedule_app.clone(),
      resource_store_verify_tls: runtime_config.resource_store_verify_tls,
      resource_store_cache_entry_ttl: runtime_config.resource_store_cache_entry_ttl,
      resource_store_cleanup_interval: runtime_config.resource_store_cleanup_interval,
      client_ip_source,
      log_format,
      metrics_enabled: runtime_config.metrics_enabled,
      trace_sample_ratio: runtime_config.trace_sample_ratio,
    };
    println!("Creating CoreRuntimeApplication with config: {native_runtime_config:?}");
    let inner = Application::new(native_runtime_config, Box::new(ProcessEnvVars::new()));
    Ok(CoreRuntimeApplication {
      inner: Arc::new(Mutex::new(inner)),
      task_locals: None,
      py_rx: None,
      py_tx: None,
      handler_registry: Arc::new(TokioMutex::new(HashMap::new())),
      consumer_handler_builder: consumer::PyConsumerEventHandlerBuilder::new(),
      pending_guards: Vec::new(),
    })
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
    CoreRuntimeAppConfig::try_from_core(app_config)
  }

  #[pyo3(signature = (path, method, handler, timeout_seconds=None))]
  fn register_http_handler(
    &mut self,
    path: String,
    method: String,
    handler: Py<PyAny>,
    timeout_seconds: Option<i64>,
  ) -> PyResult<()> {
    let handler_id = format!("{path}::{method}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }

    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    let py_tx = self
      .py_tx
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(
          "setup() must be called before registering handlers",
        )
      })?
      .clone();
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
        let auth_context_opt = req
          .extensions()
          .get::<AuthContext>()
          .and_then(|ac| ac.0.clone());
        let user_agent = req
          .extensions()
          .get::<ResolvedUserAgent>()
          .map(|ua| ua.0.clone())
          .unwrap_or_default();
        let client_ip = req
          .extensions()
          .get::<ResolvedClientIp>()
          .map(|rci| rci.0.to_string())
          .unwrap_or_default();
        let matched_route = req
          .extensions()
          .get::<MatchedRoute>()
          .map(|mr| mr.0.clone());

        let (mut parts, body) = req.into_parts();
        let body_bytes = axum::body::to_bytes(body, MAX_REQUEST_BODY_SIZE)
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
              user_agent,
            },
          )
        })
        .map_err(|err| HandlerError::new(err.to_string()))?;

        let py_req_ctx = Python::with_gil(|py| {
          let auth = match &auth_context_opt {
            Some(auth_context) => pythonize(py, auth_context)
              .map(|bound| bound.unbind())
              .unwrap_or_else(|_| Python::None(py)),
            None => Python::None(py),
          };
          Py::new(
            py,
            PyRequestContext {
              request_id,
              request_time: chrono::Utc::now(),
              auth,
              trace_context: extract_trace_context(),
              client_ip,
              matched_route,
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

        let sleep = time::sleep(Duration::from_secs(timeout_secs));
        tokio::select! {
          _ = sleep => {
            Err(HandlerError::timeout())
          }
          result = response_rx => {
            let py_obj = result
              .map_err(|_| HandlerError::new("Python worker dropped".to_string()))?
              .map_err(|e| HandlerError::new(format!("Python error: {e}")))?;
            Python::with_gil(|py| py_obj.extract::<PyResponse>(py))
              .map_err(|e| HandlerError::new(e.to_string()))
          }
        }
      }
    };

    self
      .lock_inner()?
      .register_http_handler(&path, &method, final_handler);

    Ok(())
  }

  fn register_websocket_handler(&mut self, route: String, handler: Py<PyAny>) -> PyResult<()> {
    let handler_id = format!("websocket::{route}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }

    let py_tx = self
      .py_tx
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(
          "setup() must be called before registering handlers",
        )
      })?
      .clone();
    let final_handler = WSBindingMessageHandler { handler_id, py_tx };

    self
      .lock_inner()?
      .register_websocket_message_handler(&route, final_handler);

    Ok(())
  }

  #[pyo3(signature = (handler_tag, handler, timeout_seconds=None))]
  fn register_consumer_handler(
    &mut self,
    handler_tag: String,
    handler: Py<PyAny>,
    timeout_seconds: Option<i64>,
  ) -> PyResult<()> {
    let handler_id = format!("consumer::{handler_tag}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id, handler);
    }
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    self
      .consumer_handler_builder
      .add_consumer_handler(handler_tag, timeout_secs);
    Ok(())
  }

  #[pyo3(signature = (handler_tag, handler, timeout_seconds=None))]
  fn register_schedule_handler(
    &mut self,
    handler_tag: String,
    handler: Py<PyAny>,
    timeout_seconds: Option<i64>,
  ) -> PyResult<()> {
    let handler_id = format!("schedule::{handler_tag}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id, handler);
    }
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    self
      .consumer_handler_builder
      .add_schedule_handler(handler_tag, timeout_secs);
    Ok(())
  }

  fn register_guard_handler(&mut self, name: String, handler: Py<PyAny>) -> PyResult<()> {
    let handler_id = format!("guard::{name}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }
    let py_tx = self
      .py_tx
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(
          "setup() must be called before registering handlers",
        )
      })?
      .clone();
    let guard_handler = guard::PyAuthGuardHandler::new(handler_id, py_tx);
    self.pending_guards.push((name, guard_handler));
    Ok(())
  }

  #[pyo3(signature = (handler_name, handler, timeout_seconds=None))]
  fn register_custom_handler(
    &mut self,
    handler_name: String,
    handler: Py<PyAny>,
    timeout_seconds: Option<i64>,
  ) -> PyResult<()> {
    let handler_id = format!("custom::{handler_name}");
    {
      let mut registry = self.handler_registry.blocking_lock();
      registry.insert(handler_id.clone(), handler);
    }
    let timeout_secs = timeout_seconds.unwrap_or(60) as u64;
    let py_tx = self
      .py_tx
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(
          "setup() must be called before registering handlers",
        )
      })?
      .clone();
    let invoker = invoke::PyHandlerInvoker::new(handler_id, py_tx, timeout_secs);
    self
      .lock_inner()?
      .register_handler_invoker(handler_name, Arc::new(invoker));
    Ok(())
  }

  fn websocket_registry(&self, py: Python) -> PyResult<Py<WSBindingRegistrySend>> {
    Py::new(
      py,
      WSBindingRegistrySend {
        inner: self.lock_inner()?.websocket_registry(),
      },
    )
  }

  fn shutdown(&mut self) -> PyResult<()> {
    self.lock_inner()?.shutdown();
    self.py_tx.take();
    Ok(())
  }

  // SAFETY: run can hold a std mutex lock across an await boundary as there will be no other
  // threads/tasks trying to obtain a lock on the application for the duration of the await
  // that runs the application.
  // Locks are only held on the inner core runtime application
  // for setup and handler registration which must always be called before run.
  #[allow(clippy::await_holding_lock)]
  #[pyo3(signature = (block=true))]
  fn run(&mut self, py: Python, block: bool) -> PyResult<()> {
    let inner = self.inner.clone();
    let handler_registry = self.handler_registry.clone();
    let py_rx = self.py_rx.take().ok_or_else(|| {
      PyErr::new::<pyo3::exceptions::PyRuntimeError, _>(
        "run() must be called after setup() and only once",
      )
    })?;
    let py_tx = self
      .py_tx
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>("setup() must be called before run()")
      })?
      .clone();
    let task_locals = self
      .task_locals
      .as_ref()
      .ok_or_else(|| {
        PyErr::new::<pyo3::exceptions::PyRuntimeError, _>("setup() must be called before run()")
      })?
      .clone_ref(py);

    // Take the consumer handler builder so it can be finalized in the runtime thread.
    let consumer_builder = std::mem::take(&mut self.consumer_handler_builder);

    // Take pending guard registrations for deferred async registration.
    let pending_guards = std::mem::take(&mut self.pending_guards);

    thread::spawn(move || {
      let rt = runtime::new_tokio_multi_thread().unwrap_or_else(|e| {
        eprintln!("fatal: failed to create tokio runtime: {e}");
        abort();
      });
      rt.block_on(async move {
        tokio::spawn(pyo3_async_runtimes::tokio::scope(
          task_locals,
          python_worker(py_rx, handler_registry),
        ));

        // Register pending guard handlers (requires async context).
        for (name, guard_handler) in pending_guards {
          inner
            .lock()
            .unwrap_or_else(|e| {
              eprintln!("fatal: application lock poisoned: {e}");
              abort();
            })
            .register_custom_auth_guard(&name, guard_handler)
            .await;
        }

        // Finalize and register the consumer event handler if any were registered.
        if !consumer_builder.is_empty() {
          let handler = consumer_builder.build(py_tx);
          inner
            .lock()
            .unwrap_or_else(|e| {
              eprintln!("fatal: application lock poisoned: {e}");
              abort();
            })
            .register_consumer_handler(Arc::new(handler));
        }

        match inner
          .lock()
          .unwrap_or_else(|e| {
            eprintln!("fatal: application lock poisoned: {e}");
            abort();
          })
          .run(true)
          .await
        {
          Ok(_) => {}
          Err(err) => {
            eprintln!("fatal: error running core runtime: {err}");
            abort();
          }
        }
      });
    });

    if block {
      let event_loop = self
        .task_locals
        .as_ref()
        .ok_or_else(|| {
          PyErr::new::<pyo3::exceptions::PyRuntimeError, _>("setup() must be called before run()")
        })?
        .event_loop(py);
      let run_forever_res = event_loop.call_method0("run_forever");
      if run_forever_res.is_err() {
        println!("Ctrl C pressed, shutting down...");
        abort();
      }
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
  m.add_class::<CoreRuntimeAppConfig>()?;
  m.add_class::<CoreApiConfig>()?;
  m.add_class::<CoreHttpConfig>()?;
  m.add_class::<CoreHttpHandlerDefinition>()?;
  m.add_class::<CoreGuardsConfig>()?;
  m.add_class::<CoreGuardHandlerDefinition>()?;
  m.add_class::<CoreConsumersConfig>()?;
  m.add_class::<CoreConsumerConfig>()?;
  m.add_class::<CoreEventHandlerDefinition>()?;
  m.add_class::<CoreSchedulesConfig>()?;
  m.add_class::<CoreScheduleConfig>()?;
  m.add_class::<CoreCustomHandlersConfig>()?;
  m.add_class::<CoreCustomHandlerDefinition>()?;
  m.add_class::<consumer::PyConsumerEventInput>()?;
  m.add_class::<consumer::PyConsumerMessage>()?;
  m.add_class::<consumer::PyScheduleEventInput>()?;
  m.add_class::<consumer::PyEventResult>()?;
  m.add_class::<consumer::PyMessageProcessingFailure>()?;
  m.add_class::<guard::PyGuardInput>()?;
  m.add_class::<guard::PyGuardRequestInfo>()?;
  m.add_class::<guard::PyGuardResult>()?;
  m.add_class::<HttpProtocolVersion>()?;
  Ok(())
}
