# celerity runtime workflow

[![codecov](https://codecov.io/gh/two-hundred/celerity/graph/badge.svg?token=u1SKOg58yo&flag=runtime-lib-workflow)](https://codecov.io/gh/two-hundred/celerity)

This package provides the workflow engine components designed to be used in the Celerity workflow runtime.
This crate provides a HTTP server application for the workflow engine that implements the specification defined in the [Celerity Workflow Specification](https://www.celerityframework.com/docs/applications/resources/celerity-workflow).
This package creates an executable workflow from a blueprint file and a set of handlers.
This also provides an API for the registration of handlers.

### About `${..}` Substitutions

The runtime supports a limited version of `${..}` [substitutions](https://www.celerityframework.com/docs/blueprint/specification#references--substitutions).
Only `${variables.[name]}` substitutions are recognised, all other substitutions are treated as string literals.

In the runtime, the parser will replace `${variables.[name]}` with an environment variable of the form `CELERITY_VARIABLE_[name]`, these environment variables are expected to be set at package/build time by a tool like the Celerity Deploy Engine used in the Celerity CLI.

## Additional documentation

- [Contributing](../CONTRIBUTING.md)
