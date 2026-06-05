//! Reference parsing (all reference forms) plus function calls and accessor
//! chains (`.name`, `["name"]`, `[n]`, `[]`).

use super::core::{token_span, Parser};
use super::expr::{CallArg, Expr};
use crate::errors::ParseError;
use crate::source::{Position, Span};
use crate::substitution::{
    Substitution, SubstitutionChild, SubstitutionDataSourceProperty, SubstitutionElemReference,
    SubstitutionFunctionName, SubstitutionKind, SubstitutionPathItem, SubstitutionResourceProperty,
    SubstitutionValueReference, SubstitutionVariable,
};
use crate::tokens::TokenType;

impl Parser {
    pub(crate) fn parse_reference_or_call(&mut self) -> Result<Expr, ParseError> {
        match self.peek_type() {
            TokenType::KeywordVariables => self.parse_variable_reference(),
            TokenType::KeywordValues => self.parse_value_reference(),
            TokenType::KeywordResources => self.parse_resource_reference(),
            TokenType::KeywordDatasources => self.parse_data_source_reference(),
            TokenType::KeywordChildren => self.parse_child_reference(),
            TokenType::KeywordElem => self.parse_elem_reference(),
            TokenType::KeywordI => self.parse_elem_index_reference(),
            TokenType::Identifier => {
                if self.peek_at(1).ty == TokenType::LeftParen {
                    self.parse_function_call()
                } else {
                    self.parse_bare_resource_reference()
                }
            }
            ty if ty.is_keyword() && self.peek_at(1).ty == TokenType::LeftParen => {
                // core funcs sharing a keyword name (object, list, …)
                self.parse_function_call()
            }
            _ => {
                let tkn = self.peek();
                let start = tkn.start;
                let label = tkn.ty.display_label();
                Err(self.error_at(start, format!("expected an expression, got {}", label)))
            }
        }
    }

