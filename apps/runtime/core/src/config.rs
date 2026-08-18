use std::time::Duration;

use celerity_helpers::env::EnvVars;

/// How long to wait for the handlers executable to exit after it has been asked
/// to stop, before it is killed.
const DEFAULT_HANDLERS_SHUTDOWN_TIMEOUT_SECS: u64 = 30;

/// How long to wait for the handlers executable to attach to the handler stream
/// before giving up on it.
///
/// Generous enough for an executable that has real work to do before it
/// registers, and for a cold start on a loaded host.
const DEFAULT_HANDLERS_START_TIMEOUT_SECS: u64 = 60;

/// Settings for the handlers executable that this runtime supervises.
///
/// The runtime's own settings come from
/// [`celerity_runtime_core::config::RuntimeConfig`]; these cover only the child
/// process, which is this application's reason to exist.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SupervisorConfig {
    /// Path to the compiled handlers executable, from
    /// `CELERITY_HANDLERS_EXECUTABLE`.
    pub executable: String,
    /// Arguments passed to the executable, from `CELERITY_HANDLERS_ARGS`, split
    /// on whitespace. Empty when the variable is unset.
    pub args: Vec<String>,
    /// Directory the executable is started in, from
    /// `CELERITY_HANDLERS_WORKING_DIR`. The runtime's own working directory
    /// when unset.
    pub working_dir: Option<String>,
    /// Grace period between `SIGTERM` and `SIGKILL` for the child, from
    /// `CELERITY_HANDLERS_SHUTDOWN_TIMEOUT` in seconds.
    ///
    /// This is deliberately separate from `CELERITY_DRAIN_TIMEOUT`, which
    /// bounds how long the runtime waits for in-flight events to come back over
    /// the stream. A handler that has returned every result may still have its
    /// own teardown to do.
    pub shutdown_timeout: Duration,
    /// How long the executable has to attach to the handler stream before the
    /// runtime gives up and exits, from `CELERITY_HANDLERS_START_TIMEOUT` in
    /// seconds. Zero waits indefinitely.
    ///
    /// A process that starts but never attaches is the one failure an
    /// orchestrator cannot otherwise see: it has not exited, so the container
    /// stays up, while every event is shed because nothing serves it.
    pub start_timeout: Duration,
}

#[derive(Debug)]
pub enum ConfigError {
    /// A required variable is missing, or one that is set cannot be read as the
    /// type it needs to be.
    Invalid(String),
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ConfigError::Invalid(message) => write!(f, "{message}"),
        }
    }
}

impl std::error::Error for ConfigError {}

