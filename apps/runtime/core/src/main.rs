//! The Celerity core runtime.
//!
//! Serves HTTP, WebSockets and queue consumers, and hands the events it takes
//! to a handlers executable over the Celerity IPC protocol —which is one long-lived
//! bidirectional gRPC stream.
//! The handlers executable is a separate process compiled from the developer's own code, in
//! any language with a Celerity SDK for ahead-of-time compiled languages.
//!
//! Everything the runtime itself does lives in `celerity_runtime_core`. This
//! binary is the process around it that reads the environment, starts the runtime,
//! starts the handlers executable, and holds the two together until one of them
//! stops.

use std::process::{ExitCode, ExitStatus};
use std::time::Duration;

use celerity_helpers::{env::ProcessEnvVars, runtime_types::RuntimeCallMode};
use celerity_runtime_core::{
    application::Application, config::RuntimeConfig, dispatcher::HandlerReadiness,
};
use tokio::signal::unix::{signal, SignalKind};
use tracing::{error, info};

mod config;
mod supervisor;

use config::SupervisorConfig;
use supervisor::Handlers;

/// Returned when the runtime or the handlers executable fails to start, or when
/// the handlers executable exits while the runtime is still serving.
const EXIT_FAILURE: u8 = 1;

#[tokio::main]
async fn main() -> ExitCode {
    match run().await {
        Ok(code) => code,
        Err(err) => {
            // Tracing may not be up yet, since most of what can fail here fails
            // before the runtime starts, so this goes to stderr as well.
            error!("{err}");
            eprintln!("celerity-runtime-core: {err}");
            ExitCode::from(EXIT_FAILURE)
        }
    }
}

async fn run() -> Result<ExitCode, Box<dyn std::error::Error>> {
    let env_vars = ProcessEnvVars::new();
    let runtime_config = RuntimeConfig::from_env(&env_vars);

    if runtime_config.runtime_call_mode != RuntimeCallMode::Ipc {
        return Err(format!(
            "the core runtime only serves handlers over the IPC protocol, but \
             CELERITY_RUNTIME_CALL_MODE is set to \"{:?}\". Unset it, or use the runtime for the \
             language your handlers are written in",
            runtime_config.runtime_call_mode
        )
        .into());
    }

    let supervisor_config = SupervisorConfig::from_env(&env_vars)?;

    let mut application = Application::new(runtime_config, Box::new(ProcessEnvVars::new()));
    // `ApplicationStartError` carries a `Display` message but does not
    // implement `std::error::Error`, so it cannot cross into the boxed error
    // this function returns on its own.
    application.setup().map_err(|err| err.to_string())?;

    // `false` so that this returns once everything is serving; the servers keep
    // running on their own tasks, and waiting is this function's job from here.
    let app_info = application
        .run(false)
        .await
        .map_err(|err| err.to_string())?;
    info!(
        http_server_address = ?app_info.http_server_address,
        "the core runtime is serving, starting the handlers executable"
    );

    let start_timeout = supervisor_config.start_timeout;

    // Start the handlers executable now that the handler stream is bound,
    // so the executable's first connection attempt has something to reach.
    let mut handlers = match Handlers::start(supervisor_config) {
        Ok(handlers) => handlers,
        Err(err) => {
            application.shutdown();
            return Err(err.into());
        }
    };

    let mut terminate = signal(SignalKind::terminate())?;
    let mut interrupt = signal(SignalKind::interrupt())?;

    // A handlers executable that never attaches is the one failure nothing else
    // reports: it has not exited, so the container stays up, while every event
    // is shed because no stream serves it. Giving up turns that into a restart.
    let attached = wait_until_attached(application.handler_readiness(), start_timeout);
    tokio::pin!(attached);

    let exit_code = tokio::select! {
        attached = &mut attached => {
            if !attached {
                error!(
                    timeout_secs = start_timeout.as_secs(),
                    "the handlers executable did not attach to the handler stream within the \
                     start timeout, stopping. Set CELERITY_HANDLERS_START_TIMEOUT to allow \
                     longer, or 0 to wait indefinitely"
                );
                application.shutdown();
                handlers.shutdown().await;
                return Ok(ExitCode::from(EXIT_FAILURE));
            }
            info!("the handlers executable attached, the application is ready");
            // Attaching is not a reason to stop waiting on everything else, so
            // this arm falls through to a second select that no longer watches
            // for it.
            tokio::select! {
                status = handlers.wait() => handlers_exited(status),
                _ = terminate.recv() => {
                    info!("received SIGTERM, draining");
                    ExitCode::SUCCESS
                }
                _ = interrupt.recv() => {
                    info!("received SIGINT, draining");
                    ExitCode::SUCCESS
                }
            }
        }
        status = handlers.wait() => handlers_exited(status),
        _ = terminate.recv() => {
            info!("received SIGTERM, draining");
            ExitCode::SUCCESS
        }
        _ = interrupt.recv() => {
            info!("received SIGINT, draining");
            ExitCode::SUCCESS
        }
    };

    // Shutdown the runtime first as it stops taking new work and sends `Drain` over the
    // stream, so the handlers executable has both the signal and a stream to
    // return its in-flight results on. Only then is the child told to stop.
    application.shutdown();
    handlers.shutdown().await;

    info!("the core runtime has stopped");
    Ok(exit_code)
}

/// Resolves to whether a handlers executable attached within `timeout`.
///
/// A zero timeout waits indefinitely, which is what a deployment that would
/// rather hang than restart asks for. So does the FFI call mode, where there is
/// no readiness to wait on, though this runtime refuses to start in that mode.
async fn wait_until_attached(readiness: Option<HandlerReadiness>, timeout: Duration) -> bool {
    let Some(mut readiness) = readiness else {
        // Unreachable while this runtime refuses to start outside the IPC call
        // mode, but waiting forever on a handle that will never be filled would
        // hang the process with nothing to say for it.
        error!("the runtime reported no handler readiness to wait on");
        return false;
    };

    if timeout.is_zero() {
        return readiness.wait_until_ready().await;
    }

    tokio::time::timeout(timeout, readiness.wait_until_ready())
        .await
        .unwrap_or(false)
}

/// The exit code for the handlers executable going away on its own.
///
/// Nothing is left to dispatch to, so the runtime cannot keep serving. Exiting
/// non-zero either way lets the orchestrator restart the pair, which a clean
/// exit here would not.
fn handlers_exited(status: Result<ExitStatus, supervisor::SupervisorError>) -> ExitCode {
    match status {
        Ok(status) => error!(
            ?status,
            "the handlers executable exited while the runtime was serving, stopping"
        ),
        Err(err) => error!("{err}, stopping"),
    }
    ExitCode::from(EXIT_FAILURE)
}
