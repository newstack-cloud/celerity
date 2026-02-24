use std::str::FromStr;

pub use axum_client_ip::ClientIpSource;
use celerity_blueprint_config_parser::blueprint::{
    CelerityApiAuth, CelerityApiBasePath, CelerityApiCors, WebSocketAuthStrategy,
};
use celerity_helpers::{
    env::EnvVars,
    runtime_types::{RuntimeCallMode, RuntimePlatform},
};
use serde_json::Value;
use tracing::Level;

use crate::consts::DEFAULT_LOCAL_API_PORT;

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
    /// The name of the API resource in the blueprint
    /// that should be used as the configuration source for setting
    /// up API configuration and endpoints.
    pub api_resource: Option<String>,
    /// The name of the consumer app in the blueprint
    /// that should be used as the configuration source for setting
    /// up webhook endpoints (for push model message sources) or a polling
    /// consumer (for pull model message sources).
    /// This will be either a shared `celerity.app` annotation shared by
    /// multiple consumers that are part of the same application or the name
    /// an individual `celerity/consumer` resource in the blueprint.
    /// If not set, the runtime will use the first `celerity/consumer` resource
    /// defined in the blueprint.
    pub consumer_app: Option<String>,
    /// The name of the schedule app in the blueprint
    /// that should be used as the configuration source for setting
    /// up a polling consumer or webhook endpoint specifically for scheduled messages.
    /// This will be either a shared `celerity.app` annotation shared by
    /// multiple schedules that are part of the same application or the name
    /// of an individual `celerity/schedule` resource in the blueprint.
    /// If not set, the runtime will use the first `celerity/schedule` resource
    /// defined in the blueprint.
    pub schedule_app: Option<String>,
    /// Whether to verify TLS certificates when making requests to the resource store for requesting
    /// resources such as OpenID discovery documents and JSON Web Key Sets for JWT authentication.
    /// This must be true for any production environment, and can be set to false for development
    /// environments with self-signed certificates.
    ///
    /// Defaults to true.
    pub resource_store_verify_tls: bool,
    /// The TTL in seconds for cache entries in the resource store.
    ///
    /// Defaults to 600 seconds (10 minutes).
    pub resource_store_cache_entry_ttl: i64,
    /// The interval in seconds at which the resource store cleanup task should run.
    ///
    /// Defaults to 3600 seconds (1 hour).
    pub resource_store_cleanup_interval: i64,
    /// The source to use for extracting the client IP address from incoming requests.
    /// Defaults to `ConnectInfo` (TCP socket peer address).
    /// Set to a vendor-specific source when running behind a reverse proxy or CDN.
    pub client_ip_source: ClientIpSource,
    /// Override for log format selection.
    /// "json" forces JSON output, "pretty"/"human" forces pretty-print.
    /// If unset, format is determined by platform (Local -> pretty, others -> JSON).
    pub log_format: Option<String>,
    /// Whether to enable OpenTelemetry metrics export.
    /// When enabled, runtime metrics (HTTP request counts/durations, WebSocket connection gauge,
    /// consumer processing metrics) are exported via OTLP to the same collector endpoint as traces.
    /// Disabled by default to avoid overlap with platform infrastructure metrics
    /// (e.g. ALB, Cloud Run ingress) in environments that already provide HTTP-level metrics.
    ///
    /// Defaults to false.
    pub metrics_enabled: bool,
    /// The ratio of traces to sample, between 0.0 and 1.0.
    /// 1.0 means all traces are sampled (AlwaysOn), 0.0 means none (AlwaysOff).
    /// Values between 0.0 and 1.0 use TraceIdRatioBased sampling wrapped in ParentBased
    /// so child spans inherit the parent's sampling decision.
    ///
    /// Defaults to 0.1 (10%) — a production-friendly default that avoids noise for
    /// high-volume apps while capturing enough traces for debugging.
    pub trace_sample_ratio: f64,
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

        let server_loopback_only = env.var("CELERITY_SERVER_LOOPBACK_ONLY").ok();
        let server_loopback_only = server_loopback_only.map(|val| {
            val.parse()
                .expect("Invalid server loopback only value, must be either \"true\" or \"false\"")
        });

        let local_api_port = env
            .var("CELERITY_LOCAL_API_PORT")
            .unwrap_or_else(|_| DEFAULT_LOCAL_API_PORT.to_string())
            .parse()
            .expect("Invalid local API port, must be a valid integer");

        let use_custom_health_check = env.var("CELERITY_USE_CUSTOM_HEALTH_CHECK").ok();
        let use_custom_health_check = use_custom_health_check.map(|val| {
            val.parse().expect(
                "Invalid use custom health check value, must be either \"true\" or \"false\"",
            )
        });

        let trace_otlp_collector_endpoint = env
            .var("CELERITY_TRACE_OTLP_COLLECTOR_ENDPOINT")
            .unwrap_or_default();

        let runtime_max_diagnostics_level_env_var = env
            .var("CELERITY_MAX_DIAGNOSTICS_LEVEL")
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
                    .expect("Invalid test mode value, must be either \"true\" or \"false\"")
            })
            .unwrap_or(false);

        let api_resource = env.var("CELERITY_API_RESOURCE").ok();

        let consumer_app = env.var("CELERITY_CONSUMER_APP").ok();

        let schedule_app = env.var("CELERITY_SCHEDULE_APP").ok();

        let resource_store_verify_tls = env
            .var("CELERITY_RESOURCE_STORE_VERIFY_TLS")
            .unwrap_or_else(|_| "true".to_string())
            .parse()
            .expect(
                "Invalid resource store verify TLS value, must be either \"true\" or \"false\"",
            );

        let resource_store_cache_entry_ttl = env
            .var("CELERITY_RESOURCE_STORE_CACHE_ENTRY_TTL")
            .unwrap_or_else(|_| "600".to_string())
            .parse()
            .expect("Invalid resource store cache entry TTL value, must be a valid integer");

        let resource_store_cleanup_interval = env
            .var("CELERITY_RESOURCE_STORE_CLEANUP_INTERVAL")
            .unwrap_or_else(|_| "3600".to_string())
            .parse()
            .expect("Invalid resource store cache cleanup interval value, must be a valid integer");

        let client_ip_source = env
            .var("CELERITY_CLIENT_IP_SOURCE")
            .unwrap_or_else(|_| "ConnectInfo".to_string())
            .parse::<ClientIpSource>()
            .expect(
                "Invalid client IP source, must be one of: ConnectInfo, CfConnectingIp, \
                 TrueClientIp, CloudFrontViewerAddress, RightmostXForwardedFor, XRealIp, FlyClientIp",
            );

        let log_format = env.var("CELERITY_LOG_FORMAT").ok();

        let metrics_enabled = env
            .var("CELERITY_METRICS_ENABLED")
            .map(|val| {
                val.parse()
                    .expect("Invalid metrics enabled value, must be either \"true\" or \"false\"")
            })
            .unwrap_or(false);

        let trace_sample_ratio: f64 = env
            .var("CELERITY_TRACE_SAMPLE_RATIO")
            .unwrap_or_else(|_| "0.1".to_string())
            .parse()
            .expect("Invalid trace sample ratio, must be a float between 0.0 and 1.0");

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
            api_resource,
            consumer_app,
            schedule_app,
            resource_store_verify_tls,
            resource_store_cache_entry_ttl,
            resource_store_cleanup_interval,
            client_ip_source,
            log_format,
            metrics_enabled,
            trace_sample_ratio,
        }
    }
}

