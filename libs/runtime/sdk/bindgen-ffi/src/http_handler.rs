use axum::{body::Body, response::IntoResponse};
use serde::{Deserialize, Serialize};
use std::ffi::CString;

use crate::ffi;

impl sfio_promise::FutureType<Result<String, HandlerError>> for ffi::HandleHttpRequest {
    fn on_drop() -> Result<String, HandlerError> {
        Err(HandlerError::new("Future dropped".into()))
    }

    fn complete(self, result: Result<String, HandlerError>) {
        match result {
            Ok(x) => {
                let value = CString::new(x).unwrap();
                self.on_complete(&value);
            }
            Err(err) => {
                self.on_failure(err.into());
            }
        }
    }
}

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

impl From<HandlerError> for ffi::HandleRequestError {
    fn from(err: HandlerError) -> Self {
        ffi::HandleRequestError::HandlerFailed
    }
}
