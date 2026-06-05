//! # `celerity_blueprint_lang`
//!
//! A parser for the Bluelink **blueprint language** (`.bp` / `.blueprint`
//! files). The blueprint language is the first-class configuration format for authoring blueprints,
//! alongside the interchangeable YAML and JWCC formats.
//!
//! This crate is a faithful Rust port of the Go blueprint language implementation.
//! It parses blueprint-language source into the general, format-independent
//! blueprint [`schema`] using a hand-written lexer and a recursive-descent
//! parser with Pratt/precedence-climbing for expressions. The runtime-specific
//! transformation into `celerity_blueprint_config_parser`'s narrow config model lives
//! in that crate, which depends on this one.
//!
//! ## Public API
//!
//! - [`parse_string`] / [`parse_file`] — parse source into a [`Blueprint`].
//! - [`parse_string_with_options`] — parse source into a [`Blueprint`] with options for handling `${..}` substitutions.
//! - [`tokenize`] — lex source into the full token stream (comments and
//!   newlines retained) for CST / language-server consumers.

pub mod errors;
pub mod lexer;
pub mod mapping;
pub mod scalar;
pub mod schema;
pub mod source;
pub mod substitution;
pub mod tokens;

pub mod options;

pub(crate) mod expr_transform;
pub(crate) mod flatten;
pub(crate) mod parser;

pub use errors::Errors;
pub use options::{ParseOptions, SubstitutionMode};
pub use schema::Blueprint;
pub use tokens::{Token, TokenType};

use errors::ParseError;

/// Parses blueprint-language source into a [`Blueprint`].
///
/// Returns a structured [`Errors`] envelope (carrying source positions and, for
/// the file form, a rendered snippet) on any lexical or syntactic failure.
///
/// # Errors
///
/// Returns `Err` when the source contains lexical or syntactic errors.
pub fn parse_string(source: &str) -> Result<Blueprint, Errors> {
    parse_string_with_options(source, ParseOptions::default())
}

/// Parses blueprint-language source into a [`Blueprint`], with options for
/// how to handle `${..}` substitutions.
///
/// Returns a structured [`Errors`] envelope (carrying source positions and, for
/// the file form, a rendered snippet) on any lexical or syntactic failure.
/// The `substitutions` option controls how the parser handles `${..}` interpolation
/// in declaration values: the default `SubstitutionMode::Structured` produces a rich
/// structured representation of the interpolation, while `SubstitutionMode::RawText` flattens it into a
/// single string of raw text (with `${..}` verbatim) for consumers that don't need
/// the structured form and want to avoid the complexity of handling it.
///
/// # Errors
///
/// Returns `Err` when the source contains lexical or syntactic errors. Note that
/// the `substitutions` option only affects the shape of the successfully parsed
/// `Blueprint` and does not influence error reporting: malformed `${..}`
/// interpolation already surfaces as a parse error
/// regardless of the option, so callers can safely "validate first"
/// without worrying about the option's effect on error handling.
pub fn parse_string_with_options(source: &str, options: ParseOptions) -> Result<Blueprint, Errors> {
    let mut parser = parser::core::Parser::new(source);
    let blueprint = parser.parse();
    if let Some(errors) = parser.into_errors() {
        return Err(errors); // malformed `${..}` already surfaces here ⇒ "validate first"
    }
    let mut blueprint = blueprint;
    if options.substitutions == SubstitutionMode::RawText {
        // Slice against the same normalisation the lexer used, so spans line up.
        let normalised = source.replace("\r\n", "\n");
        flatten::substitutions_to_text(&mut blueprint, &normalised);
    }
    Ok(blueprint)
}

/// Reads a `.bp` / `.blueprint` file and parses it into a [`Blueprint`].
///
/// I/O failures are folded into the returned [`Errors`] envelope so callers
/// have a single error type to handle.
///
/// # Errors
///
/// Returns `Err` when the file cannot be read or when its contents contain
/// lexical or syntactic errors.
pub fn parse_file(path: &str) -> Result<Blueprint, Errors> {
    match std::fs::read_to_string(path) {
        Ok(source) => parse_string(&source),
        Err(err) => Err(Errors::single(ParseError::message_only(format!(
            "failed to read blueprint file {path:?}: {err}"
        )))),
    }
}

/// Lexes blueprint-language source into its full token stream, terminated by a
/// single [`TokenType::Eof`] token.
///
/// Unlike [`parse_string`], the stream preserves comments and newlines, so
/// callers such as a language server can reconstruct a concrete syntax tree.
/// The token vector is always returned, even on lexical errors, to support
/// building a partial tree for an in-progress edit; any lexical errors are
/// returned alongside as `Some(Errors)` (or `None` when lexing succeeds).
pub fn tokenize(source: &str) -> (Vec<Token>, Option<Errors>) {
    let mut lexer = lexer::Lexer::new(source);
    let mut tokens = Vec::new();
    loop {
        let token = lexer.next_token();
        let is_eof = token.ty == TokenType::Eof;
        tokens.push(token);
        if is_eof {
            break;
        }
    }
    (tokens, lexer.into_errors())
}
