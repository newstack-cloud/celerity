//! The handler stream itself rather than any application served over it: the
//! socket the runtime listens on, who is allowed to reach it, and what happens
//! when one is already there.

mod common;

use celerity_runtime_core::{application::Application, config::RuntimeConfig};
use common::ipc::{ipc_env, start_runtime, start_runtime_with, HandlerStub};

#[test_log::test(tokio::test)]
async fn restricts_the_handler_socket_to_the_runtime_user() {
    use std::os::unix::fs::PermissionsExt;

    let (_app, _addr, socket) = start_runtime(
        "ipc-socket-mode",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;

    // Anything able to connect can register as a handler and be given events,
    // so the permissions on this socket are the whole of the access control.
    let mode = tokio::fs::metadata(&socket)
        .await
        .expect("the socket should exist")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o600, "socket mode was {mode:o}");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn restricts_a_socket_directory_it_creates_itself() {
    use std::os::unix::fs::PermissionsExt;

    let dir = std::env::temp_dir().join(format!("celerity-ipc-dir-{}", std::process::id()));
    let _ = tokio::fs::remove_dir_all(&dir).await;
    let socket = dir.join("runtime.sock").to_string_lossy().into_owned();

    let (_app, _addr, _) = start_runtime_with(
        "ipc-socket-dir",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
        &[("CELERITY_RUNTIME_SOCKET", &socket)],
    )
    .await;

    // The socket's own mode is not the whole story. Enforcing it on connect is
    // a Linux behaviour rather than a portable one, and a permissive umask
    // would leave a directory another user could replace the socket in.
    let mode = tokio::fs::metadata(&dir)
        .await
        .expect("the runtime should have created the directory")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o700, "directory mode was {mode:o}");

    let _ = tokio::fs::remove_dir_all(&dir).await;
}

#[test_log::test(tokio::test)]
async fn refuses_to_take_over_a_socket_another_runtime_is_listening_on() {
    let (_app, _addr, socket) = start_runtime(
        "ipc-socket-contended",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;

    // A second runtime pointed at the same socket must not unlink it. Doing so
    // would leave the first serving a socket nothing can reach any more.
    let env_vars = ipc_env(
        "ipc-socket-contended-second",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
        &socket,
        &[("CELERITY_SERVER_PORT", "0")],
    );
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut second = Application::new(runtime_config, Box::new(env_vars));
    second.setup().unwrap();

    let started = second.run(false).await;
    assert!(
        started.is_err(),
        "the second runtime should refuse to start rather than take the socket"
    );

    // The first is still reachable, which is the point of refusing.
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    let _ = tokio::fs::remove_file(&socket).await;
}
