use celerity_blueprint_config_parser::blueprint::WebSocketAuthStrategy;

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

// The default WebSocket API auth strategy.
pub const DEFAULT_WEBSOCKET_API_AUTH_STRATEGY: WebSocketAuthStrategy =
    WebSocketAuthStrategy::AuthMessage;

// The default endpoint for collecting trace data.
pub const DEFAULT_TRACE_OTLP_COLLECTOR_ENDPOINT: &str = "http://otelcollector:4317";

// The default TTL for cache entries in the resource store in seconds.
pub const DEFAULT_RESOURCE_STORE_CACHE_ENTRY_TTL: i64 = 600;

// The default interval for the resource store cleanup task in seconds.
pub const DEFAULT_RESOURCE_STORE_CLEANUP_INTERVAL: i64 = 3600;

// The name of the header to derive a request ID from.
pub const REQUEST_ID_HEADER: &str = "x-request-id";

// The error code for a Celerity WebSocket API authentication error
// when the `connect` auth strategy is used.
pub const CELERITY_WS_UNAUTHORISED_ERROR_CODE: u16 = 4001;

// The error code for a Celerity WebSocket API authorisation error
// when the `connect` auth strategy is used.
// Authorisation errors will usually be returned by a custom auth guard
// via the `Forbidden` error variant.
pub const CELERITY_WS_FORBIDDEN_ERROR_CODE: u16 = 4002;

// The route key for the connect handler for a WebSocket API.
// A handler registered with this route key will be called when a client
// connects to the WebSocket API server after authentication has been performed
// (if the `connect` auth strategy is used).
pub const CELERITY_WS_CONNECT_HANDLER_ROUTE: &str = "$connect";

// The route key for the disconnect handler for a WebSocket API.
// A handler registered with this route key will be called when a client
// disconnects from the WebSocket API server.
pub const CELERITY_WS_DISCONNECT_HANDLER_ROUTE: &str = "$disconnect";

// The route key for the default message handler for a WebSocket API.
// A handler registered with this route key will be called when a client
// sends a message to the WebSocket API server that does not match any
// other registered handler.
pub const CELERITY_WS_DEFAULT_MESSAGE_HANDLER_ROUTE: &str = "$default";

// The leeway for JWT validation in seconds.
pub const JWT_VALIDATION_CLOCK_SKEW_LEEWAY: u64 = 60;
