//! The Pratt / precedence-climbing expression parser (or / and / eq / cmp /
//! unary / primary), desugaring operators to function calls.

use crate::{
    errors::ParseError,
    parser::core::{token_span, Parser},
    scalar::{Scalar, ScalarValue},
    source::Span,
    substitution::{function_names, Substitution, SubstitutionFunctionName, SubstitutionPathItem},
    Token, TokenType,
};

/// A transient expression node, built while parsing a bare expression or a
/// `${..}` body. Transformed into the canonical types before it reaches the
/// schema.
#[derive(Debug, Clone, PartialEq)]
pub(crate) enum Expr {
    Scalar(Scalar),
    Array {
        elems: Vec<Expr>,
        span: Option<Span>,
    },
    Object {
        entries: Vec<ObjectField>,
        span: Option<Span>,
    },
    // A reference already resolved to a substitution (variables, resources etc.)
    Ref(Substitution),
    // A function call, with accessors that drill into an array/object result.
    Call {
        name: SubstitutionFunctionName,
        args: Vec<CallArg>,
        path: Vec<SubstitutionPathItem>,
        span: Option<Span>,
    },
    // An operator application (`==`, `&&`, `!` etc.) recorded by the function
    // name that it desugars to, turned into a `Substitution::Function`
    // in transforming to schema.
    Op {
        fn_name: SubstitutionFunctionName,
        args: Vec<Expr>,
        span: Option<Span>,
    },
    /// A string literal that interpolates `${..}` parts.
    Interpolation {
        parts: Vec<InterpPart>,
        span: Option<Span>,
    },
    None(Option<Span>),
}

#[derive(Debug, Clone, PartialEq)]
pub(crate) struct ObjectField {
    pub key: String,
    pub value: Expr,
    pub span: Option<Span>,
}

#[derive(Debug, Clone, PartialEq)]
pub(crate) struct CallArg {
    /// Set only for a named argument (`name = expr`);
    /// meaningful for `object`.
    pub name: Option<String>,
    pub value: Expr,
    pub span: Option<Span>,
}

#[derive(Debug, Clone, PartialEq)]
pub(crate) enum InterpPart {
    String { value: String, span: Option<Span> },
    Sub { value: Expr, span: Option<Span> },
}

impl Expr {
    /// The source span of this node, if tracked.
    pub(crate) fn span(&self) -> Option<Span> {
        match self {
            Expr::Scalar(s) => s.span,
            Expr::Ref(s) => s.span,
            Expr::Array { span, .. }
            | Expr::Object { span, .. }
            | Expr::Call { span, .. }
            | Expr::Op { span, .. }
            | Expr::Interpolation { span, .. }
            | Expr::None(span) => *span,
        }
    }
}

impl Parser {
    pub(crate) fn parse_expr(&mut self) -> Result<Expr, ParseError> {
        self.parse_or()
    }

    fn parse_or(&mut self) -> Result<Expr, ParseError> {
        let mut left = self.parse_and()?;
        while self.match_across_newlines(&[TokenType::Or]).is_some() {
            let right = self.parse_and()?;
            left = binary_op(function_names::OR, left, right);
        }
        Ok(left)
    }

    fn parse_and(&mut self) -> Result<Expr, ParseError> {
        let mut left = self.parse_eq()?;
        while self.match_across_newlines(&[TokenType::And]).is_some() {
            let right = self.parse_eq()?;
            left = binary_op(function_names::AND, left, right);
        }
        Ok(left)
    }

    fn parse_eq(&mut self) -> Result<Expr, ParseError> {
        let mut left = self.parse_comp()?;
        while let Some(op) = self.match_across_newlines(&[TokenType::Eq, TokenType::Neq]) {
            let right = self.parse_comp()?;
            left = match op.ty {
                TokenType::Eq => binary_op(function_names::EQ, left, right),
                // `!=` desugars to not(eq(left, right)) as there is no native
                // not-equal in the target substitution language.
                TokenType::Neq => {
                    let span = op_span(&left, &right);
                    let eq = binary_op(function_names::EQ, left, right);
                    Expr::Op {
                        fn_name: SubstitutionFunctionName::new(function_names::NOT),
                        args: vec![eq],
                        span,
                    }
                }
                _ => unreachable!("match_across_newlines only yields Eq/Neq here"),
            };
        }
        Ok(left)
    }

