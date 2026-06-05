//! Diagnostics for the blueprint language: individual lex/parse errors, the
//! capped collection sink used during lexing and parsing, and the public
//! [`Errors`] envelope returned to callers.

use std::error::Error;
use std::fmt;

use crate::source::{Position, Span};

/// The maximum number of diagnostics collected before further ones are elided.
/// Matches the Go scanner's cap and guards against runaway output on
/// pathological input.
pub const DEFAULT_DIAGNOSTIC_CAP: usize = 10;

/// A single error encountered during lexing, with a message and optional
/// source position.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LexError {
    pub message: String,
    pub span: Option<Span>,
}

impl LexError {
    /// Creates a lex error with a message and source span.
    pub fn new(message: impl Into<String>, span: Span) -> Self {
        Self {
            message: message.into(),
            span: Some(span),
        }
    }

    /// Creates a lex error with no source position.
    pub fn message_only(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            span: None,
        }
    }
}

impl fmt::Display for LexError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.span {
            Some(span) => write!(
                f,
                "lex error: {} at {}:{}",
                self.message, span.start.line, span.start.column
            ),
            None => write!(f, "lex error: {}", self.message),
        }
    }
}

impl Error for LexError {}

/// A single error encountered during parsing, with a message and optional
/// source position.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParseError {
    pub message: String,
    pub span: Option<Span>,
}

impl ParseError {
    /// Creates a parse error with a message and source span.
    pub fn new(message: impl Into<String>, span: Span) -> Self {
        Self {
            message: message.into(),
            span: Some(span),
        }
    }

    /// Creates a parse error with no source position.
    pub fn message_only(message: impl Into<String>) -> Self {
        Self {
            message: message.into(),
            span: None,
        }
    }
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match &self.span {
            Some(span) => write!(
                f,
                "parse error: {} at {}:{}",
                self.message, span.start.line, span.start.column
            ),
            None => write!(f, "parse error: {}", self.message),
        }
    }
}

impl Error for ParseError {}

/// A single diagnostic produced during lexing or parsing. The two variants
/// share a common shape but keep their distinct `lex error:` / `parse error:`
/// prefixes for clarity in the rendered envelope.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Diagnostic {
    Lex(LexError),
    Parse(ParseError),
}

impl Diagnostic {
    /// The source position this diagnostic points at, if any.
    pub fn span(&self) -> Option<Span> {
        match self {
            Diagnostic::Lex(err) => err.span,
            Diagnostic::Parse(err) => err.span,
        }
    }
}

impl fmt::Display for Diagnostic {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Diagnostic::Lex(err) => err.fmt(f),
            Diagnostic::Parse(err) => err.fmt(f),
        }
    }
}

impl Error for Diagnostic {}

impl From<LexError> for Diagnostic {
    fn from(err: LexError) -> Self {
        Diagnostic::Lex(err)
    }
}

impl From<ParseError> for Diagnostic {
    fn from(err: ParseError) -> Self {
        Diagnostic::Parse(err)
    }
}

/// A capped sink that collects diagnostics during lexing and parsing. Once the
/// cap is reached, further diagnostics are counted as `elided` rather than
/// stored, so the final [`Errors`] envelope can report how many were
/// suppressed.
#[derive(Debug, Clone)]
pub struct Diagnostics {
    errors: Vec<Diagnostic>,
    max: usize,
    elided: usize,
    source: Option<String>,
}

impl Diagnostics {
    /// Creates an empty sink with the default cap and an optional source string
    /// (used to render snippets in the final envelope).
    pub fn new(source: Option<String>) -> Self {
        Self {
            errors: Vec::new(),
            max: DEFAULT_DIAGNOSTIC_CAP,
            elided: 0,
            source,
        }
    }

    /// Adds a diagnostic, or increments the elided count if the cap is reached.
    pub fn add(&mut self, diagnostic: impl Into<Diagnostic>) {
        if self.errors.len() < self.max {
            self.errors.push(diagnostic.into());
        } else {
            self.elided += 1;
        }
    }

    /// Reports whether any diagnostics have been collected.
    pub fn is_empty(&self) -> bool {
        self.errors.is_empty()
    }

