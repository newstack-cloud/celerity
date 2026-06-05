# celerity runtime packages

A collection of Rust crates designed to be used in the Celerity runtime.

## Crates

### [celerity_runtime_core](./core)

This package provides the core components designed to be used in the Celerity runtime.

### [celerity_blueprint_config_parser](./blueprint-config-parser)

This package provides a Rust parser for runtime-specific configuration represented by a subset of a Bluelink [blueprint](https://www.bluelink.dev/docs/blueprint/specification).
This implementation is not an exact implementation of the blueprint specification and is designed to be used in the Celerity runtime with strong typing for Celerity-specific resource types.

### [celerity_blueprint_lang](./blueprint-lang)

This package provides a Rust parser for the Bluelink [blueprint language](https://www.bluelink.dev/docs/bluelink/blueprints/language) (`.bp`/`.blueprint` files): a hand-written lexer and recursive-descent parser that produces the general, format-independent blueprint schema.
Unlike `celerity_blueprint_config_parser`, it is an implementation of the full language; the runtime-specific transformation into the narrow config model lives in that crate, which depends on this one.

### [celerity_consumer_sqs](./consumers/consumer-sqs)

This package provides an Amazon SQS queue consumer implementation for the Celerity runtime.

### [celerity_consumer_gcloud_pubsub](./consumers/consumer-gcloud-pubsub)

This package provides a Google PubSub consumer implementation for the Celerity runtime.

### [celerity_runtime_bindgen_ffi](./sdk/bindgen-ffi)

This package provides an FFI interface for the Celerity runtime SDKs.

### [celerity_runtime_bindgen_schema](./sdk/bindgen-schema)

This package provides an [oo-bindgen](https://github.com/stepfunc/oo_bindgen) schema for generating Celerity runtime SDKs.

### [celerity_runtime_workflow](./workflow)

This package provides the workflow engine components designed to be used in the Celerity workflow runtime.

## Additional documentation

- [Contributing](CONTRIBUTING.md)
- [HTTP API Docs](./api-docs/README.md) - API docs for the Local Runtime and Handler Invoke APIs
