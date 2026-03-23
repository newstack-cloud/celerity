# Contributing to the Celerity Python Runtime

## Prerequisites

- Docker (with BuildKit)
- Python 3.13+
- [uv](https://docs.astral.sh/uv/) (Python package manager)
- Rust toolchain (if rebuilding `celerity-runtime-sdk` from source)
- Local clone of [celerity-python-sdk](https://github.com/newstack-cloud/celerity-python-sdk)
  (for local SDK development)

## Local Development Setup

```bash
cd apps/runtime/python

# Install dependencies
uv sync

# Lint
uv run ruff check .

# Type check
uv run mypy main.py sdk_compat_check.py generate_manifest.py
```

## Docker Images

The runtime ships two Docker image targets built from a single `Dockerfile`:

| Target    | Base Image                        | Contents                          |
|-----------|-----------------------------------|-----------------------------------|
| `runtime` | `dhi.io/python:3.13-debian13`     | Production — minimal, no dev tools |
| `dev`     | `dhi.io/python:3.13-debian13-dev` | Development — includes watchfiles  |

### Building Images Locally

```bash
# Production image
docker build --target runtime \
  -t celerity-runtime-python:test .

# Development image
docker build --target dev \
  -t celerity-runtime-python:dev-test .
```

### Running Locally

```bash
docker run --rm \
  -v $(pwd)/app:/opt/celerity/app \
  -e CELERITY_BLUEPRINT=/opt/celerity/app/blueprint.yaml \
  -e CELERITY_SERVICE_NAME=my-service \
  -e CELERITY_MODULE_PATH=/opt/celerity/app/src/app_module.py \
  -p 8080:8080 \
  celerity-runtime-python:dev-test
```

## Local SDK Development

Use `Dockerfile.local` to test with locally built SDK packages.
See the header comments in `Dockerfile.local` for all build variants.

```bash
# Prepare a filtered build context (excludes target/, node_modules — ~2 MB vs 40+ GB)
./scripts/prepare-runtime-sdk-context.sh

# Full rebuild — compiles Rust runtime SDK from source + overlays local Python SDK
docker build -f Dockerfile.local \
  --build-context sdk=$HOME/projects2026/celerity-python-sdk \
  --build-context runtime-sdk=/tmp/celerity-runtime-sdk-context \
  -t ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-local .

# Build runtime SDK once (cache for quick rebuilds)
docker build -f Dockerfile.local --target runtime-sdk-builder \
  --build-context runtime-sdk=/tmp/celerity-runtime-sdk-context \
  -t celerity-python-runtime-sdk:local .

# Quick rebuild — Python SDK changes only, reuses cached Rust build
docker build -f Dockerfile.local \
  --build-arg RUNTIME_SDK_IMAGE=celerity-python-runtime-sdk:local \
  --build-context sdk=$HOME/projects2026/celerity-python-sdk \
  -t ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-local .
```

## Releasing

See [RELEASING.md](RELEASING.md) for the release process.
