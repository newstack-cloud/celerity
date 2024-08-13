use std::{error::Error, fmt};

use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
    Json,
};
use celerity_blueprint_config_parser::parse::BlueprintParseError;
use tokio::{sync::mpsc::error::SendError, task::JoinError, time::error::Elapsed};

use crate::types::ResponseMessage;

/// Provides a custom error type to be used for failures
/// within message handlers.
#[derive(Debug)]
pub enum MessageHandlerError {
    MissingHandler,
    Timeout(Elapsed),
    HandlerFailure(Box<dyn Error + Send + Sync + 'static>),
}

impl fmt::Display for MessageHandlerError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            MessageHandlerError::MissingHandler => write!(
                f,
                "message handler failed: a handler must be registered to process messages"
            ),
            MessageHandlerError::Timeout(elapsed_error) => {
                write!(f, "message handler failed: timeout {}", elapsed_error)
            }
            MessageHandlerError::HandlerFailure(handler_error) => {
                write!(f, "message handler failed: {}", handler_error)
            }
        }
    }
}

/// Provides a custom error type to be used for failures
/// in gathering application configuration from a parsed blueprint.
#[derive(Debug)]
pub enum ConfigError {
    Api(String),
    ApiMissing,
}

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ConfigError::Api(api_error) => write!(f, "config error: {}", api_error),
            ConfigError::ApiMissing => write!(f, "config error: no API resource found"),
        }
    }
}

/// Provides a custom error type to be used for failures
/// in starting an application.
#[derive(Debug)]
pub enum ApplicationStartError {
    Config(ConfigError),
    BlueprintParse(BlueprintParseError),
    Environment(String),
    // An error occured while blocking on one of the long-running
    // tasks to complete. (e.g. API server or message poller/consumer)
    TaskWaitError(JoinError),
}

impl fmt::Display for ApplicationStartError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ApplicationStartError::Config(config_error) => {
                write!(f, "application start error: {}", config_error)
            }
            ApplicationStartError::BlueprintParse(parse_error) => {
                write!(f, "application start error: {}", parse_error)
            }
            ApplicationStartError::Environment(env_error) => {
                write!(f, "application start error: {}", env_error)
            }
            ApplicationStartError::TaskWaitError(join_error) => {
                write!(f, "application start error: {}", join_error)
            }
        }
    }
}

impl From<ConfigError> for ApplicationStartError {
    fn from(error: ConfigError) -> Self {
        ApplicationStartError::Config(error)
    }
}

impl From<BlueprintParseError> for ApplicationStartError {
    fn from(error: BlueprintParseError) -> Self {
        ApplicationStartError::BlueprintParse(error)
    }
}

impl From<JoinError> for ApplicationStartError {
    fn from(error: JoinError) -> Self {
        ApplicationStartError::TaskWaitError(error)
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

#[derive(Debug)]
pub enum WebSocketsMessageError {
    NotEnabled,
    UnexpectedError,
}

impl IntoResponse for WebSocketsMessageError {
    fn into_response(self) -> Response {
        let resp_tuple = match self {
            WebSocketsMessageError::NotEnabled => (
                StatusCode::FORBIDDEN,
                Json(ResponseMessage {
                    message: "WebSockets are not enabled for the current application".to_string(),
                }),
            ),
            WebSocketsMessageError::UnexpectedError => (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ResponseMessage {
                    message: "An unexpected error occurred".to_string(),
                }),
            ),
        };
        resp_tuple.into_response()
    }
}

#[derive(Debug)]
pub enum WebSocketConnError {
    SendMessageError(String),
    BroadcastMessageError(String),
}

impl From<axum::Error> for WebSocketConnError {
    fn from(error: axum::Error) -> Self {
        WebSocketConnError::SendMessageError(error.to_string())
    }
}

impl<T> From<SendError<T>> for WebSocketConnError {
    fn from(error: SendError<T>) -> Self {
        WebSocketConnError::BroadcastMessageError(error.to_string())
    }
}
