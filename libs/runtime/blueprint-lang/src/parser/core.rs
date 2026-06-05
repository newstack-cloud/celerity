//! The `Parser` struct: token buffer, lookahead, grouping-depth tracking, and
//! the top-level parse loop with error recovery (`synchronise`).

use std::collections::VecDeque;

use crate::errors::{Errors, ParseError};
use crate::lexer::Lexer;
use crate::source::{Position, Span};
use crate::tokens::{Token, TokenType};

/// Reserved words that start a top-level declaration, the synchronisation
/// targets after a parse error.
const DECLARATION_KEYWORDS: &[TokenType] = &[
    TokenType::KeywordVariable,
    TokenType::KeywordValue,
    TokenType::KeywordData,
    TokenType::KeywordResource,
    TokenType::KeywordInclude,
    TokenType::KeywordExport,
    TokenType::KeywordVersion,
    TokenType::KeywordTransform,
    TokenType::KeywordMetadata,
];

pub(crate) struct Parser {
    lexer: Lexer,
    /// Lookahead ring; the grammar needs at most two tokens of lookahead.
    buf: VecDeque<Token>,
    /// Nesting inside `( )` / `[ ]`. When > 0, newlines are insignificant.
    grouping_depth: i64,
}

impl Parser {
    pub(crate) fn new(src: &str) -> Self {
        Self {
            lexer: Lexer::new(src),
            buf: VecDeque::new(),
            grouping_depth: 0,
        }
    }

    pub(crate) fn into_errors(self) -> Option<Errors> {
        self.lexer.into_errors()
    }

    /// Ensures the buffer holds at least `n` non-comment tokens.
    fn fill(&mut self, n: usize) {
        while self.buf.len() < n {
            let token = self.lexer.next_token();
            if token.ty == TokenType::Comment {
                continue;
            }
            self.buf.push_back(token);
        }
    }

    /// Skips newline tokens while inside a grouping (depth > 0).
    fn skip_grouped_newlines(&mut self) {
        if self.grouping_depth == 0 {
            return;
        }
        loop {
            self.fill(1);
            if self.buf[0].ty != TokenType::Newline {
                return;
            }
            self.buf.pop_front();
        }
    }

    pub(crate) fn peek(&mut self) -> &Token {
        self.skip_grouped_newlines();
        self.fill(1);
        &self.buf[0]
    }

    pub(crate) fn peek_type(&mut self) -> TokenType {
        self.peek().ty
    }

    pub(crate) fn peek_at(&mut self, n: usize) -> &Token {
        self.skip_grouped_newlines();
        self.fill(n + 1);
        &self.buf[n]
    }

    /// The type of the next token past any newlines, without consuming.
    /// (Returns the type rather than `&Token` to avoid a loop-carried borrow.)
    pub(crate) fn peek_type_past_newlines(&mut self) -> TokenType {
        let mut i = 0;
        loop {
            self.fill(i + 1);
            if self.buf[i].ty != TokenType::Newline {
                return self.buf[i].ty;
            }
            i += 1;
        }
    }

    pub(crate) fn advance(&mut self) -> Token {
        self.skip_grouped_newlines();
        self.fill(1);
        let token = self.buf.pop_front().expect("fill guarantees a token");
        match token.ty {
            TokenType::LeftParen | TokenType::LeftBracket => self.grouping_depth += 1,
            TokenType::RightParen | TokenType::RightBracket if self.grouping_depth > 0 => {
                self.grouping_depth -= 1;
            }
            _ => {}
        }
        token
    }

    pub(crate) fn match_token(&mut self, ty: TokenType) -> bool {
        if self.peek_type() == ty {
            self.advance();
            true
        } else {
            false
        }
    }

