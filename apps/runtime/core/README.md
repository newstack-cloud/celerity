# core runtime

The core runtime is for applications where the handlers are written in a language that is compiled ahead of time, such as Rust, C, C++ or Go.
It serves HTTP, WebSockets and queue consumers, and hands the events it takes to a **handlers executable** — a separate process compiled from your own code — over the [Celerity IPC protocol](../../../libs/runtime/proto/README.md).

The IPC protocol is one long-lived bidirectional gRPC stream over a unix socket. It carries events, results, configuration, WebSocket sends, cancellation and shutdown. Your handlers executable is expected to use a Celerity SDK for its language rather than to speak the protocol directly.

## The image

`ghcr.io/newstack-cloud/celerity-runtime-core` is a base image: it carries the runtime and no handlers. Build your own image from it and add your compiled executable and blueprint.

```dockerfile
FROM ghcr.io/newstack-cloud/celerity-runtime-core:0.1.0

COPY --chown=nobody handlers /opt/celerity/app/handlers
COPY --chown=nobody blueprint.yaml /opt/celerity/app/blueprint.yaml
```

Both paths are the defaults, so nothing else needs configuring to start. A `dev-` tagged image is published alongside each release with a shell and the usual debugging tools, for getting inside a running application locally.

## Two processes, one lifetime

The runtime is PID 1 in the container and starts the handlers executable itself, once the handler stream is bound and ready to be connected to. The executable inherits the runtime's environment, which is how it is told where to connect: `CELERITY_RUNTIME_SOCKET` is already there for an SDK to read. It also inherits stdout and stderr, so its logs reach the container's streams as it wrote them.

The two processes stand or fall together:

- On `SIGTERM` or `SIGINT`, the runtime stops taking new work and sends `Drain` over the stream, waits for in-flight events to come back within `CELERITY_DRAIN_TIMEOUT`, then sends `SIGTERM` to the executable and gives it `CELERITY_HANDLERS_SHUTDOWN_TIMEOUT` seconds to exit before killing it.
- If the handlers executable exits while the runtime is still serving, the runtime stops and exits non-zero. There is nothing left to dispatch to, so a clean exit from the executable is no better than a crash; exiting non-zero is what gets the pair restarted.
- If the handlers executable starts but never attaches to the handler stream within `CELERITY_HANDLERS_START_TIMEOUT`, the runtime stops and exits non-zero. A process that is alive but not serving never exits on its own, so without a deadline it would sit there looking healthy while every event was shed.

The runtime does not restart the executable on its own. Restarting the container is the orchestrator's job, and it is the only way to be sure both processes come back in the state they started in.

## Telling the orchestrator something is wrong

Two signals, covering the two ways a handlers executable fails.

A process that **dies** takes the runtime with it, and the container exits non-zero, which is what a restart policy acts on.

A process that is **alive but not serving** never exits, so it shows up in the health check instead. `GET /runtime/health/check` answers the question an orchestrator is really asking which is can this instance serve? In the core runtime it returns `503` while no handlers executable is attached, and `200` once one is. It is the same endpoint, at the same path, that every Celerity runtime serves and that container orchestration services and `celerity dev test` already poll, so nothing needs repointing. The other runtimes are unaffected: their handlers run in-process and cannot be absent, so the check stays unconditionally `200` there.

An instance is therefore unhealthy between binding its port and its handlers attaching. That is accurate rather than awkward as it cannot serve a request yet but it does mean a deployment wants a start period long enough to cover the executable's own startup, `healthCheck.startPeriod` on ECS or `startupProbe` on Kubernetes.

`CELERITY_USE_CUSTOM_HEALTH_CHECK` disables the built-in endpoint as before, at which point reporting readiness is the custom handler's job.

An application with no API in its blueprint whether that be queue consumers or schedules only, has no HTTP server to serve the check from. The start timeout is what covers it, by turning a handler that never attaches into a restart.

## Configuration

Every setting is an environment variable. See [.env.example](./.env.example) for the full list with defaults; the ones specific to this runtime are:

| Variable | Default | Description |
|----------|---------|-------------|
| `CELERITY_HANDLERS_EXECUTABLE` | `/opt/celerity/app/handlers` | Path to the compiled handlers executable. Set it only to override the default path; the executable itself is required either way, and the runtime exits if it cannot be started. |
| `CELERITY_HANDLERS_ARGS` | — | Arguments passed to the executable, split on whitespace. |
| `CELERITY_HANDLERS_WORKING_DIR` | the runtime's own | Directory the executable is started in. |
| `CELERITY_HANDLERS_SHUTDOWN_TIMEOUT` | `30` | Seconds between `SIGTERM` and `SIGKILL` for the executable. |
| `CELERITY_HANDLERS_START_TIMEOUT` | `60` | Seconds the executable has to attach to the handler stream before the runtime gives up and exits non-zero. `0` waits indefinitely. |
| `CELERITY_RUNTIME_SOCKET` | `/var/run/celerity/runtime.sock` | Unix socket the handler stream is served on. |

## Building locally

The app is its own cargo workspace with path dependencies on the runtime crates under `libs/runtime`:

```bash
cd apps/runtime/core
cargo build --release
```

The container image builds from the repository root, since the build needs both directories:

```bash
docker build -f apps/runtime/core/Dockerfile --target runtime -t celerity-runtime-core:local .
```

## Additional documentation

- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)
- [Contributing](./CONTRIBUTING.md)
- [Releasing](./RELEASING.md)
- [IPC handler protocol](../../../libs/runtime/proto/README.md)
