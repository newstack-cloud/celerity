use celerity_blueprint_config_parser::blueprint::{
    CelerityWorkflowState, CelerityWorkflowStateType,
};
use serde_json::Value;
use tokio::sync::Mutex;
use tracing::error;

use crate::{
    types::{
        ExecutionCompleteEvent, StateTransitionEvent, WorkflowAppState, WorkflowExecutionEvent,
    },
    workflow_executions::{
        SaveWorkflowExecutionPayload, WorkflowExecution, WorkflowExecutionState,
        WorkflowExecutionStatus,
    },
};

/// The state machine used for a workflow execution.
/// A state machine should be created for each workflow execution
/// and disposed of when the execution ends due to completion or
/// failure.
pub struct StateMachine {
    workflow_app: WorkflowAppState,
    // The current state of the workflow execution.
    // This will be persisted with the execution service
    // for each change in state.
    workflow_execution: Mutex<WorkflowExecution>,
}

impl StateMachine {
    pub fn new(workflow_app: WorkflowAppState, initial_state: WorkflowExecution) -> Self {
        StateMachine {
            workflow_app,
            workflow_execution: Mutex::new(initial_state),
        }
    }

    pub async fn start(&self) {
        let workflow_input = {
            let workflow_execution = self.workflow_execution.lock().await;
            workflow_execution.input.clone()
        };

        let result = self
            .execute_state(
                self.workflow_app.workflow_spec.start_at.clone(),
                workflow_input,
                None,
                None,
            )
            .await;

        match result {
            Ok(_) => {}
            Err(err) => match err {
                ExecuteWorkflowStateError::PersistFailed(err_info) => {
                    error!(
                        state = err_info.state_name.as_str(),
                        parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
                        error_name = "PersistFailed",
                        "failed to persist workflow execution changes, \
                         the currently persisted state is likely to be incorrect: {}",
                        err_info.error_message
                    );
                    self.record_error(
                        err_info.state_name,
                        "PersistFailed".to_string(),
                        err_info.error_message,
                        // Record error without trying to persist changes, as it will likely fail.
                        false,
                    )
                    .await;
                }
                ExecuteWorkflowStateError::StateNotFound(err_info) => {
                    error!(
                        state = err_info.state_name.as_str(),
                        parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
                        error_name = "StateNotFound",
                        "failed to find state in workflow spec: {}",
                        err_info.error_message
                    );
                    self.record_error(
                        err_info.state_name,
                        "StateNotFound".to_string(),
                        err_info.error_message,
                        true,
                    )
                    .await;
                }
                ExecuteWorkflowStateError::InvalidState(err_info) => {
                    error!(
                        state = err_info.state_name.as_str(),
                        parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
                        error_name = "InvalidState",
                        "invalid state configuration: {}",
                        err_info.error_message
                    );
                    self.record_error(
                        err_info.state_name,
                        "InvalidState".to_string(),
                        err_info.error_message,
                        true,
                    )
                    .await;
                }
            },
        }
    }

    async fn execute_state(
        &self,
        state_name: String,
        input: Value,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state: Option<String>,
    ) -> Result<(), ExecuteWorkflowStateError> {
        let state_config = self.derive_state_config(state_name.clone(), parent_state.clone())?;

        let attempt = match prev_state {
            Some(prev_state) => {
                if prev_state.name == state_name {
                    prev_state.attempt + 1
                } else {
                    1
                }
            }
            None => 1,
        };

        let started = self.workflow_app.clock.now_millis();

        let state = WorkflowExecutionState {
            name: state_name.clone(),
            state_type: state_config.state_type.clone(),
            parent: None,
            started,
            input: input.clone(),
            attempt,
            status: WorkflowExecutionStatus::InProgress,
            // For "parallel" states, we'll record the top-level state as in progress
            // before beginning the parallel branches, each parallel branch state will be recorded
            // when they are executed.
            parallel: vec![],
            output: None,
            error: None,
            completed: None,
            duration: None,
        };

        self.record_transition(state_name.clone(), &state, prev_state, parent_state.clone())
            .await?;

        match state_config.state_type {
            CelerityWorkflowStateType::ExecuteStep => {
                self.execute_step(state_name, state, state_config, &input, parent_state)
                    .await?;
            }
            CelerityWorkflowStateType::Parallel => {
                // self.execute_parallel(state_name, state, input, parent_state)
                // .await?;
            }
            CelerityWorkflowStateType::Wait => {
                // self.wait(state_name, state, input, parent_state).await?;
            }
            CelerityWorkflowStateType::Decision => {
                // self.decide(state_name, state, input, parent_state).await?;
            }
            CelerityWorkflowStateType::Pass => {
                // self.pass(state_name, state, input, parent_state).await?;
            }
            CelerityWorkflowStateType::Success => {
                // self.success(state_name, state, input, parent_state).await?;
            }
            CelerityWorkflowStateType::Failure => {
                // self.fail(state_name, state, input, parent_state).await?;
            }
            _ => {
                self.record_error(
                    state_name.clone(),
                    "UnsupportedStateType".to_string(),
                    format!("Unsupported state type: {:?}", state_config.state_type),
                    true,
                )
                .await;
            }
        }

        Ok(())
    }

