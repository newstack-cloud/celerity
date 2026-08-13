# Runtime API Docs

The Celerity Runtimes expose the interfaces that enable key functionality for all the kinds of applications that can be built using Celerity. Most are HTTP APIs; the contract between the core runtime and a handlers executable is a gRPC stream and is documented alongside its schema.

## Core Runtime APIs

- [IPC Handler Protocol](../proto/README.md) - The contract between the core runtime and a handlers executable running as a separate process, used by SDKs for ahead-of-time compiled languages. One long-lived bidirectional gRPC stream carries events, results, configuration, WebSocket sends, cancellation and shutdown. `celerity/runtime/v1/runtime.proto` is the source of truth.

## Workflow Runtime APIs

- [Workflow API](./workflow-api/README.md) - The Workflow API allows for triggerring and monitoring the workflow along with the ability to retrieve workflow execution history.

## Shared APIs

APIs that both the core and workflow runtimes implement.

- [Handler Invoke API](./handler-invoke-api/README.md) - The Handler Invoke API allows developers to invoke handlers directly in their local development environments.
