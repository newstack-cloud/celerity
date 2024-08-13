# Architecture

The CLI provides a text UI for managing Celerity applications and blueprints.
It is built on top of the `BuildEngine` interface provided by the `github.com/two-hundred/celerity/libs/build-engine` package.

By default, the CLI is configured to use the [HTTP API](../../api/README.md) to communicate with an instance of the Build Engine.
The endpoint of the HTTP API can be configured with the `--api-endpoint` flag when running the CLI.

## Using a Custom Build Engine (Build from Source)

The default `BuildEngine` implementation can be switched out for a custom one when you are looking to build the CLI from source. The primary use case for this is to build the CLI with an integrated version of the Build Engine, removing the need for a separate process running the Build Engine. This is mostly useful when you know you will be managing state locally on the machine running the CLI.
