//! The hand-written lexer: a char scanner with a string/interpolation mode
//! stack that turns blueprint-language source into a [`crate::tokens::Token`]
//! stream.

use crate::{
    errors::{Diagnostic, Diagnostics, LexError},
    source::{Position, Span},
    tokens::keyword_token,
    Errors, Token, TokenType,
};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Mode {
    Normal,
    SingleString,
    MultiString,
    Interpolation,
}

pub(crate) struct Lexer {
    src: Vec<char>,
    // char index, not byte offset
    pos: usize,
    // 1-based line number.
    line: usize,
    // 1-based column number (in chars, not bytes).
    col: usize,
    // Current mode is at the top.
    modes: Vec<Mode>,
    interp_brace_depth: Vec<i64>,
    diags: Diagnostics,
    // Indentation prefix stripped from each
    // line of the current multiline string.
    multiline_strip: String,
}

impl Lexer {
    pub(crate) fn new(src: &str) -> Self {
        Self {
            src: src.replace("\r\n", "\n").chars().collect(),
            pos: 0,
            line: 1,
            col: 1,
            modes: vec![Mode::Normal],
            interp_brace_depth: Vec::new(),
            // Keep the original (pre-normalised) source for snippet rendering.
            diags: Diagnostics::new(Some(src.to_string())),
            multiline_strip: String::new(),
        }
    }

    pub(crate) fn into_errors(self) -> Option<Errors> {
        self.diags.into_errors()
    }

    /// Adds a parse-time diagnostic to the lexer, so lex and parse errors
    /// accumulate together and surface in a single pass.
    pub(crate) fn add_diagnostic(&mut self, diagnostic: impl Into<Diagnostic>) {
        self.diags.add(diagnostic);
    }

    fn peek(&self) -> Option<char> {
        self.src.get(self.pos).copied()
    }

    fn peek_nth(&self, offset: usize) -> Option<char> {
        self.src.get(self.pos + offset).copied()
    }

    fn at_eof(&self) -> bool {
        self.pos >= self.src.len()
    }

    fn consume(&mut self) -> Option<char> {
        let ch = self.src.get(self.pos).copied()?;
        self.pos += 1;
        if ch == '\n' {
            self.line += 1;
            self.col = 1;
        } else {
            self.col += 1;
        }
        Some(ch)
    }

    // ASCII-prefix lookahead (`${`, `"""`, `==`).
    fn has_prefix(&self, prefix: &str) -> bool {
        prefix
            .chars()
            .enumerate()
            .all(|(i, ch)| self.src.get(self.pos + i) == Some(&ch))
    }

    fn consume_prefix(&mut self, prefix: &str) -> bool {
        if !self.has_prefix(prefix) {
            return false;
        }

        for _ in prefix.chars() {
            self.consume();
        }

        true
    }

    fn take_while(&mut self, pred: impl Fn(char) -> bool) -> String {
        let start = self.pos;
        while self.peek().is_some_and(&pred) {
            self.consume();
        }
        self.src[start..self.pos].iter().collect()
    }

    fn current_pos(&self) -> Position {
        Position::new(self.line, self.col)
    }

    fn token(&self, ty: TokenType, value: impl Into<String>, start: Position) -> Token {
        Token::new(ty, value, start, self.current_pos())
    }

    fn errf(&self, message: impl Into<String>) -> LexError {
        LexError::new(message, Span::at(self.current_pos()))
    }

    pub(crate) fn next_token(&mut self) -> Token {
        while !self.at_eof() {
            let start = self.pos;
            match self.next_token_inner() {
                Ok(token) => return token,
                Err(err) => {
                    self.diags.add(err);
                    if self.pos == start {
                        // force progress, avoid an infinite loop.
                        self.consume();
                    }
                }
            }
        }
        self.eof_token()
    }

    fn next_token_inner(&mut self) -> Result<Token, LexError> {
        match self.current_mode() {
            Mode::Normal | Mode::Interpolation => self.next_expr_token(),
            Mode::SingleString => self.next_string_content(false),
            Mode::MultiString => self.next_string_content(true),
        }
    }

    fn next_expr_token(&mut self) -> Result<Token, LexError> {
        // Skip spaces and tabs but not newlines.
        self.skip_while(|c| c == ' ' || c == '\t');
        let Some(ch) = self.peek() else {
            return Ok(self.eof_token());
        };
        match ch {
            '\n' => Ok(self.lex_newline()),
            '#' => Ok(self.lex_comment()),
            c if is_ident_start(c) => Ok(self.lex_ident_or_keyword()),
            '"' => self.open_string(),
            c if is_digit(c) || c == '-' => self.lex_number(),
            _ => self.lex_punct_or_operator(),
        }
    }

