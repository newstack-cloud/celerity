use crate::{errors::is_connection_error, visibility_timeout::VisibilityTimeoutExtender};
use async_std::task;
use aws_sdk_sqs::{
    error::SdkError,
    operation::receive_message::ReceiveMessageError,
    types::{DeleteMessageBatchRequestEntry, Message, MessageSystemAttributeName},
    Client, Error,
};
use celerity_helpers::consumers::{MessageHandler, MessageHandlerError};
use std::{future::Future, pin::Pin, sync::Arc, time::Duration};
use tokio::time::timeout;
use tracing::{debug, error, instrument};

#[derive(Debug)]
pub struct SQSConsumerConfig {
    pub queue_url: String,
    pub polling_wait_time_ms: u64,
    pub batch_size: Option<i32>,
    pub message_handler_timeout: u64,
    pub visibility_timeout: Option<i32>,
    pub wait_time_seconds: Option<i32>,
    pub auth_error_timeout: Option<u64>,
    pub terminate_visibility_timeout: bool,
    pub should_delete_messages: bool,
    pub delete_messages_on_handler_failure: Option<bool>,
    pub attribute_names: Option<Vec<MessageSystemAttributeName>>,
    pub message_attribute_names: Option<Vec<String>>,
}

#[derive(Debug)]
struct SQSConsumerFinalisedConfig {
    queue_url: String,
    polling_wait_time_ms: u64,
    batch_size: i32,
    message_handler_timeout: u64,
    visibility_timeout: i32,
    wait_time_seconds: i32,
    auth_error_timeout: u64,
    terminate_visibility_timeout: bool,
    should_delete_messages: bool,
    delete_messages_on_handler_failure: bool,
    attribute_names: Option<Vec<MessageSystemAttributeName>>,
    message_attribute_names: Option<Vec<String>>,
}

type PinnedMessageHandlerFuture<'a> =
    Pin<Box<dyn Future<Output = Result<(), MessageHandlerError>> + Send + 'a>>;

/// Provides an implementation of an AWS SQS
/// message consumer that polls SQS queues
/// and fires registered event handlers.
#[derive(Debug)]
pub struct SQSMessageConsumer {
    handler: Option<Box<dyn MessageHandler<Message> + Send + Sync>>,
    client: Arc<Client>,
    visibility_timeout_extender: Arc<VisibilityTimeoutExtender>,
    config: Box<SQSConsumerFinalisedConfig>,
}

impl SQSMessageConsumer {
    pub fn new(
        client: Arc<Client>,
        visibility_timeout_extender: Arc<VisibilityTimeoutExtender>,
        config: SQSConsumerConfig,
    ) -> SQSMessageConsumer {
        let final_config = SQSConsumerFinalisedConfig {
            queue_url: config.queue_url,
            polling_wait_time_ms: config.polling_wait_time_ms,
            batch_size: config.batch_size.unwrap_or(1),
            message_handler_timeout: config.message_handler_timeout,
            visibility_timeout: config.visibility_timeout.unwrap_or(30),
            wait_time_seconds: config.wait_time_seconds.unwrap_or(20),
            auth_error_timeout: config.auth_error_timeout.unwrap_or(10),
            terminate_visibility_timeout: config.terminate_visibility_timeout,
            should_delete_messages: config.should_delete_messages,
            delete_messages_on_handler_failure: config
                .delete_messages_on_handler_failure
                .unwrap_or(true),
            attribute_names: config.attribute_names,
            message_attribute_names: config.message_attribute_names,
        };
        SQSMessageConsumer {
            handler: None,
            client,
            visibility_timeout_extender,
            config: Box::new(final_config),
        }
    }

    pub fn register_handler(&mut self, handler: Box<dyn MessageHandler<Message> + Send + Sync>) {
        self.handler = Some(handler);
    }

    #[instrument(name = "sqs_message_consumer", skip(self))]
    pub async fn start(&self) -> Result<(), Error> {
        loop {
            let mut current_polling_timeout = self.config.polling_wait_time_ms;
            let result = self.receive_messages().await;
            if let Err(SdkError::ServiceError(service_err)) = result {
                let source = service_err.err();
                let raw = service_err.raw();
                if is_connection_error(source, raw.status()) {
                    debug!("there was an authentication error. pausing before retrying.");
                    current_polling_timeout = self.config.auth_error_timeout;
                } else {
                    error!(
                        "failed to receive and handle messages from queue: {}",
                        service_err.err()
                    )
                }
            }
            task::sleep(Duration::from_millis(current_polling_timeout)).await;
        }
    }

