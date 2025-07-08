use std::{fmt, str::FromStr, sync::Arc, time::Duration};

use async_recursion::async_recursion;
use celerity_blueprint_config_parser::blueprint::{
    CelerityWorkflowCatchConfig, CelerityWorkflowRetryConfig, CelerityWorkflowState,
    CelerityWorkflowStateType, CelerityWorkflowWaitConfig,
};
use chrono::DateTime;
use jsonpath_rust::JsonPath;
use serde_json::{json, Value};
use tokio::{
    sync::Mutex,
    task::{JoinError, JoinHandle},
    time::{sleep, Instant},
};
use tracing::error;

use crate::{
    consts::{DEFAULT_STATE_RETRY_BACKOFF_RATE, DEFAULT_STATE_RETRY_INTERVAL_SECONDS},
    helpers::{as_fractional_seconds, calculate_retry_wait_time_ms},
    payload_template::PayloadTemplateEngineError,
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
    full_duration_instant: Mutex<Option<Instant>>,
}

impl StateMachine {
    pub fn new(workflow_app: WorkflowAppState, initial_state: WorkflowExecution) -> Self {
        StateMachine {
            workflow_app,
            workflow_execution: Mutex::new(initial_state),
            full_duration_instant: Mutex::new(None),
        }
    }

    pub async fn start(self: Arc<Self>) {
        let workflow_input = {
            let workflow_execution = self.workflow_execution.lock().await;
            workflow_execution.input.clone()
        };

        {
            self.full_duration_instant
                .lock()
                .await
                .replace(Instant::now());
        };

        let workflow_app = self.workflow_app.clone();

        self.execute_state_and_handle_error(
            workflow_app.workflow_spec.start_at.clone(),
            &workflow_input,
            None,
            None,
        )
        .await;
    }

    #[async_recursion]
    async fn execute_state_and_handle_error(
        self: Arc<Self>,
        state_name: String,
        input: &Value,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state: Option<String>,
    ) -> Option<Value> {
        let result = self
            .clone()
            .execute_state(state_name, input, prev_state, parent_state)
            .await;

        match result {
            Ok(output) => Some(output),
            Err(err) => {
                match err {
                    StateMachineError::PersistFailed(err_info) => {
                        self.log_and_record_error(
                            "PersistFailed",
                            err_info,
                            "failed to persist workflow execution changes, \
                            the currently persisted state is likely to be incorrect:",
                            // Record error without trying to persist changes, as it will likely fail.
                            false,
                            // Persistence failure at this level is not recoverable,
                            // fault tolerant behaviour should be a part of the "WorkflowExecutionService" implementation.
                            false,
                        )
                        .await;
                    }
                    StateMachineError::StateNotFound(err_info) => {
                        self.log_and_record_error(
                            "StateNotFound",
                            err_info,
                            "failed to find state in workflow spec:",
                            true,
                            false,
                        )
                        .await;
                    }
                    StateMachineError::InvalidState(err_info) => {
                        self.log_and_record_error(
                            "InvalidState",
                            err_info,
                            "invalid state configuration:",
                            true,
                            false,
                        )
                        .await;
                    }
                    StateMachineError::InvalidPayloadTemplate(err_info) => {
                        self.handle_catchable_error(
                            err_info,
                            input,
                            "InvalidPayloadTemplate".to_string(),
                        )
                        .await;
                    }
                    StateMachineError::PayloadTemplateFailure(err_info) => {
                        self.handle_catchable_error(
                            err_info,
                            input,
                            "PayloadTemplateFailure".to_string(),
                        )
                        .await;
                    }
                    StateMachineError::InvalidInputPath(err_info) => {
                        // Input path errors are not retryable, this differs from result path or output path
                        // where a state handler may be able to recover from producing an unexpected output.
                        // There is no way to recover from an input path error other than rewinding the state machine
                        // which is not supported in the current specification of a workflow state machine.
                        self.handle_catchable_error(
                            err_info,
                            input,
                            "InvalidInputPath".to_string(),
                        )
                        .await;
                    }
                    StateMachineError::InvalidResultPath(err_info) => {
                        // This error occurs when the result path in an `executeStep`, `pass` or `parallel` state
                        // is invalid and cannot be used to inject the output of the state into the input of the next state.
                        self.handle_retryable_error(
                            err_info,
                            input,
                            "InvalidResultPath".to_string(),
                        )
                        .await;
                    }
                    StateMachineError::InvalidOutputPath(err_info) => {
                        self.handle_retryable_error(
                            err_info,
                            input,
                            "InvalidOutputPath".to_string(),
                        )
                        .await;
                    }
                    StateMachineError::ExecuteStepHandlerFailed(err_info) => {
                        self.handle_retryable_error(err_info, input, "HandlerFailed".to_string())
                            .await;
                    }
                    StateMachineError::ParallelBranchesFailed(err_info) => {
                        self.handle_retryable_error(
                            err_info,
                            input,
                            "ParallelBranchesFailed".to_string(),
                        )
                        .await;
                    }
                }
                // None indicates to the caller that an error occurred and has been handled.
                // This return value is used in parallel branches to be able to make a decision
                // as to whether the parallel state has failed or succeeded.
                None
            }
        }
    }