impl SupervisorConfig {
    pub fn from_env(env: &dyn EnvVars) -> Result<Self, ConfigError> {
        let executable = env.var("CELERITY_HANDLERS_EXECUTABLE").map_err(|_| {
            ConfigError::Invalid(
                "CELERITY_HANDLERS_EXECUTABLE must be set to the path of the compiled handlers \
                 executable that this runtime should start and stream events to"
                    .to_string(),
            )
        })?;

        if executable.trim().is_empty() {
            return Err(ConfigError::Invalid(
                "CELERITY_HANDLERS_EXECUTABLE is set but empty, it must be the path of the \
                 compiled handlers executable"
                    .to_string(),
            ));
        }

        let args = env
            .var("CELERITY_HANDLERS_ARGS")
            .map(|value| {
                value
                    .split_whitespace()
                    .map(ToString::to_string)
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();

        let working_dir = env
            .var("CELERITY_HANDLERS_WORKING_DIR")
            .ok()
            .filter(|dir| !dir.trim().is_empty());

        let shutdown_timeout = seconds(
            env,
            "CELERITY_HANDLERS_SHUTDOWN_TIMEOUT",
            DEFAULT_HANDLERS_SHUTDOWN_TIMEOUT_SECS,
        )?;
        let start_timeout = seconds(
            env,
            "CELERITY_HANDLERS_START_TIMEOUT",
            DEFAULT_HANDLERS_START_TIMEOUT_SECS,
        )?;

        Ok(SupervisorConfig {
            executable,
            args,
            working_dir,
            shutdown_timeout,
            start_timeout,
        })
    }
}

/// Reads a whole number of seconds from `key`, or `default` when it is unset.
fn seconds(env: &dyn EnvVars, key: &str, default: u64) -> Result<Duration, ConfigError> {
    let Ok(value) = env.var(key) else {
        return Ok(Duration::from_secs(default));
    };
    value.parse::<u64>().map(Duration::from_secs).map_err(|_| {
        ConfigError::Invalid(format!(
            "{key} must be a whole number of seconds, got \"{value}\""
        ))
    })
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::env::VarError;

    use pretty_assertions::assert_eq;

    use super::*;

    struct MapEnv(HashMap<String, String>);

    impl EnvVars for MapEnv {
        fn var(&self, key: &str) -> Result<String, VarError> {
            self.0.get(key).cloned().ok_or(VarError::NotPresent)
        }

        fn clone_env_vars(&self) -> Box<dyn EnvVars> {
            Box::new(MapEnv(self.0.clone()))
        }
    }

    fn env(pairs: &[(&str, &str)]) -> MapEnv {
        MapEnv(
            pairs
                .iter()
                .map(|(key, value)| (key.to_string(), value.to_string()))
                .collect(),
        )
    }

    #[test]
    fn test_from_env_applies_defaults() {
        let config =
            SupervisorConfig::from_env(&env(&[("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers")]))
                .unwrap();

        assert_eq!(
            config,
            SupervisorConfig {
                executable: "/app/handlers".to_string(),
                args: Vec::new(),
                working_dir: None,
                shutdown_timeout: Duration::from_secs(DEFAULT_HANDLERS_SHUTDOWN_TIMEOUT_SECS),
                start_timeout: Duration::from_secs(DEFAULT_HANDLERS_START_TIMEOUT_SECS),
            }
        );
    }

    #[test]
    fn test_from_env_reads_every_setting() {
        let config = SupervisorConfig::from_env(&env(&[
            ("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers"),
            (
                "CELERITY_HANDLERS_ARGS",
                "--verbose  --config /etc/app.json",
            ),
            ("CELERITY_HANDLERS_WORKING_DIR", "/app"),
            ("CELERITY_HANDLERS_SHUTDOWN_TIMEOUT", "5"),
            ("CELERITY_HANDLERS_START_TIMEOUT", "90"),
        ]))
        .unwrap();

        assert_eq!(
            config,
            SupervisorConfig {
                executable: "/app/handlers".to_string(),
                args: vec![
                    "--verbose".to_string(),
                    "--config".to_string(),
                    "/etc/app.json".to_string(),
                ],
                working_dir: Some("/app".to_string()),
                shutdown_timeout: Duration::from_secs(5),
                start_timeout: Duration::from_secs(90),
            }
        );
    }

    #[test]
    fn test_from_env_requires_an_executable() {
        let err = SupervisorConfig::from_env(&env(&[])).unwrap_err();
        assert!(err.to_string().contains("CELERITY_HANDLERS_EXECUTABLE"));
    }

    #[test]
    fn test_from_env_rejects_an_empty_executable() {
        let err = SupervisorConfig::from_env(&env(&[("CELERITY_HANDLERS_EXECUTABLE", "  ")]))
            .unwrap_err();
        assert!(err.to_string().contains("set but empty"));
    }

    #[test]
    fn test_from_env_rejects_a_non_numeric_shutdown_timeout() {
        let err = SupervisorConfig::from_env(&env(&[
            ("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers"),
            ("CELERITY_HANDLERS_SHUTDOWN_TIMEOUT", "a while"),
        ]))
        .unwrap_err();
        assert!(err.to_string().contains("whole number of seconds"));
    }

    #[test]
    fn test_from_env_rejects_a_non_numeric_start_timeout() {
        let err = SupervisorConfig::from_env(&env(&[
            ("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers"),
            ("CELERITY_HANDLERS_START_TIMEOUT", "forever"),
        ]))
        .unwrap_err();
        assert!(err.to_string().contains("CELERITY_HANDLERS_START_TIMEOUT"));
        assert!(err.to_string().contains("whole number of seconds"));
    }

    #[test]
    fn test_from_env_takes_zero_as_waiting_indefinitely() {
        let config = SupervisorConfig::from_env(&env(&[
            ("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers"),
            ("CELERITY_HANDLERS_START_TIMEOUT", "0"),
        ]))
        .unwrap();

        assert_eq!(config.start_timeout, Duration::ZERO);
    }

    #[test]
    fn test_from_env_ignores_a_blank_working_dir() {
        let config = SupervisorConfig::from_env(&env(&[
            ("CELERITY_HANDLERS_EXECUTABLE", "/app/handlers"),
            ("CELERITY_HANDLERS_WORKING_DIR", "   "),
        ]))
        .unwrap();

        assert_eq!(config.working_dir, None);
    }
}
