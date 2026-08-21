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

use crate::consts::{DEFAULT_RUNTIME_SOCKET, DEFAULT_RUNTIME_SOCKET_FALLBACK_PORT};

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
    /// The loopback port the handler stream is served on when a Unix socket
    /// cannot be bound and [`RuntimeConfig::runtime_socket_fallback_enabled`]
    /// allows it.
    pub runtime_socket_fallback_port: u16,
    /// Whether to serve the handler stream over loopback TCP when a Unix socket
    /// cannot be bound.
    ///
    /// Off unless asked for, and the runtime refuses to start rather than
    /// serving nothing. Falling back silently would widen who can register as a
    /// handler, from the one user a socket's permissions allow to any process
    /// that can reach loopback, and would do it at the moment something is
    /// already wrong. It also hides the cause where a socket that cannot be bound is
    /// usually a misconfiguration rather than a platform without Unix sockets.
    pub runtime_socket_fallback_enabled: bool,
    /// The Unix socket the handler stream is served on in the IPC runtime call
    /// mode.
    ///
    /// When this cannot be bound, the runtime falls back to loopback TCP on
    /// `runtime_socket_fallback_port`.
    pub runtime_socket: String,
    /// How long the runtime waits for in-flight events to come back once it
    /// starts shutting down, in seconds.
    ///
    /// Leave unset to derive it from the longest handler timeout the blueprint
    /// configures, bounded by
    /// [`MAX_DERIVED_DRAIN_TIMEOUT_SECS`](crate::consts::MAX_DERIVED_DRAIN_TIMEOUT_SECS).
    /// Set it when the deployment has its own grace period to respect,
    /// which should be the larger of the two if in-flight work is to finish.
    pub drain_timeout: Option<u64>,
    /// Whether to serve `POST /runtime/handlers/invoke`, which runs any handler
    /// the blueprint declares, by name, with a payload the caller supplies.
    ///
    /// Off unless asked for, and one of two conditions rather than the whole
    /// decision; the runtime must also be on a local platform or in test mode.
    /// Setting it can therefore take the endpoint away locally but cannot
    /// introduce it anywhere else.
    ///
    /// Both are required because it bypasses whatever normally triggers a
    /// handler and is not covered by the API's auth guards, including a
    /// configured default guard, since it is not a blueprint route. Anything
    /// bringing up a local environment has to turn it on deliberately.
    pub enable_local_invoke: bool,
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
    /// The target deployment provider for body transforms and provider-specific
    /// event handling (e.g. `"aws"`, `"gcp"`, `"azure"`).
    ///
    /// Required when `platform` is `Local` or `Other` so the runtime knows which
    /// provider-specific event formats to use for datastore streams, bucket events,
    /// etc.  For cloud platforms (`AWS`, `GCP`, `Azure`) the provider is derived
    /// directly from the platform and this field is ignored.
    ///
    /// Set via `CELERITY_DEPLOY_TARGET`.
    pub deploy_target: Option<String>,
    /// How long the runtime waits for a client to acknowledge a message that
    /// asked to be acknowledged, before sending it again.
    ///
    /// Leave unset for the suggested default the WebSocket runtime protocol names,
    /// which is 10 seconds. Set via `CELERITY_WS_ACK_TIMEOUT_MS`.
    pub ws_ack_timeout_ms: Option<u64>,
    /// How many times a message asking to be acknowledged is sent before it is
    /// considered lost and the clients waiting on it are told so.
    ///
    /// Counts the first send, so 3 means the original and two more. Leave unset
    /// for the suggested default the WebSocket runtime protocol names, which is 3.
    /// Set via `CELERITY_WS_ACK_MAX_ATTEMPTS`.
    pub ws_ack_max_attempts: Option<u32>,
    /// How many of one connection's messages may be handled at the same time.
    ///
    /// Leave unset for eight. Set it to one for a connection whose messages have
    /// to be handled in the order they arrived, at the cost of capping the
    /// connection at one handler's worth of latency per message. Bounds one
    /// connection rather than the process.
    /// Set via `CELERITY_WS_HANDLER_CONCURRENCY`.
    pub ws_handler_concurrency: Option<usize>,
    /// Names this node among the others serving the same WebSocket API.
    ///
    /// This must be different for every node, since it is what an acknowledgement
    /// from another node is addressed to. Leave unset to take the host name,
    /// which is the pod or container name on the deployment targets, and a
    /// generated id where there is none.
    /// Set via `CELERITY_SERVER_NODE_NAME`.
    pub server_node_name: Option<String>,
    /// How the nodes of a WebSocket API find each other, or `None` for a
    /// single node.
    ///
    /// A single node needs no shared store, because every connection it is
    /// asked about is either its own or nowhere.
    pub ws_cluster: Option<WsClusterConfig>,
}

