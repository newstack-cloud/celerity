use tokio::sync::mpsc::error::SendError;

#[derive(Debug)]
pub enum WebSocketConnError {
    SendMessageError(String),
    BroadcastMessageError(String),
    MessageLost(String),
    AckCheckFailed(String),
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
