# celerity blueprint config parser

[![codecov](https://codecov.io/gh/newstack-cloud/celerity/graph/badge.svg?token=u1SKOg58yo&flag=runtime-lib-blueprint-config-parser)](https://codecov.io/gh/newstack-cloud/celerity)

This package provides a Rust parser for runtime-specific configuration represented by a subset of a Bluelink [blueprint](https://www.bluelink.dev/docs/blueprint/specification).

This implementation is not an exact implementation of the blueprint specification but a simplified version that can be used to parse YAML or JSONC blueprint files.

This is not designed to be used as a general purpose blueprint parser, it expects a very specific blueprint format that contains strongly typed subset of resource specifications for Celerity resource types.
This does not parse the full Celerity resource type specifications, it only parses the subset of resource specs that are used by the runtime.

Efforts to implement the full general purpose blueprint specification for Rust will not be a part of this package.

### About `${..}` Substitutions

This configuration parser resolves a subset of `${..}` substitutions in a blueprint.
The parser only supports variable substitutions that will be replaced with values from a provided
set of environment variables where `CELERITY_VARIABLE_{name}` will match the name of the variable referenced
in the blueprint.
All other `${..}` substitutions are treated as string literals.

## Additional documentation

- [Contributing](../CONTRIBUTING.md)