/// How a WebSocket API's nodes reach each other and how much they tell each
/// other.
#[derive(Debug, Clone)]
pub struct WsClusterConfig {
    /// The Redis deployment the nodes cluster over, as one URL or several for
    /// a cluster.
    /// Set via `CELERITY_WS_CLUSTER_REDIS_NODES`, comma separated.
    pub redis_nodes: Vec<String>,
    /// Set via `CELERITY_WS_CLUSTER_REDIS_PASSWORD`.
    pub redis_password: Option<String>,
    /// Whether a Redis cluster is to be connected to rather than a single instance.
    /// Set via `CELERITY_WS_CLUSTER_REDIS_CLUSTER_MODE`.
    pub redis_cluster_mode: bool,
    /// What the keys and channels the nodes share are named under.
    ///
    /// Nodes sharing this are one cluster, so it must be the same across an
    /// application's nodes and different between applications. Leave unset to
    /// take the service name, which is both.
    /// Set via `CELERITY_WS_CLUSTER_KEY_PREFIX`.
    pub key_prefix: Option<String>,
    /// How many nodes a node group holds before a new one is started.
    ///
    /// Bounds how many nodes are told about a message for a connection none of
    /// them may be holding. Leave unset to use the default of five as per the
    /// protocol spec.
    /// Set via `CELERITY_WS_CLUSTER_NODE_GROUP_CAPACITY`.
    pub node_group_capacity: Option<usize>,
    /// How long a node goes without announcing it is running before the others
    /// treat it as gone.
    ///
    /// Announced three times inside this window, so two can be missed to a stall or
    /// a slow round trip. Leave unset for thirty seconds.
    /// Set via `CELERITY_WS_CLUSTER_NODE_TTL_MS`.
    pub node_ttl_ms: Option<u64>,
    /// How long a node keeps listening to the group it has left after moving to
    /// another one.
    ///
    /// Covers a sender that read where a connection was just before it moved.
    /// Leave unset for five seconds.
    /// Set via `CELERITY_WS_CLUSTER_MIGRATION_GRACE_MS`.
    pub migration_grace_ms: Option<u64>,
    /// How long the cluster remembers that a message was forwarded to its
    /// client, which is how far apart two copies can arrive and still be
    /// recognised as one message.
    ///
    /// Must outlast a message's whole life as its sender sees it, which is the
    /// acknowledgement timeout multiplied by the attempts allowed. Leave unset
    /// for five minutes.
    /// Set via `CELERITY_WS_CLUSTER_FORWARDED_TTL_MS`.
    pub forwarded_ttl_ms: Option<u64>,
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
            "ipc" => RuntimeCallMode::Ipc,
            _ => panic!("Invalid runtime call mode, must be one of 'ffi' or 'ipc'"),
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

        let runtime_socket_fallback_enabled = env
            .var("CELERITY_RUNTIME_SOCKET_FALLBACK_ENABLED")
            .map(|val| {
                val.parse().expect(
                    "Invalid runtime socket fallback enabled value, must be either \"true\" or \"false\"",
                )
            })
            .unwrap_or(false);