#[derive(Debug)]
pub struct AppConfig {
    pub api: Option<ApiConfig>,
    pub consumers: Option<ConsumersConfig>,
    pub schedules: Option<SchedulesConfig>,
    pub events: Option<EventsConfig>,
    pub custom_handlers: Option<CustomHandlersConfig>,
}

#[derive(Debug)]
pub struct ApiConfig {
    pub http: Option<HttpConfig>,
    pub websocket: Option<WebSocketConfig>,
    pub guards: Option<GuardsConfig>,
    pub auth: Option<CelerityApiAuth>,
    pub cors: Option<CelerityApiCors>,
    pub tracing_enabled: bool,
}

#[derive(Debug)]
pub struct GuardsConfig {
    pub handlers: Vec<GuardHandlerDefinition>,
}

#[derive(Debug, Clone)]
pub struct GuardHandlerDefinition {
    pub name: String,
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
    // The ordered list of auth guard names that protect this handler.
    // If None, the default guard chain from the API auth configuration will be used.
    pub auth_guard: Option<Vec<String>>,
    // Whether the handler is explicitly public (no auth required).
    pub public: bool,
}

#[derive(Debug)]
pub struct WebSocketConfig {
    pub handlers: Vec<WebSocketHandlerDefinition>,
    // Base paths are used by the runtime to only route requests
    // with a certain base path prefix to the WebSocket API in a hybrid API
    // context.
    pub base_paths: Vec<CelerityApiBasePath>,
    pub route_key: String,
    pub auth_strategy: WebSocketAuthStrategy,
    // The ordered list of auth guard names for WebSocket connection auth.
    pub connection_auth_guard: Option<Vec<String>>,
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

#[derive(Debug, Clone)]
pub struct ConsumersConfig {
    pub consumers: Vec<ConsumerConfig>,
}

/// Distinguishes the source type of a consumer for stream name derivation.
#[derive(Debug, Clone, PartialEq)]
pub enum ConsumerSourceType {
    /// A pull-based queue (SQS, Service Bus, Pub/Sub pull subscription).
    Queue,
    /// A Celerity topic identified by the `celerity::topic::` prefix in sourceId.
    Topic,
}

#[derive(Debug, Clone)]
pub struct ConsumerConfig {
    /// The blueprint resource name of this consumer.
    pub consumer_name: String,
    pub source_id: String,
    /// Whether this consumer sources from a queue or a Celerity topic.
    pub source_type: ConsumerSourceType,
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
    // The routing key used to filter messages based on the payload of the message.
    // This is only applicable when the consumer message payload is a valid JSON object
    // that contain the specified routing key field.
    // This defaults to `event` and is only used when routing is activated through the use of
    // a `celerity.handler.consumer.route` annotation set on a handler.
    pub routing_key: Option<String>,
    /// The source ID for the dead-letter queue/stream, if configured.
    /// For queue sources: resolved from a linked DLQ queue resource in the blueprint.
    /// For topic sources: auto-generated when `celerity.consumer.deadLetterQueue` is true (default).
    pub dlq_source_id: Option<String>,
    /// Max processing attempts before a message is moved to the DLQ.
    pub max_retries: Option<i64>,
    pub handlers: Vec<EventHandlerDefinition>,
}

#[derive(Debug, Default, Clone)]
pub struct EventHandlerDefinition {
    pub name: String,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
    // The route value for consumer message routing.
    // From the `celerity.handler.consumer.route` annotation.
    pub route: Option<String>,
}

#[derive(Debug, Clone)]
pub struct SchedulesConfig {
    pub schedules: Vec<ScheduleConfig>,
}

#[derive(Debug, Clone)]
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
    // A static JSON value delivered to the schedule handler on every trigger.
    pub input: Option<Value>,
}

