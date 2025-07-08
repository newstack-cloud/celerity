use std::env::{self, VarError};

/// Provides a wrapper around variables
/// provided by the current environment.
pub trait EnvVars: Send + Sync {
    /// Fetches the environment variable `key` from the current process or equivalent
    /// environment.
    ///
    /// An implementation of this trait should return VarErrors
    /// in failure to retrieve an environment variable.
    fn var(&self, key: &str) -> Result<String, VarError>;
    /// Clones the environment variables, this will usually be a shallow clone
    /// used to share references to the environment variables provider.
    fn clone_env_vars(&self) -> Box<dyn EnvVars>;
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

impl Default for ProcessEnvVars {
    fn default() -> Self {
        Self::new()
    }
}

impl EnvVars for ProcessEnvVars {
    fn var(&self, key: &str) -> Result<String, VarError> {
        env::var(key)
    }

    fn clone_env_vars(&self) -> Box<dyn EnvVars> {
        Box::new(ProcessEnvVars {})
    }
}

impl Clone for Box<dyn EnvVars> {
    fn clone(&self) -> Self {
        self.clone_env_vars()
    }
}
