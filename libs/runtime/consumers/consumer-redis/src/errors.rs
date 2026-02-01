use std::fmt::Display;

use redis::RedisError;

#[derive(Debug)]
pub struct WorkerError {
    message: String,
}

impl WorkerError {
    pub fn new(message: String) -> Self {
        Self { message }
    }
}

impl Display for WorkerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "Worker error: {}", self.message)
    }
}

impl From<RedisError> for WorkerError {
    fn from(err: RedisError) -> Self {
        Self {
            message: err.to_string(),
        }
    }
}
