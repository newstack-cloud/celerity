# Runtime API Docs

The Celerity Runtimes provide HTTP APIs that enable key functionality for all the kinds of applications that can be built using Celerity.

## Core Runtime APIs

- [Local Runtime API](./local-runtime-api/README.md) - The Local Runtime API allows the runtime to interact with an executable containing an application's handlers.

## Workflow Runtime APIs

- [Workflow API](./workflow-api/README.md) - The Workflow API allows for triggerring and monitoring the workflow along with the ability to retrieve workflow execution history.
- [Workflow Local Runtime API](./workflow-local-runtime-api/README.md) - The Workflow Local Runtime API allows the workflow runtime to interact with an executable containing handlers to be executed for `executeStep` states.

## Shared APIs

APIs that both the core and workflow runtimes implement.

- [Handler Invoke API](./handler-invoke-api/README.md) - The Handler Invoke API allows developers to invoke handlers directly in their local development environments.
