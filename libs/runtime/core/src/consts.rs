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

// The default Unix socket the handler stream is served on in the "ipc" runtime
// call mode. A Unix socket is preferred over loopback TCP: it is consistently
// faster on Linux, needs no port allocation, and its access control is
// filesystem permissions rather than "anything that can reach localhost".
// The runtime restricts it to its own user on bind, which is what makes that
// last point true.
pub const DEFAULT_RUNTIME_SOCKET: &str = "/var/run/celerity/runtime.sock";

// The default port the handler stream falls back to when no Unix socket can be
// bound, which is the case on platforms without them.
pub const DEFAULT_RUNTIME_SOCKET_FALLBACK_PORT: &str = "8592";

// The longest a producer will wait for event queue capacity before shedding
// the event. Bounds how long a consumer can be held off its source loop, and
// how long an HTTP request waits before a 503 rather than being served.
pub const MAX_EVENT_QUEUE_ADMISSION_WAIT_SECS: u64 = 5;

// The fraction of an event's timeout that may be spent waiting for queue
// capacity, expressed as a divisor. A quarter leaves the majority of the
// budget for the handler that will eventually run.
pub const EVENT_QUEUE_ADMISSION_WAIT_DIVISOR: u32 = 4;

// The largest HTTP request body the runtime will buffer into an event in the
// IPC call mode. Matches axum's own default extractor limit.
pub const MAX_HTTP_REQUEST_BODY_BYTES: usize = 2 * 1024 * 1024;

// How long a queued event waits for a handler stream serving its tag to attach
// before the runtime gives up on it.
//
// Without a grace window, a request arriving in the moment before the handlers
// executable finishes connecting would be shed, so every restart would drop
// traffic it could have served. With one that is too long, an application whose
// handlers are not running holds requests open instead of failing them. A few
// seconds covers a connect that is already in progress and stays well inside
// any handler timeout, so an event shed here is one no handler was ever going to run.
pub const HANDLER_ATTACH_GRACE_SECS: u64 = 3;

// The longest the dispatcher will wait for in-flight events to come back once
// the runtime starts shutting down, when no drain timeout is configured.
//
// The default is derived from the longest handler timeout the blueprint
// configures, so an application whose handlers are all short shuts down
// promptly while one with a long running handler is given the time that handler
// was told it had. This caps that derivation, because a deployment cannot wait
// out an hour long handler, past this point the work is abandoned and, for a
// source that acknowledges (a queue or topic), redelivered on the next start.
pub const MAX_DERIVED_DRAIN_TIMEOUT_SECS: u64 = 300;

// The drain timeout used when nothing is configured and no handler timeout can
// be resolved, which is the case for an application with no handlers at all.
pub const DEFAULT_DRAIN_TIMEOUT_SECS: u64 = 30;

// How many cancellations may be waiting for the dispatcher before further ones
// are dropped.
//
// Cancellation is advisory: it saves a handler working on something nobody
// wants, and the event's deadline ends it either way. So a bounded channel that
// drops under saturation is the right trade, where blocking is impossible (one
// sender runs in `Drop`) and growing without limit would spend memory to deliver
// hints that arrive too late to be worth anything.
pub const CANCELLATION_BUFFER: usize = 256;

// How many messages may be waiting to be processed for one WebSocket connection
// before its read loop waits for room.
//
// Messages are handled off the read loop so that a slow handler cannot stop the
// connection answering heartbeats or noticing a close. This bounds how far a
// client can run ahead of its own handlers before it is pushed back on, which
// is a different situation from one handler being slow and deserves the
// backpressure.
pub const WS_CONNECTION_WORK_BUFFER: usize = 64;

// How long a connection's read loop waits for room in that buffer before it
// gives up on the client and closes the connection.
//
// The read loop answers heartbeats, so any time spent waiting here is time the
// connection appears dead to its client. The protocol's default heartbeat
// timeout is 5 seconds, so this stays well inside it: a burst gets a moment to
// drain, and a client that is genuinely outrunning its handlers is shed long
// before it would have concluded the connection was gone.
pub const WS_CONNECTION_WORK_SHED_GRACE_MS: u64 = 1_000;

// How long a closing connection waits for the messages it already accepted to
// finish before it abandons them and completes teardown.
//
// Until teardown completes the connection is still in the registry, still
// counted by the connection gauge, and its disconnect handler has not run, so
// this cannot wait on the queue draining at its own pace. Each queued message
// waits on its own handler timeout, and those add up, whereas a few seconds is
// enough for work that is nearly done to land.
pub const WS_CONNECTION_DRAIN_GRACE_MS: u64 = 5_000;

// The `retryAfter` hint sent in the close frame when a connection is shed for
// outrunning its handlers.
//
// The protocol has a server-initiated backoff, so the client waits at least this
// long before reconnecting rather than coming straight back into the same
// saturation. Clients take the greater of this and their own backoff and add
// their own jitter.
pub const WS_CONNECTION_SATURATED_RETRY_AFTER_MS: u64 = 5_000;

// How many dispatcher commands may be queued before a handler stream waits.
// Commands are small and are drained by a single task, so this only needs to
// absorb bursts of results arriving together.
pub const DISPATCHER_COMMAND_BUFFER: usize = 1024;

// The capacity of the bounded event channel used to hand events to handlers
// in the "ipc" runtime call mode.
// Once this many events are waiting to be picked up, producers (HTTP routes,
// WebSocket routing, consumers and schedules) apply backpressure rather than
// growing the queue without limit.
pub const DEFAULT_EVENT_QUEUE_CAPACITY: usize = 1024;

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
