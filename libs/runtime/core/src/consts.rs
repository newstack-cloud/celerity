use celerity_blueprint_config_parser::blueprint::WebSocketAuthStrategy;

// The annotation name that activates HTTP capabilities for a handler.
pub const CELERITY_HTTP_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.http";

// The annotation name that holds the HTTP method for a handler.
pub const CELERITY_HTTP_METHOD_ANNOTATION_NAME: &str = "celerity.handler.http.method";

// The annotation name that holds the HTTP path for a handler.
pub const CELERITY_HTTP_PATH_ANNOTATION_NAME: &str = "celerity.handler.http.path";

// The annotation name that holds the auth guard name to protect a handler.
// The value should reference one of the guard names defined in the API auth configuration.
pub const CELERITY_HANDLER_GUARD_ANNOTATION_NAME: &str = "celerity.handler.guard.protectedBy";

// The annotation name that marks a handler as public (no auth required),
// even when a default guard is configured for the API.
pub const CELERITY_HANDLER_PUBLIC_ANNOTATION_NAME: &str = "celerity.handler.public";

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

// Binary prefix for the server capabilities signal.
// Sent immediately after a WebSocket connection is established to indicate
// that the server supports full protocol capabilities (binary messages,
// custom close codes, and binary control frames).
// In environments where binary frames are not supported (e.g., managed
// WebSocket gateways), this frame will not reach the client, causing the
// client to fall back to constrained capabilities (text-only).
pub const CELERITY_WS_CAPABILITIES_SIGNAL: [u8; 4] = [0x1, 0x5, 0x0, 0x0];

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

// The annotation name that activates consumer capabilities for a handler.
pub const CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.consumer";

// The annotation name that holds the consumer route value for a handler.
// Used to route messages based on a payload field matching this value.
pub const CELERITY_CONSUMER_HANDLER_ROUTE_ANNOTATION_NAME: &str = "celerity.handler.consumer.route";

// The annotation name that activates schedule capabilities for a handler.
pub const CELERITY_SCHEDULE_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.schedule";

// The annotation name on a consumer resource that disambiguates
// which queue resource should be used as the source
// when multiple queue resources link to the same consumer.
pub const CELERITY_CONSUMER_QUEUE_ANNOTATION_NAME: &str = "celerity.consumer.queue";

// The annotation name on a consumer resource that disambiguates
// which datastore resource should be used as the stream source
// when multiple datastore resources link to the same consumer.
pub const CELERITY_CONSUMER_DATASTORE_ANNOTATION_NAME: &str = "celerity.consumer.datastore";

// The annotation name on a consumer resource that controls
// whether to start reading from the beginning of a datastore stream.
pub const CELERITY_CONSUMER_DATASTORE_START_ANNOTATION_NAME: &str =
    "celerity.consumer.datastore.startFromBeginning";

// The annotation name on a consumer resource that disambiguates
// which bucket resource should be used as the event source
// when multiple bucket resources link to the same consumer.
pub const CELERITY_CONSUMER_BUCKET_ANNOTATION_NAME: &str = "celerity.consumer.bucket";

// The annotation name on a consumer resource that specifies
// the bucket event types (comma-separated) to listen for.
pub const CELERITY_CONSUMER_BUCKET_EVENTS_ANNOTATION_NAME: &str = "celerity.consumer.bucket.events";

// The annotation name on a queue resource that controls
// the maximum number of delivery attempts before a message
// is moved to the dead-letter queue.
pub const CELERITY_QUEUE_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME: &str =
    "celerity.queue.deadLetterMaxAttempts";

// The annotation name on a consumer resource that controls
// whether a dead-letter queue is automatically created for topic consumers.
// Defaults to `true` when not specified.
pub const CELERITY_CONSUMER_DLQ_ANNOTATION_NAME: &str = "celerity.consumer.deadLetterQueue";

// The annotation name on a consumer resource that controls
// the maximum number of delivery attempts before a message
// is moved to the dead-letter queue for topic consumers.
pub const CELERITY_CONSUMER_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME: &str =
    "celerity.consumer.deadLetterQueueMaxAttempts";

// The leeway for JWT validation in seconds.
pub const JWT_VALIDATION_CLOCK_SKEW_LEEWAY: u64 = 60;
