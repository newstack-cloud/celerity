use std::fmt::Display;

use aws_sdk_sqs::{error::ProvideErrorMetadata, operation::receive_message::ReceiveMessageError};
use aws_smithy_runtime_api::http::StatusCode;
use http::StatusCode as StatusCodeHttp;

pub fn is_connection_error(err: &ReceiveMessageError, status: StatusCode) -> bool {
    let is_forbidden_response = status == StatusCode::from(StatusCodeHttp::FORBIDDEN);
    let err_code = err.code().unwrap_or("");
    let is_auth_err_code = err_code == "CredentialsError" || err_code == "UnknownEndpoint";
    is_forbidden_response || is_auth_err_code
}

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
