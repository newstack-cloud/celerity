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

Use `Dockerfile.local` to test with a locally built SDK:

```bash
# From a local wheel
docker build -f Dockerfile.local \
  --build-arg SDK_WHEEL=path/to/celerity_sdk-0.2.0-py3-none-any.whl \
  -t celerity-runtime-python:dev-local .

# From local source
docker build -f Dockerfile.local \
  --build-arg SDK_SOURCE_DIR=../../path/to/celerity-python-sdk \
  -t celerity-runtime-python:dev-local .
```

## Releasing

See [RELEASING.md](RELEASING.md) for the release process.
