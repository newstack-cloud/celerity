use std::{error::Error, fmt};

use celerity_blueprint_config_parser::parse::BlueprintParseError;
use tokio::time::error::Elapsed;

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