    async fn execute_state(
        self: Arc<Self>,
        state_name: String,
        input: &Value,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state: Option<String>,
    ) -> Result<Value, StateMachineError> {
        let state_config = self
            .clone()
            .derive_state_config(state_name.clone(), parent_state.clone())?;

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

        let final_input = self.clone().prepare_state_input(
            state_name.clone(),
            parent_state.clone(),
            &state_config,
            input,
        )?;

        // State start time is captured before persisting the initial state,
        // this means the duration of the state will include the time it takes
        // for the state to complete.
        // This will not produce an accurate duration in regards to the performance
        // of a state handler written by the user.
        // For the early versions of the state machine, waiting to persist the state
        // transition changes before executing the state was deemed to make it easier to ensure
        // consistency between the persisted state and the state machine's in-memory state.
        let started = self.clone().workflow_app.clock.now_millis();
        // Use an instant to capture a more accurate duration,
        // this is captured separately from started and completed timestamps as instants
        // do not yield timestamps, only time elapsed since the instant was created.
        let instant_for_duration = Instant::now();

        let state = WorkflowExecutionState {
            name: state_name.clone(),
            state_type: state_config.state_type.clone(),
            parent: None,
            started,
            input: final_input,
            attempt,
            status: WorkflowExecutionStatus::InProgress,
            // For "parallel" states, we'll record the top-level state as in progress
            // before beginning the parallel branches, each parallel branch state will be recorded
            // when they are executed.
            parallel: vec![],
            raw_output: None,
            output: None,
            error: None,
            completed: None,
            duration: None,
        };

        self.clone()
            .record_transition(state_name.clone(), &state, prev_state, parent_state.clone())
            .await?;

        match state_config.state_type {
            CelerityWorkflowStateType::ExecuteStep => {
                self.execute_step(
                    state_name,
                    state,
                    &state_config,
                    input,
                    parent_state,
                    instant_for_duration,
                )
                .await
            }
            CelerityWorkflowStateType::Parallel => {
                self.execute_parallel(
                    state_name,
                    state,
                    &state_config,
                    input,
                    parent_state,
                    instant_for_duration,
                )
                .await
            }
            CelerityWorkflowStateType::Wait => {
                self.wait(
                    state_name,
                    state,
                    &state_config,
                    input,
                    parent_state,
                    instant_for_duration,
                )
                .await
            }
            CelerityWorkflowStateType::Decision => {
                // self.decide(state_name, state, input, parent_state).await?;
                Ok(json!({}))
            }
            CelerityWorkflowStateType::Pass => {
                // self.pass(state_name, state, input, parent_state).await?;
                Ok(json!({}))
            }
            CelerityWorkflowStateType::Success => {
                // self.success(state_name, state, input, parent_state).await?;
                Ok(json!({}))
            }
            CelerityWorkflowStateType::Failure => {
                // self.fail(state_name, state, input, parent_state).await?;
                Ok(json!({}))
            }
            _ => {
                let duration = as_fractional_seconds(instant_for_duration.elapsed());
                Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: parent_state,
                    error_name: None,
                    error_message: format!("Unsupported state type: {:?}", state_config.state_type),
                    duration: Some(duration),
                }))
            }
        }
    }

    async fn execute_step(
        self: Arc<Self>,
        state_name: String,
        state: WorkflowExecutionState,
        state_config: &CelerityWorkflowState,
        input: &Value,
        parent_state: Option<String>,
        instant_for_duration: Instant,
    ) -> Result<Value, StateMachineError> {
        let self_ref = Arc::clone(&self);
        let handlers = self_ref.workflow_app.state_handlers.read().await;

        let handler_opt = handlers.get(&state_name);

        let handler = match handler_opt {
            Some(handler) => handler,
            None => {
                let duration = as_fractional_seconds(instant_for_duration.elapsed());
                return Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: parent_state,
                    error_name: None,
                    error_message: "No handler found for state".to_string(),
                    duration: Some(duration),
                }));
            }
        };

        let payload = if let Some(template) = &state_config.payload_template {
            let render_result = self
                .workflow_app
                .payload_template_engine
                .render(template, input);
            match render_result {
                Ok(rendered) => rendered,
                Err(err) => {
                    let duration = as_fractional_seconds(instant_for_duration.elapsed());
                    return Err(from_payload_template_engine_error(
                        state_name,
                        parent_state,
                        err,
                        duration,
                    ));
                }
            }
        } else {
            input.clone()
        };

        match handler.call(payload).await {
            Ok(output) => {
                self.clone()
                    .handle_state_success(
                        state_name.clone(),
                        parent_state.clone(),
                        state_config,
                        state,
                        input,
                        output,
                        instant_for_duration,
                    )
                    .await
            }
            Err(err) => {
                let duration = as_fractional_seconds(instant_for_duration.elapsed());
                Err(StateMachineError::ExecuteStepHandlerFailed(
                    WorkflowStateErrorInfo {
                        state_name,
                        parent_state_name: parent_state,
                        error_name: Some(err.name.clone()),
                        error_message: format!("Execute step failed: {err}"),
                        duration: Some(duration),
                    },
                ))
            }
        }
    }

    #[allow(clippy::too_many_arguments)]
    async fn handle_state_success(
        self: Arc<Self>,
        state_name: String,
        parent_state: Option<String>,
        state_config: &CelerityWorkflowState,
        state: WorkflowExecutionState,
        input: &Value,
        output: Value,
        instant_for_duration: Instant,
    ) -> Result<Value, StateMachineError> {
        let duration = as_fractional_seconds(instant_for_duration.elapsed());
        let final_output = self.clone().prepare_state_output(
            state_name.clone(),
            parent_state.clone(),
            state_config,
            input,
            &output,
            duration,
        )?;
        self.clone()
            .record_completed_state(
                state_name.clone(),
                duration,
                &output,
                &final_output,
                parent_state.clone(),
            )
            .await?;

        let is_end = state_config.end.unwrap_or(false);
        if let Some(next) = &state_config.next {
            self.execute_state_and_handle_error(
                next.clone(),
                &final_output,
                Some(&state),
                parent_state,
            )
            .await;
        } else if is_end {
            self.record_completed_workflow_execution().await?;
        } else {
            Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name,
                parent_state_name: parent_state,
                error_name: None,
                error_message: "State is missing next or end field".to_string(),
                duration: Some(duration),
            }))?;
        }
        Ok(final_output)
    }

    async fn execute_parallel(
        self: Arc<Self>,
        state_name: String,
        state: WorkflowExecutionState,
        state_config: &CelerityWorkflowState,
        input: &Value,
        parent_state: Option<String>,
        instant_for_duration: Instant,
    ) -> Result<Value, StateMachineError> {
        let parallel_states = state_config.parallel_branches.as_ref().ok_or_else(|| {
            let duration = as_fractional_seconds(instant_for_duration.elapsed());
            StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: parent_state.clone(),
                error_name: None,
                error_message: "Parallel state is missing parallel_branches field".to_string(),
                duration: Some(duration),
            })
        })?;

        if parallel_states.is_empty() {
            let duration = as_fractional_seconds(instant_for_duration.elapsed());
            return Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: parent_state.clone(),
                error_name: None,
                error_message: "Parallel state has no parallel branches".to_string(),
                duration: Some(duration),
            }));
        }

        let mut parallel_state_tasks = vec![];
        for parallel_state in parallel_states {
            let start_at = parallel_state.start_at.clone();
            let start_state_config_opt = parallel_state.states.get(&start_at);
            if start_state_config_opt.is_some() {
                let task = tokio::spawn({
                    let me = Arc::clone(&self);
                    let task_input = input.clone();
                    let task_state_name = state_name.clone();
                    let parent_state_name = parent_state.clone();
                    async move {
                        me.execute_state_and_handle_error(
                            start_at,
                            &task_input,
                            None,
                            Some(task_state_name.clone()),
                        )
                        .await
                        .ok_or_else(|| {
                            let duration = as_fractional_seconds(instant_for_duration.elapsed());
                            StateMachineError::InvalidState(WorkflowStateErrorInfo {
                                state_name: task_state_name,
                                parent_state_name,
                                error_name: None,
                                error_message: "Parallel state branch failed".to_string(),
                                duration: Some(duration),
                            })
                        })
                    }
                });
                parallel_state_tasks.push(task);
            } else {
                let duration = as_fractional_seconds(instant_for_duration.elapsed());
                let err = StateMachineError::InvalidState(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: parent_state.clone(),
                    error_name: None,
                    error_message: "Parallel state branch startAt state is missing".to_string(),
                    duration: Some(duration),
                });
                let task: JoinHandle<Result<Value, StateMachineError>> =
                    tokio::task::spawn(async move { Err(err) });
                parallel_state_tasks.push(task);
            }
        }

        let results = futures::future::join_all(parallel_state_tasks).await;
        self.handle_parallel_results(
            state_name,
            state,
            state_config,
            input,
            results,
            instant_for_duration,
        )
        .await
    }

    async fn wait(
        self: Arc<Self>,
        state_name: String,
        state: WorkflowExecutionState,
        state_config: &CelerityWorkflowState,
        input: &Value,
        parent_state: Option<String>,
        instant_for_duration: Instant,
    ) -> Result<Value, StateMachineError> {
        if let Some(wait_time_config) = state_config.wait_config.as_ref() {
            let wait_time = derive_wait_time_seconds(
                state_name.clone(),
                wait_time_config,
                input,
                &instant_for_duration,
                self.workflow_app.clock.now_millis() as i64,
            )?;

            sleep(Duration::from_secs(wait_time)).await;

            self.handle_state_success(
                state_name,
                parent_state,
                state_config,
                state,
                input,
                input.clone(),
                instant_for_duration,
            )
            .await
        } else {
            let duration = as_fractional_seconds(instant_for_duration.elapsed());
            Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: parent_state.clone(),
                error_name: None,
                error_message: "Wait state is missing wait_config field".to_string(),
                duration: Some(duration),
            }))
        }
    }

    async fn handle_parallel_results(
        self: Arc<Self>,
        state_name: String,
        state: WorkflowExecutionState,
        state_config: &CelerityWorkflowState,
        input: &Value,
        results: Vec<Result<Result<Value, StateMachineError>, JoinError>>,
        instant_for_duration: Instant,
    ) -> Result<Value, StateMachineError> {
        let mut succeeded = true;
        let mut result_values = vec![];
        let mut error_messages = vec![];
        for result in results {
            match result {
                Ok(Ok(value)) => {
                    result_values.push(value);
                }
                Ok(Err(err)) => {
                    error_messages.push(err.to_string());
                    succeeded = false;
                }
                Err(join_err) => {
                    error_messages.push(join_err.to_string());
                    succeeded = false;
                }
            }
        }

        if !succeeded {
            let duration = as_fractional_seconds(instant_for_duration.elapsed());
            return Err(StateMachineError::ParallelBranchesFailed(
                WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: None,
                    error_name: Some("BranchesFailed".to_string()),
                    error_message: error_messages.join(", "),
                    duration: Some(duration),
                },
            ));
        }

        let output = Value::Array(result_values);
        self.handle_state_success(
            state_name,
            None,
            state_config,
            state,
            input,
            output,
            instant_for_duration,
        )
        .await
    }

    fn derive_state_config(
        self: Arc<Self>,
        state_name: String,
        parent_state: Option<String>,
    ) -> Result<CelerityWorkflowState, StateMachineError> {
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
                    return Err(StateMachineError::StateNotFound(WorkflowStateErrorInfo {
                        state_name: state_name.clone(),
                        parent_state_name: Some(parent_state_name),
                        error_name: None,
                        error_message: "Parent state not found in workflow spec".to_string(),
                        duration: None,
                    }));
                }
            };
        }

        match self.workflow_app.workflow_spec.states.get(&state_name) {
            Some(state) => Ok(state.clone()),
            None => Err(StateMachineError::StateNotFound(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: parent_state,
                error_name: None,
                error_message: "State not found in workflow spec".to_string(),
                duration: None,
            })),
        }
    }

    async fn record_transition(
        self: Arc<Self>,
        state_name: String,
        state: &WorkflowExecutionState,
        prev_state: Option<&WorkflowExecutionState>,
        parent_state_name: Option<String>,
    ) -> Result<(), StateMachineError> {
        let mut workflow_execution = self.workflow_execution.lock().await;
        workflow_execution.states.push(state.clone());
        workflow_execution.current_state = Some(state_name.clone());
        workflow_execution.status = WorkflowExecutionStatus::InProgress;
        workflow_execution.status_detail = format!("Executing state: {state_name}");

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
                StateMachineError::PersistFailed(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name,
                    error_name: None,
                    error_message: format!("Failed to persist state transition: {err}"),
                    duration: None,
                })
            })?;

        // A send error here should not prevent the state machine from continuing to execute.
        // The reason for this is that a broadcaster send operation can only fail if there are no active receivers
        // listening for events, which is not a critical failure condition.
        // See: tokio::sync::broadcast::error::SendError
        let _ = self
            .workflow_app
            .event_broadcaster
            .send(WorkflowExecutionEvent::StateTransition(Box::new(
                StateTransitionEvent {
                    event: "stateTransition".to_string(),
                    prev_state: prev_state.cloned(),
                    new_state: state.clone(),
                },
            )));

        Ok(())
    }

    async fn record_completed_state(
        self: Arc<Self>,
        state_name: String,
        duration: f64,
        raw_output: &Value,
        final_output: &Value,
        parent_state: Option<String>,
    ) -> Result<(), StateMachineError> {
        let mut workflow_execution = self.workflow_execution.lock().await;
        let state = extract_state_from_workflow_execution_mut(
            state_name.clone(),
            &mut workflow_execution,
            parent_state.clone(),
        )
        .expect("completed state must exist in the state machine's workflow execution");

        state.status = WorkflowExecutionStatus::Succeeded;
        state.raw_output = Some(raw_output.clone());
        state.output = Some(final_output.clone());
        state.completed = Some(self.workflow_app.clock.now_millis());
        state.duration = Some(duration);

        // Only persist for recording completed state, the updated state will be broadcast as a part of recording
        // a state transition or workflow execution completion.
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
            .map(|_| ())
            .map_err(|err| {
                StateMachineError::PersistFailed(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: parent_state,
                    error_name: None,
                    error_message: format!("Failed to persist state transition: {err}"),
                    duration: None,
                })
            })
    }

    async fn record_completed_workflow_execution(self: Arc<Self>) -> Result<(), StateMachineError> {
        let mut workflow_execution = self.workflow_execution.lock().await;
        workflow_execution.status = WorkflowExecutionStatus::Succeeded;
        workflow_execution.completed = Some(self.workflow_app.clock.now_millis());
        workflow_execution.duration = Some(as_fractional_seconds(
            self.full_duration_instant
                .lock()
                .await
                .expect("full duration instant must be set")
                .elapsed(),
        ));

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
                StateMachineError::PersistFailed(WorkflowStateErrorInfo {
                    state_name: "unknown".to_string(),
                    parent_state_name: None,
                    error_name: None,
                    error_message: format!(
                        "Failed to persist workflow execution completion: {err}",
                    ),
                    duration: None,
                })
            })?;

        // A send error here should not fail the state machine.
        // The reason for this is that a broadcaster send operation can only fail if there are no active receivers
        // listening for events, which is not a critical failure condition.
        // See: tokio::sync::broadcast::error::SendError
        let _ =
            self.workflow_app
                .event_broadcaster
                .send(WorkflowExecutionEvent::ExecutionComplete(
                    ExecutionCompleteEvent {
                        event: "workflowExecutionComplete".to_string(),
                        complete_execution: workflow_execution.clone(),
                    },
                ));

        Ok(())
    }

    async fn handle_catchable_error(
        self: Arc<Self>,
        err_info: WorkflowStateErrorInfo,
        input: &Value,
        fallback_error_name: String,
    ) {
        let error_name = derive_final_error_name(&err_info, &fallback_error_name);

        error!(
            state = err_info.state_name.as_str(),
            parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
            error_name = error_name.clone(),
            err_info.error_message
        );

        let catch_config = self
            .clone()
            .derive_state_config(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
            )
            .map(|state| state.catch)
            .unwrap_or(None);

        let catch_result = self
            .clone()
            .catch(catch_config.as_ref(), &err_info, input)
            .await;
        if let Err(catch_err) = catch_result {
            self.handle_retry_or_catch_error(catch_err).await;
        }
    }

    async fn handle_retryable_error(
        self: Arc<Self>,
        err_info: WorkflowStateErrorInfo,
        input: &Value,
        fallback_error_name: String,
    ) {
        let error_name = derive_final_error_name(&err_info, &fallback_error_name);

        error!(
            state = err_info.state_name.as_str(),
            parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
            error_name = error_name.clone(),
            err_info.error_message
        );

        let retry_config_opt = self
            .clone()
            .derive_state_config(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
            )
            .map(|state| state.retry)
            .unwrap_or(None);

        let matching_retry_config = match retry_config_opt {
            Some(retry_config) => retry_config
                .iter()
                .find(|config| error_matches(&error_name, &config.match_errors))
                .cloned(),
            None => None,
        };

        let catch_config_opt = self
            .clone()
            .derive_state_config(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
            )
            .map(|state| state.catch)
            .unwrap_or(None);

        let (attempts_so_far, exceeded_max_attempts) = self
            .clone()
            .check_exceeded_max_attempts(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
                matching_retry_config.as_ref(),
            )
            .await;

        self.clone()
            .record_error(
                err_info.state_name.clone(),
                err_info.parent_state_name.clone(),
                error_name,
                err_info.error_message.clone(),
                true,
                matching_retry_config.is_some() && !exceeded_max_attempts,
                err_info.duration,
            )
            .await;

        let result = self
            .clone()
            .retry_or_catch(
                &err_info,
                matching_retry_config.as_ref(),
                catch_config_opt.as_ref(),
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
        self: Arc<Self>,
        err_info: &WorkflowStateErrorInfo,
        matching_retry_config: Option<&CelerityWorkflowRetryConfig>,
        catch_config: Option<&Vec<CelerityWorkflowCatchConfig>>,
        attempts_so_far: i64,
        exceeded_max_attempts: bool,
        input: &Value,
    ) -> Result<(), StateMachineError> {
        if let Some(retry_config) = matching_retry_config {
            if !exceeded_max_attempts {
                let wait_time_ms = calculate_retry_wait_time_ms(
                    retry_config,
                    // Retry attempts are 0-indexed, so subtract 1 from the attempts so far.
                    // For example 1 attempt so far would be retry attempt 0.
                    attempts_so_far - 1,
                    DEFAULT_STATE_RETRY_INTERVAL_SECONDS,
                    DEFAULT_STATE_RETRY_BACKOFF_RATE,
                );
                sleep(Duration::from_millis(wait_time_ms)).await;

                // Take a copy of the state to avoid holding a lock on the state machine's workflow
                // execution data while retrying the state, holding the lock would cause a deadlock.
                let state = self
                    .clone()
                    .extract_state(
                        err_info.state_name.clone(),
                        err_info.parent_state_name.clone(),
                    )
                    .await;

                self.execute_state_and_handle_error(
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
        self: Arc<Self>,
        catch_config: Option<&Vec<CelerityWorkflowCatchConfig>>,
        err_info: &WorkflowStateErrorInfo,
        input: &Value,
    ) -> Result<(), StateMachineError> {
        if let Some(catch_config) = catch_config {
            let matching_catcher_opt = catch_config.iter().find(|config| {
                error_matches(
                    &err_info.error_name.clone().unwrap_or_default(),
                    &config.match_errors,
                )
            });

            if let Some(matching_catcher) = matching_catcher_opt {
                let state_name = matching_catcher.next.clone();
                let state = self
                    .clone()
                    .extract_state(state_name.clone(), err_info.parent_state_name.clone())
                    .await;

                let input_from_catch =
                    self.clone()
                        .prepare_input_from_catch(matching_catcher, err_info, input)?;

                self.execute_state_and_handle_error(
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

    async fn handle_retry_or_catch_error(self: Arc<Self>, retry_or_catch_err: StateMachineError) {
        // The only errors that should occur when handling retry or catch errors are those
        // that can occur when preparing the input for the next state to transition to using the
        // catch `resultPath` configuration.
        if let StateMachineError::InvalidResultPath(err_info) = retry_or_catch_err {
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
                err_info.duration,
            )
            .await;
        } else {
            error!(
                state = "unknown",
                parent_state = "unknown",
                error_name = "UnknownError",
                "an unexpected error occurred while handling retry or catch behaviour: {}",
                retry_or_catch_err
            );
            self.record_error(
                "unknown".to_string(),
                None,
                "UnknownError".to_string(),
                format!("An unexpected error occurred: {retry_or_catch_err}"),
                true,
                false,
                None,
            )
            .await;
        }
    }

    // This should be used only when necessary as it will clone the
    // state in the workflow.
    async fn extract_state(
        self: Arc<Self>,
        state_name: String,
        parent_state_name: Option<String>,
    ) -> Option<WorkflowExecutionState> {
        let workflow_execution = self.workflow_execution.lock().await;
        extract_state_from_workflow_execution(state_name, &workflow_execution, parent_state_name)
            .cloned()
    }

    async fn check_exceeded_max_attempts(
        self: Arc<Self>,
        state_name: String,
        parent_state_name: Option<String>,
        retry_config_opt: Option<&CelerityWorkflowRetryConfig>,
    ) -> (i64, bool) {
        let workflow_execution = self.workflow_execution.lock().await;
        let state = extract_state_from_workflow_execution(
            state_name.clone(),
            &workflow_execution,
            parent_state_name.clone(),
        );

        if let Some(state) = state {
            if let Some(retry_config) = retry_config_opt {
                if let Some(max_retry_attempts) = retry_config.max_attempts {
                    // The first attempt does not count as a retry so subtract 1
                    // before comparing to the max attempts configured.
                    let attempts = i64::from(state.attempt) - 1;
                    if attempts >= max_retry_attempts {
                        // The caller of this function expects to receive the total number of attempts
                        // so far including the first attempt.
                        return (attempts + 1, true);
                    }
                }
            }
        }

        (0, false)
    }

    async fn log_and_record_error(
        self: Arc<Self>,
        error_name: &str,
        err_info: WorkflowStateErrorInfo,
        log_message_prefix: &str,
        persist: bool,
        can_continue: bool,
    ) {
        error!(
            state = err_info.state_name.as_str(),
            parent_state = err_info.parent_state_name.as_deref().unwrap_or(""),
            error_name = error_name,
            "{} {}",
            log_message_prefix,
            err_info.error_message
        );
        self.record_error(
            err_info.state_name,
            err_info.parent_state_name,
            error_name.to_string(),
            err_info.error_message,
            persist,
            can_continue,
            err_info.duration,
        )
        .await;
    }

    #[allow(clippy::too_many_arguments)]
    async fn record_error(
        self: Arc<Self>,
        state_name: String,
        parent_state_name: Option<String>,
        error_name: String,
        error_message: String,
        persist: bool,
        can_continue: bool,
        duration: Option<f64>,
    ) {
        // Only acquire a lock to update error state and take a copy of the current state to persist
        // and broadcast.
        // It's important to avoid holding the lock while persisting state and broadcasting events to
        // prevent significant blocking when executing parallel branches.
        let (captured_workflow_execution, captured_state) = {
            let mut workflow_execution = self.workflow_execution.lock().await;
            let status_detail =
                format!("Error executing state \"{state_name}\" [{error_name}]: {error_message}",);

            if !can_continue {
                workflow_execution.status = WorkflowExecutionStatus::Failed;
                workflow_execution.status_detail = status_detail.clone();
                workflow_execution.duration = Some(as_fractional_seconds(
                    self.full_duration_instant
                        .lock()
                        .await
                        .expect("full duration instant must be set")
                        .elapsed(),
                ));
            }

            let state = extract_state_from_workflow_execution_mut(
                state_name.clone(),
                &mut workflow_execution,
                parent_state_name.clone(),
            )
            .expect("Failed state must exist in workflow execution");

            state.error = Some(status_detail);
            state.status = WorkflowExecutionStatus::Failed;
            state.duration = duration;
            state.completed = Some(self.workflow_app.clock.now_millis());
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

        // Send errors for these broadcaster.send calls should not prevent the state machine from continuing to execute.
        // The reason for this is that a broadcaster send operation can only fail if there are no active receivers
        // listening for events, which is not a critical failure condition.
        // See: tokio::sync::broadcast::error::SendError

        let _ = self
            .workflow_app
            .event_broadcaster
            .send(WorkflowExecutionEvent::StateFailure(StateFailureEvent {
                event: "stateFailed".to_string(),
                failed_state: captured_state,
            }));

        if !can_continue {
            // Completion event will contain a workflow execution with the failed status
            // and error information in the status detail field.
            let _ = self.workflow_app.event_broadcaster.send(
                WorkflowExecutionEvent::ExecutionComplete(ExecutionCompleteEvent {
                    event: "workflowExecutionComplete".to_string(),
                    complete_execution: captured_workflow_execution,
                }),
            );
        }
    }

    fn prepare_input_from_catch(
        self: Arc<Self>,
        catch_config: &CelerityWorkflowCatchConfig,
        err_info: &WorkflowStateErrorInfo,
        input: &Value,
    ) -> Result<Value, StateMachineError> {
        if let Some(result_path) = &catch_config.result_path {
            self.workflow_app
                .payload_template_engine
                .inject(
                    input,
                    result_path,
                    json!({
                        "error": err_info.error_name.clone(),
                        "cause": err_info.error_message.clone(),
                    }),
                )
                .map_err(|err| {
                    StateMachineError::InvalidResultPath(WorkflowStateErrorInfo {
                        state_name: err_info.state_name.clone(),
                        parent_state_name: err_info.parent_state_name.clone(),
                        error_name: Some("InvalidResultPath".to_string()),
                        error_message: format!(
                            "Failed to inject error information from result path into input: {err}"
                        ),
                        // Capture the duration of the caught error.
                        duration: err_info.duration,
                    })
                })
        } else {
            Ok(input.clone())
        }
    }

    fn prepare_state_input(
        self: Arc<Self>,
        state_name: String,
        parent_state_name: Option<String>,
        state_config: &CelerityWorkflowState,
        input: &Value,
    ) -> Result<Value, StateMachineError> {
        if let Some(input_path) = &state_config.input_path {
            self.workflow_app
                .payload_template_engine
                .extract(input, input_path)
                .map_err(|err| {
                    StateMachineError::InvalidInputPath(WorkflowStateErrorInfo {
                        state_name,
                        parent_state_name,
                        error_name: Some("InvalidInputPath".to_string()),
                        error_message: format!(
                            "Failed to extract value from input JSON path: {err}"
                        ),
                        duration: None,
                    })
                })
        } else {
            Ok(input.clone())
        }
    }

    fn prepare_state_output(
        self: Arc<Self>,
        state_name: String,
        parent_state_name: Option<String>,
        state_config: &CelerityWorkflowState,
        input: &Value,
        output: &Value,
        duration: f64,
    ) -> Result<Value, StateMachineError> {
        if let Some(result_path) = &state_config.result_path {
            self.workflow_app
                .payload_template_engine
                .inject(input, result_path, output.clone())
                .map_err(|err| {
                    StateMachineError::InvalidResultPath(WorkflowStateErrorInfo {
                        state_name,
                        parent_state_name,
                        error_name: Some("InvalidResultPath".to_string()),
                        error_message: format!(
                            "Failed to inject output from result path into input: {err}"
                        ),
                        duration: Some(duration),
                    })
                })
        } else if let Some(output_path) = &state_config.output_path {
            self.workflow_app
                .payload_template_engine
                .extract(output, output_path)
                .map_err(|err| {
                    StateMachineError::InvalidOutputPath(WorkflowStateErrorInfo {
                        state_name,
                        parent_state_name,
                        error_name: Some("InvalidOutputPath".to_string()),
                        error_message: format!(
                            "Failed to extract value from output JSON path: {err}"
                        ),
                        duration: Some(duration),
                    })
                })
        } else {
            Ok(output.clone())
        }
    }
}

fn find_parallel_child_state_config(
    parent_state_name: String,
    parent_state: &CelerityWorkflowState,
    state_name: String,
) -> Result<CelerityWorkflowState, StateMachineError> {
    match parent_state.state_type {
        CelerityWorkflowStateType::Parallel => {
            if let Some(parallel_branches) = &parent_state.parallel_branches {
                let child_state = parallel_branches
                    .iter()
                    .filter_map(|branch| branch.states.get(&state_name).cloned())
                    .next();

                if let Some(state) = child_state {
                    return Ok(state.clone());
                } else {
                    return Err(StateMachineError::StateNotFound(WorkflowStateErrorInfo {
                        state_name,
                        parent_state_name: Some(parent_state_name),
                        error_name: None,
                        error_message: "State could not be found in any of the \
                            parent state's parallel branches"
                            .to_string(),
                        duration: None,
                    }));
                }
            }

            Err(StateMachineError::StateNotFound(WorkflowStateErrorInfo {
                state_name,
                parent_state_name: Some(parent_state_name),
                error_name: None,
                error_message: "Parallel state not found in parent state".to_string(),
                duration: None,
            }))
        }
        _ => Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
            state_name,
            parent_state_name: Some(parent_state_name),
            error_name: None,
            error_message: "Parent state is not a parallel state".to_string(),
            duration: None,
        })),
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

    None
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

    None
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
                .filter_map(|branch| branch.iter_mut().find(|state| state.name == state_name))
                .next();

            child_state
        }
        _ => None,
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
                .filter_map(|branch| branch.iter().find(|state| state.name == state_name))
                .next();

            child_state
        }
        _ => None,
    }
}

fn derive_wait_time_seconds(
    state_name: String,
    wait_time_config: &CelerityWorkflowWaitConfig,
    input: &Value,
    instant_for_duration: &Instant,
    now_millis: i64,
) -> Result<u64, StateMachineError> {
    if let Some(wait_time_seconds) = &wait_time_config.seconds {
        return derive_wait_time_from_seconds_field(
            state_name,
            wait_time_seconds,
            input,
            instant_for_duration,
        );
    } else if let Some(timestamp) = &wait_time_config.timestamp {
        return derive_wait_time_from_timestamp_field(
            state_name,
            timestamp,
            input,
            instant_for_duration,
            now_millis,
        );
    }

    Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
        state_name,
        parent_state_name: None,
        error_name: None,
        error_message: "Wait state must have a value for the seconds or timestamp field"
            .to_string(),
        duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
    }))
}

