//! Starts the handlers executable and ties its lifetime to the runtime's.
//!
//! The two processes are a pair where neither is useful without the other, so when
//! one goes the other follows. The runtime is PID 1 in the container and owns
//! the ordering, which matters on shutdown as handlers have to be drained over
//! the IPC stream before the process serving that stream is allowed to die.

use std::process::{ExitStatus, Stdio};

use tokio::process::{Child, Command};
use tracing::{error, info, warn};

use crate::config::SupervisorConfig;

/// A running handlers executable.
pub struct Handlers {
    child: Child,
    config: SupervisorConfig,
}

#[derive(Debug)]
pub enum SupervisorError {
    /// The executable could not be started at all.
    Spawn {
        executable: String,
        source: std::io::Error,
    },
    /// The child was started but waiting on it failed.
    Wait(std::io::Error),
}

impl std::fmt::Display for SupervisorError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SupervisorError::Spawn { executable, source } => write!(
                f,
                "could not start the handlers executable at {executable}: {source}"
            ),
            SupervisorError::Wait(source) => {
                write!(f, "could not wait for the handlers executable: {source}")
            }
        }
    }
}

impl std::error::Error for SupervisorError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            SupervisorError::Spawn { source, .. } => Some(source),
            SupervisorError::Wait(source) => Some(source),
        }
    }
}

impl Handlers {
    /// Starts the handlers executable.
    ///
    /// Call this only once the runtime is serving the handler stream, so that
    /// the executable's first connection attempt has something to reach.
    ///
    /// The child inherits this process's environment, which is how it is told
    /// where to connect: `CELERITY_RUNTIME_SOCKET` and the loopback fallback
    /// settings are already there. It also inherits stdout and stderr, so its
    /// logs reach the container's streams in the form the SDK wrote them rather
    /// than wrapped in a line of ours.
    pub fn start(config: SupervisorConfig) -> Result<Self, SupervisorError> {
        let mut command = Command::new(&config.executable);
        command
            .args(&config.args)
            .stdin(Stdio::null())
            .stdout(Stdio::inherit())
            .stderr(Stdio::inherit())
            // Without this the child is left running whenever the runtime dies
            // in a way that skips `shutdown`.
            .kill_on_drop(true);

        if let Some(working_dir) = &config.working_dir {
            command.current_dir(working_dir);
        }

        let child = command.spawn().map_err(|source| SupervisorError::Spawn {
            executable: config.executable.clone(),
            source,
        })?;

        info!(
            executable = %config.executable,
            pid = child.id().unwrap_or_default(),
            "started the handlers executable"
        );

        Ok(Handlers { child, config })
    }

    /// Resolves when the handlers executable exits on its own.
    ///
    /// A handler process that exits while the runtime is still serving is a
    /// failure however it exited, there is nothing left to dispatch to, so a
    /// zero exit code is no better than a crash.
    pub async fn wait(&mut self) -> Result<ExitStatus, SupervisorError> {
        self.child.wait().await.map_err(SupervisorError::Wait)
    }

    /// Asks the handlers executable to stop and waits for it to.
    ///
    /// `SIGTERM` first, so an SDK gets to run whatever teardown it registered,
    /// then `SIGKILL` once the grace period is up. Returns the exit status when
    /// the child was still running, and `None` when it had already gone.
    pub async fn shutdown(&mut self) -> Option<ExitStatus> {
        let Some(pid) = self.child.id() else {
            // Already cleaned up, which is the case whenever the child exiting is
            // what started the shutdown.
            return None;
        };

        info!(pid, "asking the handlers executable to stop");
        signal_terminate(pid);

        match tokio::time::timeout(self.config.shutdown_timeout, self.child.wait()).await {
            Ok(Ok(status)) => {
                info!(pid, ?status, "the handlers executable stopped");
                Some(status)
            }
            Ok(Err(err)) => {
                warn!(pid, "could not wait for the handlers executable: {err}");
                None
            }
            Err(_) => {
                warn!(
                    pid,
                    timeout_secs = self.config.shutdown_timeout.as_secs(),
                    "the handlers executable did not stop within the grace period, killing it"
                );
                if let Err(err) = self.child.kill().await {
                    error!(pid, "could not kill the handlers executable: {err}");
                }
                self.child.wait().await.ok()
            }
        }
    }
}

/// Sends `SIGTERM` to `pid`.
///
/// `Child::kill` sends `SIGKILL`, which gives the handlers executable no chance
/// to close its stream cleanly, so the polite signal has to go through libc.
fn signal_terminate(pid: u32) {
    // Safe: `kill` takes two integers by value and touches no memory owned
    // here. The pid comes from a child this process has not yet reaped, so it
    // cannot have been recycled onto an unrelated process.
    let result = unsafe { libc::kill(pid as libc::pid_t, libc::SIGTERM) };
    if result != 0 {
        let err = std::io::Error::last_os_error();
        warn!(
            pid,
            "could not send SIGTERM to the handlers executable: {err}"
        );
    }
}
