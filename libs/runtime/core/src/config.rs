use std::str::FromStr;

use celerity_blueprint_config_parser::blueprint::{CelerityApiAuth, CelerityApiCors};
use tracing::Level;

use crate::{
    consts::{DEFAULT_LOCAL_API_PORT, DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT},
    env::EnvVars,
};

/// Core runtime configuration
/// that is used to locate blueprint files
/// and determine how to set up an application.
#[derive(Debug)]
pub struct RuntimeConfig {
    pub blueprint_config_path: String,
    pub runtime_call_mode: RuntimeCallMode,
    /// The name of the service that will be used for tracing
    /// and logs.
    pub service_name: String,
    pub server_port: i32,
    /// Optional flag to determine whether the
    /// HTTP/WebSocket server should only be exposed
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
    /// Set to true if one of your handlers defines a custom health check endpoint.
    ///
    /// Defaults to false.
    /// The `GET /runtime/health/check` endpoint is set by the runtime
    /// to return a 200 OK status code when this is set to false.
    /// The default health check is not accessible under custom base paths
    /// defined for an API, and is only accessible from the root path.
    /// The health check endpoint exists to be called directly by a
    /// container/machine orchestrator service that has direct access
    /// to the instance of the runtime API via the exposed container port.
    pub use_custom_health_check: Option<bool>,
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

impl RuntimeConfig {
    /// Creates a new instance of runtime configuration,
    /// sourcing config from the current process environment
    /// variables.
    pub fn from_env(env: &impl EnvVars) -> Self {
        let blueprint_config_path = env
            .var("CELERITY_BLUEPRINT")
            .expect("Missing blueprint path");

        let runtime_call_mode = env
            .var("CELERITY_RUNTIME_CALL_MODE")
            .expect("Missing runtime call mode");

        let runtime_call_mode = match runtime_call_mode.as_str() {
            "ffi" => RuntimeCallMode::Ffi,
            "http" => RuntimeCallMode::Http,
            _ => panic!("Invalid runtime call mode, must be one of 'ffi' or 'http'"),
        };

        let service_name = env
            .var("CELERITY_SERVICE_NAME")
            .expect("Missing service name");

        let server_port = env
            .var("CELERITY_SERVER_PORT")
            .unwrap()
            .parse()
            .expect("Invalid server port, must be a valid integer");

        let server_loopback_only_env_var = env
            .var("CELERITY_SERVER_LOOPBACK_ONLY")
            .map(Some)
            .unwrap_or_else(|_| None);
        let server_loopback_only = server_loopback_only_env_var.map(|val| {
            val.parse().expect(
                "Invalid server loopback only value, must be either \\\"true\\\" or \\\"false\\\"",
            )
        });

        let local_api_port = env
            .var("CELERITY_LOCAL_API_PORT")
            .unwrap_or_else(|_| DEFAULT_LOCAL_API_PORT.to_string())
            .parse()
            .expect("Invalid local API port, must be a valid integer");

        let use_custom_health_check_env_var = env
            .var("CELERITY_USE_CUSTOM_HEALTH_CHECK")
            .map(Some)
            .unwrap_or_else(|_| None);
        let use_custom_health_check = use_custom_health_check_env_var.map(|val| {
            val.parse().expect(
                "Invalid use custom health check value, must be either \\\"true\\\" or \\\"false\\\"",
            )
        });

        let trace_otlp_collector_endpoint = env
            .var("CELERITY_TRACE_OTLP_COLLECTOR_ENDPOINT")
            .unwrap_or_else(|_| DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT.to_string());

        let runtime_max_diagnostics_level_env_var = env
            .var("CELERITY_MAX_DIAGNOSTICS_LEVEL")
            .map(|level| level)
            .unwrap_or_else(|_| "info".to_string());
        let runtime_max_diagnostics_level =
            Level::from_str(runtime_max_diagnostics_level_env_var.as_str())
                .expect("Invalid runtime max diagnostics level");

        let platform = env.var("CELERITY_RUNTIME_PLATFORM").unwrap();
        let platform = match platform.as_str() {
            "aws" => RuntimePlatform::AWS,
            "azure" => RuntimePlatform::Azure,
            "gcp" => RuntimePlatform::GCP,
            "local" => RuntimePlatform::Local,
            _ => RuntimePlatform::Other,
        };

        let test_mode = env
            .var("CELERITY_TEST_MODE")
            .map(|val| {
                val.parse()
                    .expect("Invalid test mode value, must be either \\\"true\\\" or \\\"false\\\"")
            })
            .unwrap_or(false);

        RuntimeConfig {
            blueprint_config_path,
            runtime_call_mode,
            service_name,
            server_port,
            server_loopback_only,
            local_api_port,
            use_custom_health_check,
            trace_otlp_collector_endpoint,
            runtime_max_diagnostics_level,
            platform,
            test_mode,
        }
    }
}

/// Determines the mode in which the runtime interacts
/// with handlers.
#[derive(Debug, PartialEq)]
pub enum RuntimeCallMode {
    // FFI mode, where the runtime calls into a handler
    // via a foreign function interface.
    Ffi,
    // HTTP mode, where the runtime exposes a HTTP API on localhost
    // on a port that must not be exposed outside of the container
    // or host machine.
    // The HTTP API allows the handlers to retrieve the latest message/request
    // from the runtime and send a response back to the runtime.
    // This mode is useful for languages that are compiled ahead of time
    // such as Go, Rust, C and C++.
    Http,
}

/// The platform that the runtime hosted application is running on.
#[derive(Debug, Clone, PartialEq)]
pub enum RuntimePlatform {
    AWS,
    Azure,
    GCP,
    Local,
    Other,
}

#[derive(Debug)]
pub struct AppConfig {
    pub api: Option<ApiConfig>,
    pub consumers: Option<ConsumersConfig>,
    pub schedules: Option<SchedulesConfig>,
    pub events: Option<EventsConfig>,
}

#[derive(Debug)]
pub struct ApiConfig {
    pub http: Option<HttpConfig>,
    pub websocket: Option<WebSocketConfig>,
    pub auth: Option<CelerityApiAuth>,
    pub cors: Option<CelerityApiCors>,
    pub tracing_enabled: bool,
}

#[derive(Debug)]
pub struct HttpConfig {
    pub handlers: Vec<HttpHandlerDefinition>,
    // Base paths are used by the runtime to only route requests
    // with a certain base path prefix to the HTTP API in a hybrid API
    // context.
    pub base_paths: Vec<String>,
}

#[derive(Debug, Clone, Default)]
pub struct HttpHandlerDefinition {
    pub name: String,
    pub path: String,
    pub method: String,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}

#[derive(Debug)]
pub struct WebSocketConfig {
    pub handlers: Vec<WebSocketHandlerDefinition>,
    // Base paths are used by the runtime to only route requests
    // with a certain base path prefix to the WebSocket API in a hybrid API
    // context.
    pub base_paths: Vec<String>,
    pub route_key: String,
}

#[derive(Debug, Default)]
pub struct WebSocketHandlerDefinition {
    pub name: String,
    pub route_key: String,
    pub route: String,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}

#[derive(Debug)]
pub struct ConsumersConfig {
    pub consumers: Vec<ConsumerConfig>,
}

#[derive(Debug)]
pub struct ConsumerConfig {
    pub source_id: String,
    // Depending on the deployment environment,
    // this may be overridden if the provided
    // value is not within the allowed range.
    pub batch_size: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub visibility_timeout: Option<i64>,
    pub wait_time_seconds: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub partial_failures: Option<bool>,
    pub handlers: Vec<EventHandlerDefinition>,
}

#[derive(Debug, Default)]
pub struct EventHandlerDefinition {
    pub name: String,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}

#[derive(Debug)]
pub struct SchedulesConfig {
    pub schedules: Vec<ScheduleConfig>,
}

#[derive(Debug)]
pub struct ScheduleConfig {
    // The schedule ID provided in messages polled from the
    // schedule message queue.
    pub schedule_id: String,
    // The schedule in cron or rate format as per the original
    // in the blueprint.
    // This is used for debugging purposes in the runtime.
    pub schedule_value: String,
    // The ID or URL of the queue to which scheduled messages
    // are sent.
    pub queue_id: String,
    // Depending on the deployment environment,
    // this may be overridden if the provided
    // value is not within the allowed range.
    pub batch_size: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub visibility_timeout: Option<i64>,
    pub wait_time_seconds: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub partial_failures: Option<bool>,
    pub handlers: Vec<EventHandlerDefinition>,
}

#[derive(Debug)]
pub struct EventsConfig {
    pub events: Vec<EventConfig>,
}

#[derive(Debug)]
pub enum EventConfig {
    // An event trigger (e.g. file uploaded to Amazon S3)
    EventTrigger(EventTriggerConfig),
    // A stream of events or data into the runtime.
    Stream(StreamConfig),
}

#[derive(Debug)]
pub struct EventTriggerConfig {
    // The event type provided in messages polled from the
    // events message queue.
    pub event_type: String,
    // The ID or URL of the queue from which event messages
    // are consumed.
    pub queue_id: String,
    // Depending on the deployment environment,
    // this may be overridden if the provided
    // value is not within the allowed range.
    pub batch_size: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub visibility_timeout: Option<i64>,
    pub wait_time_seconds: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub partial_failures: Option<bool>,
    pub handlers: Vec<EventHandlerDefinition>,
}

#[derive(Debug)]
pub struct StreamConfig {
    // The ID of the stream from which event messages
    // are consumed.
    pub stream_id: String,
    // Depending on the deployment environment,
    // this may be overridden if the provided
    // value is not within the allowed range.
    pub batch_size: Option<i64>,
    // Depending on the deployment environment,
    // this may not be used.
    pub partial_failures: Option<bool>,
    pub handlers: Vec<EventHandlerDefinition>,
}
