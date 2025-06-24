![Celerity](/resources/logo.svg)

The backend toolkit that gets you moving fast

- [Contributing](./CONTRIBUTING.md)
- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)
- [Docs Site](https://celerityframework.io)

# Components of Celerity

## CLI

The Celerity CLI brings all the components of Celerity together. It is a command line tool that can be used to create, build, deploy and manage Celerity applications.

Under the hood, the CLI uses [Bluelink](https://bluelink.dev) to parse and validate blueprints, along with [OpenTofu](https://opentofu.org/) to plan and deploy applications to target environments.

_For future versions, Bluelink will become the default deploy engine once it has all the integrations required to support Celerity applications._

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
