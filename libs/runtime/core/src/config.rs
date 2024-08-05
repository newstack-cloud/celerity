use crate::env::EnvVars;

/// Core runtime configuration
/// that is used to locate blueprint files
/// and determine how to set up an application.
pub struct RuntimeConfig {
    pub blueprint_config_path: String,
    pub runtime_call_mode: RuntimeCallMode,
    pub server_port: i32,
    /// Optional flag to determine whether the
    /// HTTP/WebSocket server should only be exposed
    /// on the loopback interface (127.0.0.1).
    ///
    /// When running in an environment such as a docker
    /// container, this should be set to false
    /// so that the server can be accessed from outside
    /// the container.
    ///
    /// Defaults to true.
    pub server_loopback_only: Option<bool>,
}

impl RuntimeConfig {
    /// Creates a new instance of runtime configuration,
    /// sourcing config from the current process environment
    /// variables.
    pub fn from_env(env: &impl EnvVars) -> Self {
        let blueprint_config_path = env.var("CELERITY_BLUEPRINT").unwrap();
        let runtime_call_mode = env.var("CELERITY_RUNTIME_CALL_MODE").unwrap();
        let runtime_call_mode = match runtime_call_mode.as_str() {
            "ffi" => RuntimeCallMode::Ffi,
            "execHttp" => RuntimeCallMode::ExecHttp,
            _ => panic!("Invalid runtime call mode, must be one of 'ffi' or 'execHttp'"),
        };
        let server_port = env
            .var("CELERITY_SERVER_PORT")
            .unwrap()
            .parse()
            .expect("Invalid server port, must be a valid integer");
        let server_loopback_only_env_var = env
            .var("CELERITY_SERVER_LOOPBACK_ONLY")
            .map(Some)
            .unwrap_or_else(|_| None);
        let server_loopback_only = server_loopback_only_env_var.map(|val| {
            val.parse().expect(
                "Invalid server loopback only value, must be either \\\"true\\\" or \\\"false\\\"",
            )
        });
        RuntimeConfig {
            blueprint_config_path,
            runtime_call_mode,
            server_port,
            server_loopback_only,
        }
    }
}

/// Determines the mode in which the runtime interacts
/// with handlers.
#[derive(Debug, PartialEq)]
pub enum RuntimeCallMode {
    // FFI mode, where the runtime calls into a handler
    // via a foreign function interface.
    Ffi,
    // Exec/HTTP mode, where the runtime executes a handler
    // binary and exposes a HTTP API on localhost
    // on a port that must not be exposed outside of the container
    // or host machine.
    // The HTTP API allows the handler to retrieve the latest message/request
    // from the runtime and send a response back to the runtime.
    ExecHttp,
}
