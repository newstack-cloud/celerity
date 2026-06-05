//! Single-line and multi-line string literal assembly (joining string parts
//! and `${..}` interpolations into the transient expression AST).

use super::core::Parser;
use crate::errors::ParseError;
use crate::parser::core::token_span;
use crate::parser::expr::{Expr, InterpPart};
use crate::scalar::{Scalar, ScalarValue};
use crate::source::Span;
use crate::tokens::TokenType;

impl Parser {
    /// Consumes a `StringStart … StringEnd` run as a static string (no
    /// interpolation, single-line only).
    pub(crate) fn parse_plain_string_literal(&mut self) -> Result<(String, Span), ParseError> {
        self.collect_string_literal(false)
    }

    pub(crate) fn collect_string_literal(
        &mut self,
        allow_multiline: bool,
    ) -> Result<(String, Span), ParseError> {
        let start = self.expect(TokenType::StringStart)?;
        if start.value == "\"\"\"" && !allow_multiline {
            return Err(self.error_at(
                start.start,
                "multi-line strings are not allowed in this position",
            ));
        }

        let mut text = String::new();
        loop {
            match self.peek_type() {
                TokenType::StringLiteral | TokenType::MultilineStringLiteral => {
                    text.push_str(&self.advance().value);
                }
                TokenType::InterpolationStart => {
                    let start = self.peek().start;
                    return Err(
                        self.error_at(start, "interpolation is not allowed in this position")
                    );
                }
                TokenType::StringEnd => {
                    let end = self.advance();
                    return Ok((text, Span::new(start.start, end.end)));
                }
                _ => {
                    let start = self.peek().start;
                    return Err(self.error_at(start, "unterminated string literal"));
                }
            }
        }
    }

    /// Assembles a `StringStart … StringEnd` run into an `Expr`: a plain string
    /// (no `${..}`) collapses to a scalar; otherwise an interpolation.
    pub(crate) fn parse_string_expr(&mut self) -> Result<Expr, ParseError> {
        let start = self.expect(TokenType::StringStart)?;
        let mut parts = Vec::new();
        loop {
            match self.peek_type() {
                TokenType::StringLiteral | TokenType::MultilineStringLiteral => {
                    let t = self.advance();
                    parts.push(InterpPart::String {
                        value: t.value.clone(),
                        span: Some(token_span(&t)),
                    });
                }
                TokenType::InterpolationStart => parts.push(self.parse_interpolation_part()?),
                TokenType::StringEnd => {
                    let end = self.advance();
                    return Ok(finish_string_expr(parts, Span::new(start.start, end.end)));
                }
                _ => {
                    let pos = self.peek().start;
                    return Err(self.error_at(pos, "unterminated string literal"));
                }
            }
        }
    }

    /// Parses a string-literal field that may interpolate `${..}`, returning the
    /// canonical `StringOrSubstitutions` used by stringy schema fields
    /// (description, include path, …). Requires a string literal at the cursor.
    pub(crate) fn parse_interpolated_string(
        &mut self,
    ) -> Result<crate::substitution::StringOrSubstitutions, ParseError> {
        let e = self.parse_string_expr()?;
        Ok(crate::expr_transform::expr_to_string_or_substitutions(e))
    }

    fn parse_interpolation_part(&mut self) -> Result<InterpPart, ParseError> {
        let open = self.advance(); // '${'
        let value = self.parse_expr()?; // full expression grammar inside ${..}
        let close = self.expect(TokenType::InterpolationEnd)?;
        Ok(InterpPart::Sub {
            value,
            span: Some(Span::new(open.start, close.end)),
        })
    }
}

/// A string with only literal parts becomes a scalar; one with any `${..}`
/// becomes an interpolation. (An empty `""` is a plain empty scalar.)
fn finish_string_expr(parts: Vec<InterpPart>, span: Span) -> Expr {
    match join_plain_parts(&parts) {
        Some(text) => Expr::Scalar(Scalar::new(ScalarValue::String(text), span)),
        None => Expr::Interpolation {
            parts,
            span: Some(span),
        },
    }
}

/// Joins the parts into one string if they are all literal text; `None` if any
/// part is a `${..}` substitution.
pub(crate) fn join_plain_parts(parts: &[InterpPart]) -> Option<String> {
    let mut text = String::new();
    for part in parts {
        match part {
            InterpPart::String { value, .. } => text.push_str(value),
            InterpPart::Sub { .. } => return None,
        }
    }
    Some(text)
}