fn derive_wait_time_from_seconds_field(
    state_name: String,
    wait_time_seconds: &str,
    input: &Value,
    instant_for_duration: &Instant,
) -> Result<u64, StateMachineError> {
    if wait_time_seconds.starts_with("$") {
        let path = JsonPath::from_str(wait_time_seconds).map_err(|err| {
            StateMachineError::InvalidPayloadTemplate(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: None,
                error_name: None,
                error_message: format!(
                    "Failed to parse JSON path for wait time seconds in state configuration: {err}",
                ),
                duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
            })
        })?;

        let seconds_value = path.find(input);
        match seconds_value {
            Value::Number(num) => num.as_u64().ok_or_else(|| {
                StateMachineError::InvalidState(WorkflowStateErrorInfo {
                    state_name: state_name.clone(),
                    parent_state_name: None,
                    error_name: None,
                    error_message: "Wait state seconds must be a positive integer".to_string(),
                    duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
                })
            }),
            _ => Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name,
                parent_state_name: None,
                error_name: None,
                error_message: "Wait state seconds must be a positive integer".to_string(),
                duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
            })),
        }
    } else {
        wait_time_seconds.parse::<u64>().map_err(|_| {
            StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: None,
                error_name: None,
                error_message: "Wait state seconds must be a positive integer".to_string(),
                duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
            })
        })
    }
}

