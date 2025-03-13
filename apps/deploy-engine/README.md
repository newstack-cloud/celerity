# Celerity Deploy Engine

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-deploy-engine&metric=coverage)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-deploy-engine)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-deploy-engine&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-deploy-engine)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-deploy-engine&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-deploy-engine)

The engine that validates and deploys blueprints for applications
created with the Celerity framework an addition to more general infrastruture as code deployments.

The deploy engine bundles the plugin framework's gRPC-based plugin system that allows for the creation of custom plugins for providers and transformers that can be pulled in at runtime.
The deploy engine also bundles a limited set of state persistence implementations for blueprint instances, the persistence implementation can be chosen with configuration.

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