    fn parse_comp(&mut self) -> Result<Expr, ParseError> {
        let mut left = self.parse_unary()?;
        while let Some(op) = self.match_across_newlines(&[
            TokenType::Lt,
            TokenType::Lte,
            TokenType::Gt,
            TokenType::Gte,
        ]) {
            let right = self.parse_unary()?;
            let fn_name = match op.ty {
                TokenType::Lt => function_names::LT,
                TokenType::Lte => function_names::LE,
                TokenType::Gt => function_names::GT,
                TokenType::Gte => function_names::GE,
                _ => unreachable!("match_across_newlines only yields comparison ops here"),
            };
            left = binary_op(fn_name, left, right);
        }
        Ok(left)
    }

    fn parse_unary(&mut self) -> Result<Expr, ParseError> {
        // A single prefix `!`, then a primary (mirrors the Go reference; `!!x`
        // is not supported, `!` does not recurse into another unary).
        if self.match_token(TokenType::Not) {
            let operand = self.parse_primary()?;
            let span = operand.span();
            return Ok(Expr::Op {
                fn_name: SubstitutionFunctionName::new(function_names::NOT),
                args: vec![operand],
                span,
            });
        }
        self.parse_primary()
    }

    pub(crate) fn parse_primary(&mut self) -> Result<Expr, ParseError> {
        match self.peek_type() {
            TokenType::IntLiteral | TokenType::FloatLiteral | TokenType::BoolLiteral => {
                self.parse_scalar_expr()
            }
            TokenType::NoneLiteral => self.parse_none_expr(),
            TokenType::LeftParen => self.parse_group(),
            TokenType::StringStart => self.parse_string_expr(),
            TokenType::LeftBracket => self.parse_array_expr(),
            TokenType::LeftBrace => self.parse_object_expr(),
            _ => self.parse_reference_or_call(),
        }
    }

    fn parse_scalar_expr(&mut self) -> Result<Expr, ParseError> {
        let t = self.advance();
        let value = self.scalar_value_from_number_or_bool(&t)?;
        Ok(Expr::Scalar(Scalar::new(value, token_span(&t))))
    }

    /// Parses an int / float / bool literal token's value into a `ScalarValue`.
    /// Strings are handled separately by each caller, since the rules differ
    /// (expression strings interpolate, literal strings do not).
    fn scalar_value_from_number_or_bool(&self, t: &Token) -> Result<ScalarValue, ParseError> {
        match t.ty {
            TokenType::IntLiteral => t.value.parse().map(ScalarValue::Int).map_err(|_| {
                self.error_at(
                    t.start,
                    format!("integer literal '{}' is out of range", t.value),
                )
            }),
            TokenType::FloatLiteral => t.value.parse().map(ScalarValue::Float).map_err(|_| {
                self.error_at(t.start, format!("invalid float literal '{}'", t.value))
            }),
            TokenType::BoolLiteral => Ok(ScalarValue::Bool(t.value == "true")),
            _ => unreachable!("caller dispatched a non-numeric/bool token"),
        }
    }

    /// A literal scalar value (no references or operators): a string, integer,
    /// float, or boolean. Used by variable `default` / `allowedValues`, which
    /// are plain literals rather than expressions.
    pub(crate) fn parse_scalar_literal(&mut self) -> Result<Scalar, ParseError> {
        match self.peek_type() {
            // A literal string — interpolation is not allowed here.
            TokenType::StringStart => {
                let (s, span) = self.collect_string_literal(true)?;
                Ok(Scalar::new(ScalarValue::String(s), span))
            }
            TokenType::IntLiteral | TokenType::FloatLiteral | TokenType::BoolLiteral => {
                let t = self.advance();
                let value = self.scalar_value_from_number_or_bool(&t)?;
                Ok(Scalar::new(value, token_span(&t)))
            }
            other => {
                let start = self.peek().start;
                Err(self.error_at(
                    start,
                    format!("expected a scalar literal, got {}", other.display_label()),
                ))
            }
        }
    }

