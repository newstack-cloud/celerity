use async_trait::async_trait;
use std::{
    error::Error,
    fmt::{self, Debug},
};
use tokio::time::error::Elapsed;

#[async_trait]
pub trait MessageHandler<MessageType> {
    async fn handle(&self, message: MessageType) -> Result<(), MessageHandlerError>;
    async fn handle_batch(&self, messages: Vec<MessageType>) -> Result<(), MessageHandlerError>;
}

impl<MessageType> Debug for dyn MessageHandler<MessageType> + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "MessageHandler")
    }
}

// Provides a custom error type to be used for failures
// within message handlers.
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
                write!(f, "message handler failed: timeout {elapsed_error}")
            }
            MessageHandlerError::HandlerFailure(handler_error) => {
                write!(f, "message handler failed: {handler_error}")
            }
        }
    }
}