#[derive(Debug, Clone)]
pub struct EventsConfig {
    pub events: Vec<EventConfig>,
}

#[derive(Debug, Clone)]
pub enum EventConfig {
    // An event trigger (e.g. file uploaded to Amazon S3)
    EventTrigger(EventTriggerConfig),
    // A stream of events or data into the runtime.
    Stream(StreamConfig),
}

#[derive(Debug, Clone)]
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

/// Distinguishes the source type of a stream for naming and routing.
#[derive(Debug, Clone, PartialEq)]
pub enum StreamSourceType {
    /// Database change stream (DynamoDB Streams, Cosmos DB Change Feed, etc.).
    Datastore,
    /// Standalone data stream (Kinesis, Event Hubs, etc.).
    DataStream,
}

#[derive(Debug, Clone)]
pub struct StreamConfig {
    /// The source type determines the Valkey stream naming prefix.
    pub source_type: StreamSourceType,
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
    // Whether to start reading from the beginning of the stream.
    pub start_from_beginning: Option<bool>,
    pub handlers: Vec<EventHandlerDefinition>,
}

#[derive(Debug)]
pub struct CustomHandlersConfig {
    pub handlers: Vec<CustomHandlerDefinition>,
}

#[derive(Debug)]
pub struct CustomHandlerDefinition {
    pub name: String,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}
