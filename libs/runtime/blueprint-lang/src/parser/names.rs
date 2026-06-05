//! Element-name parsing (bare and quoted) and element-type segment parsing.

use super::core::Parser;
use crate::errors::ParseError;
use crate::parser::core::token_span;
use crate::source::Span;
use crate::tokens::TokenType;
use crate::Token;

impl Parser {
    /// A declaration name: bare identifier or quoted name.
    /// A reserved work is rejected bare and consumed so recovery doesn't
    /// re-dispatch it.
    pub(crate) fn parse_element_name(&mut self) -> Result<(String, Span), ParseError> {
        match self.peek_type() {
            TokenType::Identifier => {
                let tkn = self.advance();
                Ok((tkn.value.clone(), token_span(&tkn)))
            }
            TokenType::StringStart => self.parse_quoted_name(),
            _ => {
                let bad = self.advance();
                let msg = if bad.ty.is_keyword() {
                    format!(
                        "{} is reserved and cannot be a bare name; quote it to use it as a name",
                        bad.ty.display_label()
                    )
                } else {
                    format!("expected element name, got {}", bad.ty.display_label())
                };
                Err(self.error_at(bad.start, msg))
            }
        }
    }

    /// A field key (left of `=` in a block): identifier, keyword, or quoted name.
    pub(crate) fn parse_field_key(&mut self) -> Result<(String, Span), ParseError> {
        let ty = self.peek_type();
        if ty == TokenType::Identifier || ty.is_keyword() {
            let tkn = self.advance();
            return Ok((tkn.value.clone(), token_span(&tkn)));
        }

        if ty == TokenType::StringStart {
            return self.parse_quoted_name();
        }
        let start = self.peek().start;
        Err(self.error_at(
            start,
            format!("expected a field name, got {}", ty.display_label()),
        ))
    }

    /// An object key: identifier/keyword, or any-character quoted string (wider
    /// than a field key, object keys are data, not referenceable names).
    pub(crate) fn parse_object_key(&mut self) -> Result<(String, Span), ParseError> {
        let ty = self.peek_type();
        if ty == TokenType::Identifier || ty.is_keyword() {
            let t = self.advance();
            return Ok((t.value.clone(), super::core::token_span(&t)));
        }
        if ty == TokenType::StringStart {
            return self.parse_plain_string_literal(); // any chars allowed
        }
        let start = self.peek().start;
        Err(self.error_at(
            start,
            format!("expected an object key, got {}", ty.display_label()),
        ))
    }

    /// A quoted name restricted to the referenceable set (letters/digits/_/-/.).
    pub(crate) fn parse_quoted_name(&mut self) -> Result<(String, Span), ParseError> {
        let (name, span) = self.parse_plain_string_literal()?;
        if !is_valid_quoted_name(&name) {
            return Err(self.error_at(
                span.start,
                format!(
                    "invalid quoted name {name:?}: only letters, digits, '_', '-' and '.' are allowed"
                ),
            ));
        }
        Ok((name, span))
    }

    /// `aws/ec2/instance`, two or more `/`-joined segments. A segment lexes as an
    /// identifier or a keyword, then is validated against the stricter
    /// letters-then-letters/digits grammar.
    pub(crate) fn parse_element_type(&mut self) -> Result<(String, Span), ParseError> {
        let first = self.parse_type_segment()?;
        let start = first.start;
        let mut text = first.value.clone();
        self.expect(TokenType::Slash)?;
        text.push('/');
        let second = self.parse_type_segment()?;
        text.push_str(&second.value);
        let mut end = second.end;

        while self.match_token(TokenType::Slash) {
            text.push('/');
            let seg = self.parse_type_segment()?;
            text.push_str(&seg.value);
            end = seg.end;
        }

        Ok((text, Span::new(start, end)))
    }

    fn parse_type_segment(&mut self) -> Result<Token, ParseError> {
        let t = self.peek();
        let (ty, start, value) = (t.ty, t.start, t.value.clone());
        let is_segment = ty == TokenType::Identifier
            || ty == TokenType::BoolLiteral
            || ty == TokenType::NoneLiteral
            || ty.is_keyword();

        if !is_segment {
            return Err(self.error_at(
                start,
                format!(
                    "expected an element type segment, got {}",
                    ty.display_label()
                ),
            ));
        }

        let token = self.advance();
        if !is_valid_type_segment(&value) {
            return Err(self.error_at(start,
            format!("invalid element type segment {value:?}: a letter followed by letters or digits")));
        }
        Ok(token)
    }
}

fn is_valid_quoted_name(s: &str) -> bool {
    !s.is_empty()
        && s.chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '_' | '-' | '.'))
}

fn is_valid_type_segment(s: &str) -> bool {
    let mut chars = s.chars();
    matches!(chars.next(), Some(c) if c.is_ascii_alphabetic())
        && chars.all(|c| c.is_ascii_alphanumeric())
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_bare_element_name() {
        let mut p = Parser::new("region");
        assert_eq!(p.parse_element_name().unwrap().0, "region");
    }

    #[test]
    fn test_quoted_element_name_allows_dots() {
        let mut p = Parser::new("\"primary.region\"");
        assert_eq!(p.parse_element_name().unwrap().0, "primary.region");
    }

    #[test]
    fn test_keyword_rejected_as_bare_element_name() {
        let mut p = Parser::new("variable");
        assert!(p
            .parse_element_name()
            .unwrap_err()
            .message
            .contains("reserved"));
    }

    #[test]
    fn test_invalid_quoted_name_characters_rejected() {
        // A ':' is outside the referenceable name set.
        let mut p = Parser::new("\"a:b\"");
        assert!(p
            .parse_element_name()
            .unwrap_err()
            .message
            .contains("invalid quoted name"));
    }

    #[test]
    fn test_field_key_allows_a_keyword() {
        // `value` is reserved, but it is a valid bare field key.
        let mut p = Parser::new("value");
        assert_eq!(p.parse_field_key().unwrap().0, "value");
    }

    #[test]
    fn test_object_key_allows_arbitrary_quoted_characters() {
        // Object keys are data, so any character is allowed (e.g. an IAM key).
        let mut p = Parser::new("\"aws:SourceArn\"");
        assert_eq!(p.parse_object_key().unwrap().0, "aws:SourceArn");
    }

    #[test]
    fn test_is_valid_quoted_name() {
        assert!(is_valid_quoted_name("a.b-c_1"));
        assert!(!is_valid_quoted_name(""));
        assert!(!is_valid_quoted_name("a:b"));
    }
}
