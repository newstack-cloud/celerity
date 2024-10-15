use std::{
    collections::{HashMap, VecDeque},
    convert::Infallible,
    future::Future,
    sync::{Arc, Mutex, RwLock},
    time::Duration,
};

use axum::{
    extract::{self, Path, State},
    response::{sse::Event, Sse},
    routing::{get, post},
    Json, Router,
};
use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};
use celerity_helpers::{
    runtime_types::{HealthCheckResponse, RuntimeCallMode},
    time::{Clock, DefaultClock},
};
use futures::{stream, Stream, TryStreamExt as _};
use serde::Deserialize;
use serde_json::{json, Value};
use tokio::sync::broadcast::{self, Sender};
use tokio_stream::{
    wrappers::{errors::BroadcastStreamRecvError, BroadcastStream},
    StreamExt as _,
};
use tracing::{error, info_span, span, Instrument};
use uuid::Uuid;

use crate::{
    config::{WorkflowAppConfig, WorkflowRuntimeConfig},
    consts::{EVENT_BROADCASTER_CAPACITY, WORKFLOW_RUNTIME_HEALTH_CHECK_ENDPOINT},
    errors::WorkflowApplicationStartError,
    handlers::{BoxedWorkflowStateHandler, WorkflowStateHandler},
    state_machine::StateMachine,
    transform_config::collect_workflow_app_config,
    types::{EventTuple, Response, WorkflowAppState, WorkflowExecutionEvent},
    workflow_executions::{
        SaveWorkflowExecutionPayload, WorkflowExecutionService, WorkflowExecutionStatus,
    },
    workflow_runtime_local_api::create_workflow_runtime_local_api,
};

/// Provides an application for a workflow
/// with a HTTP API to trigger, monitor and retrieve historical
/// executions for the workflow.
pub struct WorkflowApplication {
    runtime_config: WorkflowRuntimeConfig,
    app_tracing_enabled: bool,
    workflow_api: Option<Router<WorkflowAppState>>,
    state_handlers: Arc<RwLock<HashMap<String, BoxedWorkflowStateHandler>>>,
    workflow_app_config: Option<WorkflowAppConfig>,
    runtime_local_api: Option<Router>,
    event_queue: Option<Arc<Mutex<VecDeque<EventTuple>>>>,
    processing_events_map: Option<Arc<Mutex<HashMap<String, EventTuple>>>>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    local_api_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    clock: Arc<dyn Clock + Send + Sync>,
    execution_service: Arc<dyn WorkflowExecutionService + Send + Sync>,
    event_broadcaster: Sender<WorkflowExecutionEvent>,
}

impl WorkflowApplication {
    pub fn new(
        runtime_config: WorkflowRuntimeConfig,
        execution_service: Arc<dyn WorkflowExecutionService + Send + Sync>,
    ) -> Self {
        WorkflowApplication {
            runtime_config,
            app_tracing_enabled: false,
            workflow_api: None,
            state_handlers: Arc::new(RwLock::new(HashMap::new())),
            workflow_app_config: None,
            runtime_local_api: None,
            event_queue: None,
            processing_events_map: None,
            server_shutdown_signal: None,
            local_api_shutdown_signal: None,
            clock: Arc::new(DefaultClock::new()),
            execution_service,
            event_broadcaster: broadcast::channel(EVENT_BROADCASTER_CAPACITY).0,
        }
    }

    pub fn setup(&mut self) -> Result<WorkflowAppConfig, WorkflowApplicationStartError> {
        let blueprint_config = self.load_and_parse_blueprint()?;
        let workflow_app_config = match collect_workflow_app_config(blueprint_config) {
            Ok(app_config) => {
                self.workflow_api = Some(self.setup_workflow_api()?);
                self.workflow_app_config = Some(app_config.clone());
                app_config
            }
            Err(err) => return Err(WorkflowApplicationStartError::Config(err)),
        };
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Http {
            self.runtime_local_api = Some(self.setup_runtime_local_api(&workflow_app_config)?);
        }
        Ok(workflow_app_config)
    }

    fn load_and_parse_blueprint(&self) -> Result<BlueprintConfig, BlueprintParseError> {
        if self.runtime_config.blueprint_config_path.ends_with(".json") {
            BlueprintConfig::from_json_file(&self.runtime_config.blueprint_config_path)
        } else {
            BlueprintConfig::from_yaml_file(&self.runtime_config.blueprint_config_path)
        }
    }

