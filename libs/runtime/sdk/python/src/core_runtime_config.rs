use celerity_helpers::{env::ProcessEnvVars, runtime_types::RuntimePlatform};
use celerity_runtime_core::{
  config::RuntimeConfig,
  consts::{
    DEFAULT_RESOURCE_STORE_CACHE_ENTRY_TTL, DEFAULT_RESOURCE_STORE_CLEANUP_INTERVAL,
    DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT,
  },
};
use pyo3::{prelude::*, pyclass, pymethods, Python};

#[pyclass(eq, eq_int, name = "RuntimePlatform")]
#[derive(PartialEq, Clone)]
pub enum CoreRuntimePlatform {
  #[pyo3(name = "AWS")]
  Aws,
  #[pyo3(name = "AZURE")]
  Azure,
  #[pyo3(name = "GCP")]
  Gcp,
  #[pyo3(name = "LOCAL")]
  Local,
  #[pyo3(name = "OTHER")]
  Other,
}

impl From<RuntimePlatform> for CoreRuntimePlatform {
  fn from(platform: RuntimePlatform) -> Self {
    match platform {
      RuntimePlatform::AWS => CoreRuntimePlatform::Aws,
      RuntimePlatform::Azure => CoreRuntimePlatform::Azure,
      RuntimePlatform::GCP => CoreRuntimePlatform::Gcp,
      RuntimePlatform::Local => CoreRuntimePlatform::Local,
      RuntimePlatform::Other => CoreRuntimePlatform::Other,
    }
  }
}

impl From<CoreRuntimePlatform> for RuntimePlatform {
  fn from(platform: CoreRuntimePlatform) -> Self {
    match platform {
      CoreRuntimePlatform::Aws => RuntimePlatform::AWS,
      CoreRuntimePlatform::Azure => RuntimePlatform::Azure,
      CoreRuntimePlatform::Gcp => RuntimePlatform::GCP,
      CoreRuntimePlatform::Local => RuntimePlatform::Local,
      CoreRuntimePlatform::Other => RuntimePlatform::Other,
    }
  }
}

#[pyclass]
pub struct CoreRuntimeConfig {
  #[pyo3(get)]
  pub blueprint_config_path: String,
  #[pyo3(get)]
  pub service_name: String,
  #[pyo3(get)]
  pub server_port: i32,
  #[pyo3(get)]
  pub server_loopback_only: Option<bool>,
  #[pyo3(get)]
  pub use_custom_health_check: Option<bool>,
  #[pyo3(get)]
  pub trace_otlp_collector_endpoint: String,
  #[pyo3(get)]
  pub runtime_max_diagnostics_level: String,
  #[pyo3(get)]
  pub platform: CoreRuntimePlatform,
  #[pyo3(get)]
  pub test_mode: bool,
  #[pyo3(get)]
  pub api_resource: Option<String>,
  #[pyo3(get)]
  pub consumer_app: Option<String>,
  #[pyo3(get)]
  pub schedule_app: Option<String>,
  #[pyo3(get)]
  pub resource_store_verify_tls: bool,
  #[pyo3(get)]
  pub resource_store_cache_entry_ttl: i64,
  #[pyo3(get)]
  pub resource_store_cleanup_interval: i64,
}

#[pymethods]
impl CoreRuntimeConfig {
  #[staticmethod]
  fn from_env(py: Python) -> PyResult<Py<CoreRuntimeConfig>> {
    let env = ProcessEnvVars::new();
    let runtime_config = RuntimeConfig::from_env(&env);
    Py::new(py, CoreRuntimeConfig::from(runtime_config))
  }
}

struct InternalCoreRuntimeConfig {
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
}

#[pyclass]
pub struct CoreRuntimeConfigBuilder {
  core_runtime_config: InternalCoreRuntimeConfig,
}

#[pymethods]
impl CoreRuntimeConfigBuilder {
  #[new]
  fn new(blueprint_config_path: String, service_name: String, server_port: i32) -> Self {
    Self {
      core_runtime_config: InternalCoreRuntimeConfig {
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
      },
    }
  }

  fn set_server_loopback_only(mut self_: PyRefMut<Self>, server_loopback_only: bool) -> Py<Self> {
    self_.core_runtime_config.server_loopback_only = Some(server_loopback_only);
    self_.into()
  }

  fn set_use_custom_health_check(
    mut self_: PyRefMut<Self>,
    use_custom_health_check: bool,
  ) -> Py<Self> {
    self_.core_runtime_config.use_custom_health_check = Some(use_custom_health_check);
    self_.into()
  }

  fn set_trace_otlp_collector_endpoint(
    mut self_: PyRefMut<Self>,
    trace_otlp_collector_endpoint: String,
  ) -> Py<Self> {
    self_.core_runtime_config.trace_otlp_collector_endpoint = Some(trace_otlp_collector_endpoint);
    self_.into()
  }

  fn set_runtime_max_diagnostics_level(
    mut self_: PyRefMut<Self>,
    runtime_max_diagnostics_level: String,
  ) -> Py<Self> {
    self_.core_runtime_config.runtime_max_diagnostics_level = Some(runtime_max_diagnostics_level);
    self_.into()
  }

