![Celerity](/resources/logo.svg)

The backend toolkit that gets you moving fast

- [Contributing](./CONTRIBUTING.md)
- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)
- [Docs Site](https://celerityframework.io)

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

### Deploy Engine

The deploy engine is the application that is responsible for deploying blueprints to target environments. It brings together the blueprint framework, the plugin framework and persistence layers to provide a complete and extensible solution for deploying Celerity applications and infrastructure-as-code.

The deploy engine exposes a HTTP API that can be used to validate blueprints, stage changes and deploy changes to target environments. As a part of the HTTP API, events can be streamed over Server-Sent Events (SSE) to provide real-time updates on the status of deployments, change staging and validation. 

[Deploy Engine](./apps/deploy-engine)

## Plugins

Plugins are foundational to the Celerity framework. Developers can create plugins to extend the capabilities of the deploy engine to deploy resources, source data and manage the lifecycle of resources in upstream providers (such as AWS, Azure, GCP).
There are two types of plugins, `Providers` and `Transformers`.
- **Providers** are plugins that are responsible for deploying resources to a target environment. Providers can be used to deploy resources to any environment, including cloud providers, on-premises environments and local development environments. In addition to resources, providers can also implement links between resource types, data sources and custom variable types.
- **Transformers** are plugins that are responsible for transforming blueprints. These are powerful plugins that enable abstract resources that can be defined by users and then transformed into concrete resources that can be deployed to a concrete target environment. For example, the Celerity application primitives are abstract resources that are transformed into concrete resources at deploy time that can be deployed to a target environment.

### Plugin Framework

The plugin framework provides the foundations for a plugin system that uses gRPC over a local network that includes a Go SDK that provides a smooth plugin development experience where knowledge of the ins and outs of the underlying communication protocol is not required.

[Plugin Framework](./libs/plugin-framework)

## CLI

The Celerity CLI brings all the components of Celerity together. It is a command line tool that can be used to create, build, deploy and manage Celerity applications.
It also provides commands for installing and managing plugins, using the [Registry protocol](https://www.celerityframework.io/plugin-framework/docs/registry-protocols-formats/registry-protocol) to source, verify and install plugins from the official and custom plugin registries.

Under the hood, the CLI uses the deploy engine to validate blueprints, stage changes and deploy applications (or standalone blueprints) to target environments.
The CLI can use local or remote instances of the deploy engine, this can be configured using environment variables or command line options.

[CLI](./apps/cli)

## Runtime

One of the main ideas behind Celerity is to remove the need to build applications differently depending on the target environment. This means that you can develop and test your applications locally, and then deploy them to a serverless or containerised environment without having to make any changes to the application code.
You could say containers is the answer, however, when opting for this approach, you sacrifice a lot of the powerful tools that come with managed services built around FaaS. Celerity leverages the power of FaaS, managed services such as API Gateways and event buses where possible instead of bundling containerised application into cloud functions.

The runtime allows you to run your Celerity applications in containerized/custom server environments. Celerity applications consist of a set of handlers and a declarative definition of the type of application that hosts these handlers. This approach is akin to the serverless model made popular by cloud providers with the likes of AWS Lambda and Google Cloud Functions. Celerity applications can run in FaaS-based serverless environments or in containerized/custom server environments, The runtime enables the latter.

The Celerity runtime supports multiple programming languages.

_"FaaS" stands for Function as a Service._

[Supported Runtimes](./apps/runtime)

# Additional Documentation

- [Index of Projects](./docs/INDEX.md) - A full index of all the projects in the core Celerity monorepo.
