# Celerity

Celerity is a backend framework: the CLI that develops and builds Celerity applications, the
Rust runtime that hosts them, the runtime images, and the language bindings the SDKs build on.
The SDKs (celerity-node-sdk, celerity-python-sdk, celerity-go-sdk) and celerity-ws-client
depend on the runtime's bindings and protocols. The CLI uses the Bluelink deploy engine for
`validate` and its `--instance-name` commands; deploy commands aren't in it yet.

## Workspace

This repo is one of NewStack's. NewStack's maintainers develop it alongside the others in a
shared workspace, using the NewStack Claude Code plugin for how work is planned, reviewed,
committed and coordinated across repos. The plugin isn't public, and nothing here depends on it:
this file and the repo's own docs cover what a contribution needs.

## Layout

[docs/INDEX.md](docs/INDEX.md) lists the released projects and their components, and
[SOURCE_CONTROL_RELEASE_STRATEGY.md](SOURCE_CONTROL_RELEASE_STRATEGY.md) how they release.

- `apps/cli`, `apps/local-events`, `apps/dev-auth`: Go modules, tied by `go.work`. The CLI's
  bluelink libraries and deploy-cli-sdk aren't in it and build at the versions in `go.mod`.
- `libs/runtime`: one Cargo workspace for the runtime crates, consumers, WebSocket crates and
  bindings (`sdk/node`, napi-rs; `sdk/python`, PyO3); `proto/` holds the handler IPC protocol.
- `apps/runtime/core` is its own Cargo workspace, with path dependencies on `libs/runtime`;
  `apps/runtime/nodejs` and `apps/runtime/python` wrap the runtime in images.
- Due to be removed (as of 2026-11-22), since C# will use the IPC protocol as Go does and Java
  is dropped: `sdk/bindgen-*`, `sdk/runtime-bindings`, `sdk/bindings/{java,dotnet}`,
  `scripts/build-test-sdk-bindgen.*`, `lib-java-csharp-sdk-*.yml` and the `runtime-sdk-ffi`
  component. Don't extend or fix them.

## Checks

Commands are in `apps/cli/docs/CONTRIBUTING.md`, `apps/local-events/docs/CONTRIBUTING.md`,
`libs/runtime/CONTRIBUTING.md` (all the crates) and each binding's and image's
`CONTRIBUTING.md`; dev-auth has only `scripts/lint.sh`.

