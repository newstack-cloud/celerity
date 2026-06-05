//! Data-source inline statements: `filter` statements and `export` statements
//! (`export *` and `export field [as alias]: type [ { block } ]`).

use super::core::{token_span, Parser};
use crate::errors::ParseError;
use crate::expr_transform::expr_to_string_or_substitutions;
use crate::parser::core::merge_end;
use crate::parser::expr::Expr;
use crate::scalar::{Scalar, ScalarValue};
use crate::schema::{
    DataSource, DataSourceExport, DataSourceExportMap, DataSourceFieldType, DataSourceMetadata,
    Filter, FilterOperator,
};
use crate::source::{Position, Span, Spanned};
use crate::{Blueprint, TokenType};
use std::collections::BTreeMap;

impl Parser {
    pub(crate) fn parse_data_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'data'
        let (name, name_span) = self.parse_element_name()?;
        self.expect(TokenType::Colon)?;
        let (type_str, type_span) = self.parse_element_type()?;

        let mut ds = DataSource {
            ds_type: Spanned::new(type_str, type_span),
            metadata: None,
            filters: vec![],
            exports: DataSourceExportMap::default(),
            description: None,
            span: Some(name_span),
            fields_source_meta: BTreeMap::new(),
        };
        self.parse_brace_block(|p| p.parse_data_decl_entry(&mut ds))?;
        bp.data_sources.insert(name, ds, Some(name_span));
        Ok(())
    }

    fn parse_data_decl_entry(&mut self, ds: &mut DataSource) -> Result<(), ParseError> {
        match self.peek_type() {
            TokenType::KeywordMetadata => self.parse_data_source_metadata_block(ds),
            TokenType::KeywordFilter => self.parse_filter_statement(ds),
            TokenType::KeywordExport => self.parse_data_source_export_statement(ds),
            TokenType::Identifier => self.parse_data_source_field_assignment(ds),
            _ => {
                let t = self.peek();
                let (start, label) = (t.start, t.ty.display_label());
                Err(self.error_at(start, format!(
                    "expected 'metadata', 'filter', 'export', or a field assignment in data declaration, got {label}")))
            }
        }
    }

    fn parse_data_source_field_assignment(
        &mut self,
        ds: &mut DataSource,
    ) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        match field.as_str() {
            "description" => {
                let sos = self.parse_interpolated_string()?;
                let end = sos.span.and_then(|s| s.end);
                ds.description = Some(sos);
                ds.fields_source_meta
                    .insert(field, merge_end(field_span, end));
            }
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in data declaration"),
                ))
            }
        }
        Ok(())
    }

    fn parse_data_source_metadata_block(&mut self, ds: &mut DataSource) -> Result<(), ParseError> {
        let mut meta = DataSourceMetadata::default();
        let span =
            self.parse_metadata_keyword_block(|p| p.parse_data_source_metadata_field(&mut meta))?;
        meta.span = Some(span);
        ds.fields_source_meta.insert("metadata".into(), span);
        ds.metadata = Some(meta);
        Ok(())
    }

    fn parse_data_source_metadata_field(
        &mut self,
        m: &mut DataSourceMetadata,
    ) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_key()?;
        match field.as_str() {
            "displayName" => m.display_name = Some(self.parse_display_name_field()?),
            "annotations" => m.annotations = Some(self.parse_annotations_field()?),
            "custom" => m.custom = Some(self.parse_custom_field()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in data source metadata"),
                ))
            }
        }
        m.fields_source_meta.insert(field, field_span);
        Ok(())
    }

    fn parse_filter_statement(&mut self, ds: &mut DataSource) -> Result<(), ParseError> {
        let open = self.advance(); // 'filter'
        let field = self.parse_filter_field()?;
        let operator = self.parse_filter_operator()?;

        let search_expr = self.parse_expr()?;
        let end = search_expr.span().and_then(|s| s.end).unwrap_or(open.end);
        // An array search becomes multiple values (for `in` / `not in`); any
        // other expression is a single value.
        let search = match search_expr {
            Expr::Array { elems, .. } => elems
                .into_iter()
                .map(expr_to_string_or_substitutions)
                .collect(),
            other => vec![expr_to_string_or_substitutions(other)],
        };

        ds.filters.push(Filter {
            field,
            operator,
            search,
            span: Some(Span::new(open.start, end)),
        });
        Ok(())
    }

    fn parse_filter_field(&mut self) -> Result<Scalar, ParseError> {
        if self.peek_type() != TokenType::StringStart {
            let t = self.peek();
            let (start, label) = (t.start, t.ty.display_label());
            return Err(self.error_at(
                start,
                format!("expected a string literal naming the filter field, got {label}"),
            ));
        }
        let (value, span) = self.collect_string_literal(false)?;
        Ok(Scalar::new(ScalarValue::String(value), span))
    }

    fn parse_filter_operator(&mut self) -> Result<Spanned<FilterOperator>, ParseError> {
        let start = self.peek().start;
        let negated = self.match_token(TokenType::KeywordNot);
        let (base, end) = self.parse_filter_operator_body(negated, start)?;
        let op = if negated {
            match base {
                FilterOperator::In => FilterOperator::NotIn,
                FilterOperator::HasKey => FilterOperator::NotHasKey,
                FilterOperator::Contains => FilterOperator::NotContains,
                FilterOperator::StartsWith => FilterOperator::NotStartsWith,
                FilterOperator::EndsWith => FilterOperator::NotEndsWith,
                _ => unreachable!(
                    "parse_filter_operator_body rejects 'not' before non-negatable operators"
                ),
            }
        } else {
            base
        };
        Ok(Spanned::new(op, Span::new(start, end)))
    }

    /// Reads the operator body, returning the non-negated form and its end. The
    /// word operators are negatable; the symbol operators are not (so `not` is
    /// rejected before them).
    fn parse_filter_operator_body(
        &mut self,
        negated: bool,
        not_pos: Position,
    ) -> Result<(FilterOperator, Position), ParseError> {
        match self.peek_type() {
            TokenType::KeywordIn => return Ok((FilterOperator::In, self.advance().end)),
            TokenType::KeywordContains => {
                return Ok((FilterOperator::Contains, self.advance().end))
            }
            TokenType::KeywordHas => {
                self.advance();
                let key = self.expect(TokenType::KeywordKey)?;
                return Ok((FilterOperator::HasKey, key.end));
            }
            TokenType::KeywordStarts => {
                self.advance();
                let with = self.expect(TokenType::KeywordWith)?;
                return Ok((FilterOperator::StartsWith, with.end));
            }
            TokenType::KeywordEnds => {
                self.advance();
                let with = self.expect(TokenType::KeywordWith)?;
                return Ok((FilterOperator::EndsWith, with.end));
            }
            _ => {}
        }
        if negated {
            return Err(self.error_at(not_pos,
                "'not' is only valid before 'in', 'has key', 'contains', 'starts with', or 'ends with'"));
        }
        // Symbol operators. `==` is the blueprint-language equals; it maps to the
        // blueprint schema model `=` (FilterOperator::Equals).
        let op = match self.peek_type() {
            TokenType::Eq => FilterOperator::Equals,
            TokenType::Neq => FilterOperator::NotEquals,
            TokenType::Gt => FilterOperator::GreaterThan,
            TokenType::Lt => FilterOperator::LessThan,
            TokenType::Gte => FilterOperator::GreaterThanOrEqual,
            TokenType::Lte => FilterOperator::LessThanOrEqual,
            _ => {
                let t = self.peek();
                let (start, label) = (t.start, t.ty.display_label());
                return Err(
                    self.error_at(start, format!("expected a filter operator, got {label}"))
                );
            }
        };
        Ok((op, self.advance().end))
    }

    fn parse_data_source_export_statement(
        &mut self,
        ds: &mut DataSource,
    ) -> Result<(), ParseError> {
        self.advance(); // 'export'

        // `export *`
        if self.peek_type() == TokenType::Star {
            self.advance();
            ds.exports.export_all = true;
            return Ok(());
        }

        let (source_name, name_span) = self.parse_element_name()?;
        // `export <source> as <exposed>`
        let (exposed_name, exposed_span, alias_for) = if self.peek_type() == TokenType::KeywordAs {
            self.advance();
            let (alias, alias_span) = self.parse_element_name()?;
            (
                alias,
                alias_span,
                Some(Scalar::new(ScalarValue::String(source_name), name_span)),
            )
        } else {
            (source_name, name_span, None)
        };

        self.expect(TokenType::Colon)?;
        let field_type = self.parse_data_source_field_type()?;

        let mut export = DataSourceExport {
            field_type,
            alias_for,
            description: None,
            span: Some(exposed_span),
        };
        // Optional `{ description = … }`.
        if self.peek_type() == TokenType::LeftBrace {
            self.parse_brace_block(|p| p.parse_data_source_export_field(&mut export))?;
        }
        ds.exports
            .exports
            .insert(exposed_name, export, Some(exposed_span));
        Ok(())
    }

    fn parse_data_source_export_field(
        &mut self,
        e: &mut DataSourceExport,
    ) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        match field.as_str() {
            "description" => e.description = Some(self.parse_interpolated_string()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in data source export"),
                ))
            }
        }
        Ok(())
    }

    fn parse_data_source_field_type(&mut self) -> Result<Spanned<DataSourceFieldType>, ParseError> {
        let field_type = match self.peek_type() {
            TokenType::KeywordString => DataSourceFieldType::String,
            TokenType::KeywordInteger => DataSourceFieldType::Integer,
            TokenType::KeywordFloat => DataSourceFieldType::Float,
            TokenType::KeywordBoolean => DataSourceFieldType::Boolean,
            TokenType::KeywordArray => DataSourceFieldType::Array,
            other => {
                let start = self.peek().start;
                return Err(self.error_at(start, format!(
                    "expected a data source export type (string, integer, float, boolean, or array), got {}",
                    other.display_label())));
            }
        };
        let t = self.advance();
        Ok(Spanned::new(field_type, token_span(&t)))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    fn parse(src: &str) -> Blueprint {
        crate::parse_string(src).unwrap_or_else(|e| panic!("parse failed:\n{e}"))
    }

    #[test]
    fn test_data_source_full() {
        let bp = parse(
            r#"version "2025-11-02"
data network: aws/vpc {
    description = "Network source"
    metadata { displayName = "Network" }
    filter "state" == "available"
    filter "tags" has key variables.environment
    filter "type" not in ["a", "b"]
    export subnets: array
    export vpcId as vpc: string
    export securityGroups: array {
        description = "the security groups"
    }
}"#,
        );
        let ds = bp.data_sources.get("network").unwrap();
        assert_eq!(ds.ds_type.value, "aws/vpc");
        assert!(ds.description.is_some() && ds.metadata.is_some());

        assert_eq!(ds.filters.len(), 3);
        assert!(matches!(
            ds.filters[0].operator.value,
            FilterOperator::Equals
        ));
        assert!(matches!(
            ds.filters[1].operator.value,
            FilterOperator::HasKey
        ));
        assert!(matches!(
            ds.filters[2].operator.value,
            FilterOperator::NotIn
        ));
        assert_eq!(ds.filters[2].search.len(), 2); // array search

        assert_eq!(ds.exports.exports.len(), 3);
        let vpc = ds.exports.exports.get("vpc").unwrap();
        assert_eq!(
            vpc.alias_for.as_ref().unwrap().value,
            ScalarValue::String("vpcId".into())
        );
        assert!(matches!(vpc.field_type.value, DataSourceFieldType::String));
        assert!(ds
            .exports
            .exports
            .get("securityGroups")
            .unwrap()
            .description
            .is_some());
    }

    #[test]
    fn test_export_wildcard() {
        let bp = parse("version \"2025-11-02\"\ndata d: x/y { export * }");
        let exports = &bp.data_sources.get("d").unwrap().exports;
        assert!(exports.export_all);
        assert!(exports.exports.is_empty());
    }

    #[test]
    fn test_all_filter_operators_parse() {
        use FilterOperator::*;
        let cases = [
            ("\"f\" == 1", Equals),
            ("\"f\" != 1", NotEquals),
            ("\"f\" > 1", GreaterThan),
            ("\"f\" < 1", LessThan),
            ("\"f\" >= 1", GreaterThanOrEqual),
            ("\"f\" <= 1", LessThanOrEqual),
            ("\"f\" in [1]", In),
            ("\"f\" not in [1]", NotIn),
            ("\"f\" has key variables.x", HasKey),
            ("\"f\" not has key variables.x", NotHasKey),
            ("\"f\" contains \"a\"", Contains),
            ("\"f\" not contains \"a\"", NotContains),
            ("\"f\" starts with \"a\"", StartsWith),
            ("\"f\" not starts with \"a\"", NotStartsWith),
            ("\"f\" ends with \"a\"", EndsWith),
            ("\"f\" not ends with \"a\"", NotEndsWith),
        ];
        for (filter, expected) in cases {
            let bp = parse(&format!(
                "version \"2025-11-02\"\ndata d: x/y {{ filter {filter} }}"
            ));
            let op = bp.data_sources.get("d").unwrap().filters[0].operator.value;
            assert_eq!(op, expected, "for filter {filter:?}");
        }
    }

    #[test]
    fn test_not_before_a_symbol_operator_is_rejected() {
        let err =
            crate::parse_string("version \"2025-11-02\"\ndata d: x/y { filter \"f\" not == 1 }")
                .unwrap_err();
        assert!(err.to_string().contains("'not' is only valid before"));
    }

    #[test]
    fn test_filter_field_must_be_a_string_literal() {
        let err = crate::parse_string("version \"2025-11-02\"\ndata d: x/y { filter state == 1 }")
            .unwrap_err();
        assert!(err
            .to_string()
            .contains("string literal naming the filter field"));
    }
}