        let runtime_socket_fallback_port = env
            .var("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT")
            .unwrap_or_else(|_| DEFAULT_RUNTIME_SOCKET_FALLBACK_PORT.to_string())
            .parse()
            .expect("Invalid runtime socket fallback port, must be a whole number from 0 to 65535");

        // A port of zero asks the operating system for whichever one is free,
        // which a handlers executable has no way to discover. Harmless while
        // the fallback is off, since nothing binds it then.
        assert!(
            !(runtime_socket_fallback_enabled && runtime_socket_fallback_port == 0),
            "Invalid runtime socket fallback port, must be a specific port when the fallback \
             is enabled, since a handlers executable has to know where to connect"
        );

        let runtime_socket = env
            .var("CELERITY_RUNTIME_SOCKET")
            .unwrap_or_else(|_| DEFAULT_RUNTIME_SOCKET.to_string());

        let drain_timeout = env.var("CELERITY_DRAIN_TIMEOUT").ok().map(|val| {
            val.parse()
                .expect("Invalid drain timeout, must be a whole number of seconds")
        });

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

        let enable_local_invoke = env
            .var("CELERITY_ENABLE_LOCAL_INVOKE")
            .map(|val| {
                val.parse().expect(
                    "Invalid enable local invoke value, must be either \"true\" or \"false\"",
                )
            })
            .unwrap_or(false);

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

        let deploy_target = env.var("CELERITY_DEPLOY_TARGET").ok();

        let ws_ack_timeout_ms = ws_ack_timeout_ms_from_env(env);
        let ws_ack_max_attempts = ws_ack_max_attempts_from_env(env);
        let ws_handler_concurrency = ws_handler_concurrency_from_env(env);
        let server_node_name = env.var("CELERITY_SERVER_NODE_NAME").ok();
        let ws_cluster = ws_cluster_from_env(env);

        RuntimeConfig {
            blueprint_config_path,
            runtime_call_mode,
            service_name,
            server_port,
            server_loopback_only,
            runtime_socket_fallback_port,
            runtime_socket_fallback_enabled,
            runtime_socket,
            drain_timeout,
            enable_local_invoke,
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
            deploy_target,
            ws_ack_timeout_ms,
            ws_ack_max_attempts,
            ws_handler_concurrency,
            server_node_name,
            ws_cluster,
        }
    }

    /// Resolves the body transform provider identifier based on the runtime
    /// platform and optional deploy target.
    ///
    /// For cloud platforms the provider is derived directly:
    /// - `AWS` → `"aws"`, `GCP` → `"gcp"`, `Azure` → `"azure"`.
    ///
    /// For `Local` and `Other` platforms the provider comes from the
    /// `CELERITY_DEPLOY_TARGET` env var.  Returns `None` when no deploy target
    /// is configured, meaning provider-specific body transforms will be skipped.
    pub fn resolve_body_transform_provider(&self) -> Option<&str> {
        match &self.platform {
            RuntimePlatform::AWS => Some("aws"),
            RuntimePlatform::GCP => Some("gcp"),
            RuntimePlatform::Azure => Some("azure"),
            RuntimePlatform::Local | RuntimePlatform::Other => self.deploy_target.as_deref(),
        }
    }
}

