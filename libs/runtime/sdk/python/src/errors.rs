use axum::{body::Body, response::IntoResponse};
use celerity_runtime_core::errors::WebSocketsMessageError;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct HandlerError {
  pub message: String,
}

impl HandlerError {
  pub fn new(message: String) -> Self {
    Self { message }
  }
}

impl std::fmt::Display for HandlerError {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    write!(f, "{}", self.message)
  }
}

impl IntoResponse for HandlerError {
  fn into_response(self) -> axum::response::Response<Body> {
    axum::response::Json(self).into_response()
  }
}

impl From<HandlerError> for WebSocketsMessageError {
  fn from(err: HandlerError) -> Self {
    WebSocketsMessageError::UnexpectedError(err.message)
  }
}
