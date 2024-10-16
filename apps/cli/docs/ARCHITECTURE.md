# Architecture

The CLI provides a text UI for managing Celerity applications and blueprints.
It is built on top of the `BuildEngine` interface provided by the `github.com/two-hundred/celerity/libs/build-engine` package.

By default, the CLI is configured to use the [HTTP API](../../api/README.md) to communicate with an instance of the Build Engine.
The endpoint of the HTTP API can be configured with the `--api-endpoint` flag when running the CLI.

## Using an Embedded Build Engine

The default `BuildEngine` implementation can be switched out for an embedded one with the use of command line options. Using an embedded build engine is mostly useful when you know you will be managing state locally on the machine running the CLI.

## Validation and Remote APIs

When using the CLI with a remote API, the CLI will not be able to validate projects on the client machine. Instead, you should point the CLI to a local API, use an embedded build engine or use a remote file source such as an S3 bucket.
When installing the Celerity tooling, by default, the CLI will use the local API.
