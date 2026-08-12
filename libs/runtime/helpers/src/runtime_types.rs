use serde::{Deserialize, Serialize};

/// Determines the mode in which the runtime interacts
/// with handlers.
#[derive(Debug, PartialEq)]
pub enum RuntimeCallMode {
    /// Handlers run in-process, called through a foreign function interface.
    /// Used by the Node.js (NAPI) and Python (PyO3) SDKs.
    Ffi,
    /// Handlers run in a separate executable, driven over an IPC transport.
    /// This mode is useful for languages that are compiled ahead of time
    /// such as Go, Rust, C and C++.
    Ipc,
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
