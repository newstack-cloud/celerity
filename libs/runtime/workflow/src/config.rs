use celerity_blueprint_config_parser::blueprint::CelerityWorkflowSpec;
use celerity_helpers::runtime_types::{RuntimeCallMode, RuntimePlatform};
use tracing::Level;

/// Workflow runtime configuration
/// that is used to locate blueprint files
/// and determine how to set up an application.
#[derive(Debug)]
pub struct WorkflowRuntimeConfig {
    pub blueprint_config_path: String,
    pub runtime_call_mode: RuntimeCallMode,
    /// The name of the service that will be used for tracing
    /// and logs.
    pub service_name: String,
    pub server_port: i32,
    /// Optional flag to determine whether the
    /// HTTP server for the workflow should only be exposed
    /// on the loopback interface (127.0.0.1).
    ///
    /// When running in an environment such as a docker
    /// container, this should be set to false
    /// so that the server can be accessed from outside
    /// the container.
    ///
    /// Defaults to true.
    pub server_loopback_only: Option<bool>,
    /// The port on which the local HTTP API server
    /// should run.
    /// This is only used when the runtime call mode
    /// is set to `RuntimeCallMode::Http`.
    pub local_api_port: i32,
    /// Sets the endpoint to be used for sending trace data to an OTLP collector.
    ///
    /// Defaults to "http://otelcollector:4317".
    /// The default value assumes the common use case of running the OpenTelemetry Collector
    /// in a sidecar container named "otelcollector" in the same container network as the runtime.
    pub trace_otlp_collector_endpoint: String,
    /// The maximum diagnostics level that the runtime should use for logging and tracing.
    /// This is used to control the verbosity of exported/captured traces and events
    /// in the runtime.
    pub runtime_max_diagnostics_level: Level,
    /// The platform the application hosted by the runtime is running on.
    /// This is essential in determining which features are available in the current environment.
    /// For example, if the runtime platform is AWS, the runtime can set up telemetry to use an
    /// AWS X-Ray propagator to enrich traces and events with AWS-specific trace IDs.
    ///
    /// Defaults to `RuntimePlatform::Other`.
    pub platform: RuntimePlatform,
    /// Whether the runtime is running in test mode (e.g. integration tests).
    ///
    /// Defaults to false.
    pub test_mode: bool,
}

#[derive(Debug, Clone)]
pub struct WorkflowAppConfig {
    pub state_handlers: Option<Vec<StateHandlerDefinition>>,
    pub workflow: CelerityWorkflowSpec,
}

#[derive(Debug, Default, Clone)]
pub struct StateHandlerDefinition {
    pub state: String,
    pub name: String,
    pub location: String,
    pub handler: String,
    /// Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}