    fn setup_workflow_api(
        &mut self,
    ) -> Result<Router<WorkflowAppState>, WorkflowApplicationStartError> {
        // Workflow applications should always have tracing enabled to provide
        // observability into each workflow execution.
        self.app_tracing_enabled = true;

        let mut workflow_api_router = Router::new();
        let clock = self.clock.clone();
        workflow_api_router = workflow_api_router
            .route(
                WORKFLOW_RUNTIME_HEALTH_CHECK_ENDPOINT,
                get(|| async move {
                    Json(HealthCheckResponse {
                        timestamp: clock.now(),
                    })
                }),
            )
            .route("/run", post(run_handler))
            .route("/executions/:id", get(get_execution_handler))
            .route("/executions/:id/stream", get(execution_stream_handler))
            .route("/executions", get(get_executions_handler));

        Ok(workflow_api_router)
    }

    fn setup_runtime_local_api(
        &mut self,
        app_config: &WorkflowAppConfig,
    ) -> Result<Router, WorkflowApplicationStartError> {
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        self.event_queue = Some(event_queue.clone());
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        self.processing_events_map = Some(processing_events_map.clone());
        create_workflow_runtime_local_api(app_config, event_queue, processing_events_map)
    }

    pub fn register_workflow_state_handler(
        &mut self,
        workflow_state: &str,
        handler: BoxedWorkflowStateHandler,
    ) {
        self.state_handlers
            .write()
            .expect("lock should not to be poisoned")
            .insert(workflow_state.to_string(), handler);
    }
}

#[derive(Deserialize)]
struct RunHandlerPayload {
    #[serde(rename = "executionName")]
    execution_name: Option<String>,
    input: Value,
}

async fn run_handler(
    State(state): State<WorkflowAppState>,
    extract::Json(payload): extract::Json<RunHandlerPayload>,
) -> Response {
    let execution_id = payload
        .execution_name
        .unwrap_or_else(|| Uuid::new_v4().to_string());

    let span = info_span!("run_handler", execution_id = execution_id.clone());
    async move {
        let save_result = state
            .execution_service
            .save_workflow_execution(
                execution_id.clone(),
                SaveWorkflowExecutionPayload {
                    started: state.clock.now_millis(),
                    input: payload.input,
                    completed: None,
                    duration: None,
                    status: WorkflowExecutionStatus::Preparing,
                    status_detail: "The execution is currently being prepared".to_string(),
                    current_state: None,
                    states: vec![],
                },
            )
            .instrument(info_span!("save_workflow_execution"))
            .await;

        let execution_state = match save_result {
            Ok(initial_state) => initial_state,
            Err(err) => {
                error!("failed to save workflow execution: {}", err);
                return Response {
                    status: 500,
                    headers: None,
                    body: Some("an unexpected error occurred".to_string()),
                };
            }
        };

        // Spawn the state machine in a separate task to run in the background so we can
        // return the execution ID immediately.
        // Progress can be monitored by using the Stream API or
        // the GET /executions/:id endpoint.
        tokio::spawn(
            async move {
                let state_machine = Arc::new(StateMachine::new(state, execution_state));
                state_machine.start().await;
            }
            .instrument(info_span!("state_machine")),
        );

        Response {
            status: 200,
            headers: Some(HashMap::from([(
                "Content-Type".to_string(),
                "application/json".to_string(),
            )])),
            body: Some(
                json!({
                    "id": execution_id,
                })
                .to_string(),
            ),
        }
    }
    .instrument(span)
    .await
}

async fn get_execution_handler(
    Path(id): Path<String>,
    State(state): State<WorkflowAppState>,
) -> Json<Option<String>> {
    Json(None)
}

async fn get_executions_handler(State(state): State<WorkflowAppState>) -> Json<Option<String>> {
    Json(None)
}

async fn execution_stream_handler(
    State(state): State<WorkflowAppState>,
) -> Sse<impl Stream<Item = Result<Event, BroadcastStreamRecvError>>> {
    let rx = state.event_broadcaster.subscribe();
    let stream = BroadcastStream::new(rx).map_ok(|event| {
        Event::default()
            .data(serde_json::to_string(&event).expect("event should be serialisable to json"))
    });
    Sse::new(stream).keep_alive(
        axum::response::sse::KeepAlive::new()
            .interval(Duration::from_secs(1))
            .text("keep-alive-text"),
    )
}