fn derive_wait_time_from_timestamp_field(
    state_name: String,
    timestamp: &str,
    input: &Value,
    instant_for_duration: &Instant,
    now_millis: i64,
) -> Result<u64, StateMachineError> {
    if timestamp.starts_with("$") {
        let path = JsonPath::from_str(timestamp).map_err(|err| {
            StateMachineError::InvalidPayloadTemplate(WorkflowStateErrorInfo {
                state_name: state_name.clone(),
                parent_state_name: None,
                error_name: None,
                error_message: format!(
                    "Failed to parse JSON path for wait time timestamp in state configuration: {err}",
                ),
                duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
            })
        })?;

        let timestamp_value = path.find(input);
        match timestamp_value {
            Value::String(timestamp_str) => {
                wait_time_from_timestamp(timestamp_str.as_str(), now_millis)
            }
            _ => Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
                state_name,
                parent_state_name: None,
                error_name: None,
                error_message: "Wait state timestamp must be a string".to_string(),
                duration: Some(as_fractional_seconds(instant_for_duration.elapsed())),
            })),
        }
    } else {
        wait_time_from_timestamp(timestamp, now_millis)
    }
}

fn wait_time_from_timestamp(timestamp: &str, now_millis: i64) -> Result<u64, StateMachineError> {
    let parsed_timestamp = DateTime::parse_from_rfc3339(timestamp).map_err(|err| {
        StateMachineError::InvalidState(WorkflowStateErrorInfo {
            state_name: "unknown".to_string(),
            parent_state_name: None,
            error_name: None,
            error_message: format!("Failed to parse timestamp for wait state: {err}",),
            duration: None,
        })
    })?;

    let current_time = DateTime::from_timestamp_millis(now_millis)
        .expect("captured current unix timestamp in milliseconds must be valid");
    let wait_time = parsed_timestamp
        .signed_duration_since(current_time)
        .num_seconds();
    if wait_time < 0 {
        return Err(StateMachineError::InvalidState(WorkflowStateErrorInfo {
            state_name: "unknown".to_string(),
            parent_state_name: None,
            error_name: None,
            error_message: "Wait state timestamp must be in the future".to_string(),
            duration: None,
        }));
    }

    Ok(wait_time as u64)
}

