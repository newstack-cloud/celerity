# Celerity Python Runtime

The Python runtime host for [Celerity](https://celerityframework.com) applications.
Bridges the Rust core runtime (via PyO3 FFI bindings) with Python application code
using the `celerity-sdk` framework.

## Quick Start

See the [Contributing Guide](CONTRIBUTING.md) for development setup.

## Docker Images

| Image | Description |
|-------|-------------|
| `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:{version}` | Production runtime |
| `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-{version}` | Development (with auto-reload) |

## Environment Variables

See [.env.example](.env.example) for the full list of configuration options.

## Releasing

See [RELEASING.md](RELEASING.md).