    #[instrument(skip(self, message))]
    async fn handle_single_message(&self, message: Message) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle(message.clone())),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        self.handle_messages_future(future_result).await
    }

    #[instrument(skip(self, messages))]
    async fn handle_messages(&self, messages: Vec<Message>) -> Result<(), MessageHandlerError> {
        let future_result = match &self.handler {
            Some(handler) => Ok(handler.handle_batch(messages.clone())),
            _ => Err(MessageHandlerError::MissingHandler),
        };

        self.handle_messages_future(future_result).await
    }

    async fn handle_messages_future(
        &self,
        future_result: Result<PinnedMessageHandlerFuture<'_>, MessageHandlerError>,
    ) -> Result<(), MessageHandlerError> {
        if future_result.is_err() {
            return Err(future_result.err().unwrap());
        }

        debug!(
            timeout = self.config.message_handler_timeout.clone(),
            "running message handler with timeout",
        );
        match timeout(
            Duration::from_secs(self.config.message_handler_timeout),
            future_result.unwrap(),
        )
        .await
        {
            Err(timeout_err) => Err(MessageHandlerError::Timeout(timeout_err)),
            Ok(result) => result,
        }
    }

    fn derive_handler_future(&self, messages: Vec<Message>) -> PinnedMessageHandlerFuture<'_> {
        if messages.len() == 1 {
            Box::pin(self.handle_single_message(messages[0].clone()))
        } else {
            Box::pin(self.handle_messages(messages))
        }
    }

    async fn terminate_visibility_timeout(&self, messages: Vec<Message>) -> Result<(), Error> {
        if !self.config.terminate_visibility_timeout {
            debug!("sqs consumer not configured to terminate visibility timeout, moving on");
            return Ok(());
        }

        let result = self
            .visibility_timeout_extender
            .change_visibility_timeout(messages, Some(0))
            .await;

        if result.is_err() {
            let err = result.err().unwrap();
            error!("failed to terminate visibility timeout: {}", err);
        }
        Ok(())
    }

    async fn delete_messages(
        &self,
        messages: Vec<Message>,
        handler_failed: bool,
    ) -> Result<(), Error> {
        if !self.config.should_delete_messages {
            debug!("skipping message deletion as should_delete_messages is set to false");
            return Ok(());
        }

        if handler_failed && !self.config.delete_messages_on_handler_failure {
            debug!(concat!(
                "skipping message deletion as handler failed and ",
                "delete_messages_on_handler_failure is set to false"
            ));
            return Ok(());
        }

        if messages.is_empty() {
            debug!("skipping message deletion as there are no messages to delete");
            return Ok(());
        }

        debug!("deleting handled message batch");
        let result = self
            .client
            .delete_message_batch()
            .queue_url(self.config.queue_url.clone())
            .set_entries(Some(
                messages
                    .into_iter()
                    .map(|message| {
                        DeleteMessageBatchRequestEntry::builder()
                            .set_id(message.message_id)
                            .set_receipt_handle(message.receipt_handle)
                            .build()
                            .unwrap()
                    })
                    .collect(),
            ))
            .send()
            .await;

        if result.is_err() {
            let err = result.err().unwrap();
            error!("failed to delete messages from queue: {}", err);
        }
        Ok(())
    }

    #[instrument(skip(self))]
    async fn receive_messages(&self) -> Result<(), SdkError<ReceiveMessageError>> {
        let rcv_message_output = self
            .client
            .receive_message()
            .queue_url(self.config.queue_url.clone())
            .set_wait_time_seconds(Some(self.config.wait_time_seconds))
            .set_max_number_of_messages(Some(self.config.batch_size))
            .set_visibility_timeout(Some(self.config.visibility_timeout))
            .set_message_system_attribute_names(self.config.attribute_names.clone())
            .set_message_attribute_names(self.config.message_attribute_names.clone())
            .send()
            .await?;

        let messages = rcv_message_output.messages.unwrap_or_default();
        let handle_msg_future = self.derive_handler_future(messages.clone());

        // May or may not start a heartbeat for the visibility timeout extender,
        // it's at the discretion of the visibility timeout extender.
        let send_kill_heartbeat_opt = self
            .visibility_timeout_extender
            .clone()
            .start_heartbeat(messages.clone());

        let result = handle_msg_future.await;
        if let Some(send_kill_heartbeat) = send_kill_heartbeat_opt {
            debug!("sending kill signal to visibility timeout extender");
            match send_kill_heartbeat.send(()) {
                Ok(_) => (),
                Err(_) => error!("the heartbeat task receiver dropped"),
            }
        }
        let _res = self
            .delete_messages(messages.clone(), result.is_err())
            .await;

        match result {
            Ok(_) => (),
            Err(error) => {
                let _res = self.terminate_visibility_timeout(messages).await;

                match error {
                    MessageHandlerError::Timeout(_) => {
                        let message_handler_timeout = self.config.message_handler_timeout;
                        error!(
                            "did not finish processing message(s) within {:?} seconds",
                            message_handler_timeout
                        );
                    }
                    MessageHandlerError::MissingHandler => {
                        error!("message handler was not registered")
                    }
                    MessageHandlerError::HandlerFailure(handler_error) => {
                        error!("message handler failed: {}", handler_error)
                    }
                }
            }
        }
        Ok(())
    }
}