fn error_matches(error: &str, match_errors: &[String]) -> bool {
    if match_errors.contains(&"*".to_string()) {
        return true;
    }

    match_errors.contains(&error.to_string())
}

fn derive_final_error_name(
    error_info: &WorkflowStateErrorInfo,
    fallback_error_name: &str,
) -> String {
    error_info
        .error_name
        .clone()
        .map(|err_name| {
            if err_name.is_empty() {
                fallback_error_name.to_string()
            } else {
                err_name
            }
        })
        .unwrap_or_else(|| fallback_error_name.to_string())
}

fn from_payload_template_engine_error(
    state_name: String,
    parent_state_name: Option<String>,
    err: PayloadTemplateEngineError,
    duration: f64,
) -> StateMachineError {
    match err {
        PayloadTemplateEngineError::FunctionCallFailed(func_call_err) => {
            StateMachineError::PayloadTemplateFailure(WorkflowStateErrorInfo {
                state_name,
                parent_state_name,
                error_name: None,
                error_message: format!(
                    "Function call failed in payload template engine: {func_call_err}",
                ),
                duration: Some(duration),
            })
        }
        PayloadTemplateEngineError::FunctionNotFound(function_name) => {
            StateMachineError::PayloadTemplateFailure(WorkflowStateErrorInfo {
                state_name,
                parent_state_name,
                error_name: None,
                error_message: format!(
                    "Function \"{function_name}\" not found in payload template engine",
                ),
                duration: Some(duration),
            })
        }
        PayloadTemplateEngineError::ParseFunctionCallError(err_message) => {
            StateMachineError::InvalidPayloadTemplate(WorkflowStateErrorInfo {
                state_name,
                parent_state_name,
                error_name: None,
                error_message: format!(
                    "Failed to parse function call in payload template engine: {err_message}",
                ),
                duration: Some(duration),
            })
        }
        PayloadTemplateEngineError::JsonPathError(err_message) => {
            StateMachineError::InvalidPayloadTemplate(WorkflowStateErrorInfo {
                state_name,
                parent_state_name,
                error_name: None,
                error_message: format!(
                    "Failed to parse JSON path in payload template engine: {err_message}",
                ),
                duration: Some(duration),
            })
        }
    }
}

