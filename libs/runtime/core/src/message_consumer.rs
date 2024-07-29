use async_trait::async_trait;

use crate::message_handler::MessageHandler;

/// Provides a trait for a message consumer
/// that listens for messages on a queue
/// or message broker and fires registered
/// event handlers.
#[async_trait]
pub trait MessageConsumer<MessageType> {
    type Error;

    /// Registers the handler to process 1 or more messages at a time.
    /// This handler will be called when a batch of messages are returned
    /// by the message broker or queue in each request to fetch messages
    /// from the consumer.
    fn register_handler(&mut self, handler: Box<dyn MessageHandler<MessageType> + Send + Sync>);

    /// Starts the message consumer and listens for messages on the queue
    /// or message broker.
    async fn start(&self) -> Result<(), Self::Error>;
}
