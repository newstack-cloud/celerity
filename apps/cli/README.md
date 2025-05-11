# Celerity CLI

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-cli&metric=coverage)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-cli)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-cli&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-cli)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-cli&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-cli)

The CLI for managing Celerity applications and blueprints used more generally for Infrastructure as Code.

The CLI provides the following main features:

- **Initialise projects**: Create a new Celerity application or blueprint project from a template.
- **Validate blueprints**: Validate a Celerity blueprint, ensuring the blueprint is well-formed and meets the requirements of resource providers.
- **Build applications**: Build a Celerity application.
- **Deploy projects**: Deploy a Celerity application to a target environment or deploy standalone blueprints used for Infrastructure as Code.
- **Manage plugins**: Install, update, and remove Provider and Transformer plugins for the Deploy Engine running on the same machine as the CLI.

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
- [Architecture](docs/ARCHITECTURE.md)
