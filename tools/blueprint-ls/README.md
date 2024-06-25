# blueprint language server

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-ls&metric=coverage)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-ls)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-ls&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-ls)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-ls&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-ls)

The blueprint language server is an LSP compatible language server for the [Blueprint Specification](https://celerityframework.com/docs/blueprint/specification).

## Language Server Protocol SDK

This project exposes a set of public packages that implement the [Language Server Protocol (LSP) 3.17.0](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/) that can be used freely within the constraints of the Celerity project license found in the [root directory](../../LICENSE) of this monorepo.

- [LSP SDK Docs](https://celerityframework.com/docs/blueprint/lsp-sdk)
- [Protocol Go Reference Docs](https://pkg.go.dev/github.com/two-hundred/celerity/libs/blueprint-ls/pkg/lsp_3_17)
- [Common Types Go Reference Docs](https://pkg.go.dev/github.com/two-hundred/celerity/libs/blueprint-ls/pkg/common)
- [Transport Go Reference Docs](https://pkg.go.dev/github.com/two-hundred/celerity/libs/blueprint-ls/pkg/server)

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