    fn lex_number(&mut self) -> Result<Token, LexError> {
        let start = self.current_pos();
        let mut s = String::new();
        if self.consume_char('-') {
            s.push('-');
        }
        let int_part = self.take_while(is_digit);
        if int_part.is_empty() {
            return Err(self.errf("expected digit"));
        }
        s.push_str(&int_part);

        // Is a float only when a digit follows '.':
        // `1.0` is a float but `1.` is not.
        if self.peek() == Some('.') && self.peek_nth(1).is_some_and(is_digit) {
            self.consume(); // consume '.'
            s.push('.');
            s.push_str(&self.take_while(is_digit));
            return Ok(self.token(TokenType::FloatLiteral, s, start));
        }
        Ok(self.token(TokenType::IntLiteral, s, start))
    }

    fn lex_punct_or_operator(&mut self) -> Result<Token, LexError> {
        let start = self.current_pos();
        // Multi-char first, longest-to-shortest.
        for (lexeme, ty) in [
            ("==", TokenType::Eq),
            ("!=", TokenType::Neq),
            ("<=", TokenType::Lte),
            (">=", TokenType::Gte),
            ("&&", TokenType::And),
            ("||", TokenType::Or),
        ] {
            if self.consume_prefix(lexeme) {
                return Ok(self.token(ty, lexeme, start));
            }
        }

        let ch = self.consume().expect("caller guarantees a char is present");
        let ty = match ch {
            '[' => TokenType::LeftBracket,
            ']' => TokenType::RightBracket,
            '(' => TokenType::LeftParen,
            ')' => TokenType::RightParen,
            ':' => TokenType::Colon,
            '=' => TokenType::Assign,
            ',' => TokenType::Comma,
            '.' => TokenType::Period,
            '<' => TokenType::Lt,
            '>' => TokenType::Gt,
            '*' => TokenType::Star,
            '/' => TokenType::Slash,
            '!' => TokenType::Not,
            '{' => {
                if self.current_mode() == Mode::Interpolation {
                    if let Some(top) = self.interp_brace_depth.last_mut() {
                        *top += 1; // so a nested object's `}` doesn't close the ${..}
                    }
                }
                TokenType::LeftBrace
            }
            '}' => return Ok(self.close_brace_or_interpolation(start)),
            other => {
                return Err(LexError::new(
                    format!("unexpected character: {other:?}"),
                    Span::at(start),
                ));
            }
        };
        Ok(self.token(ty, ch.to_string(), start))
    }

    fn close_brace_or_interpolation(&mut self, start: Position) -> Token {
        if self.current_mode() == Mode::Interpolation {
            if let Some(top) = self.interp_brace_depth.last_mut() {
                *top -= 1;
                if *top == 0 {
                    self.interp_brace_depth.pop();
                    self.pop_mode();
                    return self.token(TokenType::InterpolationEnd, "}", start);
                }
            }
        }
        self.token(TokenType::RightBrace, "}", start)
    }

    fn lex_ident_or_keyword(&mut self) -> Token {
        let start = self.current_pos();
        let word = self.take_while(is_ident_char);
        let ty = match word.as_str() {
            "true" | "false" => TokenType::BoolLiteral,
            "none" => TokenType::NoneLiteral,
            other => keyword_token(other).unwrap_or(TokenType::Identifier),
        };
        self.token(ty, word, start)
    }

    fn lex_newline(&mut self) -> Token {
        let start = self.current_pos();
        self.consume(); // '\n'
        self.token(TokenType::Newline, "\n", start)
    }

    fn lex_comment(&mut self) -> Token {
        let start = self.current_pos();
        self.consume(); // '#'
        let text = self.take_while(|c| c != '\n');
        self.token(TokenType::Comment, text, start)
    }

    fn eof_token(&self) -> Token {
        let pos = self.current_pos();
        Token::new(TokenType::Eof, "", pos, pos)
    }

    fn open_string(&mut self) -> Result<Token, LexError> {
        let start = self.current_pos();

        if self.consume_prefix("\"\"\"") {
            // The opening """ must be followed by a line break
            // as per spec.
            if self.peek() != Some('\n') {
                return Err(self.errf("opening \"\"\" must be followed by a line break"));
            }
            self.consume(); // consume the line break

            self.multiline_strip = self.scan_multiline_strip_indent()?;
            // Strip the indentation from the first content line; subsequent
            // lines are stripped after each newline in next_string_content.
            self.skip_multiline_strip()?;

            self.push_mode(Mode::MultiString);
            return Ok(self.token(TokenType::StringStart, "\"\"\"", start));
        }

        self.consume(); // consume the '"'
        self.push_mode(Mode::SingleString);
        Ok(self.token(TokenType::StringStart, "\"", start))
    }