    async fn execute_step(
        &self,
        state_name: String,
        state: WorkflowExecutionState,
        state_config: &CelerityWorkflowState,
        input: &Value,
        parent_state: Option<String>,
    ) -> Result<(), ExecuteWorkflowStateError> {
        let handlers = self.workflow_app.state_handlers.read().await;

        let handler_opt = handlers.get(&state_name);

        let handler = match handler_opt {
            Some(handler) => handler,
            None => {
                return Err(ExecuteWorkflowStateError::InvalidState(
                    WorkflowStateErrorInfo {
                        state_name: state_name.clone(),
                        parent_state_name: parent_state,
                        error_message: "No handler found for state".to_string(),
                    },
                ));
            }
        };

        let payload = if let Some(template) = &state_config.payload_template {
            let render_result = self
                .workflow_app
                .payload_template_engine
                .render(&template, &input);
            match render_result {
                Ok(rendered) => rendered,
                Err(_err) => {
                    // Persist the error state for the attempt in the workflow execution.
                    // Based on retry config, determine whether to retry the state or fail the execution.
                    // If the state is to be retried, the state machine will be called again with the same state.
                    // If the state is to be failed, the error will be recorded and the execution will be marked as failed.
                    // If the state has catch configuration, the error will be handled according to the catch configuration.
                    // If the error matches one of the catch configurations, the state machine will transition to the next state in the catch config.
                    return Ok(());
                }
            }
        } else {
            input.clone()
        };

        match handler.call(payload).await {
            Ok(output) => {
                // Persist the output and completion of the state.
                // The state machine will transition to the next state in the workflow execution
                // passing the output of the current state as input to the next state.
                return Ok(());
            }
            Err(err) => {
                // Persist the error state for the attempt in the workflow execution.
                // Based on retry config, determine whether to retry the state or fail the execution.
                // If the state is to be retried, the state machine will be called again with the same state.
                // If the state is to be failed, the error will be recorded and the execution will be marked as failed.
                // If the state has catch configuration, the error will be handled according to the catch configuration.
                // If the error matches one of the catch configurations, the state machine will transition to the next state in the catch config.
                return Ok(());
            }
        }
    }

    fn derive_state_config(
        &self,
        state_name: String,
        parent_state: Option<String>,
    ) -> Result<&CelerityWorkflowState, ExecuteWorkflowStateError> {
        if let Some(parent_state_name) = parent_state {
            match self
                .workflow_app
                .workflow_spec
                .states
                .get(&parent_state_name)
            {
                Some(parent_state) => {
                    return find_parallel_child_state(parent_state_name, parent_state, state_name);
                }
                None => {
                    return Err(ExecuteWorkflowStateError::StateNotFound(
                        WorkflowStateErrorInfo {
                            state_name: state_name.clone(),
                            parent_state_name: Some(parent_state_name),
                            error_message: "Parent state not found in workflow spec".to_string(),
                        },
                    ));
                }
            };
        }

        match self.workflow_app.workflow_spec.states.get(&state_name) {
            Some(state) => Ok(state),
            None => {
                return Err(ExecuteWorkflowStateError::StateNotFound(
                    WorkflowStateErrorInfo {
                        state_name: state_name.clone(),
                        parent_state_name: parent_state,
                        error_message: "State not found in workflow spec".to_string(),
                    },
                ));
            }
        }
    }

    async fn record_transition(
        &self,
        state_name: String,
        state: &WorkflowExecutionState,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state_name: Option<String>,
    ) -> Result<(), ExecuteWorkflowStateError> {
        let mut workflow_execution = self.workflow_execution.lock().await;
        workflow_execution.states.push(state.clone());
        workflow_execution.current_state = Some(state_name.clone());
        workflow_execution.status = WorkflowExecutionStatus::InProgress;
        workflow_execution.status_detail = format!("Executing state: {}", state_name);

        // Persist the workflow execution changes before broadcasting events
        // to ensure consistency between clients streaming events and the persisted
        // state of the workflow execution.
        // This will incur a delay in the event being broadcasted to listeners
        // depending on the execution service implementation.
        self.workflow_app
            .execution_service
            .save_workflow_execution(
                workflow_execution.id.clone(),
                SaveWorkflowExecutionPayload {
                    input: workflow_execution.input.clone(),
                    started: workflow_execution.started,
                    completed: workflow_execution.completed,
                    duration: workflow_execution.duration,
                    status: workflow_execution.status.clone(),
                    status_detail: workflow_execution.status_detail.clone(),
                    current_state: workflow_execution.current_state.clone(),
                    states: workflow_execution.states.clone(),
                },
            )
            .await
            .map_err(|err| {
                ExecuteWorkflowStateError::PersistFailed(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name,
                    error_message: format!("Failed to persist state transition: {}", err),
                })
            })?;

