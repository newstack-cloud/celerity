use base64::{prelude::BASE64_STANDARD, Engine};
use celerity_helpers::consumers::Message;
use redis::{streams::StreamId, ToRedisArgs, Value};

#[derive(Debug, Clone, Default)]
pub struct RedisMessageMetadata {
    /// A timestamp in seconds since the Unix epoch.
    pub timestamp: u64,
    /// A timestamp in seconds since the Unix epoch
    /// for when the last attempt to process the message failed.
    pub failed_at: Option<u64>,
    /// The number of retry attempts that have been made to process the message.
    pub retry_count: Option<u64>,
    /// The reason why the message failed to be processed in previous attempts.
    /// This will most often appear in messages that have been moved to a dead letter queue.
    pub failure_reason: Option<String>,
    /// The type of message that was received.
    /// Useful for determining how to parse the body of the message.
    pub message_type: RedisMessageType,
}

/// The type of message that was received.
/// Useful for determining how to parse the body of the message,
/// binary messages are base64-encoded strings in the `Message` struct
/// `body` field.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RedisMessageType {
    Text,
    Binary,
}

impl Default for RedisMessageType {
    fn default() -> Self {
        Self::Text
    }
}

impl From<u64> for RedisMessageType {
    fn from(value: u64) -> Self {
        match value {
            0 => Self::Text,
            1 => Self::Binary,
            _ => panic!("invalid message type"),
        }
    }
}

pub trait FromRedisStreamId {
    fn from_redis_stream_id(stream_id: &StreamId) -> Self;
}

impl FromRedisStreamId for Message<RedisMessageMetadata> {
    fn from_redis_stream_id(stream_id: &StreamId) -> Self {
        let body = from_redis_stream_field_value(
            "body",
            stream_id
                .map
                .get("body")
                .expect("body must be present in stream message"),
        )
        .expect("failed to get body from redis stream field value");

        let md5_of_body = match stream_id.map.get("md5_of_body") {
            Some(value) => from_redis_stream_field_value("md5_of_body", value).ok(),
            _ => None,
        };

        let timestamp = match stream_id.map.get("timestamp") {
            Some(value) => from_redis_stream_field_value("timestamp", value)
                .map(|s| s.parse::<u64>().expect("timestamp must be an integer"))
                .expect("timestamp must be present and valid in stream message"),
            _ => panic!("timestamp must be an integer"),
        };

        let failed_at = match stream_id.map.get("failed_at") {
            Some(value) => from_redis_stream_field_value("failed_at", value)
                .map(|s| s.parse::<u64>().expect("failed_at must be an integer"))
                .ok(),
            _ => None,
        };

        let message_type = match stream_id.map.get("message_type") {
            Some(value) => from_redis_stream_field_value("message_type", value)
                .map(|s| s.parse::<u64>().expect("message_type must be an integer"))
                .map(RedisMessageType::from)
                .ok(),
            _ => None,
        };

        let retry_count = match stream_id.map.get("retry_count") {
            Some(value) => from_redis_stream_field_value("retry_count", value)
                .map(|s| s.parse::<u64>().expect("retry_count must be an integer"))
                .ok(),
            _ => None,
        };

        let failure_reason = match stream_id.map.get("failure_reason") {
            Some(value) => from_redis_stream_field_value("failure_reason", value).ok(),
            _ => None,
        };

        Self {
            message_id: stream_id.id.clone(),
            body: Some(body),
            md5_of_body,
            metadata: RedisMessageMetadata {
                timestamp,
                failed_at,
                retry_count,
                failure_reason,
                message_type: message_type.unwrap_or_default(),
            },
            trace_context: None,
        }
    }
}

#[derive(Debug)]
pub struct StreamRedisArgParts<'a> {
    pub id: &'a str,
    pub fields: Vec<(&'a str, StreamRedisArgFieldValue)>,
}

#[derive(Debug)]
pub enum StreamRedisArgFieldValue {
    String(String),
    Int(i64),
    Uint(u64),
    Double(f64),
    Bool(bool),
    Null,
}

impl ToRedisArgs for StreamRedisArgFieldValue {
    fn write_redis_args<W>(&self, out: &mut W)
    where
        W: ?Sized + redis::RedisWrite,
    {
        match self {
            StreamRedisArgFieldValue::String(string) => string.write_redis_args(out),
            StreamRedisArgFieldValue::Int(int) => int.write_redis_args(out),
            StreamRedisArgFieldValue::Uint(uint) => uint.write_redis_args(out),
            StreamRedisArgFieldValue::Double(double) => double.write_redis_args(out),
            StreamRedisArgFieldValue::Bool(bool) => bool.write_redis_args(out),
            StreamRedisArgFieldValue::Null => None::<String>.write_redis_args(out),
        }
    }
}

pub trait ToStreamRedisArgParts<'a> {
    fn to_stream_redis_arg_parts(&'a self, for_dlq: bool) -> StreamRedisArgParts<'a>;
}

impl<'a> ToStreamRedisArgParts<'a> for Message<RedisMessageMetadata> {
    fn to_stream_redis_arg_parts(&'a self, for_dlq: bool) -> StreamRedisArgParts<'a> {
        let mut fields = vec![];
        fields.push((
            "timestamp",
            StreamRedisArgFieldValue::Uint(self.metadata.timestamp),
        ));

        if let Some(failed_at) = self.metadata.failed_at {
            fields.push(("failed_at", StreamRedisArgFieldValue::Uint(failed_at)));
        }

        fields.push((
            "message_type",
            StreamRedisArgFieldValue::Uint(self.metadata.message_type.clone() as u64),
        ));

        if let Some(body) = &self.body {
            fields.push(("body", StreamRedisArgFieldValue::String(body.clone())));
        }

        if let Some(md5_of_body) = &self.md5_of_body {
            fields.push((
                "md5_of_body",
                StreamRedisArgFieldValue::String(md5_of_body.clone()),
            ));
        }

        if let Some(failure_reason) = &self.metadata.failure_reason {
            fields.push((
                "failure_reason",
                StreamRedisArgFieldValue::String(failure_reason.clone()),
            ));
        }

        if let Some(retry_count) = self.metadata.retry_count {
            fields.push(("retry_count", StreamRedisArgFieldValue::Uint(retry_count)));
        }

        if for_dlq {
            fields.push((
                "original_message_id",
                StreamRedisArgFieldValue::String(self.message_id.clone()),
            ));
        }

        StreamRedisArgParts {
            id: &self.message_id,
            fields,
        }
    }
}

fn from_redis_stream_field_value(field_name: &str, field_value: &Value) -> Result<String, String> {
    match field_value {
        Value::BulkString(data) => {
            Ok(String::from_utf8(data.to_vec()).unwrap_or_else(|_| BASE64_STANDARD.encode(data)))
        }
        Value::SimpleString(data) => Ok(data.clone()),
        _ => Err(format!("{field_name} must be a simple or bulk string")),
    }
}
