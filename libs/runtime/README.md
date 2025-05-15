# celerity runtime packages

A collection of Rust crates designed to be used in the Celerity runtime.

## Crates

### [celerity_runtime_core](./core)

This package provides the core components designed to be used in the Celerity runtime.

### [celerity_blueprint_config_parser](./blueprint-config-parser)

This package provides a Rust parser for runtime-specific configuration represented by a subset of a Celerity [blueprint](https://www.celerityframework.io/docs/blueprint/specification).
This implementation is not an exact implementation of the blueprint specification and is designed to be used in the Celerity runtime with strong typing for Celerity-specific resource types.

### [celerity_runtime_consumer_sqs](./consumer-sqs)

This package provides an Amazon SQS queue consumer implementation for the Celerity runtime.

### [celerity_runtime_consumer_aqs](./consumer-aqs)

This package provides an Azure Queue Storage consumer implementation for the Celerity runtime.

### [celerity_runtime_consumer_google_pubsub](./consumer-google-pubsub)

This package provides a Google PubSub consumer implementation for the Celerity runtime.

### [celerity_runtime_sdk_ffi](./sdk-ffi)

This package provides an FFI interface for the Celerity runtime SDKs.

### [celerity_runtime_sdk_schema](./sdk-schema)

This package provides an [oo-bindgen](https://github.com/stepfunc/oo_bindgen) schema for generating Celerity runtime SDKs.

## Additional documentation

- [Contributing](CONTRIBUTING.md)
- [HTTP API Docs](./api-docs/README.md) - API docs for the Local Runtime and Handler Invoke APIs
