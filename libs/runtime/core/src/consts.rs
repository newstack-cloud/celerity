// The annotation name that activates HTTP capabilities for a handler.
pub const CELERITY_HTTP_HANDLER_ANNOTATION_NAME: &str = "celerity.handler.http";

// The annotation name that holds the HTTP method for a handler.
pub const CELERITY_HTTP_METHOD_ANNOTATION_NAME: &str = "celerity.handler.http.method";

// The annotation name that holds the HTTP path for a handler.
pub const CELERITY_HTTP_PATH_ANNOTATION_NAME: &str = "celerity.handler.http.path";

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