        self.workflow_app
            .event_broadcaster
            .send(WorkflowExecutionEvent::StateTransition(
                StateTransitionEvent {
                    event: "stateTransition".to_string(),
                    prev_state: prev_state.cloned(),
                    new_state: state.clone(),
                },
            ));

        Ok(())
    }

    async fn record_error(
        &self,
        state_name: String,
        error_name: String,
        error_message: String,
        persist: bool,
    ) {
        let mut workflow_execution = self.workflow_execution.lock().await;

        workflow_execution.status = WorkflowExecutionStatus::Failed;
        let status_detail = format!(
            "Error executing state \"{}\" [{}]: {}",
            state_name, error_name, error_message
        );
        workflow_execution.status_detail = status_detail.clone();

        let state_opt = workflow_execution.states.last_mut();
        if let Some(state) = state_opt {
            state.error = Some(status_detail);
        }

        if persist {
            let persist_result = self
                .workflow_app
                .execution_service
                .save_workflow_execution(
                    workflow_execution.id.clone(),
                    SaveWorkflowExecutionPayload {
                        input: workflow_execution.input.clone(),
                        started: workflow_execution.started,
                        completed: workflow_execution.completed,
                        duration: workflow_execution.duration,
                        status: workflow_execution.status.clone(),
                        status_detail: workflow_execution.status_detail.clone(),
                        current_state: workflow_execution.current_state.clone(),
                        states: workflow_execution.states.clone(),
                    },
                )
                .await;

            if let Err(err) = persist_result {
                error!(
                    state = state_name.as_str(),
                    error_name = "PersistFailed",
                    "failed to persist workflow execution changes in response to an error, \
                     the currently persisted state is likely to be incorrect: {}",
                    err
                );
            }
        }

        // Completion event will contain a workflow execution with the failed status
        // and error information in the status detail field.
        self.workflow_app
            .event_broadcaster
            .send(WorkflowExecutionEvent::ExecutionComplete(
                ExecutionCompleteEvent {
                    event: "workflowExecutionComplete".to_string(),
                    complete_execution: workflow_execution.clone(),
                },
            ));
    }
}

fn find_parallel_child_state(
    parent_state_name: String,
    parent_state: &CelerityWorkflowState,
    state_name: String,
) -> Result<&CelerityWorkflowState, ExecuteWorkflowStateError> {
    match parent_state.state_type {
        CelerityWorkflowStateType::Parallel => {
            if let Some(parallel_branches) = &parent_state.parallel_branches {
                let child_state = parallel_branches
                    .iter()
                    .filter_map(|branch| {
                        if let Some(state) = branch.states.get(&state_name) {
                            Some(state)
                        } else {
                            None
                        }
                    })
                    .next();

                if let Some(state) = child_state {
                    return Ok(state);
                } else {
                    return Err(ExecuteWorkflowStateError::StateNotFound(
                        WorkflowStateErrorInfo {
                            state_name,
                            parent_state_name: Some(parent_state_name),
                            error_message: "State could not be found in any of the \
                            parent state's parallel branches"
                                .to_string(),
                        },
                    ));
                }
            }

            return Err(ExecuteWorkflowStateError::StateNotFound(
                WorkflowStateErrorInfo {
                    state_name,
                    parent_state_name: Some(parent_state_name),
                    error_message: "Parallel state not found in parent state".to_string(),
                },
            ));
        }
        _ => {
            return Err(ExecuteWorkflowStateError::InvalidState(
                WorkflowStateErrorInfo {
                    state_name,
                    parent_state_name: Some(parent_state_name),
                    error_message: "Parent state is not a parallel state".to_string(),
                },
            ));
        }
    }
}

// Internal error type used for state machine errors.
#[derive(Debug)]
enum ExecuteWorkflowStateError {
    PersistFailed(WorkflowStateErrorInfo),
    StateNotFound(WorkflowStateErrorInfo),
    InvalidState(WorkflowStateErrorInfo),
}

// Information about an error that occurred during the execution of a workflow state.
#[derive(Debug)]
struct WorkflowStateErrorInfo {
    state_name: String,
    parent_state_name: Option<String>,
    error_message: String,
}
