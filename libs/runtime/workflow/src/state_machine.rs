use std::time::Duration;

use async_recursion::async_recursion;
use celerity_blueprint_config_parser::blueprint::{
    CelerityWorkflowCatchConfig, CelerityWorkflowRetryConfig, CelerityWorkflowState,
    CelerityWorkflowStateType,
};
use serde_json::{json, Value};
use tokio::{sync::Mutex, time::sleep};
use tracing::error;

use crate::{
    consts::{DEFAULT_STATE_RETRY_BACKOFF_RATE, DEFAULT_STATE_RETRY_INTERVAL_SECONDS},
    helpers::calculate_retry_wait_time_ms,
    types::{
        ExecutionCompleteEvent, StateFailureEvent, StateTransitionEvent, WorkflowAppState,
        WorkflowExecutionEvent,
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

        self.execute_state_handle_error(
            self.workflow_app.workflow_spec.start_at.clone(),
            &workflow_input,
            None,
            None,
        )
        .await;
    }

    #[async_recursion]
    async fn execute_state_handle_error(
        &self,
        state_name: String,
        input: &Value,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state: Option<String>,
    ) {
        let result = self
            .execute_state(
                self.workflow_app.workflow_spec.start_at.clone(),
                input,
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
                        err_info.parent_state_name,
                        "PersistFailed".to_string(),
                        err_info.error_message,
                        // Record error without trying to persist changes, as it will likely fail.
                        false,
                        // Persistence failure at this level is not recoverable,
                        // fault tolerant behaviour should be a part of the "WorkflowExecutionService" implementation.
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
                        err_info.parent_state_name,
                        "StateNotFound".to_string(),
                        err_info.error_message,
                        true,
                        false,
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
                        err_info.parent_state_name,
                        "InvalidState".to_string(),
                        err_info.error_message,
                        true,
                        false,
                    )
                    .await;
                }
                ExecuteWorkflowStateError::InvalidResultPath(err_info) => {
                    // This error occurs when the result path in an `executeStep`, `pass` or `parallel` state
                    // is invalid and cannot be used to inject the output of the state into the input of the next state.
                    error!(
                        state = err_info.state_name.as_str(),
                        parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
                        error_name = "InvalidResultPath",
                        "invalid result path in state configuration: {}",
                        err_info.error_message
                    );
                    self.record_error(
                        err_info.state_name,
                        err_info.parent_state_name,
                        "InvalidResultPath".to_string(),
                        err_info.error_message,
                        true,
                        false,
                    )
                    .await;
                }
                ExecuteWorkflowStateError::ExecuteStepHandlerFailed(err_info) => {
                    self.handle_execute_step_error(err_info, input).await;
                }
            },
        }
    }

    async fn execute_state(
        &self,
        state_name: String,
        input: &Value,
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
                return Err(ExecuteWorkflowStateError::InvalidState(
                    WorkflowStateErrorInfo {
                        state_name: state_name.clone(),
                        parent_state_name: parent_state,
                        error_name: None,
                        error_message: format!(
                            "Unsupported state type: {:?}",
                            state_config.state_type
                        ),
                    },
                ));
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
                        error_name: None,
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
                Err(err) => {
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
                Ok(())
            }
            Err(err) => Err(ExecuteWorkflowStateError::ExecuteStepHandlerFailed(
                WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: parent_state,
                    error_name: Some(err.name.clone()),
                    error_message: format!("Execute step failed: {}", err),
                },
            )),
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
                    return find_parallel_child_state_config(
                        parent_state_name,
                        parent_state,
                        state_name,
                    );
                }
                None => {
                    return Err(ExecuteWorkflowStateError::StateNotFound(
                        WorkflowStateErrorInfo {
                            state_name: state_name.clone(),
                            parent_state_name: Some(parent_state_name),
                            error_name: None,
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
                        error_name: None,
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
                    error_name: None,
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

    async fn handle_execute_step_error(&self, err_info: WorkflowStateErrorInfo, input: &Value) {
        let error_name = err_info
            .error_name
            .clone()
            .unwrap_or_else(|| "HandlerFailed".to_string());

        error!(
            state = err_info.state_name.as_str(),
            parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
            error_name = error_name.clone(),
            "failed to execute step handler: {}",
            err_info.error_message
        );

        let retry_config_opt = self
            .workflow_app
            .workflow_spec
            .states
            .get(&err_info.state_name)
            .and_then(|state| state.retry.as_ref());

        let matching_retry_config = match retry_config_opt {
            Some(retry_config) => retry_config
                .iter()
                .find(|config| config.match_errors.contains(&error_name)),
            None => None,
        };

        let catch_config_opt = self
            .workflow_app
            .workflow_spec
            .states
            .get(&err_info.state_name)
            .and_then(|state| state.catch.as_ref());

        let (attempts_so_far, exceeded_max_attempts) = self
            .check_exceeded_max_attempts(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
                matching_retry_config,
            )
            .await;

        self.record_error(
            err_info.state_name.clone(),
            err_info.parent_state_name.clone(),
            error_name,
            err_info.error_message.clone(),
            true,
            matching_retry_config.is_some() && !exceeded_max_attempts,
        )
        .await;

        let result = self
            .retry_or_catch(
                &err_info,
                matching_retry_config,
                catch_config_opt,
                attempts_so_far,
                exceeded_max_attempts,
                input,
            )
            .await;

        if let Err(retry_or_catch_err) = result {
            self.handle_retry_or_catch_error(retry_or_catch_err).await;
        }
    }

    async fn retry_or_catch(
        &self,
        err_info: &WorkflowStateErrorInfo,
        matching_retry_config: Option<&CelerityWorkflowRetryConfig>,
        catch_config: Option<&Vec<CelerityWorkflowCatchConfig>>,
        attempts_so_far: i64,
        exceeded_max_attempts: bool,
        input: &Value,
    ) -> Result<(), ExecuteWorkflowStateError> {
        if let Some(retry_config) = matching_retry_config {
            if !exceeded_max_attempts {
                let wait_time_ms = calculate_retry_wait_time_ms(
                    retry_config,
                    attempts_so_far,
                    DEFAULT_STATE_RETRY_INTERVAL_SECONDS,
                    DEFAULT_STATE_RETRY_BACKOFF_RATE,
                );
                sleep(Duration::from_millis(wait_time_ms)).await;

                // Take a copy of the state to avoid holding a lock on the state machine's workflow
                // execution data while retrying the state, holding the lock would cause a deadlock.
                let state = self
                    .extract_state(
                        err_info.state_name.clone(),
                        err_info.parent_state_name.clone(),
                    )
                    .await;

                self.execute_state_handle_error(
                    err_info.state_name.clone(),
                    input,
                    state.as_ref(),
                    err_info.parent_state_name.clone(),
                )
                .await;

                return Ok(());
            }
        }

        // The error cannot be retried, next up is to check if the error can be caught.
        // If the error can be caught, the state machine will transition to the `next` state
        // defined in the catcher.
        self.catch(catch_config, err_info, input).await
    }

    async fn catch(
        &self,
        catch_config: Option<&Vec<CelerityWorkflowCatchConfig>>,
        err_info: &WorkflowStateErrorInfo,
        input: &Value,
    ) -> Result<(), ExecuteWorkflowStateError> {
        if let Some(catch_config) = catch_config {
            let matching_catcher_opt = catch_config.iter().find(|config| {
                config.match_errors.contains(
                    &err_info
                        .error_name
                        .clone()
                        .unwrap_or_else(|| "".to_string()),
                )
            });

            if let Some(matching_catcher) = matching_catcher_opt {
                let state_name = matching_catcher.next.clone();
                let state = self
                    .extract_state(state_name.clone(), err_info.parent_state_name.clone())
                    .await;

                let input_from_catch =
                    self.prepare_input_from_catch(matching_catcher, err_info, input)?;

                self.execute_state_handle_error(
                    state_name,
                    &input_from_catch,
                    state.as_ref(),
                    err_info.parent_state_name.clone(),
                )
                .await;
            }
        }

        Ok(())
    }

    async fn handle_retry_or_catch_error(&self, retry_or_catch_err: ExecuteWorkflowStateError) {
        // todo: handle retry or catch error
    }

    // This should be used only when necessary as it will clone the
    // state in the workflow.
    async fn extract_state(
        &self,
        state_name: String,
        parent_state_name: Option<String>,
    ) -> Option<WorkflowExecutionState> {
        let workflow_execution = self.workflow_execution.lock().await;
        extract_state_from_workflow_execution(state_name, &workflow_execution, parent_state_name)
            .cloned()
    }

    async fn check_exceeded_max_attempts(
        &self,
        state_name: String,
        parent_state_name: Option<String>,
        retry_config_opt: Option<&CelerityWorkflowRetryConfig>,
    ) -> (i64, bool) {
        let mut workflow_execution = self.workflow_execution.lock().await;
        let state = extract_state_from_workflow_execution(
            state_name.clone(),
            &mut workflow_execution,
            parent_state_name.clone(),
        );

        if let Some(state) = state {
            if let Some(retry_config) = retry_config_opt {
                if let Some(max_attempts) = retry_config.max_attempts {
                    let attempts = i64::from(state.attempt);
                    if attempts + 1 >= max_attempts {
                        return (attempts, true);
                    }
                }
            }
        }

        (0, false)
    }

    async fn record_error(
        &self,
        state_name: String,
        parent_state_name: Option<String>,
        error_name: String,
        error_message: String,
        persist: bool,
        can_retry: bool,
    ) {
        // Only acquire a lock to update error state and take a copy of the current state to persist
        // and broadcast.
        // It's important to avoid holding the lock while persisting state and broadcasting events to
        // prevent significant blocking when executing parallel branches.
        let (captured_workflow_execution, captured_state) = {
            let mut workflow_execution = self.workflow_execution.lock().await;
            let status_detail = format!(
                "Error executing state \"{}\" [{}]: {}",
                state_name, error_name, error_message
            );

            if !can_retry {
                workflow_execution.status = WorkflowExecutionStatus::Failed;
                workflow_execution.status_detail = status_detail.clone();
            }

            let state = extract_state_from_workflow_execution_mut(
                state_name.clone(),
                &mut workflow_execution,
                parent_state_name.clone(),
            )
            .expect("Failed state must exist in workflow execution");

            state.error = Some(status_detail);
            let captured_state = state.clone();
            (workflow_execution.clone(), captured_state)
        };

        if persist {
            let persist_result = self
                .workflow_app
                .execution_service
                .save_workflow_execution(
                    captured_workflow_execution.id.clone(),
                    SaveWorkflowExecutionPayload {
                        input: captured_workflow_execution.input.clone(),
                        started: captured_workflow_execution.started,
                        completed: captured_workflow_execution.completed,
                        duration: captured_workflow_execution.duration,
                        status: captured_workflow_execution.status.clone(),
                        status_detail: captured_workflow_execution.status_detail.clone(),
                        current_state: captured_workflow_execution.current_state.clone(),
                        states: captured_workflow_execution.states.clone(),
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

        // Broadcast the error event, even if the error will be retried or caught
        // so that clients can report detailed error information in real-time
        // indicating the reason for the failure that lead to the retry or catch.
        self.workflow_app
            .event_broadcaster
            .send(WorkflowExecutionEvent::StateFailure(StateFailureEvent {
                event: "stateFailed".to_string(),
                failed_state: captured_state,
            }));

        if !can_retry {
            // Completion event will contain a workflow execution with the failed status
            // and error information in the status detail field.
            self.workflow_app
                .event_broadcaster
                .send(WorkflowExecutionEvent::ExecutionComplete(
                    ExecutionCompleteEvent {
                        event: "workflowExecutionComplete".to_string(),
                        complete_execution: captured_workflow_execution,
                    },
                ));
        }
    }

    fn prepare_input_from_catch(
        &self,
        catch_config: &CelerityWorkflowCatchConfig,
        err_info: &WorkflowStateErrorInfo,
        input: &Value,
    ) -> Result<Value, ExecuteWorkflowStateError> {
        if let Some(result_path) = &catch_config.result_path {
            self.workflow_app
                .payload_template_engine
                .inject(
                    input,
                    result_path.clone(),
                    json!({
                        "error": err_info.error_name.clone(),
                        "cause": err_info.error_message.clone(),
                    }),
                )
                .map_err(|err| {
                    ExecuteWorkflowStateError::InvalidResultPath(WorkflowStateErrorInfo {
                        state_name: err_info.state_name.clone(),
                        parent_state_name: err_info.parent_state_name.clone(),
                        error_name: err_info.error_name.clone(),
                        error_message: format!(
                            "Failed to inject error information from result path into input: {}",
                            err
                        ),
                    })
                })
        } else {
            Ok(input.clone())
        }
    }
}

fn find_parallel_child_state_config(
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
                            error_name: None,
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
                    error_name: None,
                    error_message: "Parallel state not found in parent state".to_string(),
                },
            ));
        }
        _ => {
            return Err(ExecuteWorkflowStateError::InvalidState(
                WorkflowStateErrorInfo {
                    state_name,
                    parent_state_name: Some(parent_state_name),
                    error_name: None,
                    error_message: "Parent state is not a parallel state".to_string(),
                },
            ));
        }
    }
}

fn extract_state_from_workflow_execution_mut(
    state_name: String,
    workflow_execution: &mut WorkflowExecution,
    parent_state_name: Option<String>,
) -> Option<&mut WorkflowExecutionState> {
    for state in workflow_execution.states.iter_mut() {
        if state.name == state_name && parent_state_name.is_none() {
            return Some(state);
        } else if let Some(parent_name) = &parent_state_name {
            if *parent_name == state.name {
                return find_parallel_child_state_mut(state, state_name.clone());
            }
        }
    }

    return None;
}

fn extract_state_from_workflow_execution(
    state_name: String,
    workflow_execution: &WorkflowExecution,
    parent_state_name: Option<String>,
) -> Option<&WorkflowExecutionState> {
    for state in workflow_execution.states.iter() {
        if state.name == state_name && parent_state_name.is_none() {
            return Some(state);
        } else if let Some(parent_name) = &parent_state_name {
            if *parent_name == state.name {
                return find_parallel_child_state(state, state_name.clone());
            }
        }
    }

    return None;
}

fn find_parallel_child_state_mut(
    parent_state: &mut WorkflowExecutionState,
    state_name: String,
) -> Option<&mut WorkflowExecutionState> {
    match parent_state.state_type {
        CelerityWorkflowStateType::Parallel => {
            let child_state = parent_state
                .parallel
                .iter_mut()
                .filter_map(|branch| {
                    if let Some(state) = branch.iter_mut().find(|state| state.name == state_name) {
                        Some(state)
                    } else {
                        None
                    }
                })
                .next();

            return child_state;
        }
        _ => {
            return None;
        }
    }
}

fn find_parallel_child_state(
    parent_state: &WorkflowExecutionState,
    state_name: String,
) -> Option<&WorkflowExecutionState> {
    match parent_state.state_type {
        CelerityWorkflowStateType::Parallel => {
            let child_state = parent_state
                .parallel
                .iter()
                .filter_map(|branch| {
                    if let Some(state) = branch.iter().find(|state| state.name == state_name) {
                        Some(state)
                    } else {
                        None
                    }
                })
                .next();

            return child_state;
        }
        _ => {
            return None;
        }
    }
}

// Internal error type used for state machine errors.
#[derive(Debug)]
enum ExecuteWorkflowStateError {
    PersistFailed(WorkflowStateErrorInfo),
    StateNotFound(WorkflowStateErrorInfo),
    InvalidState(WorkflowStateErrorInfo),
    ExecuteStepHandlerFailed(WorkflowStateErrorInfo),
    InvalidResultPath(WorkflowStateErrorInfo),
}

// Information about an error that occurred during the execution of a workflow state.
#[derive(Debug)]
struct WorkflowStateErrorInfo {
    state_name: String,
    parent_state_name: Option<String>,
    error_name: Option<String>,
    error_message: String,
}
