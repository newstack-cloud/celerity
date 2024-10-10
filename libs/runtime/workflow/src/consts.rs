// The endpoint used for the workflow runtime health check.
pub const WORKFLOW_RUNTIME_HEALTH_CHECK_ENDPOINT: &str = "/runtime/health/check";

// The maximum number of events that can be held at any given time in the event broadcaster
// channel.
pub const EVENT_BROADCASTER_CAPACITY: usize = 100;