/// Reads how the nodes of a WebSocket API reach each other, or `None` where no
/// Redis deployment is configured.
///
/// Configuring a Redis deployment is what turns clustering on. Everything else
/// here refines a decision that has already been made, so it can all be left
/// unset.
///
/// See [`ws_ack_timeout_ms_from_env`] for why this is not only read in
/// [`RuntimeConfig::from_env`].
pub fn ws_cluster_from_env(env: &impl EnvVars) -> Option<WsClusterConfig> {
    // A comma separated list, since a single instance is the usual case and a
    // cluster is given as several. Empty entries are dropped, so a trailing
    // comma is not a node called nothing.
    let redis_nodes: Vec<String> = env
        .var("CELERITY_WS_CLUSTER_REDIS_NODES")
        .ok()?
        .split(',')
        .map(str::trim)
        .filter(|node| !node.is_empty())
        .map(str::to_string)
        .collect();
    if redis_nodes.is_empty() {
        return None;
    }

    Some(WsClusterConfig {
        redis_nodes,
        redis_password: env.var("CELERITY_WS_CLUSTER_REDIS_PASSWORD").ok(),
        redis_cluster_mode: env
            .var("CELERITY_WS_CLUSTER_REDIS_CLUSTER_MODE")
            .map(|val| {
                val.parse().expect(
                    "Invalid WebSocket cluster mode value, must be either \"true\" or \"false\"",
                )
            })
            .unwrap_or(false),
        key_prefix: env.var("CELERITY_WS_CLUSTER_KEY_PREFIX").ok(),
        node_group_capacity: env
            .var("CELERITY_WS_CLUSTER_NODE_GROUP_CAPACITY")
            .ok()
            .map(|val| {
                let capacity: usize = val
                    .parse()
                    .expect("Invalid WebSocket node group capacity, must be a whole number");
                assert!(
                    capacity > 0,
                    "Invalid WebSocket node group capacity, a group has to hold at least one node"
                );
                capacity
            }),
        node_ttl_ms: env.var("CELERITY_WS_CLUSTER_NODE_TTL_MS").ok().map(|val| {
            let ttl: u64 = val
                .parse()
                .expect("Invalid WebSocket node TTL, must be a whole number of milliseconds");
            assert!(
                ttl > 0,
                "Invalid WebSocket node TTL, a node needs some time to say it is running in"
            );
            ttl
        }),
        migration_grace_ms: env
            .var("CELERITY_WS_CLUSTER_MIGRATION_GRACE_MS")
            .ok()
            .map(|val| {
                val.parse().expect(
                    "Invalid WebSocket migration grace period, must be a whole number of \
                     milliseconds",
                )
            }),
        forwarded_ttl_ms: env
            .var("CELERITY_WS_CLUSTER_FORWARDED_TTL_MS")
            .ok()
            .map(|val| {
                let ttl: u64 = val.parse().expect(
                    "Invalid WebSocket forwarded message TTL, must be a whole number of \
                     milliseconds",
                );
                assert!(
                    ttl > 0,
                    "Invalid WebSocket forwarded message TTL, a message forgotten immediately \
                     is never recognised as one already sent"
                );
                ttl
            }),
    })
}

/// Reads the WebSocket acknowledgement timeout from the environment.
///
/// Separate from [`RuntimeConfig::from_env`] so that the SDKs, which rebuild a
/// runtime configuration from their own, read it the same way rather than each
/// deciding what the variable is called.
pub fn ws_ack_timeout_ms_from_env(env: &impl EnvVars) -> Option<u64> {
    env.var("CELERITY_WS_ACK_TIMEOUT_MS").ok().map(|val| {
        let timeout: u64 = val
            .parse()
            .expect("Invalid WebSocket ack timeout, must be a whole number of milliseconds");
        assert!(
            timeout > 0,
            "Invalid WebSocket ack timeout, a client needs some time to answer in"
        );
        timeout
    })
}

/// Reads the WebSocket acknowledgement attempt limit from the environment.
///
/// See [`ws_ack_timeout_ms_from_env`] for why this is not only read in
/// [`RuntimeConfig::from_env`].
pub fn ws_ack_max_attempts_from_env(env: &impl EnvVars) -> Option<u32> {
    env.var("CELERITY_WS_ACK_MAX_ATTEMPTS").ok().map(|val| {
        let attempts: u32 = val
            .parse()
            .expect("Invalid WebSocket ack max attempts, must be a whole number");
        assert!(
            attempts > 0,
            "Invalid WebSocket ack max attempts, a message has to be sent at least once"
        );
        attempts
    })
}