    /// Matches one of `types` across intervening newlines (operator continuation
    /// at a line boundary), committing the newlines as continuation if so.
    pub(crate) fn match_across_newlines(&mut self, types: &[TokenType]) -> Option<Token> {
        if types.contains(&self.peek_type_past_newlines()) {
            self.skip_newlines(); // commit: newline was a continuation
            let op = self.advance();
            self.skip_newlines(); // tolerate operator-at-end-of-line
            Some(op)
        } else {
            None
        }
    }

    pub(crate) fn expect(&mut self, ty: TokenType) -> Result<Token, ParseError> {
        let found = self.peek();
        let (found_ty, start) = (found.ty, found.start);
        if found_ty != ty {
            return Err(self.error_at(
                start,
                format!(
                    "expected {}, got {}",
                    ty.display_label(),
                    found_ty.display_label()
                ),
            ));
        }
        Ok(self.advance())
    }

    /// Consumes a run of comma/newline separators; reports whether any were seen.
    pub(crate) fn consume_separators(&mut self) -> bool {
        let mut consumed = false;
        while matches!(self.peek_type(), TokenType::Comma | TokenType::Newline) {
            self.advance();
            consumed = true;
        }
        consumed
    }

    pub(crate) fn skip_newlines(&mut self) {
        loop {
            self.fill(1);
            if self.buf[0].ty != TokenType::Newline {
                return;
            }
            self.buf.pop_front();
        }
    }

    pub(crate) fn error_at(&self, pos: Position, message: impl Into<String>) -> ParseError {
        ParseError::new(message, Span::at(pos))
    }

    pub(crate) fn add_error(&mut self, err: ParseError) {
        self.lexer.add_diagnostic(err);
    }

    /// After a parse error, skip to the next declaration header (resetting
    /// grouping depth so an unclosed `(`/`[` doesn't swallow significant
    /// newlines), then resume.
    pub(crate) fn synchronise(&mut self) {
        self.grouping_depth = 0;
        loop {
            let ty = self.peek_type();
            if ty == TokenType::Eof || DECLARATION_KEYWORDS.contains(&ty) {
                return;
            }
            self.advance();
        }
    }
}

pub(crate) fn token_span(token: &Token) -> Span {
    Span::new(token.start, token.end)
}