    fn parse_variable_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'variables'
        let (name, name_span) = self.parse_name_accessor()?;
        // variables resolve to scalars: no further accessors.
        if matches!(self.peek_type(), TokenType::Period | TokenType::LeftBracket) {
            let tkn = self.peek();
            let start = tkn.start;
            let label = tkn.ty.display_label();
            return Err(self.error_at(
                start,
                format!(
                    "variables resolve to scalars and cannot have nested values, got {}",
                    label
                ),
            ));
        }
        let span = Span::new(keyword.start, span_end(&name_span));
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::Variable(SubstitutionVariable {
                variable_name: name,
            }),
            span,
        )))
    }

    fn parse_value_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'values'
        let (name, name_span) = self.parse_name_accessor()?;
        let path = self.parse_accessor_chain()?;
        let span = Span::new(keyword.start, path_end(&path, &name_span));
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::ValueReference(SubstitutionValueReference {
                value_name: name,
                path,
            }),
            span,
        )))
    }

    fn parse_data_source_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'datasources'
        let (ds_name, _) = self.parse_name_accessor()?;
        let (field_name, field_span) = self.parse_name_accessor()?;
        // Optional index — but only the [N] / [] form, not ["name"].
        let mut primitive_arr_index = None;
        let mut end = span_end(&field_span);
        if self.peek_type() == TokenType::LeftBracket {
            let item = self.parse_bracket_accessor()?;
            match item {
                SubstitutionPathItem::Index { index, span } => {
                    primitive_arr_index = Some(index);
                    end = span_end_opt(span);
                }
                SubstitutionPathItem::Field { span, .. } => return Err(self.error_at(
                    span_start_opt(span),
                    "data source reference accepts only an index accessor here, not a quoted name",
                )),
            }
        }
        if matches!(self.peek_type(), TokenType::Period | TokenType::LeftBracket) {
            let tkn = self.peek();
            let start = tkn.start;
            let label = tkn.ty.display_label();
            return Err(self.error_at(
                start,
                format!(
                    "data source references end after the field name (and optional index), got {}",
                    label
                ),
            ));
        }
        let span = Span::new(keyword.start, end);
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::DataSourceProperty(SubstitutionDataSourceProperty {
                data_source_name: ds_name,
                field_name,
                primitive_arr_index,
            }),
            span,
        )))
    }

    fn parse_resource_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'resources'
        let (name, name_span) = self.parse_name_accessor()?;
        self.finish_resource_reference(keyword.start, name, name_span)
    }

    fn parse_bare_resource_reference(&mut self) -> Result<Expr, ParseError> {
        let name_tkn = self.expect(TokenType::Identifier)?;
        let span = token_span(&name_tkn);
        self.finish_resource_reference(name_tkn.start, name_tkn.value.clone(), span)
    }

    fn finish_resource_reference(
        &mut self,
        start: Position,
        resource_name: String,
        name_span: Span,
    ) -> Result<Expr, ParseError> {
        let each = self.parse_optional_each_index()?; // Option<(i64, Span)>
        let path = self.parse_accessor_chain()?;
        let mut end = span_end(&name_span);
        if let Some((_, ref s)) = each {
            end = span_end(s);
        }
        end = path_end(&path, &Span::new(start, end)); // last accessor wins
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::ResourceProperty(SubstitutionResourceProperty {
                resource_name,
                resource_each_template_index: each.map(|(i, _)| i),
                path,
            }),
            Span::new(start, end),
        )))
    }

    fn parse_child_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'children'
        let (name, name_span) = self.parse_name_accessor()?;
        let path = self.parse_accessor_chain()?;
        let span = Span::new(keyword.start, path_end(&path, &name_span));
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::Child(SubstitutionChild {
                child_name: name,
                path,
            }),
            span,
        )))
    }

    fn parse_elem_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'elem'
        let keyword_span = token_span(&keyword);
        let path = self.parse_accessor_chain()?;
        let span = Span::new(
            keyword.start,
            path_end(
                &path,
                &Span::new(keyword.start, path_end(&path, &keyword_span)),
            ),
        );
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::ElemReference(SubstitutionElemReference { path }),
            span,
        )))
    }

    fn parse_elem_index_reference(&mut self) -> Result<Expr, ParseError> {
        let keyword = self.advance(); // 'i'
        let keyword_span = token_span(&keyword);
        Ok(Expr::Ref(Substitution::new(
            SubstitutionKind::ElemIndexReference,
            keyword_span,
        )))
    }

    /// A single leading name accessor: `.name` (plain ident) or `["name"]`.
    fn parse_name_accessor(&mut self) -> Result<(String, Span), ParseError> {
        match self.peek_type() {
            TokenType::Period => {
                let dot = self.advance();
                let name = self.expect(TokenType::Identifier)?;
                Ok((name.value.clone(), Span::new(dot.start, name.end)))
            }
            TokenType::LeftBracket => {
                let open = self.advance();
                let (name, _) = self.parse_quoted_name()?;
                let close = self.expect(TokenType::RightBracket)?;
                Ok((name, Span::new(open.start, close.end)))
            }
            _ => {
                let tkn = self.peek();
                let start = tkn.start;
                let label = tkn.ty.display_label();
                Err(self.error_at(
                    start,
                    format!("expected '.<name>' or '[\"<name>\"]', got {}", label),
                ))
            }
        }
    }

    /// Zero or more `.name` / `["name"]` / `[n]` / `[]` accessors.
    fn parse_accessor_chain(&mut self) -> Result<Vec<SubstitutionPathItem>, ParseError> {
        let mut path = Vec::new();
        loop {
            let item = match self.peek_type() {
                TokenType::Period => self.parse_dot_accessor()?,
                TokenType::LeftBracket => self.parse_bracket_accessor()?,
                _ => return Ok(path),
            };
            path.push(item);
        }
    }

    fn parse_dot_accessor(&mut self) -> Result<SubstitutionPathItem, ParseError> {
        let dot = self.advance();
        let ty = self.peek_type();
        if ty != TokenType::Identifier && !ty.is_keyword() {
            // chain dot allows keyword field names
            let tkn = self.peek();
            let start = tkn.start;
            let label = tkn.ty.display_label();
            return Err(self.error_at(start, format!("expected a field name, got {}", label)));
        }
        let name = self.advance();
        Ok(SubstitutionPathItem::Field {
            name: name.value.clone(),
            span: Some(Span::new(dot.start, name.end)),
        })
    }

    /// `["name"]` → Field, `[n]` → Index, `[]` → Index(0).
    fn parse_bracket_accessor(&mut self) -> Result<SubstitutionPathItem, ParseError> {
        let open = self.advance();
        let item = match self.peek_type() {
            TokenType::StringStart => {
                let (name, _) = self.parse_quoted_name()?;
                let close = self.expect(TokenType::RightBracket)?;
                return Ok(SubstitutionPathItem::Field {
                    name,
                    span: Some(Span::new(open.start, close.end)),
                });
            }
            TokenType::IntLiteral => {
                let int_tkn = self.advance();
                let index: i64 = int_tkn.value.parse().map_err(|_| {
                    self.error_at(
                        int_tkn.start,
                        format!("array index '{}' is too large", int_tkn.value),
                    )
                })?;
                if index < 0 {
                    return Err(self.error_at(
                        int_tkn.start,
                        format!("array index must be non-negative, got {index}"),
                    ));
                }
                index
            }
            TokenType::RightBracket => 0, // `[]` == `[0]`
            _ => {
                let tkn = self.peek();
                let start = tkn.start;
                let label = tkn.ty.display_label();
                return Err(self.error_at(
                    start,
                    format!("expected a quoted name, an index or ']', got {}", label),
                ));
            }
        };
        let close = self.expect(TokenType::RightBracket)?;
        Ok(SubstitutionPathItem::Index {
            index: item,
            span: Some(Span::new(open.start, close.end)),
        })
    }

    /// An each-template index `[n]` straight after a resource name.
    fn parse_optional_each_index(&mut self) -> Result<Option<(i64, Span)>, ParseError> {
        if self.peek_type() != TokenType::LeftBracket
            || self.peek_at(1).ty == TokenType::StringStart
        {
            return Ok(None);
        }

        match self.parse_bracket_accessor()? {
            SubstitutionPathItem::Index { index, span } => Ok(Some((
                index,
                span.unwrap_or_else(|| Span::at(Position::new(1, 1))),
            ))),
            // unreachable: we excluded the StringStart (Field) case above.
            SubstitutionPathItem::Field { span, .. } => {
                Err(self.error_at(span_start_opt(span), "expected an index"))
            }
        }
    }

    fn parse_function_call(&mut self) -> Result<Expr, ParseError> {
        let name_tkn = self.peek().clone();
        if name_tkn.ty != TokenType::Identifier && !name_tkn.ty.is_keyword() {
            return Err(self.error_at(
                name_tkn.start,
                format!(
                    "expected a function name, got {}",
                    name_tkn.ty.display_label()
                ),
            ));
        }

        self.advance();
        self.expect(TokenType::LeftParen)?;
        let args = self.parse_call_args()?;
        let close = self.expect(TokenType::RightParen)?;
        let path = self.parse_accessor_chain()?;
        let end = path_end(&path, &Span::new(name_tkn.start, close.end));
        Ok(Expr::Call {
            name: SubstitutionFunctionName::new(name_tkn.value),
            args,
            path,
            span: Some(Span::new(name_tkn.start, end)),
        })
    }

    fn parse_call_args(&mut self) -> Result<Vec<CallArg>, ParseError> {
        let mut args = Vec::new();
        if self.peek_type() == TokenType::RightParen {
            return Ok(args);
        }
        loop {
            args.push(self.parse_call_arg()?);
            if !self.match_token(TokenType::Comma) {
                return Ok(args);
            }
            if self.peek_type() == TokenType::RightParen {
                return Ok(args);
            } // trailing comma
        }
    }

    fn parse_call_arg(&mut self) -> Result<CallArg, ParseError> {
        // `name = expr` (named) vs `expr` (positional): one-token lookahead for '='.
        if self.peek_type() == TokenType::Identifier && self.peek_at(1).ty == TokenType::Assign {
            let name = self.advance();
            self.advance(); // '='
            let value = self.parse_expr()?;
            let span = value.span().map(|v| Span::new(name.start, span_end(&v)));
            return Ok(CallArg {
                name: Some(name.value.clone()),
                value,
                span,
            });
        }

        let value = self.parse_expr()?;
        let span = value.span();
        Ok(CallArg {
            name: None,
            value,
            span,
        })
    }
}

