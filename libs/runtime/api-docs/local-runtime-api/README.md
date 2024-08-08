# Local Runtime API

The Celerity Local Runtime API allows the Celerity runtime to interact with an executable containing an application's handlers running on the same host (VM or container).
This is an essential part of the [`core`](../../../../apps/runtime/core/ARCHITECTURE_OVERVIEW.md) runtime (also referred to as `os-only`) which allows developers to write handlers in a compiled language of their choice. (e.g. Rust, Go, C++, etc.)

[local-runtime-api-v1](./local-runtime-api-v1.yaml) - The Celerity Local Runtime API v1 specification.