- **Tools:** Go 1.26.4 (`go.work`'s toolchain; an older Go downloads it), Rust 1.98.0
  (`rust-toolchain.toml`, repo-wide), `staticcheck`, `cargo-nextest`, `cargo-llvm-cov`, uv,
  Python 3.13, Node 24 with Yarn 4, docker; `buf` only to regenerate the proto stubs.
- **Go apps:** `scripts/lint.sh` (`staticcheck` and `go vet` per package) in each.
  - CLI: `scripts/run-tests.sh` starts `docker-compose.test-deps.yml` (DynamoDB Local 48000,
    MinIO 49000, postgres 45433, Valkey 46379); `--no-infra` skips it. Integration tests skip
    themselves when their stack is unreachable, so a down stack looks like a pass: check
    `go test -v` for SKIP. The MySQL tests run only with all five `CELERITY_TEST_MYSQL_*`
    variables set, which nothing in the repo does, so they skip by default.
  - local-events: `scripts/run-tests.sh` from `apps/local-events` (Valkey 16379, DynamoDB
    18000, MinIO 19000-19001). dev-auth has no tests. `--update-snapshots` does nothing.
- **`libs/runtime`:** `scripts/lint.sh`, which the hook and CI both run, is
  `cargo clippy -- -D warnings` then `cargo fmt`: it rewrites rather than checks formatting,
  and skips tests and features. Also run `cargo fmt --all -- --check` and clippy with
  `--all-targets --features celerity_local_consumers,celerity_runtime_core/ws_clustering`.
  - Tests: `uv sync --group dev`, then
    `uv run python scripts/package-test-tools.py --localdeps [--package <name>]`. It starts
    `docker-compose.test-deps.yml` (LocalStack 44566, Valkey 6379, a Valkey cluster on
    7100-7105 and 17100-17105), runs nextest with coverage, then a pass against the cluster.
    `--package` filters tests but still builds the workspace.
  - Always set `AWS_ACCESS_KEY_ID=test-key AWS_SECRET_ACCESS_KEY=test-secret`, as CI does:
    otherwise the harness and SQS consumer tests resolve the shell's credentials, which for an
    SSO or assume-role profile calls AWS before anything reaches LocalStack.
- **Bindings:** `sdk/node`: `yarn test` starts the runtime stack's Valkey, runs `napi build`
  and ava. `sdk/python`: `scripts/run-tests.sh` starts the same Valkey, runs
  `maturin develop` and pytest. Their cleanup runs `docker compose down` on the `libs/runtime`
  project, tearing down a concurrent runtime run. Their CI runs only on their own directories,
  so after a `libs/runtime` change run them yourself.
- **Runtime images:** `apps/runtime/core`: `scripts/lint.sh` (`clippy --all-targets`,
  `fmt --check`) and `cargo test`. nodejs and python have no local checks; CI lints python
  with ruff and mypy. Image builds pull from `dhi.io` (a login) and compile with the
  Dockerfile's Rust (1.97 for core), not `rust-toolchain.toml`.
- **Stacks:** one of each at a time. Compose projects (`cli`, `local-events`, `runtime`) and
  container names are fixed, as is `celerity-schema-devdb` (`schema diff|validate|drift`).
  Valkey on 6379 clashes with any local Redis and with `celerity dev run`.
- **Generated files:** `libs/runtime/core/src/generated/celerity.runtime.v1.rs` is committed;
  after changing `proto/`, run `cargo run -p celerity-proto-gen` from `libs/runtime`. CI runs
  `buf lint` and fails if the stubs differ. `sdk/node`'s `index.js` and `index.d.ts` are
  written by `napi build` and committed; the `.node` binary isn't, so build before testing.
- **Gates:**
  - With `git config core.hooksPath .githooks`, pre-commit runs `scripts/lint.sh` for each of
    `libs/runtime`, `apps/cli`, `apps/local-events`, `apps/dev-auth` and `apps/runtime/core`
    with tracked changes. commit-msg runs commitlint through `npm exec`; without a root
    `node_modules` (`YARN_NODE_LINKER=node-modules yarn install` at the root), it downloads
    commitlint each time.
  - CI has a workflow per project (`app-*-ci.yml`, `lib-*-ci.yml`), path-filtered, plus
    `cargo-deny.yml`, `codeql.yml` (Go, `apps/cli` only) and `infrastructure-security.yml`.
    No CI step runs commitlint.

## Running locally

`celerity dev run`, in an application's directory, runs the runtime image for its language
with the emulators its blueprint needs (Valkey, postgres, MinIO, DynamoDB Local, local-events,
dev-auth, stubs, Jaeger) as compose project `celerity-dev-<dir>`, in the foreground unless
`--detached`. It writes `.celerity/dev.state.json`, the merged blueprint and any compiled
handlers under `.celerity/`; `dev status|logs|stop` act on it. `dev test` does the same at
ports +100. Ports are fixed (8080, 6379, 5432, ...), so run it only in a scratch app and when
asked. Images come from GHCR (`dev-<major.minor>`): to try runtime changes, build the image's
`Dockerfile.local` and pass `--image`.

## Compatibility

- **CLI:** commands, flags, `CELERITY_CLI_*` environment variables and app config files:
  additive only. It pins deploy-cli-sdk and the bluelink libraries in `go.mod`, the plugin
  constraints it sends the engine (`defaultPluginConstraints` in
  `internal/deployconfig/convert.go`; the transformer is unconstrained until it releases),
  the core image (`CoreRuntimeImageVersion`) and a fallback image
  (`DefaultRuntimeImageVersion`), and picks the Node or Python image from the app's SDK
  version. The newstack plugin's `cross-repo` skill sets out the rest.
- **Protocols:** the handler IPC protocol (`libs/runtime/proto`) is used by celerity-go-sdk,
  whose copy of `runtime.proto` CI requires to match `main` byte for byte, and by the planned
  C# SDK. celerity-ws-client implements the WebSocket protocol. Both are documented in
  celerity-docs, and both are additive only: keep field numbers and message shapes.
- **Bindings:** `@celerity-sdk/runtime` (npm) and `celerity-runtime-sdk` (PyPI) are what the
  Node and Python SDKs import. A breaking change says what each SDK needs.
- **Runtime images:** core, nodejs and python release only with a `Release-As:` footer (each
  image's `RELEASING.md`); release-please closes their release PRs otherwise. The Node and
  Python images track their SDK's major.minor.
- **Blueprint config:** `blueprint-config-parser` and the CLI follow the Celerity resource
  types defined in bluelink-transformer-celerity; a type change starts there.
- **Persisted state:** deploy state lives in the Bluelink engine. The CLI owns:
  - `atlas_schema_revisions` in users' databases, its own layout named for Atlas (on
    postgres, in its own schema, adopting rows from the older location): changes must keep
    reading the layouts already deployed.
  - `.celerity/build-manifest.json` (versioned in `buildmanifest`) and the files bundled with
    it, read by the transformer and, at cold start, by the SDKs, which can be older or newer.
  - `.celerity/dev.state.json`, local and versioned in `internal/devstate`.

## Conventions

- Commits: [COMMIT_GUIDELINES.md](COMMIT_GUIDELINES.md) and `commitlint.config.js`. Scope by
  what changed: `lib-rt-*` per crate, `runtime-libs` across `libs/runtime`, `runtime-core`
  for `apps/runtime/core` (component `runtime-core-app`), `cli` and so on; no scope for
  repo-wide files. release-please picks the component by path. Don't use the scopes for
  projects that don't exist (`runtime-java`, `runtime-csharp`, `*workflow*`).
- Releases: one release pull request per component; tags are `{component}/vX.Y.Z`, plus
  `{path}/vX.Y.Z` for `cli` and `local-events`. Don't edit `CHANGELOG.md` files, versions or
  `.release-please-manifest.json` by hand.
- Dependency updates: Renovate (`renovate.json`, [RENOVATE.md](RENOVATE.md)). Releases of
  `runtime-core`, `runtime-consumers` or `runtime-ws` reach the bindings only when someone
  dispatches `cascade-core-to-sdks.yml`.

## Safety

Unless asked, don't run CLI commands that reach deployed resources or real cloud:

- `build` for `aws-serverless`, which creates an S3 bucket and uploads.
- Any `config` command: `set`, `delete` and `import` write to the deployed store, and `get`,
  `list`, `export` and the TUI print its secrets.
- `schema apply|drift|show|history --instance-name`, which look the instance up in the
  engine and fetch credentials from Secrets Manager; `apply` also invokes the maintenance
  runner, which creates database roles. `schema apply --url` against anything not local.

Don't create tags or releases, push images, merge release pull requests, add a `Release-As:`
footer, or dispatch `cascade-core-to-sdks.yml` unless asked.

## Known mistakes

<!-- One line per mistake an agent has made here, phrased as the rule that prevents it. -->
