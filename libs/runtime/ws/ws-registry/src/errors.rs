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
    // A binary message that is not in the Celerity Binary Message Format, which
    // a client has no way to read as anything other than a framed message.
    MalformedBinaryMessage(String),
    // The shared record of where a connection is could not be written or taken
    // away. The connection itself is unaffected, and what is lost is the other
    // nodes being able to find it.
    ConnectionLocationError(String),
    // The shared record of what has been forwarded to a client could not be
    // read or written. The message is forwarded anyway, so what is risked is a
    // client seeing it twice rather than not at all.
    ForwardedMessageError(String),
    // Something a registry is given once was given to it twice. Naming what,
    // since the second one is refused and the caller is about to act as though
    // it had been taken.
    AlreadyAttached(String),
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
            WebSocketConnError::MalformedBinaryMessage(e) => {
                write!(f, "MalformedBinaryMessage: {e}")
            }
            WebSocketConnError::ConnectionLocationError(e) => {
                write!(f, "ConnectionLocationError: {e}")
            }
            WebSocketConnError::ForwardedMessageError(e) => {
                write!(f, "ForwardedMessageError: {e}")
            }
            WebSocketConnError::AlreadyAttached(e) => write!(f, "AlreadyAttached: {e}"),
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
