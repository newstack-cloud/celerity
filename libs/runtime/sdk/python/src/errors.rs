use axum::{body::Body, http::StatusCode, response::IntoResponse};
use celerity_runtime_core::errors::WebSocketsMessageError;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct HandlerError {
  pub message: String,
  #[serde(skip)]
  pub is_timeout: bool,
}

impl HandlerError {
  pub fn new(message: String) -> Self {
    Self {
      message,
      is_timeout: false,
    }
  }

  pub fn timeout() -> Self {
    Self {
      message: "handler timed out".to_string(),
      is_timeout: true,
    }
  }
}

impl std::fmt::Display for HandlerError {
  fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
    write!(f, "{}", self.message)
  }
}

impl IntoResponse for HandlerError {
  fn into_response(self) -> axum::response::Response<Body> {
    let status = if self.is_timeout {
      StatusCode::GATEWAY_TIMEOUT
    } else {
      StatusCode::INTERNAL_SERVER_ERROR
    };
    (status, axum::response::Json(self)).into_response()
  }
}

impl From<HandlerError> for WebSocketsMessageError {
  fn from(err: HandlerError) -> Self {
    WebSocketsMessageError::UnexpectedError(err.message)
  }
}
