use std::{collections::HashMap, fmt::Debug, sync::Arc};

use axum::{body::Body, response::IntoResponse};
use celerity_blueprint_config_parser::blueprint::CelerityWorkflowSpec;
use celerity_helpers::{runtime_types::RuntimePlatform, time::Clock};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::sync::{broadcast::Sender, oneshot, RwLock};

use crate::{
    handlers::BoxedWorkflowStateHandler,
    payload_template::Engine,
    workflow_executions::{WorkflowExecution, WorkflowExecutionService, WorkflowExecutionState},
};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventData {
    pub id: String,
    #[serde(rename = "eventType")]
    pub event_type: EventType,
    #[serde(rename = "handlerTag")]
    pub handler_tag: String,
    pub timestamp: u64,
    pub data: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum EventType {
    #[serde(rename = "executeStep")]
    ExecuteStep,
}

// A tuple that contains a oneshot sender and received event data.
// The purpose of this tuple is to hand off processing of an event
// to another process or task. (That takes the event from a queue asynchronously)
// The oneshot sender allows the caller to wait to receive the result of processing the
// event along with the original event data to carry out any further tasks using the input
// data.
pub type EventTuple = (oneshot::Sender<(EventData, EventResult)>, EventData);

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct EventResult {
    #[serde(rename = "eventId")]
    pub event_id: String,
    pub data: Value,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub context: Option<Value>,
}

// WorkflowAppState holds shared Workflow application state to be used in axum
// middleware and handlers.
// This is cloned for every request, meaning the potentially deep workflow spec
// structure will be cloned.
#[derive(Debug, Clone)]
pub struct WorkflowAppState {
    pub platform: RuntimePlatform,
    pub workflow_spec: CelerityWorkflowSpec,
    pub state_handlers: Arc<RwLock<HashMap<String, BoxedWorkflowStateHandler>>>,
    pub execution_service: Arc<dyn WorkflowExecutionService + Send + Sync>,
    pub clock: Arc<dyn Clock + Send + Sync>,
    pub event_broadcaster: Sender<WorkflowExecutionEvent>,
    pub payload_template_engine: Arc<dyn Engine + Send + Sync>,
}

/// A response type that can be converted into an axum response.
pub struct Response {
    pub status: u16,
    pub headers: Option<HashMap<String, String>>,
    pub body: Option<String>,
}

impl IntoResponse for Response {
    fn into_response(self) -> axum::response::Response<Body> {
        let mut builder = axum::response::Response::builder();
        for (key, value) in self.headers.unwrap_or_default() {
            builder = builder.header(key, value);
        }
        builder = builder.status(self.status);
        builder
            .body(Body::from(self.body.unwrap_or_default()))
            .unwrap()
    }
}

/// An event that is emitted during the execution of a workflow.
/// These are primarily used as the events of the Workflow Stream API that allow
/// clients to stream events from a workflow execution without polling.
#[derive(Debug, Clone, Serialize)]
pub enum WorkflowExecutionEvent {
    StateTransition(StateTransitionEvent),
    StateFailure(StateFailureEvent),
    StateRetry(StateRetryEvent),
    ExecutionComplete(ExecutionCompleteEvent),
}

/// An event that is emitted when there is a transition
/// between states in a workflow execution.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateTransitionEvent {
    pub event: String,
    #[serde(rename = "prevState")]
    pub prev_state: Option<WorkflowExecutionState>,
    #[serde(rename = "newState")]
    pub new_state: WorkflowExecutionState,
}

/// An event that is emitted when a state in a workflow execution
/// fails.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateFailureEvent {
    pub event: String,
    #[serde(rename = "failedState")]
    pub failed_state: WorkflowExecutionState,
}

/// An event that is emitted when a state in a workflow execution
/// is retried.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateRetryEvent {
    pub event: String,
    #[serde(rename = "retryState")]
    pub retry_state: WorkflowExecutionState,
    #[serde(rename = "prevAttemptStates")]
    pub prev_attempt_states: Vec<WorkflowExecutionState>,
}

/// An event that is emitted when a workflow execution is complete.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionCompleteEvent {
    pub event: String,
    #[serde(rename = "completeExecution")]
    pub complete_execution: WorkflowExecution,
}
