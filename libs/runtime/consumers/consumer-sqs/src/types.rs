use std::collections::HashMap;

use aws_sdk_sqs::types::{
    Message as SQSMessage, MessageAttributeValue, MessageSystemAttributeName,
};
use celerity_helpers::{
    aws_telemetry::AWS_XRAY_TRACE_HEADER_NAME, consumers::Message,
    telemetry::CELERITY_CONTEXT_ID_KEY,
};
use serde::{Deserialize, Serialize};

/// A lightweight structure for holding the message ID and receipt handle
/// used to identify an SQS message in operations like delete
/// and changing the visibility timeout.
#[derive(Debug, Clone)]
pub struct MessageHandle {
    pub message_id: Option<String>,
    pub receipt_handle: Option<String>,
}

impl From<SQSMessage> for MessageHandle {
    fn from(message: SQSMessage) -> Self {
        MessageHandle {
            message_id: message.message_id,
            receipt_handle: message.receipt_handle,
        }
    }
}

#[derive(Debug, Clone, Default)]
pub struct SQSMessageMetadata {
    /// An identifier associated with the act of receiving the message.
    /// A new receipt handle is returned every time you receive a message.
    /// When deleting a message, you provide the last received receipt handle
    /// to delete the message.
    pub receipt_handle: Option<String>,
    /// <p>A map of the attributes requested in <code> <code>ReceiveMessage</code> </code> to their respective values. Supported attributes:</p>
    /// <ul>
    /// <li>
    /// <p><code>ApproximateReceiveCount</code></p></li>
    /// <li>
    /// <p><code>ApproximateFirstReceiveTimestamp</code></p></li>
    /// <li>
    /// <p><code>MessageDeduplicationId</code></p></li>
    /// <li>
    /// <p><code>MessageGroupId</code></p></li>
    /// <li>
    /// <p><code>SenderId</code></p></li>
    /// <li>
    /// <p><code>SentTimestamp</code></p></li>
    /// <li>
    /// <p><code>SequenceNumber</code></p></li>
    /// </ul>
    /// <p><code>ApproximateFirstReceiveTimestamp</code> and <code>SentTimestamp</code> are each returned as an integer representing the <a href="http://en.wikipedia.org/wiki/Unix_time">epoch time</a> in milliseconds.</p>
    pub attributes: Option<HashMap<MessageSystemAttributeName, String>>,
    /// <p>An MD5 digest of the non-URL-encoded message attribute string.
    /// You can use this attribute to verify that Amazon SQS received the message correctly.
    /// Amazon SQS URL-decodes the message before creating the MD5 digest.
    /// For information about MD5, see <a href="https://www.ietf.org/rfc/rfc1321.txt">RFC1321</a>.</p>
    pub md5_of_message_attributes: ::std::option::Option<::std::string::String>,
    /// <p>Each message attribute consists of a <code>Name</code>, <code>Type</code>, and <code>Value</code>. For more information, see
    /// <a href="https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-message-metadata.html#sqs-message-attributes">Amazon SQS message attributes</a>
    /// in the <i>Amazon SQS Developer Guide</i>.</p>
    pub message_attributes: Option<HashMap<String, MessageAttributeValue>>,
    /// The data of an embedded SNS message in the body of the SQS message.
    /// This is only present if the SQS message body is an SNS message.
    /// The body set for the primary message is set to the body of the embedded
    /// SNS message, this should be used to get access to the full SNS message.
    pub sns_data: Option<SNSMessage>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SNSMessage {
    #[serde(rename = "Type")]
    pub message_type: String,
    #[serde(rename = "MessageId")]
    pub message_id: String,
    #[serde(rename = "TopicArn")]
    pub topic_arn: String,
    #[serde(rename = "Message")]
    pub message: String,
    #[serde(rename = "Timestamp")]
    pub timestamp: String,
    #[serde(rename = "SignatureVersion")]
    pub signature_version: String,
    #[serde(rename = "Signature")]
    pub signature: String,
    #[serde(rename = "SigningCertURL")]
    pub signing_cert_url: String,
    #[serde(rename = "UnsubscribeURL")]
    pub unsubscribe_url: String,
    #[serde(rename = "MessageAttributes")]
    pub message_attributes: Option<HashMap<String, SNSMessageAttribute>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SNSMessageAttribute {
    #[serde(rename = "Type")]
    pub data_type: String,
    #[serde(rename = "Value")]
    pub value: String,
}

pub trait ToSQSMessageHandle {
    fn to_sqs_message_handle(&self) -> MessageHandle;
}

impl ToSQSMessageHandle for Message<SQSMessageMetadata> {
    fn to_sqs_message_handle(&self) -> MessageHandle {
        MessageHandle {
            message_id: Some(self.message_id.clone()),
            receipt_handle: self.metadata.receipt_handle.clone(),
        }
    }
}

pub trait FromSQSMessage {
    fn from_sqs_message(message: SQSMessage) -> Self;
}

impl FromSQSMessage for Message<SQSMessageMetadata> {
    fn from_sqs_message(message: SQSMessage) -> Self {
        let (sns_message, body) =
            match serde_json::from_str::<SNSMessage>(&message.body.clone().unwrap_or_default()) {
                Ok(sns_message) => {
                    let sns_message_body = sns_message.message.clone();
                    (Some(sns_message), Some(sns_message_body))
                }
                Err(_) => (None, message.body),
            };

        let mut message = Message {
            message_id: message.message_id.unwrap_or_default(),
            body,
            md5_of_body: message.md5_of_body,
            metadata: SQSMessageMetadata {
                receipt_handle: message.receipt_handle,
                attributes: message.attributes,
                md5_of_message_attributes: message.md5_of_message_attributes,
                message_attributes: message.message_attributes,
                sns_data: sns_message,
            },
            trace_context: None,
        };
        message.trace_context = extract_trace_context(&message);
        message
    }
}

fn extract_trace_context(message: &Message<SQSMessageMetadata>) -> Option<HashMap<String, String>> {
    let mut trace_context = HashMap::new();

    if let Some(attributes) = &message.metadata.attributes {
        if let Some(aws_trace_header) = attributes.get(&MessageSystemAttributeName::AwsTraceHeader)
        {
            trace_context.insert(
                AWS_XRAY_TRACE_HEADER_NAME.to_string(),
                aws_trace_header.clone(),
            );
        }
    }

    if let Some(sns_message) = &message.metadata.sns_data {
        if let Some(message_attributes) = &sns_message.message_attributes {
            if let Some(celerity_context_id) = message_attributes.get(CELERITY_CONTEXT_ID_KEY) {
                trace_context.insert(
                    CELERITY_CONTEXT_ID_KEY.to_string(),
                    celerity_context_id.value.clone(),
                );
            }
        }
    }

    if trace_context.is_empty() {
        None
    } else {
        Some(trace_context)
    }
}
