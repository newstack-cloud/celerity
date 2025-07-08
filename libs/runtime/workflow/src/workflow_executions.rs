use std::collections::HashMap;
use std::fmt;
use std::sync::{Arc, Mutex, RwLock};
use std::{error::Error, fmt::Debug};

use async_trait::async_trait;
use celerity_blueprint_config_parser::blueprint::CelerityWorkflowStateType;
use serde::{Deserialize, Serialize};
use serde_json::Value;

/// A trait for a service that handles persistence of workflow executions.
/// This is to be used to provide the functionality to store, update and retrieve
/// workflow executions.
#[async_trait]
pub trait WorkflowExecutionService {
    /// Saves a workflow execution resource for a provided workflow execution ID.
    /// If the workflow execution resource already exists, it will be updated.
    ///
    /// The following is an example of how you might call this method:
    /// ```
    /// # use workflow::workflow_executions::{SaveWorkflowExecutionPayload, WorkflowExecutionService};
    ///
    /// let saved_workflow_execution = service.save_workflow_execution(
    ///    "7e4f50be-6ecf-4ab3-8974-f65964615c44".to_string(),
    ///     SaveWorkflowExecutionPayload {
    ///         started: 1728232655000,
    ///         completed: Some(1728232657000),
    ///         duration: Some(2),
    ///         status: "completed".to_string(),
    ///         status_detail: "Workflow execution completed successfully".to_string(),
    ///         current_state: "uploadProcessedDocument".to_string(),
    ///         states: updated_states,
    ///     },
    /// )?;
    /// ```
    async fn save_workflow_execution(
        &self,
        id: String,
        payload: SaveWorkflowExecutionPayload,
    ) -> Result<WorkflowExecution, WorkflowExecutionServiceError>;

    /// Retrieves a workflow execution resource for a provided workflow execution ID.
    /// If the workflow execution resource does not exist, a `WorkflowExecutionServiceError::NotFound`
    /// error will be returned.
    ///
    /// The following is an example of how you might call this method:
    /// ```
    /// # use workflow::workflow_executions::WorkflowExecutionService;
    ///
    /// let workflow_execution = service.get_workflow_execution(
    ///     "7e4f50be-6ecf-4ab3-8974-f65964615c44".to_string(),
    /// )?;
    /// ```
    async fn get_workflow_execution(
        &self,
        id: String,
    ) -> Result<WorkflowExecution, WorkflowExecutionServiceError>;

    /// Retrieves the most recent workflow executions up to the provided limit.
    /// This method is not designed to support pagination, all requested workflow
    /// executions will be returned in a single response.
    /// This means that the `limit` parameter should be used carefully to avoid
    /// performance issues.
    ///
    /// The following is an example of how you might call this method:
    /// ```
    /// # use workflow::workflow_executions::WorkflowExecutionService;
    ///
    /// let workflow_executions = service.get_latest_workflow_executions(10)?;
    /// ```
    async fn get_latest_workflow_executions(
        &self,
        limit: u64,
    ) -> Result<Vec<WorkflowExecution>, WorkflowExecutionServiceError>;
}

impl Debug for dyn WorkflowExecutionService + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "WorkflowExecutionService")
    }
}

/// The error type used for workflow execution service
/// implementations.
#[derive(Debug)]
pub enum WorkflowExecutionServiceError {
    NotFound(String),
    InternalError(Box<dyn Error + Send + Sync + 'static>),
}

impl fmt::Display for WorkflowExecutionServiceError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            WorkflowExecutionServiceError::NotFound(execution_id) => {
                write!(f, "workflow execution \"{execution_id}\" not found")
            }
            WorkflowExecutionServiceError::InternalError(error) => {
                write!(f, "internal error: {error}")
            }
        }
    }
}

/// The payload to save a workflow execution resource
/// for a provided workflow execution ID.
#[derive(Debug, Serialize, Deserialize)]
pub struct SaveWorkflowExecutionPayload {
    pub input: Value,
    pub started: u64,
    pub completed: Option<u64>,
    pub duration: Option<f64>,
    pub status: WorkflowExecutionStatus,
    pub status_detail: String,
    pub current_state: Option<String>,
    pub states: Vec<WorkflowExecutionState>,
}

