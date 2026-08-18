# Contributing to the core runtime

## Setup

The app is its own cargo workspace, with path dependencies on the runtime crates under `libs/runtime`. It is not a member of that workspace: it pins exact dependency versions and is released as a container image on its own cadence, neither of which fits a workspace whose members are libraries released together.

```bash
cd apps/runtime/core
cargo build
```

Changes to `libs/runtime` are picked up straight away, since the dependencies are paths rather than versions.

## Before committing

```bash
cargo fmt
cargo clippy --all-targets -- -D warnings
cargo test
```

## Running it locally

The runtime needs a blueprint and a handlers executable to start:

```bash
cp .env.example .env
# Point CELERITY_BLUEPRINT and CELERITY_HANDLERS_EXECUTABLE at your own
cargo run
```

Or build the image, from the repository root, since the build context needs both directories:

```bash
docker build -f apps/runtime/core/Dockerfile --target dev -t celerity-runtime-core:dev .
```

## What belongs here

Only the process around the runtime including reading the environment, starting the runtime, starting and supervising the handlers executable, and shutdown ordering. Everything the runtime itself does belongs in `libs/runtime/core` under the `lib-rt-core` commit scope, where it is shared with the language-specific runtimes.

## Commit scope

`runtime-core`

```
feat(runtime-core): restart the handlers executable on failure
```

The release component is `runtime-core-app` rather than `runtime-core`, because the `runtime-core` component name is already the tag prefix for the Rust crates under `libs/runtime`. Scopes and components are separate, release-please routes a commit by the paths it changes, not by its scope, so a `runtime-core` commit touching this directory lands in the `runtime-core-app` component.

See [COMMIT_GUIDELINES.md](../../../COMMIT_GUIDELINES.md) for the full conventions.