    fn next_string_content(&mut self, multiline: bool) -> Result<Token, LexError> {
        let start = self.current_pos();
        let mut buf = String::new();

        while !self.at_eof() {
            if self.at_string_close(multiline) {
                if !buf.is_empty() {
                    if multiline {
                        // The newline just before the closing """ is not part of the value
                        // as per spec.
                        if buf.ends_with('\n') {
                            buf.pop();
                        }
                        return Ok(self.token(TokenType::MultilineStringLiteral, buf, start));
                    }
                    return Ok(self.token(TokenType::StringLiteral, buf, start));
                }
                return Ok(self.close_string(start, multiline));
            }

            if !multiline && self.peek() == Some('\n') {
                return Err(self.errf("unterminated string literal: newline in single-line string"));
            }

            if self.has_prefix("${") {
                if !buf.is_empty() {
                    return Ok(self.token(string_content_token_type(multiline), buf, start));
                }
                self.consume_prefix("${");
                self.push_mode(Mode::Interpolation);
                self.interp_brace_depth.push(1);
                return Ok(self.token(TokenType::InterpolationStart, "${", start));
            }

            // The only escape is \" (spec); every other character is taken
            // literally. A multi-line string allows " unescaped, so the escape
            // only applies to single-line strings.
            if !multiline && self.has_prefix("\\\"") {
                self.consume_prefix("\\\"");
                buf.push('"');
                continue;
            }

            let ch = self
                .consume()
                .expect("loop guard guarantees a char is present");
            buf.push(ch);

            if multiline && ch == '\n' {
                self.skip_multiline_strip()?;
            }
        }

        Err(self.errf("unexpected end of input in string literal"))
    }

    /// Forward textual scan from the current position to the closing `"""`,
    /// returning the indentation that preceds it.
    /// The closer must sit on its own line (only spaces or tabs before it).
    /// Note: the first `"""` found wins, even inside an interpolation,
    /// nested multi-line strings aren't supported.
    fn scan_multiline_strip_indent(&self) -> Result<String, LexError> {
        let mut pos = self.pos;
        let mut line_start = pos;
        while pos < self.src.len() {
            if self.src[pos] == '"'
                && self.src.get(pos + 1) == Some(&'"')
                && self.src.get(pos + 2) == Some(&'"')
            {
                let indent: String = self.src[line_start..pos].iter().collect();
                if indent.chars().any(|c| c != ' ' && c != '\t') {
                    return Err(self.errf("closing \"\"\" must be on its own line"));
                }
                return Ok(indent);
            }

            if self.src[pos] == '\n' {
                line_start = pos + 1;
            }
            pos += 1;
        }
        Err(self.errf("unterminated multi-line string"))
    }

    /// After each newline inside a multi-line string, consume the captured
    /// indentation prefix. A short or blank line (newline/EOF before the prefix is
    /// exhausted) is allowed.
    fn skip_multiline_strip(&mut self) -> Result<(), LexError> {
        // Clone to avoid holding a borrow of `self` across the consume() calls.
        let strip = self.multiline_strip.clone();
        for expected in strip.chars() {
            if self.at_eof() || self.peek() == Some('\n') {
                return Ok(());
            }
            if self.peek() != Some(expected) {
                return Err(self.errf("multi-line string line not indented correctly"));
            }
            self.consume();
        }
        Ok(())
    }

    fn at_string_close(&self, multiline: bool) -> bool {
        if multiline {
            self.has_prefix("\"\"\"")
        } else {
            self.peek() == Some('"')
        }
    }

    fn close_string(&mut self, start: Position, multiline: bool) -> Token {
        let closer = if multiline {
            self.multiline_strip.clear();
            "\"\"\""
        } else {
            "\""
        };
        self.consume_prefix(closer);
        self.pop_mode();
        self.token(TokenType::StringEnd, closer, start)
    }

    fn consume_char(&mut self, ch: char) -> bool {
        if self.peek() == Some(ch) {
            self.consume();
            true
        } else {
            false
        }
    }

    fn skip_while(&mut self, pred: impl Fn(char) -> bool) {
        while self.peek().is_some_and(&pred) {
            self.consume();
        }
    }

    fn pop_mode(&mut self) {
        self.modes.pop();
    }

    fn push_mode(&mut self, mode: Mode) {
        self.modes.push(mode);
    }

    fn current_mode(&self) -> Mode {
        self.modes.last().copied().unwrap_or(Mode::Normal)
    }
}

