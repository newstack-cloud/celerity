use std::fmt::Display;

use base64::DecodeError;
use tokio::sync::mpsc::error::SendError;

#[derive(Debug)]
pub enum WebSocketConnError {
    SendMessageError(String),
    BroadcastMessageError(String),
    MessageLost(String),
    AckCheckFailed(String),
    Base64DecodeError(String),
}

impl Display for WebSocketConnError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WebSocketConnError::SendMessageError(e) => write!(f, "SendMessageError: {e}"),
            WebSocketConnError::BroadcastMessageError(e) => {
                write!(f, "BroadcastMessageError: {e}")
            }
            WebSocketConnError::MessageLost(e) => write!(f, "MessageLost: {e}"),
            WebSocketConnError::AckCheckFailed(e) => write!(f, "AckCheckFailed: {e}"),
            WebSocketConnError::Base64DecodeError(e) => write!(f, "Base64DecodeError: {e}"),
        }
    }
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

impl From<DecodeError> for WebSocketConnError {
    fn from(error: DecodeError) -> Self {
        WebSocketConnError::Base64DecodeError(error.to_string())
    }
}
