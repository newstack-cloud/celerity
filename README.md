![Celerity](/resources/logo.svg)

The backend toolkit that gets you moving fast

- [Contributing](./CONTRIBUTING.md)
- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)

# Components of Celerity

## Blueprints

Blueprints are a way to define and manage the lifecycle of resources needed to run your applications. Blueprints adhere to a [specification](https://celerityframework.com/docs/blueprint/specification) that is the foundation of the Celerity resource model used at build time and runtime.
The specification is broad and can be used to define resources in any environment (e.g. Cloud provider resources from the likes of AWS, Azure and Google Cloud).

### Blueprint Framework

The blueprint framework provides a set of interfaces and tools to deploy and manage the lifecycle of resources that can be represented as a blueprint. The framework is designed to be minimal at its core, yet extensible and can be used to deploy resources in any environment.

The blueprint framework is an implementation of the [Celerity Blueprint Specification](https://celerityframework.com/docs/blueprint/specification).

[Blueprint Framework](./libs/blueprint)

### Blueprint Language Server (blueprint-ls)

`blueprint-ls` is a language server that provides LSP support for the Celerity Blueprint Specification. The language server provides features such as code completion, go to definitions and diagnostics.

The language server can be used with any language server protocol compatible editor such as Visual Studio Code, NeoVim,  Atom etc.

The language server only supports `yaml` files due to [intended limitations](https://github.com/golang/go/issues/43513) of Go's built-in `json` encoding library.

[Blueprint Language Server](./tools/blueprint-ls)

## Runtime

One of the main ideas behind Celerity is to remove the need to build applications differently depending on the target environment. This means that you can develop and test your applications locally, and then deploy them to a serverless or containerised environment without having to make any changes to the application code.

The runtime allows you to run your Celerity applications in containerized/custom server environments. Celerity applications consist of a set of handlers and a declarative definition of the type of application that hosts these handlers. This approach is akin to the serverless model made popular by cloud providers with the likes of AWS Lambda and Google Cloud Functions. Celerity applications can run in FaaS-based serverless environments or in containerized/custom server environments, The runtime enables the latter.

The Celerity runtime supports multiple programming languages.

_"FaaS" stands for Function as a Service._

[Supported Runtimes](./apps/runtime)
