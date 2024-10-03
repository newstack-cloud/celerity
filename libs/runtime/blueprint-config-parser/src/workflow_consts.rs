/// The state type for a state that executes a handler.
pub const CELERITY_WORKFLOW_STATE_TYPE_EXECUTE_STEP: &str = "executeStep";

/// The state type for a state that passes the input to the output without
/// doing anything, a pass step can inject extra data into the output.
pub const CELERITY_WORKFLOW_STATE_TYPE_PASS: &str = "pass";

/// The state type for a state that executes multiple steps in parallel.
pub const CELERITY_WORKFLOW_STATE_TYPE_PARALLEL: &str = "parallel";

/// The state type for a state that waits for a specific amount of time before
/// transitioning to the next state.
pub const CELERITY_WORKFLOW_STATE_TYPE_WAIT: &str = "wait";

/// The state type for a state that makes a decision on the next state based on the output
/// of a previous state.
pub const CELERITY_WORKFLOW_STATE_TYPE_DECISION: &str = "decision";

/// The state type for a state that indicates a specific failure state in the workflow,
/// this is a terminal state.
pub const CELERITY_WORKFLOW_STATE_TYPE_FAILURE: &str = "failure";

/// The state type for a state that indicates a successful completion of the workflow,
/// this is a terminal state.
pub const CELERITY_WORKFLOW_STATE_TYPE_SUCCESS: &str = "success";
