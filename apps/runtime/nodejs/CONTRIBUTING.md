# Contributing to the Node.js Runtime

## Prerequisites

- Docker (with BuildKit)
- Rust toolchain (for the NAPI addon)
- Node.js >= 24 and Yarn 4
- Go (for the Celerity CLI)
- Local clones of:
  - This repo (`celerity`)
  - The [Node SDK monorepo](https://github.com/newstack-cloud/celerity-node-sdk)

## Project Layout

| File | Purpose |
|------|---------|
| `Dockerfile` | Production and dev images published to GHCR |
| `Dockerfile.local` | Local e2e testing image (overlays local SDK + NAPI builds) |
| `entrypoint.sh` | Dev image entrypoint (SWC loader, file watching) |
| `index.mjs` | Runtime bootstrap |
| `register-hooks.mjs` | ESM import hook that registers the SDK resolver |
| `sdk-resolver.mjs` | Resolves `@celerity-sdk/*` imports from the runtime host |
| `sdk-compat-check.mjs` | Validates SDK version compatibility at startup |
| `generate-manifest.mjs` | Writes `runtime-manifest.json` with installed SDK versions |

## Docker Images

### Published images (`Dockerfile`)

Built and pushed to GHCR by CI. Two targets:

- **`runtime`** — Production. Distroless (no shell), direct `node` entrypoint.
- **`dev`** — Development. Includes shell, SWC (for TypeScript decorator support), pino-pretty, and `--watch-path` file watching via `entrypoint.sh`.

Both use [Docker Hardened Images](https://hub.docker.com/hardened-images/catalog) (`dhi.io/node:24-debian13`) as the base.

### Local testing image (`Dockerfile.local`)

`Dockerfile.local` is intended for local end-to-end testing of the `celerity dev run` command in the CLI for a Celerity Node.js application. It builds a dev image that overlays locally built SDK packages and the Rust NAPI addon on top of registry-installed dependencies, so you can test changes to the SDK, runtime core, or runtime host files without publishing anything.

The Rust NAPI addon must be compiled for Linux (the container platform), so macOS or Windows host builds can't be used directly. `Dockerfile.local` supports two workflows:

**Full rebuild** (first time or after Rust changes):

```bash
docker build -f Dockerfile.local \
  --build-context sdk=$HOME/projects2026/celerity-node-sdk/packages \
  --build-context runtime=$HOME/projects2023/celerity/libs/runtime \
  -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-local .
```

**Quick rebuild** (Node SDK or runtime host changes only — skips Rust):

First, cache the NAPI binary once:

```bash
docker build -f Dockerfile.local --target napi-cache \
  --build-context runtime=$HOME/projects2023/celerity/libs/runtime \
  -t celerity-napi:local .
```

Then rebuild using the cached image:

```bash
docker build -f Dockerfile.local \
  --build-arg NAPI_IMAGE=celerity-napi:local \
  --build-context sdk=$HOME/projects2026/celerity-node-sdk/packages \
  -t ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-local .
```

After building, retag to match the current SDK version so `celerity dev run` picks it up:

```bash
docker tag ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-local \
           ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-{version}
```

For full local e2e testing instructions (building SDK packages, linking, running the auth server, testing endpoints), see the test project's `docs/local-e2e-testing.md` guide.

## ESM Hook Ordering

The runtime uses Node.js `--import` flags to register ESM loader hooks. Order matters — Node.js calls the last-registered hook first (outermost in the chain):

1. `@swc-node/register/esm-register` — TypeScript/decorator transpilation (dev only)
2. `./register-hooks.mjs` — SDK resolver (must be outermost so it runs first)
3. `@celerity-sdk/telemetry/setup` — OpenTelemetry early init

SWC must register **before** the SDK resolver so the resolver is the outermost hook and can delegate `.ts` files to SWC via `nextResolve`.

## Releasing

See [RELEASING.md](./RELEASING.md) for the version strategy and release flow.

Runtime releases require an explicit `Release-As: x.y.z` commit footer to align the runtime version with the SDK. Commits without this footer will not trigger a release — release-please PRs opened without one are automatically closed.
