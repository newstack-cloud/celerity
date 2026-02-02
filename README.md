![Celerity](/resources/logo.svg)

The backend toolkit that gets you moving fast

- [Contributing](./CONTRIBUTING.md)
- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)
- [Docs Site](https://celerityframework.io)

Celerity lets you write your application code once and deploy it to any cloud provider — or on-premises — without modification. Your handlers and application logic stay the same whether you target AWS, Google Cloud, Azure, or a self-hosted environment; Celerity takes care of the mapping between your application and the underlying platform.

# Components of Celerity

## CLI

The Celerity CLI brings all the components of Celerity together. It is a command line tool that can be used to create, build, deploy and manage Celerity applications.

Under the hood, the CLI uses [Bluelink](https://bluelink.dev) to parse and validate blueprints along with managing the deployment life cycle of the underlying resources that power Celerity applications.

[CLI](./apps/cli)

## Runtime

One of the main ideas behind Celerity is to remove the need to build applications differently depending on the target environment. You can develop and test your applications locally and then deploy them to a serverless or containerised environment without changing your application code.

Containers alone could achieve this, but you would sacrifice the powerful managed services built around FaaS. Celerity leverages FaaS, API Gateways, event buses, and other managed services where possible instead of bundling containerised applications into cloud functions.

Celerity applications consist of a set of handlers and a declarative blueprint that defines the type of application hosting them — an approach similar to the serverless model popularised by AWS Lambda and Google Cloud Functions. Applications can run in FaaS-based serverless environments or in containerised/custom server environments; the runtime enables the latter.

The Celerity runtime supports multiple programming languages.

_"FaaS" stands for Function as a Service._

[Supported Runtimes](./apps/runtime/)

# Additional Documentation

- [Index of Projects](./docs/INDEX.md) - A full index of all the projects in the core Celerity monorepo.