// Internal error type used for state machine errors.
#[derive(Debug, Clone)]
enum StateMachineError {
    PersistFailed(WorkflowStateErrorInfo),
    StateNotFound(WorkflowStateErrorInfo),
    InvalidState(WorkflowStateErrorInfo),
    ExecuteStepHandlerFailed(WorkflowStateErrorInfo),
    ParallelBranchesFailed(WorkflowStateErrorInfo),
    InvalidInputPath(WorkflowStateErrorInfo),
    InvalidResultPath(WorkflowStateErrorInfo),
    InvalidOutputPath(WorkflowStateErrorInfo),
    PayloadTemplateFailure(WorkflowStateErrorInfo),
    InvalidPayloadTemplate(WorkflowStateErrorInfo),
}

impl fmt::Display for StateMachineError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            StateMachineError::PersistFailed(err_info) => {
                write!(
                    f,
                    "failed to persist workflow execution changes: {}",
                    err_info.error_message
                )
            }
            StateMachineError::StateNotFound(err_info) => {
                write!(
                    f,
                    "failed to find state in workflow spec: {}",
                    err_info.error_message
                )
            }
            StateMachineError::InvalidState(err_info) => {
                write!(f, "invalid state configuration: {}", err_info.error_message)
            }
            StateMachineError::ExecuteStepHandlerFailed(err_info) => {
                write!(
                    f,
                    "failed to execute step handler: {}",
                    err_info.error_message
                )
            }
            StateMachineError::ParallelBranchesFailed(err_info) => {
                write!(
                    f,
                    "parallel state branches failed: {}",
                    err_info.error_message
                )
            }
            StateMachineError::InvalidInputPath(err_info) => {
                write!(
                    f,
                    "invalid input path in state configuration: {}",
                    err_info.error_message
                )
            }
            StateMachineError::InvalidResultPath(err_info) => {
                write!(
                    f,
                    "invalid result path in state configuration: {}",
                    err_info.error_message
                )
            }
            StateMachineError::InvalidOutputPath(err_info) => {
                write!(
                    f,
                    "invalid output path in state configuration: {}",
                    err_info.error_message
                )
            }
            StateMachineError::PayloadTemplateFailure(err_info) => {
                write!(
                    f,
                    "failed to render payload template: {}",
                    err_info.error_message
                )
            }
            StateMachineError::InvalidPayloadTemplate(err_info) => {
                write!(
                    f,
                    "invalid payload template configuration: {}",
                    err_info.error_message
                )
            }
        }
    }
}

// Information about an error that occurred during the execution of a workflow state.
#[derive(Debug, Clone)]
struct WorkflowStateErrorInfo {
    state_name: String,
    parent_state_name: Option<String>,
    error_name: Option<String>,
    error_message: String,
    duration: Option<f64>,
}
