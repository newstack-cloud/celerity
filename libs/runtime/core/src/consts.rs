// The annotation name that activates HTTP capabilities for a handler.
pub const CELERITY_HTTP_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.http";

// The annotation name that holds the HTTP method for a handler.
pub const CELERITY_HTTP_METHOD_ANNOTATION_NAME: &str = "celerity.handler.http.method";

// The annotation name that holds the HTTP path for a handler.
pub const CELERITY_HTTP_PATH_ANNOTATION_NAME: &str = "celerity.handler.http.path";

// The annotation name that activates WebSocket capabilities for a handler.
pub const CELERITY_WS_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.websocket";

// The annotation name that holds the WebSocket route value for a handler.
// For example, "$connect" for a route key "action" in the message object.
// The message object in this case would look like this:
// { "action": "$connect", "data": {} }
pub const CELERITY_WS_ROUTE_ANNOTATION_NAME: &str = "celerity.handler.websocket.route";

// The maximum timeout for a handler in seconds.
pub const MAX_HANDLER_TIMEOUT: i64 = 3600;

// The default timeout for a handler in seconds.
pub const DEFAULT_HANDLER_TIMEOUT: i64 = 60;

// The default value for whether or not tracing is enabled for a handler.
pub const DEFAULT_TRACING_ENABLED: bool = false;

// The default port that the local API runs on in the "http" runtime call mode.
pub const DEFAULT_LOCAL_API_PORT: &str = "8592";

// The default endpoint used for the runtime health check.
pub const DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT: &str = "/runtime/health/check";

// The default message object property that is used to route WebSocket messages.
pub const DEFAULT_WEBSOCKET_API_ROUTE_KEY: &str = "event";

// The default endpoint for collecting trace data.
pub const DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT: &str = "http://otelcollector:4317";

// The name of the header to derive a request ID from.
pub const REQUEST_ID_HEADER: &str = "x-request-id";
