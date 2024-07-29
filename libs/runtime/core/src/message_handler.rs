use crate::errors::MessageHandlerError;
use async_trait::async_trait;
use std::fmt::Debug;

/// Provides a trait for a message handler
/// that processes messages received from
/// a message consumer (queue or message broker).
#[async_trait]
pub trait MessageHandler<MessageType> {
    /// Handles a single message of a given type.
    async fn handle(&self, message: MessageType) -> Result<(), MessageHandlerError>;

    /// Handles a batch of messages of a given type.
    async fn handle_batch(&self, messages: Vec<MessageType>) -> Result<(), MessageHandlerError>;
}

impl<MessageType> Debug for dyn MessageHandler<MessageType> + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "MessageHandler")
    }
}
