use std::{error::Error, fmt};

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
