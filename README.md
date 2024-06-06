![Celerity](/resources/logo.svg)

The backend toolkit that gets you moving fast

- [Contributing](./CONTRIBUTING.md)
- [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)

## Components of Celerity

### Blueprint Framework

The blueprint framework provides a set of interfaces and tools to deploy and manage the lifecycle of resources that can be represented as a blueprint. The framework is designed to be minimal at its core, yet extensible and can be used to deploy resources in any environment.

The blueprint framework is an implementation of the [Celerity Blueprint Specification](https://celerityframework.com/docs/blueprint/specification).

[Blueprint Framework](./libs/blueprint)

### Blueprint Language Server (blueprint-ls)

`blueprint-ls` is a language server that provides LSP support for the Celerity Blueprint Specification. The language server provides features such as syntax highlighting, code completion, and diagnostics.

The language server can be used with any language server protocol compatible editor such as Visual Studio Code, NeoVim,  Atom etc.

The language server only supports `yaml` files due to [intended limitations](https://github.com/golang/go/issues/43513) of Go's built-in `json` encoding library.

[Blueprint LSP](./tools/blueprint-ls)
