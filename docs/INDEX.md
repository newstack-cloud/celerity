# Index of Projects

All projects in the Celerity monorepo, grouped by area. Each release component is independently versioned via [release-please](../SOURCE_CONTROL_RELEASE_STRATEGY.md).

## Applications

| Path | Description | Language | Release Component |
|------|-------------|----------|-------------------|
| [apps/cli](../apps/cli) | CLI for developing, building, deploying, and managing Celerity applications and blueprints. | Go | `cli` |
| [apps/local-events](../apps/local-events) | Sidecar that captures events from local sources and forwards them to Celerity runtime applications for integrated testing. | Go | `local-events` |

## Runtime Applications

Container images that host developer application code, interfacing with handlers through the core runtime.

| Path | Description | Language | Release Component |
|------|-------------|----------|-------------------|
| [apps/runtime/nodejs](../apps/runtime/nodejs) | Node.js runtime application using bi-directional FFI calls to host handler code. | Node.js | `runtime-nodejs` |
| [apps/runtime/python](../apps/runtime/python) | Python runtime application. | Python | `runtime-python` |
| [apps/runtime/java](../apps/runtime/java) | Java runtime application (not yet implemented). | Java | — |
| [apps/runtime/csharp](../apps/runtime/csharp) | C# runtime application (not yet implemented). | C# | — |

## Runtime Core Libraries

Rust crates that make up the core runtime engine. Released together under the `runtime-core` component.

| Path | Description |
|------|-------------|
| [libs/runtime/core](../libs/runtime/core) | Core runtime library providing HTTP/WebSocket server, event handling, auth, CORS, telemetry, and consumer integration (Axum + Tokio). |
| [libs/runtime/blueprint-config-parser](../libs/runtime/blueprint-config-parser) | YAML blueprint configuration parser with resource definitions and variable substitution support. |
| [libs/runtime/helpers](../libs/runtime/helpers) | Shared utilities for HTTP, Redis, tracing, and async operations. |
| [libs/runtime/workflow](../libs/runtime/workflow) | State machine and workflow orchestration for runtime event processing. |
| [libs/runtime/signature](../libs/runtime/signature) | Cryptographic signature verification for HTTP request authentication. |
| [libs/runtime/aws-helpers](../libs/runtime/aws-helpers) | AWS SDK helpers for SQS, configuration, and account operations. |

## Runtime SDK Crates

Language-specific bindings that bridge the core runtime with handler code. Each SDK is a separate release component.

| Path | Description | Release Component |
|------|-------------|-------------------|
| [libs/runtime/sdk/node](../libs/runtime/sdk/node) | NAPI-based Node.js bindings with async handler registration and event processing. | `runtime-sdk-node` |
| [libs/runtime/sdk/python](../libs/runtime/sdk/python) | PyO3-based Python bindings with async handler support. | `runtime-sdk-python` |
| [libs/runtime/sdk/bindgen-ffi](../libs/runtime/sdk/bindgen-ffi) | FFI binding generator for cross-language interoperability (Java, C#). | `runtime-sdk-ffi` |
| [libs/runtime/sdk/bindgen-ffi-java](../libs/runtime/sdk/bindgen-ffi-java) | JNI wrapper for Java FFI bindings. | `runtime-sdk-ffi` |
| [libs/runtime/sdk/bindgen-schema](../libs/runtime/sdk/bindgen-schema) | Schema definition builder for FFI code generation. | `runtime-sdk-ffi` |

## WebSocket Crates

Crates for WebSocket connection management and multi-node clustering. Released together under the `runtime-ws` component.

| Path | Description |
|------|-------------|
| [libs/runtime/ws/ws-registry](../libs/runtime/ws/ws-registry) | WebSocket connection registry for managing client connections. |
| [libs/runtime/ws/ws-redis](../libs/runtime/ws/ws-redis) | Redis pub/sub adapter for distributed WebSocket support across multiple nodes. |

## Consumer Crates

Message and event consumer implementations for cloud provider services. Released together under the `runtime-consumers` component.

| Path | Description |
|------|-------------|
| [libs/runtime/consumers/consumer-sqs](../libs/runtime/consumers/consumer-sqs) | AWS SQS consumer with OpenTelemetry tracing. |
| [libs/runtime/consumers/consumer-kinesis](../libs/runtime/consumers/consumer-kinesis) | AWS Kinesis stream consumer. |
| [libs/runtime/consumers/consumer-redis](../libs/runtime/consumers/consumer-redis) | Redis stream consumer with cluster support. |
| [libs/runtime/consumers/consumer-gcloud-pubsub](../libs/runtime/consumers/consumer-gcloud-pubsub) | Google Cloud Pub/Sub consumer. |
| [libs/runtime/consumers/consumer-gcloud-tasks](../libs/runtime/consumers/consumer-gcloud-tasks) | Google Cloud Tasks consumer. |
| [libs/runtime/consumers/consumer-azure-service-bus](../libs/runtime/consumers/consumer-azure-service-bus) | Azure Service Bus consumer. |
| [libs/runtime/consumers/consumer-azure-events-hub](../libs/runtime/consumers/consumer-azure-events-hub) | Azure Event Hubs consumer. |
