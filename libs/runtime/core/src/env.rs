use std::env::{self, VarError};

/// Provides a wrapper around variables
/// provided by the current environment.
pub trait EnvVars {
    /// Fetches the environment variable `key` from the current process or equivalent
    /// environment.
    ///
    /// An implementation of this trait should return VarErrors
    /// in failure to retrieve an environment variable.
    fn var(&self, key: &str) -> Result<String, VarError>;
}

/// Environment variables sourced from the current process.
pub struct ProcessEnvVars {}

impl ProcessEnvVars {
    /// Creates a new instance of environment variables
    /// sourced from the current process.
    pub fn new() -> Self {
        ProcessEnvVars {}
    }
}

impl EnvVars for ProcessEnvVars {
    fn var(&self, key: &str) -> Result<String, VarError> {
        env::var(key)
    }
}