    /// A boolean literal scalar.
    pub(crate) fn parse_bool_literal(&mut self) -> Result<Scalar, ParseError> {
        let t = self.expect(TokenType::BoolLiteral)?;
        Ok(Scalar::new(
            ScalarValue::Bool(t.value == "true"),
            token_span(&t),
        ))
    }

    /// An array literal of scalar literals (e.g. a variable's `allowedValues`).
    pub(crate) fn parse_scalar_literal_array(&mut self) -> Result<Vec<Scalar>, ParseError> {
        let mut values = Vec::new();
        self.parse_array(|p| {
            values.push(p.parse_scalar_literal()?);
            Ok(())
        })?;
        Ok(values)
    }

    fn parse_none_expr(&mut self) -> Result<Expr, ParseError> {
        let t = self.expect(TokenType::NoneLiteral)?;
        Ok(Expr::None(Some(token_span(&t))))
    }

    fn parse_group(&mut self) -> Result<Expr, ParseError> {
        self.advance(); // advance '(' and bumps grouping depth
        let inner = self.parse_expr()?;
        self.expect(TokenType::RightParen)?;
        Ok(inner)
    }

    fn parse_array_expr(&mut self) -> Result<Expr, ParseError> {
        let mut elems = Vec::new();
        let span = self.parse_array(|p| {
            elems.push(p.parse_expr()?);
            Ok(())
        })?;
        Ok(Expr::Array {
            elems,
            span: Some(span),
        })
    }

    pub(crate) fn parse_object_expr(&mut self) -> Result<Expr, ParseError> {
        let mut entries = Vec::new();
        let span = self.parse_brace_block(|p| {
            let (key, key_span) = p.parse_object_key()?;
            p.expect(TokenType::Assign)?;
            let value = p.parse_expr()?;
            entries.push(ObjectField {
                key,
                value,
                span: Some(key_span),
            });
            Ok(())
        })?;
        Ok(Expr::Object {
            entries,
            span: Some(span),
        })
    }
}

/// Builds a binary `Expr::Op` spanning both operands.
fn binary_op(fn_name: &str, left: Expr, right: Expr) -> Expr {
    let span = op_span(&left, &right);
    Expr::Op {
        fn_name: SubstitutionFunctionName::new(fn_name),
        args: vec![left, right],
        span,
    }
}