fn is_ident_start(ch: char) -> bool {
    ch.is_ascii_alphabetic() || ch == '_'
}

fn is_ident_char(ch: char) -> bool {
    is_ident_start(ch) || ch.is_ascii_digit() || ch == '-'
}

fn is_digit(ch: char) -> bool {
    ch.is_ascii_digit()
}

fn string_content_token_type(multiline: bool) -> TokenType {
    if multiline {
        TokenType::MultilineStringLiteral
    } else {
        TokenType::StringLiteral
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    /// Lexes `src`, asserting no diagnostics, and returns the token types.
    fn types(src: &str) -> Vec<TokenType> {
        let (tokens, errs) = crate::tokenize(src);
        assert!(errs.is_none(), "unexpected lex errors: {errs:?}");
        tokens.iter().map(|t| t.ty).collect()
    }

    /// Lexes `src`, asserting no diagnostics, and returns `(type, value)` pairs.
    fn pairs(src: &str) -> Vec<(TokenType, String)> {
        let (tokens, errs) = crate::tokenize(src);
        assert!(errs.is_none(), "unexpected lex errors: {errs:?}");
        tokens.iter().map(|t| (t.ty, t.value.clone())).collect()
    }

    #[test]
    fn test_empty_input_is_just_eof() {
        assert_eq!(types(""), vec![TokenType::Eof]);
    }

    #[test]
    fn test_keywords_namespaces_and_identifiers() {
        use TokenType::*;
        assert_eq!(
            types("variable resource variables myThing"),
            vec![
                KeywordVariable,
                KeywordResource,
                KeywordVariables,
                Identifier,
                Eof
            ],
        );
    }

    #[test]
    fn test_bool_and_none_literals() {
        use TokenType::*;
        assert_eq!(
            types("true false none"),
            vec![BoolLiteral, BoolLiteral, NoneLiteral, Eof],
        );
    }

    #[test]
    fn test_operators_and_punctuation() {
        use TokenType::*;
        assert_eq!(
            types("== != <= >= && || ! < > = ( ) [ ] { } : , . * /"),
            vec![
                Eq,
                Neq,
                Lte,
                Gte,
                And,
                Or,
                Not,
                Lt,
                Gt,
                Assign,
                LeftParen,
                RightParen,
                LeftBracket,
                RightBracket,
                LeftBrace,
                RightBrace,
                Colon,
                Comma,
                Period,
                Star,
                Slash,
                Eof,
            ],
        );
    }

    #[test]
    fn test_longest_match_operators() {
        use TokenType::*;
        assert_eq!(
            types("a == b = c"),
            vec![Identifier, Eq, Identifier, Assign, Identifier, Eof],
        );
    }

    #[test]
    fn test_int_vs_float_boundary() {
        use TokenType::*;
        assert_eq!(
            pairs("1.0"),
            vec![(FloatLiteral, "1.0".into()), (Eof, "".into())],
        );
        assert_eq!(
            pairs("-5"),
            vec![(IntLiteral, "-5".into()), (Eof, "".into())]
        );
        // `1.` is an integer followed by a period, not a float.
        assert_eq!(types("1."), vec![IntLiteral, Period, Eof]);
        // `.5` is a period followed by an integer.
        assert_eq!(types(".5"), vec![Period, IntLiteral, Eof]);
    }

    #[test]
    fn test_element_type_segments_lex_as_idents_and_slashes() {
        use TokenType::*;
        // The parser joins these on '/'; the lexer just emits the pieces.
        assert_eq!(
            types("aws/ec2/instance"),
            vec![Identifier, Slash, Identifier, Slash, Identifier, Eof],
        );
    }

    #[test]
    fn test_comment_runs_to_end_of_line() {
        use TokenType::*;
        assert_eq!(
            pairs("# hello world\nx"),
            vec![
                (Comment, " hello world".into()),
                (Newline, "\n".into()),
                (Identifier, "x".into()),
                (Eof, "".into()),
            ],
        );
    }

    #[test]
    fn test_token_positions_track_lines_and_columns() {
        let (tokens, errs) = crate::tokenize("ab\ncd");
        assert!(errs.is_none());
        // `ab` on line 1, columns 1..3.
        assert_eq!(tokens[0].ty, TokenType::Identifier);
        assert_eq!(tokens[0].start, Position::new(1, 1));
        assert_eq!(tokens[0].end, Position::new(1, 3));
        // The newline spans the end of line 1 to the start of line 2.
        assert_eq!(tokens[1].ty, TokenType::Newline);
        assert_eq!(tokens[1].start, Position::new(1, 3));
        assert_eq!(tokens[1].end, Position::new(2, 1));
        // `cd` on line 2, columns 1..3.
        assert_eq!(tokens[2].ty, TokenType::Identifier);
        assert_eq!(tokens[2].start, Position::new(2, 1));
        assert_eq!(tokens[2].end, Position::new(2, 3));
    }

    #[test]
    fn test_unexpected_char_is_collected_not_fatal() {
        let (tokens, errs) = crate::tokenize("a @ b");
        // The stream still terminates with EOF despite the bad character.
        assert_eq!(tokens.last().unwrap().ty, TokenType::Eof);
        let errs = errs.expect("expected a diagnostic for '@'");
        assert!(errs.to_string().contains("unexpected character"));
    }

    #[test]
    fn test_braces_are_plain_in_normal_mode() {
        use TokenType::*;
        // Outside interpolation a `}` is always a right brace.
        assert_eq!(types("{ x }"), vec![LeftBrace, Identifier, RightBrace, Eof]);
    }

    #[test]
    fn test_plain_single_line_string() {
        use TokenType::*;
        assert_eq!(
            pairs(r#""hello""#),
            vec![
                (StringStart, "\"".into()),
                (StringLiteral, "hello".into()),
                (StringEnd, "\"".into()),
                (Eof, "".into()),
            ],
        );
    }

    #[test]
    fn test_single_line_string_with_interpolation() {
        use TokenType::*;
        assert_eq!(
            pairs(r#""a ${variables.x} b""#),
            vec![
                (StringStart, "\"".into()),
                (StringLiteral, "a ".into()),
                (InterpolationStart, "${".into()),
                (KeywordVariables, "variables".into()),
                (Period, ".".into()),
                (Identifier, "x".into()),
                (InterpolationEnd, "}".into()),
                (StringLiteral, " b".into()),
                (StringEnd, "\"".into()),
                (Eof, "".into()),
            ],
        );
    }

    #[test]
    fn test_escaped_quote_in_single_line_string() {
        use TokenType::*;
        assert_eq!(
            pairs(r#""say \"hi\"""#),
            vec![
                (StringStart, "\"".into()),
                (StringLiteral, r#"say "hi""#.into()),
                (StringEnd, "\"".into()),
                (Eof, "".into()),
            ],
        );
    }

    #[test]
    fn test_unterminated_single_line_string_is_reported() {
        let (_tokens, errs) = crate::tokenize("\"abc\n");
        let errs = errs.expect("expected an unterminated-string error");
        assert!(errs.to_string().contains("unterminated string"));
    }

    #[test]
    fn test_nested_interpolation_braces_do_not_close_early() {
        use TokenType::*;
        // A `}` closing the inner object literal must not close the ${..};
        // only the final `}` is the interpolation end.
        let kinds = types(r#""${ object(x = { a = 1 }) }""#);
        assert!(kinds.contains(&LeftBrace) && kinds.contains(&RightBrace));
        assert_eq!(
            kinds.iter().filter(|t| **t == InterpolationEnd).count(),
            1,
            "exactly one interpolation end expected"
        );
        assert_eq!(
            kinds.iter().filter(|t| **t == InterpolationStart).count(),
            1,
        );
    }

    #[test]
    fn test_multiline_string_strips_indentation() {
        use TokenType::*;
        // The closing """ is indented four spaces, so a four-space prefix is
        // stripped from every content line and the bounding newlines removed.
        let src = "\"\"\"\n    hello\n    world\n    \"\"\"";
        assert_eq!(
            pairs(src),
            vec![
                (StringStart, "\"\"\"".into()),
                (MultilineStringLiteral, "hello\nworld".into()),
                (StringEnd, "\"\"\"".into()),
                (Eof, "".into()),
            ],
        );
    }

    #[test]
    fn test_multiline_string_supports_interpolation() {
        use TokenType::*;
        let src = "\"\"\"\n    Hello ${variables.name}!\n    \"\"\"";
        assert_eq!(
            types(src),
            vec![
                StringStart,
                MultilineStringLiteral, // "Hello "
                InterpolationStart,
                KeywordVariables,
                Period,
                Identifier,
                InterpolationEnd,
                MultilineStringLiteral, // "!" (the text after the interpolation)
                StringEnd,
                Eof,
            ],
        );
    }

    #[test]
    fn test_multiline_string_closer_not_on_own_line_is_reported() {
        let (_tokens, errs) = crate::tokenize("\"\"\"\n    hello \"\"\"");
        let errs = errs.expect("expected a closing-delimiter error");
        assert!(errs.to_string().contains("must be on its own line"));
    }
}
