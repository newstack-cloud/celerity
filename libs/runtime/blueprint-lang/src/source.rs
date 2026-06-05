//! Source position tracking for the blueprint language.

use serde::{Deserialize, Serialize};

/// A 1-indexed position in blueprint-language source.
///
/// `line` and `column` both start at 1, matching the Go implementation
///  and the convention used by editors and language servers.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct Position {
    pub line: usize,
    pub column: usize,
}

impl Position {
    /// Creates a position at the given 1-indexed line and column.
    pub fn new(line: usize, column: usize) -> Self {
        Self { line, column }
    }
}

/// A range in blueprint-language source.
///
/// `start` is inclusive and points at the first character of the spanned
/// construct. `end`, when present, is exclusive and points at the position
/// immediately after the construct's final character (mirroring the lexer's
/// inclusive-start/exclusive-end token bounds). `end` is optional because some
/// nodes only carry a start position.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct Span {
    pub start: Position,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end: Option<Position>,
}

impl Span {
    /// Creates a span with only a start position.
    pub fn at(start: Position) -> Self {
        Self { start, end: None }
    }

    /// Creates a span with an inclusive start and exclusive end.
    pub fn new(start: Position, end: Position) -> Self {
        Self {
            start,
            end: Some(end),
        }
    }
}

/// A value paired with its optional source span. Mirrors the Go wrapper types
/// that attach a source location to a type annotation or other leaf value.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Spanned<T> {
    pub value: T,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

impl<T> Spanned<T> {
    /// Creates a spanned value carrying a source span.
    pub fn new(value: T, span: Span) -> Self {
        Self {
            value,
            span: Some(span),
        }
    }

    /// Creates a spanned value with no source span (useful in tests and
    /// synthesised values).
    pub fn untracked(value: T) -> Self {
        Self { value, span: None }
    }
}
