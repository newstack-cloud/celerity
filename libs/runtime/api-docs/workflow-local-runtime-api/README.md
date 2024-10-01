# Workflow Local Runtime API

The Celerity Workflow Local Runtime API allows the Celerity workflow runtime to interact with an executable containing an application's handlers running on the same host (VM or container).
This is an essential part of the [`workflow`](../../../../apps/runtime/workflow/ARCHITECTURE_OVERVIEW.md) runtime (also referred to as `os-only`) which allows developers to write handlers in a compiled language of their choice. (e.g. Rust, Go, C++, etc.)

[workflow-local-runtime-api-v1](./workflow-local-runtime-api-v1.yaml) - The Celerity Workflow Local Runtime API v1 specification.
