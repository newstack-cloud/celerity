# celerity_blueprint_lang

A parser for the Bluelink **blueprint language** (`.bp` / `.blueprint` files), the first-class
configuration format for authoring blueprints. This is the preferred format that is an addition to the interchangeable YAML and JWCC formats.

This crate is a Rust port of the Go `libs/blueprint/lang` package. It parses
blueprint-language source into the general, format-independent blueprint schema using a hand-written
lexer and a recursive-descent parser with Pratt / precedence-climbing for expressions.

The runtime-specific transformation that reduces the general schema into the narrow `BlueprintConfig` consumed
by the Celerity runtime lives in [`celerity_blueprint_config_parser`](../blueprint-config-parser),
which depends on this crate.

## Public API

- `parse_string(&str) -> Result<Blueprint, Errors>` — parse source into a `Blueprint`.
- `parse_string_with_options(&str, ParseOptions) -> Result<Blueprint, Errors>` — parse source into a `Blueprint` with options for handling `${..}` substitutions.
- `parse_file(&str) -> Result<Blueprint, Errors>` — read and parse a `.bp` / `.blueprint` file.
- `tokenize(&str) -> (Vec<Token>, Option<Errors>)` — lex source into the full token stream (comments
  and newlines retained) for CST / language-server consumers.