/// Holds the state of a workflow execution.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct WorkflowExecution {
    pub id: String,
    pub input: Value,
    pub output: Option<Value>,
    pub started: u64,
    pub completed: Option<u64>,
    pub duration: Option<f64>,
    pub status: WorkflowExecutionStatus,
    #[serde(rename = "statusDetail")]
    pub status_detail: String,
    #[serde(rename = "currentState")]
    pub current_state: Option<String>,
    pub states: Vec<WorkflowExecutionState>,
}

/// Holds information about an individual state within a workflow execution.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct WorkflowExecutionState {
    pub name: String,
    // Store the state type for historical purposes,
    // especially useful if a state in a workflow application
    // is updated to a new state type.
    #[serde(rename = "type")]
    pub state_type: CelerityWorkflowStateType,
    // The parent state name if the state is a child of a parallel state.
    pub parent: Option<String>,
    pub input: Value,
    pub started: u64,
    pub completed: Option<u64>,
    pub duration: Option<f64>,
    pub status: WorkflowExecutionStatus,
    pub attempt: u32,
    pub error: Option<String>,
    pub parallel: Vec<Vec<WorkflowExecutionState>>,
    #[serde(rename = "rawOutput")]
    pub raw_output: Option<Value>,
    pub output: Option<Value>,
}

/// The status of a workflow execution or an individual state
/// within a workflow execution.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum WorkflowExecutionStatus {
    #[serde(rename = "PREPARING")]
    Preparing,
    #[serde(rename = "IN_PROGRESS")]
    InProgress,
    #[serde(rename = "FAILED")]
    Failed,
    #[serde(rename = "SUCCEEDED")]
    Succeeded,
}

/// An in-memory implementation of the `WorkflowExecutionService` trait.
/// This is intended to be used in test and sandbox environments;
/// this should not be used in production.
#[derive(Default)]
pub struct MemoryWorkflowExecutionService {
    executions: Arc<RwLock<HashMap<String, WorkflowExecution>>>,
    execution_ids_in_order: Arc<Mutex<Vec<String>>>,
}

impl MemoryWorkflowExecutionService {
    /// Creates a new `MemoryWorkflowExecutionService` instance.
    pub fn new() -> Self {
        MemoryWorkflowExecutionService {
            executions: Arc::new(RwLock::new(HashMap::new())),
            execution_ids_in_order: Arc::new(Mutex::new(Vec::new())),
        }
    }
}

#[async_trait]
impl WorkflowExecutionService for MemoryWorkflowExecutionService {
    async fn save_workflow_execution(
        &self,
        id: String,
        payload: SaveWorkflowExecutionPayload,
    ) -> Result<WorkflowExecution, WorkflowExecutionServiceError> {
        let execution = WorkflowExecution {
            id: id.clone(),
            input: payload.input,
            output: None,
            started: payload.started,
            completed: payload.completed,
            duration: payload.duration,
            status: payload.status,
            status_detail: payload.status_detail,
            current_state: payload.current_state,
            states: payload.states,
        };
        self.executions
            .write()
            .expect("lock should not be poisoned")
            .insert(id.clone(), execution.clone());

        let mut executions_in_order = self
            .execution_ids_in_order
            .lock()
            .expect("lock should not be poisoned");

        if !executions_in_order.contains(&id) {
            executions_in_order.push(id.clone());
        }

        Ok(execution)
    }

    async fn get_workflow_execution(
        &self,
        id: String,
    ) -> Result<WorkflowExecution, WorkflowExecutionServiceError> {
        match self
            .executions
            .read()
            .expect("lock should not be poisoned")
            .get(&id)
        {
            Some(execution) => Ok(execution.clone()),
            None => Err(WorkflowExecutionServiceError::NotFound(id)),
        }
    }

    async fn get_latest_workflow_executions(
        &self,
        limit: u64,
    ) -> Result<Vec<WorkflowExecution>, WorkflowExecutionServiceError> {
        let execution_ids = self
            .execution_ids_in_order
            .lock()
            .expect("lock should not be poisoned");

        let results_size = if limit > execution_ids.len() as u64 {
            execution_ids.len()
        } else {
            limit as usize
        };

        let result_ids = execution_ids
            .iter()
            .rev()
            .take(results_size)
            .cloned()
            // Reverse back to original order.
            .rev()
            .collect::<Vec<String>>();

        let executions = self.executions.read().expect("lock should not be poisoned");
        let results = result_ids
            .iter()
            .filter_map(|id| executions.get(id).cloned())
            .collect();

        Ok(results)
    }
}