/// Extends `field_span` to end at `value_end`, so a fields_source_meta entry
/// covers the whole `field = value`. Falls back to the field span.
pub(crate) fn merge_end(field_span: Span, value_end: Option<Position>) -> Span {
    match value_end {
        Some(end) => Span::new(field_span.start, end),
        None => field_span,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_advance_skips_comments_and_yields_eof() {
        let mut p = Parser::new("a # note\nb");
        assert_eq!(p.advance().ty, TokenType::Identifier); // a
        assert_eq!(p.advance().ty, TokenType::Newline); // comment skipped
        assert_eq!(p.advance().ty, TokenType::Identifier); // b
        assert_eq!(p.peek_type(), TokenType::Eof);
    }

    #[test]
    fn test_peek_at_looks_ahead_without_consuming() {
        let mut p = Parser::new("a : b");
        assert_eq!(p.peek_type(), TokenType::Identifier);
        assert_eq!(p.peek_at(1).ty, TokenType::Colon);
        assert_eq!(p.peek_at(2).ty, TokenType::Identifier);
        // Lookahead did not consume the head of the stream.
        assert_eq!(p.advance().ty, TokenType::Identifier);
    }

    #[test]
    fn test_match_token_consumes_only_on_match() {
        let mut p = Parser::new(": a");
        assert!(!p.match_token(TokenType::Comma));
        assert!(p.match_token(TokenType::Colon));
        assert_eq!(p.peek_type(), TokenType::Identifier);
    }

    #[test]
    fn test_newlines_insignificant_inside_grouping() {
        let mut p = Parser::new("(\n a \n)");
        assert_eq!(p.advance().ty, TokenType::LeftParen); // depth -> 1
        assert_eq!(p.peek_type(), TokenType::Identifier); // newline skipped
        assert_eq!(p.advance().ty, TokenType::Identifier);
        assert_eq!(p.advance().ty, TokenType::RightParen); // depth -> 0
    }

    #[test]
    fn test_newlines_significant_outside_grouping() {
        let mut p = Parser::new("a\nb");
        assert_eq!(p.advance().ty, TokenType::Identifier);
        assert_eq!(p.peek_type(), TokenType::Newline); // not skipped at depth 0
    }

    #[test]
    fn test_nested_grouping_keeps_newlines_insignificant_until_closed() {
        let mut p = Parser::new("[ (\n a \n) \n ]");
        assert_eq!(p.advance().ty, TokenType::LeftBracket); // depth 1
        assert_eq!(p.advance().ty, TokenType::LeftParen); // depth 2
        assert_eq!(p.advance().ty, TokenType::Identifier); // a (newline skipped)
        assert_eq!(p.advance().ty, TokenType::RightParen); // depth 1
                                                           // The newline before `]` is still inside the bracket grouping (depth 1).
        assert_eq!(p.advance().ty, TokenType::RightBracket); // depth 0
        assert_eq!(p.peek_type(), TokenType::Eof);
    }

    #[test]
    fn test_match_across_newlines_finds_continuation_operator() {
        let mut p = Parser::new("a\n  && b");
        assert_eq!(p.advance().ty, TokenType::Identifier);
        let op = p.match_across_newlines(&[TokenType::And]);
        assert!(op.is_some());
        assert_eq!(op.unwrap().ty, TokenType::And);
        assert_eq!(p.peek_type(), TokenType::Identifier); // positioned at `b`
    }

    #[test]
    fn test_match_across_newlines_does_not_consume_on_miss() {
        let mut p = Parser::new("a\nb");
        assert_eq!(p.advance().ty, TokenType::Identifier);
        // `b` is not an operator, so nothing is consumed.
        assert!(p.match_across_newlines(&[TokenType::And]).is_none());
        assert_eq!(p.peek_type(), TokenType::Newline);
    }

    #[test]
    fn test_consume_separators_consumes_commas_and_newlines() {
        let mut p = Parser::new("a , \n , b");
        assert_eq!(p.advance().ty, TokenType::Identifier); // a
        assert!(p.consume_separators());
        assert_eq!(p.peek_type(), TokenType::Identifier); // b
                                                          // No separators present now.
        assert!(!p.consume_separators());
    }

    #[test]
    fn test_expect_consumes_on_match_and_errors_otherwise() {
        let mut p = Parser::new("a : b");
        assert_eq!(
            p.expect(TokenType::Identifier).unwrap().ty,
            TokenType::Identifier
        );
        let err = p.expect(TokenType::Comma).unwrap_err();
        assert!(err.message.contains("expected ','"));
        assert!(err.message.contains("got ':'"));
    }

    #[test]
    fn test_synchronise_lands_on_declaration_keyword() {
        let mut p = Parser::new("@ ! ) resource foo: x/y {}");
        p.synchronise();
        assert_eq!(p.peek_type(), TokenType::KeywordResource);
    }

    #[test]
    fn test_synchronise_stops_at_eof_when_no_declaration_follows() {
        let mut p = Parser::new("@ @ @");
        p.synchronise();
        assert_eq!(p.peek_type(), TokenType::Eof);
    }

    #[test]
    fn test_add_error_surfaces_via_into_errors() {
        let mut p = Parser::new("a");
        let err = p.error_at(Position::new(1, 1), "boom");
        p.add_error(err);
        let errors = p.into_errors().expect("expected diagnostics");
        assert!(errors.to_string().contains("boom"));
    }

    #[test]
    fn test_into_errors_merges_lexer_diagnostics() {
        // A stray character is a lex error; it should surface through the parser.
        let mut p = Parser::new("a @ b");
        while p.advance().ty != TokenType::Eof {}
        let errors = p.into_errors().expect("expected diagnostics");
        assert!(errors.to_string().contains("unexpected character"));
    }
}
