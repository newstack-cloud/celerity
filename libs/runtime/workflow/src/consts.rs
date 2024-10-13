// The endpoint used for the workflow runtime health check.
pub const WORKFLOW_RUNTIME_HEALTH_CHECK_ENDPOINT: &str = "/runtime/health/check";

// The maximum number of events that can be held at any given time in the event broadcaster
// channel.
pub const EVENT_BROADCASTER_CAPACITY: usize = 100;

// The default interval in seconds to retry a state that has failed
// and is configured to be retried.
pub const DEFAULT_STATE_RETRY_INTERVAL_SECONDS: i64 = 3;

// The default backoff rate to use for retrying a state that has failed
// and is configured to be retried.
pub const DEFAULT_STATE_RETRY_BACKOFF_RATE: f64 = 2.0;