fn span_end(s: &Span) -> Position {
    s.end.unwrap_or(s.start)
}

fn span_end_opt(s: Option<Span>) -> Position {
    s.map(|s| span_end(&s)).unwrap_or(Position::new(1, 1))
}

fn span_start_opt(s: Option<Span>) -> Position {
    s.map(|s| s.start).unwrap_or(Position::new(1, 1))
}

fn path_end(path: &[SubstitutionPathItem], fallback: &Span) -> Position {
    path.last()
        .and_then(|i| i.span())
        .map(|s| span_end(&s))
        .unwrap_or(span_end(fallback))
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

    fn parse_err(src: &str) -> String {
        let mut p = Parser::new(src);
        p.parse_expr().expect_err("expected a parse error").message
    }

    /// Parses `src`, asserting it is a reference, and returns its kind.
    fn ref_kind(src: &str) -> SubstitutionKind {
        match parse(src) {
            Expr::Ref(sub) => sub.kind,
            other => panic!("expected a reference, got {other:?}"),
        }
    }

    #[test]
    fn test_variable_reference_bare_and_quoted() {
        match ref_kind("variables.region") {
            SubstitutionKind::Variable(v) => assert_eq!(v.variable_name, "region"),
            other => panic!("{other:?}"),
        }
        match ref_kind("variables[\"primary.region\"]") {
            SubstitutionKind::Variable(v) => assert_eq!(v.variable_name, "primary.region"),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_variable_drill_in_is_rejected() {
        assert!(parse_err("variables.x.y").contains("scalars"));
    }

    #[test]
    fn test_value_reference_with_path() {
        match ref_kind("values.cacheConfig.hosts") {
            SubstitutionKind::ValueReference(v) => {
                assert_eq!(v.value_name, "cacheConfig");
                assert_eq!(v.path.len(), 1); // .hosts
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_value_index_shorthand_reads_first_element() {
        match ref_kind("values.hosts[]") {
            SubstitutionKind::ValueReference(v) => assert!(matches!(
                v.path.last(),
                Some(SubstitutionPathItem::Index { index: 0, .. })
            )),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_resources_prefix_with_each_index_and_path() {
        match ref_kind("resources.bucket[0].spec.name") {
            SubstitutionKind::ResourceProperty(r) => {
                assert_eq!(r.resource_name, "bucket");
                assert_eq!(r.resource_each_template_index, Some(0));
                assert_eq!(r.path.len(), 2); // .spec, .name
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_bare_resource_reference_with_path() {
        match ref_kind("saveOrder.spec.functionArn") {
            SubstitutionKind::ResourceProperty(r) => {
                assert_eq!(r.resource_name, "saveOrder");
                assert_eq!(r.resource_each_template_index, None);
                assert_eq!(r.path.len(), 2);
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_resource_quoted_accessor_is_not_an_each_index() {
        // `["a.b"]` straight after the name is a field accessor, not an each-index.
        match ref_kind("resources.foo[\"a.b\"].spec") {
            SubstitutionKind::ResourceProperty(r) => {
                assert_eq!(r.resource_each_template_index, None);
                assert_eq!(r.path.len(), 2); // ["a.b"], .spec
                assert!(matches!(
                    &r.path[0],
                    SubstitutionPathItem::Field { name, .. } if name == "a.b"
                ));
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_datasource_reference_with_and_without_index() {
        match ref_kind("datasources.network.subnets[0]") {
            SubstitutionKind::DataSourceProperty(d) => {
                assert_eq!(d.data_source_name, "network");
                assert_eq!(d.field_name, "subnets");
                assert_eq!(d.primitive_arr_index, Some(0));
            }
            other => panic!("{other:?}"),
        }
        // `[]` shorthand is index 0.
        match ref_kind("datasources.network.subnets[]") {
            SubstitutionKind::DataSourceProperty(d) => assert_eq!(d.primitive_arr_index, Some(0)),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_datasource_rejects_extra_accessor_and_quoted_index() {
        assert!(parse_err("datasources.network.subnets.extra").contains("end after the field name"));
        assert!(parse_err("datasources.network.subnets[\"x\"]").contains("only an index accessor"));
    }

    #[test]
    fn test_child_reference() {
        match ref_kind("children.coreInfra.databasePassword") {
            SubstitutionKind::Child(c) => {
                assert_eq!(c.child_name, "coreInfra");
                assert_eq!(c.path.len(), 1);
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_elem_and_elem_index_references() {
        match ref_kind("elem.name") {
            SubstitutionKind::ElemReference(e) => assert_eq!(e.path.len(), 1),
            other => panic!("{other:?}"),
        }
        assert!(matches!(
            ref_kind("i"),
            SubstitutionKind::ElemIndexReference
        ));
    }

    #[test]
    fn test_negative_array_index_is_rejected() {
        assert!(parse_err("values.hosts[-1]").contains("non-negative"));
    }

    #[test]
    fn test_function_call_positional_args() {
        match parse("max(variables.a, variables.b)") {
            Expr::Call {
                name, args, path, ..
            } => {
                assert_eq!(name.as_str(), "max");
                assert_eq!(args.len(), 2);
                assert!(args.iter().all(|a| a.name.is_none()));
                assert!(path.is_empty());
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_function_call_named_arg_and_result_accessor() {
        // `object` is a keyword usable as a function name; named arg + accessor.
        match parse("object(host = values.h).host") {
            Expr::Call {
                name, args, path, ..
            } => {
                assert_eq!(name.as_str(), "object");
                assert_eq!(args.len(), 1);
                assert_eq!(args[0].name.as_deref(), Some("host"));
                assert_eq!(path.len(), 1); // .host on the result
            }
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_function_call_empty_and_trailing_comma() {
        match parse("cwd()") {
            Expr::Call { args, .. } => assert!(args.is_empty()),
            other => panic!("{other:?}"),
        }
        assert!(matches!(parse("list(variables.a,)"), Expr::Call { .. }));
    }

    #[test]
    fn test_unexpected_token_is_an_error() {
        assert!(parse_err(",").contains("expected an expression"));
    }
}
