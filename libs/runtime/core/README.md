# celerity runtime core

This package provides the core components designed to be used in the Celerity runtime.
This includes a HTTP server, WebSocket server, Queue consumers and a plugin system.
This package provides an API to start different kinds of servers and consumers from a blueprint file and a set of handlers.
This also provides APIs for the registration of handlers and plugins.

This package only provides traits for queue consumers and queue event message handlers.
Implementations of these traits can be found in specific packages such as `celerity_runtime_consumer_sqs`.

### About `${..}` Substitutions

The runtime supports a limited version of `${..}` [substitutions](https://www.bluelink.dev/docs/blueprint/specification#references--substitutions).
Only `${variables.[name]}` substitutions are recognised, all other substitutions are treated as string literals or will lead to parsing errors if used in a field that is not a string.

In the runtime, the parser will replace `${variables.[name]}` with an environment variable of the form `CELERITY_VARIABLE_[name]`.
These environment variables are expected to be set at package/build time by the Celerity CLI or other tools.

In the initial version of the runtime, Celerity application blueprint files should only contain `${variables.[name]}` substitutions, this may be extended in the future for full support of the [Bluelink Blueprint specification](https://www.bluelink.dev/docs/blueprint/specification).

## Additional documentation

- [Contributing](../CONTRIBUTING.md)