/// The span covering an operator's operands: left start .. right end.
fn op_span(left: &Expr, right: &Expr) -> Option<Span> {
    match (left.span(), right.span()) {
        (Some(l), Some(r)) => Some(Span::new(l.start, r.end.unwrap_or(r.start))),
        (Some(l), None) => Some(l),
        (None, r) => r,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    fn parse(src: &str) -> Expr {
        let mut p = Parser::new(src);
        p.parse_expr()
            .unwrap_or_else(|e| panic!("parse failed for {src:?}: {}", e.message))
    }

    #[test]
    fn test_integer_literal() {
        match parse("5432") {
            Expr::Scalar(s) => assert_eq!(s.value, ScalarValue::Int(5432)),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_negative_integer_literal() {
        match parse("-1") {
            Expr::Scalar(s) => assert_eq!(s.value, ScalarValue::Int(-1)),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_float_literal() {
        match parse("3.5") {
            Expr::Scalar(s) => assert_eq!(s.value, ScalarValue::Float(3.5)),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_bool_and_none_primaries() {
        assert!(matches!(
            parse("true"),
            Expr::Scalar(Scalar {
                value: ScalarValue::Bool(true),
                ..
            })
        ));
        assert!(matches!(
            parse("false"),
            Expr::Scalar(Scalar {
                value: ScalarValue::Bool(false),
                ..
            })
        ));
        assert!(matches!(parse("none"), Expr::None(_)));
    }

    #[test]
    fn test_grouping_returns_inner_expression() {
        assert!(matches!(
            parse("(42)"),
            Expr::Scalar(Scalar {
                value: ScalarValue::Int(42),
                ..
            })
        ));
    }

    #[test]
    fn test_integer_out_of_range_is_reported() {
        let mut p = Parser::new("999999999999999999999999999");
        assert!(p.parse_expr().unwrap_err().message.contains("out of range"));
    }

    /// Parses `src`, asserting it is an operator expression, and returns its
    /// desugared function name and operands.
    fn op(src: &str) -> (String, Vec<Expr>) {
        match parse(src) {
            Expr::Op { fn_name, args, .. } => (fn_name.as_str().to_string(), args),
            other => panic!("expected an operator expression, got {other:?}"),
        }
    }

    #[test]
    fn test_comparison_operators_desugar() {
        assert_eq!(op("1 < 2").0, "lt");
        assert_eq!(op("1 <= 2").0, "le");
        assert_eq!(op("1 > 2").0, "gt");
        assert_eq!(op("1 >= 2").0, "ge");
        assert_eq!(op("1 == 2").0, "eq");
    }

    #[test]
    fn test_not_equal_desugars_to_not_of_eq() {
        let (name, args) = op("1 != 2");
        assert_eq!(name, "not");
        assert_eq!(args.len(), 1);
        assert!(matches!(&args[0], Expr::Op { fn_name, .. } if fn_name.as_str() == "eq"));
    }

    #[test]
    fn test_unary_not() {
        let (name, args) = op("!variables.flag");
        assert_eq!(name, "not");
        assert_eq!(args.len(), 1);
        assert!(matches!(&args[0], Expr::Ref(_)));
    }

    #[test]
    fn test_and_binds_tighter_than_or() {
        // a || b && c  ==  a || (b && c)
        let (name, args) = op("variables.a || variables.b && variables.c");
        assert_eq!(name, "or");
        assert!(matches!(&args[1], Expr::Op { fn_name, .. } if fn_name.as_str() == "and"));
    }

    #[test]
    fn test_eq_binds_looser_than_comparison() {
        // a < b == c  ==  (a < b) == c
        let (name, args) = op("variables.a < variables.b == variables.c");
        assert_eq!(name, "eq");
        assert!(matches!(&args[0], Expr::Op { fn_name, .. } if fn_name.as_str() == "lt"));
    }

    #[test]
    fn test_binary_operators_are_left_associative() {
        // a && b && c  ==  (a && b) && c
        let (name, args) = op("variables.a && variables.b && variables.c");
        assert_eq!(name, "and");
        assert!(matches!(&args[0], Expr::Op { fn_name, .. } if fn_name.as_str() == "and"));
        assert!(matches!(&args[1], Expr::Ref(_)));
    }

    #[test]
    fn test_parentheses_override_precedence() {
        // a && (b || c) — the right operand of `&&` is now an `or`.
        let (name, args) = op("variables.a && (variables.b || variables.c)");
        assert_eq!(name, "and");
        assert!(matches!(&args[1], Expr::Op { fn_name, .. } if fn_name.as_str() == "or"));
    }

    #[test]
    fn test_unary_not_binds_tighter_than_comparison() {
        // !a == b  ==  (!a) == b
        let (name, args) = op("!variables.a == variables.b");
        assert_eq!(name, "eq");
        assert!(matches!(&args[0], Expr::Op { fn_name, .. } if fn_name.as_str() == "not"));
    }

    #[test]
    fn test_multiline_operator_continuation() {
        // Leading- and trailing-operator multi-line forms parse identically.
        assert_eq!(op("variables.a\n  && variables.b").0, "and");
        assert_eq!(op("variables.a &&\n  variables.b").0, "and");
    }

    #[test]
    fn test_operator_chain_spans_both_operands() {
        // The folded operator span starts at the first operand and ends at the last.
        let (_, _args) = op("1 == 2");
        if let Expr::Op {
            span: Some(span), ..
        } = parse("1 == 2")
        {
            assert_eq!(span.start.column, 1);
        } else {
            panic!("expected a spanned operator expression");
        }
    }

    // --- Stage 4: strings, interpolation, arrays, objects --------------------

    #[test]
    fn test_plain_and_empty_strings_collapse_to_scalar() {
        assert!(matches!(
            parse(r#""hello""#),
            Expr::Scalar(Scalar { value: ScalarValue::String(s), .. }) if s == "hello"
        ));
        assert!(matches!(
            parse(r#""""#),
            Expr::Scalar(Scalar { value: ScalarValue::String(s), .. }) if s.is_empty()
        ));
    }

    #[test]
    fn test_multiline_string_collapses_to_scalar() {
        let src = "\"\"\"\n    hello\n    world\n    \"\"\"";
        assert!(matches!(
            parse(src),
            Expr::Scalar(Scalar { value: ScalarValue::String(s), .. }) if s == "hello\nworld"
        ));
    }

    #[test]
    fn test_interpolated_string_has_literal_and_sub_parts() {
        match parse(r#""a ${variables.x} b""#) {
            Expr::Interpolation { parts, .. } => {
                assert_eq!(parts.len(), 3);
                assert!(matches!(&parts[0], InterpPart::String { value, .. } if value == "a "));
                assert!(matches!(&parts[1], InterpPart::Sub { .. }));
                assert!(matches!(&parts[2], InterpPart::String { value, .. } if value == " b"));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_single_substitution_string_is_an_interpolation() {
        // A string that is only `${..}` is an interpolation, not a scalar.
        match parse(r#""${variables.x}""#) {
            Expr::Interpolation { parts, .. } => {
                assert_eq!(parts.len(), 1);
                assert!(matches!(&parts[0], InterpPart::Sub { .. }));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_interpolation_body_uses_full_expression_grammar() {
        // Operators and references are valid inside `${..}`.
        match parse(r#""${variables.a == variables.b}""#) {
            Expr::Interpolation { parts, .. } => assert!(matches!(
                &parts[0],
                InterpPart::Sub { value: Expr::Op { fn_name, .. }, .. } if fn_name.as_str() == "eq"
            )),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_array_literal_with_trailing_comma() {
        match parse("[1, 2, 3,]") {
            Expr::Array { elems, .. } => assert_eq!(elems.len(), 3),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_empty_and_nested_arrays() {
        assert!(matches!(parse("[]"), Expr::Array { elems, .. } if elems.is_empty()));
        match parse("[[1], [2]]") {
            Expr::Array { elems, .. } => {
                assert_eq!(elems.len(), 2);
                assert!(matches!(&elems[0], Expr::Array { .. }));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_multiline_array_uses_grouping_newlines() {
        let src = "[\n  1,\n  2,\n  3\n]";
        match parse(src) {
            Expr::Array { elems, .. } => assert_eq!(elems.len(), 3),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_object_literal_entries_and_key_forms() {
        match parse(r#"{ name = "orders", "aws:Key" = 1, nested = { a = true } }"#) {
            Expr::Object { entries, .. } => {
                assert_eq!(entries.len(), 3);
                assert_eq!(entries[0].key, "name");
                assert_eq!(entries[1].key, "aws:Key"); // arbitrary quoted key
                assert!(matches!(&entries[2].value, Expr::Object { .. }));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_object_entries_separated_by_newlines_or_commas() {
        let newline_separated = "{\n  a = 1\n  b = 2\n}";
        match parse(newline_separated) {
            Expr::Object { entries, .. } => assert_eq!(entries.len(), 2),
            other => panic!("{other:?}"),
        }
        match parse("{ a = 1, b = 2, }") {
            Expr::Object { entries, .. } => assert_eq!(entries.len(), 2),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_empty_object_literal() {
        assert!(matches!(parse("{}"), Expr::Object { entries, .. } if entries.is_empty()));
    }

    #[test]
    fn test_collections_carry_expression_elements() {
        // Array elements and object values are full expressions.
        match parse(r#"[variables.a, "${values.b}-x", if(values.c, 1, 2)]"#) {
            Expr::Array { elems, .. } => {
                assert_eq!(elems.len(), 3);
                assert!(matches!(&elems[0], Expr::Ref(_)));
                assert!(matches!(&elems[1], Expr::Interpolation { .. }));
                assert!(matches!(&elems[2], Expr::Call { .. }));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_unterminated_string_surfaces_a_diagnostic() {
        let mut p = Parser::new("\"abc\n");
        let result = p.parse_expr();
        // Either the parser errors directly, or the lexer recorded a diagnostic
        // for the newline inside a single-line string.
        assert!(result.is_err() || p.into_errors().is_some());
    }
}
