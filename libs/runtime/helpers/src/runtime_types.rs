use serde::{Deserialize, Serialize};

/// Determines the mode in which the runtime interacts
/// with handlers.
#[derive(Debug, PartialEq)]
pub enum RuntimeCallMode {
    // FFI mode, where the runtime calls into a handler
    // via a foreign function interface.
    Ffi,
    // HTTP mode, where the runtime exposes a HTTP API on localhost
    // on a port that must not be exposed outside of the container
    // or host machine.
    // The HTTP API allows the handlers to retrieve the latest message/request
    // from the runtime and send a response back to the runtime.
    // This mode is useful for languages that are compiled ahead of time
    // such as Go, Rust, C and C++.
    Http,
}

/// The platform that the runtime hosted application is running on.
#[derive(Debug, Clone, PartialEq)]
pub enum RuntimePlatform {
    AWS,
    Azure,
    GCP,
    Local,
    Other,
}

// Represents a response message to be used in runtime-specific
// API responses such as that of the local runtime API.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ResponseMessage {
    pub message: String,
}

// Represents a HTTP response for a health check of one of the
// Celerity runtimes.
#[derive(Deserialize, Serialize)]
pub struct HealthCheckResponse {
    pub timestamp: u64,
}