/// Reads how many of a connection's messages may be handled at the same time.
///
/// See [`ws_ack_timeout_ms_from_env`] for why this is not only read in
/// [`RuntimeConfig::from_env`].
pub fn ws_handler_concurrency_from_env(env: &impl EnvVars) -> Option<usize> {
    env.var("CELERITY_WS_HANDLER_CONCURRENCY").ok().map(|val| {
        let concurrency: usize = val
            .parse()
            .expect("Invalid WebSocket handler concurrency, must be a whole number");
        assert!(
            concurrency > 0,
            "Invalid WebSocket handler concurrency, a connection has to handle at least one \
             message at a time"
        );
        concurrency
    })
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
    /// The name the blueprint publishes this handler under, from
    /// `spec.handlerName`, as distinct from `name`, which is the blueprint
    /// resource it is declared as.
    pub published_name: Option<String>,
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
    /// The name the blueprint publishes this handler under, from
    /// `spec.handlerName`, as distinct from `name`, which is the blueprint
    /// resource it is declared as.
    pub published_name: Option<String>,
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
    /// The name the blueprint publishes this handler under, from
    /// `spec.handlerName`, as distinct from `name`, which is the blueprint
    /// resource it is declared as.
    pub published_name: Option<String>,
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
    // The name of the consumer resource in the blueprint.
    pub consumer_name: String,
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
    // The name of the consumer resource in the blueprint.
    pub consumer_name: String,
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
    /// The name the blueprint publishes this handler under, from
    /// `spec.handlerName`, as distinct from `name`, which is the blueprint
    /// resource it is declared as.
    pub published_name: Option<String>,
    pub location: String,
    pub handler: String,
    // Timeout in seconds.
    pub timeout: i64,
    pub tracing_enabled: bool,
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use celerity_helpers::env::EnvVars;

    use super::*;

    #[derive(Clone)]
    struct MapEnv(HashMap<&'static str, String>);

    impl EnvVars for MapEnv {
        fn var(&self, key: &str) -> Result<String, std::env::VarError> {
            self.0
                .get(key)
                .cloned()
                .ok_or(std::env::VarError::NotPresent)
        }

        fn clone_env_vars(&self) -> Box<dyn EnvVars> {
            Box::new(self.clone())
        }
    }

    /// The smallest environment `from_env` will accept, plus whatever a test
    /// is actually about.
    fn env(overrides: &[(&'static str, &str)]) -> MapEnv {
        let mut vars: HashMap<&'static str, String> = vec![
            ("CELERITY_BLUEPRINT", "blueprint.yaml".to_string()),
            ("CELERITY_SERVICE_NAME", "test".to_string()),
            ("CELERITY_SERVER_PORT", "8080".to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ipc".to_string()),
        ]
        .into_iter()
        .collect();
        for (key, value) in overrides {
            vars.insert(key, value.to_string());
        }
        MapEnv(vars)
    }

    #[test]
    fn reads_a_fallback_port_within_the_range_a_port_can_take() {
        let config =
            RuntimeConfig::from_env(&env(&[("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "9000")]));

        assert_eq!(config.runtime_socket_fallback_port, 9000);
    }

    #[test]
    fn defaults_the_fallback_port_when_it_is_not_set() {
        let config = RuntimeConfig::from_env(&env(&[]));

        assert_eq!(
            config.runtime_socket_fallback_port.to_string(),
            DEFAULT_RUNTIME_SOCKET_FALLBACK_PORT
        );
    }

    #[test]
    #[should_panic(expected = "Invalid runtime socket fallback port")]
    fn refuses_a_fallback_port_above_the_range_a_port_can_take() {
        // Held as a u16 so this cannot quietly wrap to 4464 and bind the wrong
        // port, which is what an i32 truncated at the point of use would do.
        RuntimeConfig::from_env(&env(&[("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "70000")]));
    }

    #[test]
    #[should_panic(expected = "Invalid runtime socket fallback port")]
    fn refuses_a_negative_fallback_port() {
        RuntimeConfig::from_env(&env(&[("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "-1")]));
    }

    #[test]
    #[should_panic(expected = "must be a specific port when the fallback")]
    fn refuses_an_ephemeral_fallback_port_when_the_fallback_is_enabled() {
        RuntimeConfig::from_env(&env(&[
            ("CELERITY_RUNTIME_SOCKET_FALLBACK_ENABLED", "true"),
            ("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "0"),
        ]));
    }

    #[test]
    fn allows_an_ephemeral_fallback_port_while_the_fallback_is_off() {
        // Nothing binds it, so the value is never used.
        let config =
            RuntimeConfig::from_env(&env(&[("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "0")]));

        assert_eq!(config.runtime_socket_fallback_port, 0);
        assert!(!config.runtime_socket_fallback_enabled);
    }

    /// Unset means the good defaults the protocol names, which the worker
    /// applies, rather than a value chosen here.
    #[test]
    fn test_ack_timings_are_left_to_the_worker_when_unset() {
        let config = RuntimeConfig::from_env(&env(&[]));

        assert_eq!(config.ws_ack_timeout_ms, None);
        assert_eq!(config.ws_ack_max_attempts, None);
    }

    #[test]
    fn test_ack_timings_are_read_from_the_environment() {
        let config = RuntimeConfig::from_env(&env(&[
            ("CELERITY_WS_ACK_TIMEOUT_MS", "2500"),
            ("CELERITY_WS_ACK_MAX_ATTEMPTS", "5"),
        ]));

        assert_eq!(config.ws_ack_timeout_ms, Some(2500));
        assert_eq!(config.ws_ack_max_attempts, Some(5));
    }

    /// A message has to be sent once before it can be resent, so no attempts at
    /// all is a configuration that cannot be honoured rather than one meaning
    /// never send.
    #[test]
    #[should_panic(expected = "sent at least once")]
    fn test_no_ack_attempts_at_all_is_refused() {
        RuntimeConfig::from_env(&env(&[("CELERITY_WS_ACK_MAX_ATTEMPTS", "0")]));
    }

    /// No time at all leaves every message overdue as soon as it is sent, so
    /// the first check resends it and the attempts run out in milliseconds.
    #[test]
    #[should_panic(expected = "some time to answer in")]
    fn test_an_ack_timeout_of_nothing_is_refused() {
        RuntimeConfig::from_env(&env(&[("CELERITY_WS_ACK_TIMEOUT_MS", "0")]));
    }

    #[test]
    #[should_panic(expected = "whole number of milliseconds")]
    fn test_an_ack_timeout_that_is_not_a_number_is_refused() {
        RuntimeConfig::from_env(&env(&[("CELERITY_WS_ACK_TIMEOUT_MS", "ten seconds")]));
    }

    /// No Redis deployment configured means a single node, which is the shape
    /// most deployments have.
    #[test]
    fn test_a_deployment_with_no_redis_configured_is_a_single_node() {
        assert!(RuntimeConfig::from_env(&env(&[])).ws_cluster.is_none());
    }

    /// An empty list is nothing configured, rather than a cluster of no nodes.
    #[test]
    fn test_an_empty_redis_list_is_a_single_node() {
        assert!(
            RuntimeConfig::from_env(&env(&[("CELERITY_WS_CLUSTER_REDIS_NODES", " , ")]))
                .ws_cluster
                .is_none()
        );
    }

    #[test]
    fn test_the_cluster_settings_are_read_from_the_environment() {
        let config = RuntimeConfig::from_env(&env(&[
            ("CELERITY_SERVER_NODE_NAME", "api-node-1"),
            (
                "CELERITY_WS_CLUSTER_REDIS_NODES",
                "redis://one:6379, redis://two:6379,",
            ),
            ("CELERITY_WS_CLUSTER_REDIS_PASSWORD", "hunter2"),
            ("CELERITY_WS_CLUSTER_REDIS_CLUSTER_MODE", "true"),
            ("CELERITY_WS_CLUSTER_KEY_PREFIX", "chat"),
            ("CELERITY_WS_CLUSTER_NODE_GROUP_CAPACITY", "3"),
            ("CELERITY_WS_CLUSTER_NODE_TTL_MS", "15000"),
            ("CELERITY_WS_CLUSTER_MIGRATION_GRACE_MS", "2000"),
        ]));

        assert_eq!(config.server_node_name, Some("api-node-1".to_string()));
        let cluster = config.ws_cluster.expect("the nodes should be clustered");
        // Trimmed, and the trailing comma is not a node called nothing.
        assert_eq!(
            cluster.redis_nodes,
            vec![
                "redis://one:6379".to_string(),
                "redis://two:6379".to_string()
            ]
        );
        assert_eq!(cluster.redis_password, Some("hunter2".to_string()));
        assert!(cluster.redis_cluster_mode);
        assert_eq!(cluster.key_prefix, Some("chat".to_string()));
        assert_eq!(cluster.node_group_capacity, Some(3));
        assert_eq!(cluster.node_ttl_ms, Some(15000));
        assert_eq!(cluster.migration_grace_ms, Some(2000));
    }

    /// Configuring a Redis deployment is the whole decision, so everything else
    /// can be left to the runtime.
    #[test]
    fn test_configuring_a_redis_is_enough_to_cluster() {
        let config = RuntimeConfig::from_env(&env(&[(
            "CELERITY_WS_CLUSTER_REDIS_NODES",
            "redis://one:6379",
        )]));

        let cluster = config.ws_cluster.expect("the nodes should be clustered");
        assert!(!cluster.redis_cluster_mode);
        assert_eq!(cluster.key_prefix, None);
        assert_eq!(cluster.node_group_capacity, None);
        assert_eq!(cluster.node_ttl_ms, None);
        assert_eq!(cluster.migration_grace_ms, None);
    }

    /// A group holding nothing has nowhere to put the node that is asking.
    #[test]
    #[should_panic(expected = "at least one node")]
    fn test_a_node_group_holding_nothing_is_refused() {
        RuntimeConfig::from_env(&env(&[
            ("CELERITY_WS_CLUSTER_REDIS_NODES", "redis://one:6379"),
            ("CELERITY_WS_CLUSTER_NODE_GROUP_CAPACITY", "0"),
        ]));
    }

    /// No time at all leaves a node dead the moment it says it is running.
    #[test]
    #[should_panic(expected = "some time to say it is running in")]
    fn test_a_node_ttl_of_nothing_is_refused() {
        RuntimeConfig::from_env(&env(&[
            ("CELERITY_WS_CLUSTER_REDIS_NODES", "redis://one:6379"),
            ("CELERITY_WS_CLUSTER_NODE_TTL_MS", "0"),
        ]));
    }

    #[test]
    fn test_handler_concurrency_is_left_to_the_runtime_when_unset() {
        assert_eq!(
            RuntimeConfig::from_env(&env(&[])).ws_handler_concurrency,
            None
        );
    }

    #[test]
    fn test_handler_concurrency_is_read_from_the_environment() {
        let config = RuntimeConfig::from_env(&env(&[("CELERITY_WS_HANDLER_CONCURRENCY", "8")]));

        assert_eq!(config.ws_handler_concurrency, Some(8));
    }

    /// Nothing at all would leave a connection unable to handle anything,
    /// rather than meaning handle nothing in parallel.
    #[test]
    #[should_panic(expected = "one message at a time")]
    fn test_no_handler_concurrency_at_all_is_refused() {
        RuntimeConfig::from_env(&env(&[("CELERITY_WS_HANDLER_CONCURRENCY", "0")]));
    }
}