  fn set_platform(mut self_: PyRefMut<Self>, platform: CoreRuntimePlatform) -> Py<Self> {
    self_.core_runtime_config.platform = Some(platform);
    self_.into()
  }

  fn set_test_mode(mut self_: PyRefMut<Self>, test_mode: bool) -> Py<Self> {
    self_.core_runtime_config.test_mode = Some(test_mode);
    self_.into()
  }

  fn set_api_resource(mut self_: PyRefMut<Self>, api_resource: String) -> Py<Self> {
    self_.core_runtime_config.api_resource = Some(api_resource);
    self_.into()
  }

  fn set_consumer_app(mut self_: PyRefMut<Self>, consumer_app: String) -> Py<Self> {
    self_.core_runtime_config.consumer_app = Some(consumer_app);
    self_.into()
  }

  fn set_schedule_app(mut self_: PyRefMut<Self>, schedule_app: String) -> Py<Self> {
    self_.core_runtime_config.schedule_app = Some(schedule_app);
    self_.into()
  }

  fn set_resource_store_verify_tls(
    mut self_: PyRefMut<Self>,
    resource_store_verify_tls: bool,
  ) -> Py<Self> {
    self_.core_runtime_config.resource_store_verify_tls = Some(resource_store_verify_tls);
    self_.into()
  }

  fn set_resource_store_cache_entry_ttl(
    mut self_: PyRefMut<Self>,
    resource_store_cache_entry_ttl: i64,
  ) -> Py<Self> {
    self_.core_runtime_config.resource_store_cache_entry_ttl = Some(resource_store_cache_entry_ttl);
    self_.into()
  }

  fn set_resource_store_cleanup_interval(
    mut self_: PyRefMut<Self>,
    resource_store_cleanup_interval: i64,
  ) -> Py<Self> {
    self_.core_runtime_config.resource_store_cleanup_interval =
      Some(resource_store_cleanup_interval);
    self_.into()
  }

  fn build(&self, py: Python) -> PyResult<Py<CoreRuntimeConfig>> {
    let runtime_config = CoreRuntimeConfig {
      blueprint_config_path: self.core_runtime_config.blueprint_config_path.clone(),
      service_name: self.core_runtime_config.service_name.clone(),
      server_port: self.core_runtime_config.server_port,
      server_loopback_only: self.core_runtime_config.server_loopback_only,
      use_custom_health_check: self.core_runtime_config.use_custom_health_check,
      trace_otlp_collector_endpoint: self
        .core_runtime_config
        .trace_otlp_collector_endpoint
        .clone()
        .unwrap_or_else(|| DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT.to_string()),
      runtime_max_diagnostics_level: self
        .core_runtime_config
        .runtime_max_diagnostics_level
        .clone()
        .unwrap_or_else(|| "info".to_string()),
      platform: self
        .core_runtime_config
        .platform
        .clone()
        .unwrap_or(CoreRuntimePlatform::Other),
      test_mode: self.core_runtime_config.test_mode.unwrap_or(false),
      api_resource: self.core_runtime_config.api_resource.clone(),
      consumer_app: self.core_runtime_config.consumer_app.clone(),
      schedule_app: self.core_runtime_config.schedule_app.clone(),
      resource_store_verify_tls: self
        .core_runtime_config
        .resource_store_verify_tls
        .unwrap_or(true),
      resource_store_cache_entry_ttl: self
        .core_runtime_config
        .resource_store_cache_entry_ttl
        .unwrap_or(DEFAULT_RESOURCE_STORE_CACHE_ENTRY_TTL),
      resource_store_cleanup_interval: self
        .core_runtime_config
        .resource_store_cleanup_interval
        .unwrap_or(DEFAULT_RESOURCE_STORE_CLEANUP_INTERVAL),
    };
    Py::new(py, runtime_config)
  }
}

impl From<RuntimeConfig> for CoreRuntimeConfig {
  fn from(runtime_config: RuntimeConfig) -> Self {
    Self {
      blueprint_config_path: runtime_config.blueprint_config_path,
      server_port: runtime_config.server_port,
      service_name: runtime_config.service_name,
      server_loopback_only: runtime_config.server_loopback_only,
      use_custom_health_check: runtime_config.use_custom_health_check,
      trace_otlp_collector_endpoint: runtime_config.trace_otlp_collector_endpoint,
      runtime_max_diagnostics_level: runtime_config.runtime_max_diagnostics_level.to_string(),
      platform: runtime_config.platform.into(),
      test_mode: runtime_config.test_mode,
      api_resource: runtime_config.api_resource,
      consumer_app: runtime_config.consumer_app,
      schedule_app: runtime_config.schedule_app,
      resource_store_verify_tls: runtime_config.resource_store_verify_tls,
      resource_store_cache_entry_ttl: runtime_config.resource_store_cache_entry_ttl,
      resource_store_cleanup_interval: runtime_config.resource_store_cleanup_interval,
    }
  }
}
