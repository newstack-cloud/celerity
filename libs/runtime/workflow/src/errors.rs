use std::fmt;

use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use celerity_blueprint_config_parser::parse::BlueprintParseError;
use celerity_helpers::runtime_types::ResponseMessage;
use opentelemetry::trace::TraceError as OTelTraceError;
use tokio::task::JoinError;
use tracing_subscriber::{filter::ParseError, util::TryInitError};

/// Provides a custom error type to be used for failures
/// in gathering workflow application configuration from a parsed blueprint.
#[derive(Debug)]
pub enum ConfigError {
    Workflow(String),
    WorkflowMissing,
}

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ConfigError::Workflow(workflow_error) => write!(f, "config error: {workflow_error}"),
            ConfigError::WorkflowMissing => write!(f, "config error: no workflow resource found"),
        }
    }
}

/// Provides a custom error type to be used for failures
/// in starting an application.
#[derive(Debug)]
pub enum WorkflowApplicationStartError {
    Config(ConfigError),
    BlueprintParse(BlueprintParseError),
    Environment(String),
    // An error occured while blocking on one of the long-running
    // tasks to complete. (e.g. Workflow API server or local runtime API server)
    TaskWaitError(JoinError),
    OpenTelemetryTrace(OTelTraceError),
    TracerTryInit(TryInitError),
    TracingFilterParse(ParseError),
}

impl fmt::Display for WorkflowApplicationStartError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            WorkflowApplicationStartError::Config(config_error) => {
                write!(f, "application start error: {config_error}")
            }
            WorkflowApplicationStartError::BlueprintParse(parse_error) => {
                write!(f, "application start error: {parse_error}")
            }
            WorkflowApplicationStartError::Environment(env_error) => {
                write!(f, "application start error: {env_error}")
            }
            WorkflowApplicationStartError::TaskWaitError(join_error) => {
                write!(f, "application start error: {join_error}")
            }
            WorkflowApplicationStartError::OpenTelemetryTrace(trace_error) => {
                write!(f, "application start error: {trace_error}")
            }
            WorkflowApplicationStartError::TracerTryInit(try_init_error) => {
                write!(f, "application start error: {try_init_error}")
            }
            WorkflowApplicationStartError::TracingFilterParse(parse_error) => {
                write!(f, "application start error: {parse_error}")
            }
        }
    }
}

impl From<ConfigError> for WorkflowApplicationStartError {
    fn from(error: ConfigError) -> Self {
        WorkflowApplicationStartError::Config(error)
    }
}

impl From<BlueprintParseError> for WorkflowApplicationStartError {
    fn from(error: BlueprintParseError) -> Self {
        WorkflowApplicationStartError::BlueprintParse(error)
    }
}

impl From<JoinError> for WorkflowApplicationStartError {
    fn from(error: JoinError) -> Self {
        WorkflowApplicationStartError::TaskWaitError(error)
    }
}

impl From<OTelTraceError> for WorkflowApplicationStartError {
    fn from(error: OTelTraceError) -> Self {
        WorkflowApplicationStartError::OpenTelemetryTrace(error)
    }
}

impl From<TryInitError> for WorkflowApplicationStartError {
    fn from(error: TryInitError) -> Self {
        WorkflowApplicationStartError::TracerTryInit(error)
    }
}

impl From<ParseError> for WorkflowApplicationStartError {
    fn from(error: ParseError) -> Self {
        WorkflowApplicationStartError::TracingFilterParse(error)
    }
}

#[derive(Debug)]
pub enum EventResultError {
    EventNotFound,
    UnexpectedError,
}

impl IntoResponse for EventResultError {
    fn into_response(self) -> Response {
        let resp_tuple = match self {
            EventResultError::EventNotFound => (
                StatusCode::NOT_FOUND,
                Json(ResponseMessage {
                    message: "Event with provided ID was not found".to_string(),
                }),
            ),
            EventResultError::UnexpectedError => (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ResponseMessage {
                    message: "An unexpected error occurred".to_string(),
                }),
            ),
        };
        resp_tuple.into_response()
    }
}