    /// Consumes the sink, returning `Some(Errors)` when at least one diagnostic
    /// was collected, or `None` when lexing/parsing succeeded. When some
    /// diagnostics were elided, a trailing summary diagnostic is appended.
    pub fn into_errors(self) -> Option<Errors> {
        if self.errors.is_empty() {
            return None;
        }
        let mut children = self.errors;
        if self.elided > 0 {
            children.push(Diagnostic::Parse(ParseError::message_only(format!(
                "{} more error(s) elided (raise the diagnostic cap to see them)",
                self.elided
            ))));
        }
        Some(Errors {
            children,
            source: self.source,
        })
    }
}

/// Aggregates the diagnostics collected across lexing and parsing into a single
/// returnable error. When [`Errors::source`] is set, each child diagnostic is
/// rendered with a source snippet showing the offending line and a caret
/// column-aligned under the column the error points at.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Errors {
    pub children: Vec<Diagnostic>,
    /// The original blueprint-language source. Optional; when set, `Display`
    /// renders a source snippet under each child diagnostic.
    pub source: Option<String>,
}

impl Errors {
    /// Creates an envelope from a single diagnostic with no source context.
    pub fn single(diagnostic: impl Into<Diagnostic>) -> Self {
        Self {
            children: vec![diagnostic.into()],
            source: None,
        }
    }
}

impl fmt::Display for Errors {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "blueprint language errors:")?;
        for child in &self.children {
            write!(f, "\n\t- {child}")?;
            if let Some(source) = &self.source {
                if let Some(span) = child.span() {
                    if let Some(snippet) = render_source_snippet(source, span.start) {
                        f.write_str(&snippet)?;
                    }
                }
            }
        }
        Ok(())
    }
}

impl Error for Errors {}

/// Renders a two-line snippet showing the source line the error references and
/// a caret column-aligned under the offending column, rustc-style. The leading
/// `\n\t` matches the bullet indentation used by [`Errors`] so the snippet sits
/// cleanly under its diagnostic. Returns `None` when the position is out of
/// range.
fn render_source_snippet(source: &str, pos: Position) -> Option<String> {
    let lines: Vec<&str> = source.split('\n').collect();
    if pos.line < 1 || pos.line > lines.len() {
        return None;
    }
    let line = lines[pos.line - 1];

    let line_num = pos.line.to_string();
    let gutter_pad = " ".repeat(line_num.len());

    let caret_col = pos.column.max(1);
    let caret_pad = " ".repeat(caret_col - 1);

    Some(format!(
        "\n\t  {line_num} | {line}\n\t  {gutter_pad} | {caret_pad}^"
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::source::Position;

    fn span(line: usize, column: usize) -> Span {
        Span::at(Position::new(line, column))
    }

    #[test]
    fn test_diagnostics_caps_and_elides() {
        let mut diags = Diagnostics::new(None);
        for n in 0..DEFAULT_DIAGNOSTIC_CAP + 3 {
            diags.add(ParseError::new(format!("error {n}"), span(1, 1)));
        }
        let errors = diags.into_errors().expect("expected errors");
        // Capped collection plus a single trailing "elided" summary diagnostic.
        assert_eq!(errors.children.len(), DEFAULT_DIAGNOSTIC_CAP + 1);
        let summary = errors.children.last().unwrap().to_string();
        assert!(summary.contains("3 more error(s) elided"));
    }

    #[test]
    fn test_empty_diagnostics_yields_no_errors() {
        let diags = Diagnostics::new(None);
        assert!(diags.into_errors().is_none());
    }

    #[test]
    fn test_errors_render_source_snippet_with_caret() {
        let source = "version \"2025-11-02\"\nvariable region: string {".to_string();
        let errors = Errors {
            children: vec![Diagnostic::Parse(ParseError::new("boom", span(2, 10)))],
            source: Some(source),
        };
        let rendered = errors.to_string();
        assert!(rendered.contains("parse error: boom at 2:10"));
        // Caret sits under column 10: the "| " gutter separator, then nine
        // spaces of padding (column - 1), then the caret.
        let expected_caret = format!("| {}^", " ".repeat(9));
        assert!(rendered.contains(&expected_caret));
    }
}
