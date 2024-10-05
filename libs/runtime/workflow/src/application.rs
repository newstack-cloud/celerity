use std::{
    collections::{HashMap, VecDeque},
    sync::{Arc, Mutex},
};

use axum::{
    extract::{Path, State},
    routing::{get, post},
    Json, Router,
};
use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};
use celerity_helpers::{
    runtime_types::{HealthCheckResponse, RuntimeCallMode},
    time::{Clock, DefaultClock},
};

use crate::{
    config::{WorkflowAppConfig, WorkflowRuntimeConfig},
    consts::WORKFLOW_RUNTIME_HEALTH_CHECK_ENDPOINT,
    errors::WorkflowApplicationStartError,
    transform_config::collect_workflow_app_config,
    types::{EventTuple, WorkflowAppState},
    workflow_runtime_local_api::create_workflow_runtime_local_api,
};

/// Provides an application for a workflow
/// with a HTTP API to trigger, monitor and retrieve historical
/// executions for the workflow.
pub struct WorkflowApplication {
    runtime_config: WorkflowRuntimeConfig,
    app_tracing_enabled: bool,
    workflow_api: Option<Router<WorkflowAppState>>,
    workflow_app_config: Option<WorkflowAppConfig>,
    runtime_local_api: Option<Router>,
    event_queue: Option<Arc<Mutex<VecDeque<EventTuple>>>>,
    processing_events_map: Option<Arc<Mutex<HashMap<String, EventTuple>>>>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    local_api_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    clock: Arc<dyn Clock + Send + Sync>,
}

impl WorkflowApplication {
    pub fn new(runtime_config: WorkflowRuntimeConfig) -> Self {
        WorkflowApplication {
            runtime_config,
            app_tracing_enabled: false,
            workflow_api: None,
            workflow_app_config: None,
            runtime_local_api: None,
            event_queue: None,
            processing_events_map: None,
            server_shutdown_signal: None,
            local_api_shutdown_signal: None,
            clock: Arc::new(DefaultClock::new()),
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
}

async fn run_handler(State(state): State<WorkflowAppState>) -> Json<Option<String>> {
    Json(None)
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
