//! Top-level declaration dispatch and the per-kind declaration parsers
//! (variable, value, data, resource, include, export, version, transform,
//! metadata).

use std::collections::BTreeMap;

use super::core::{token_span, Parser};
use crate::expr_transform::expr_to_mapping_node;
use crate::mapping::MappingNode;
use crate::parser::expr::Expr;
use crate::schema::{Export, ExportType, Include, Value, ValueType, Variable, VariableType};
use crate::substitution::substitution_to_path_string;
use crate::{
    errors::ParseError,
    scalar::{Scalar, ScalarValue},
    source::{Position, Spanned},
    Blueprint, TokenType,
};

impl Parser {
    pub(crate) fn parse(&mut self) -> Blueprint {
        let mut bp = Blueprint::default();
        loop {
            self.skip_newlines();
            if self.peek_type() == TokenType::Eof {
                break;
            }
            if let Err(err) = self.parse_top_level_item(&mut bp) {
                self.add_error(err);
                self.synchronise();
            }
        }
        if bp.version.is_none() {
            self.add_error(ParseError::message_only(
                "missing required 'version' directive",
            ));
        }
        bp
    }

    fn parse_top_level_item(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        match self.peek_type() {
            TokenType::KeywordVersion => self.parse_version_directive(bp),
            TokenType::KeywordTransform => self.parse_transform_directive(bp),
            TokenType::KeywordVariable => self.parse_variable_decl(bp),
            TokenType::KeywordValue => self.parse_value_decl(bp),
            TokenType::KeywordResource => self.parse_resource_decl(bp),
            TokenType::KeywordData => self.parse_data_decl(bp),
            TokenType::KeywordInclude => self.parse_include_decl(bp),
            TokenType::KeywordMetadata => self.parse_metadata_block(bp),
            TokenType::KeywordExport => self.parse_export_decl(bp),
            _ => {
                let t = self.peek();
                let (start, label) = (t.start, t.ty.display_label());
                Err(self.error_at(start, format!("unexpected {label} at top level")))
            }
        }
    }

    fn parse_version_directive(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        let kw = self.advance(); // 'version'
        let (value, span) = self.parse_plain_string_literal()?;
        if bp.version.is_some() {
            return Err(self.error_at(kw.start, "version directive already declared"));
        }
        bp.version = Some(Scalar::new(ScalarValue::String(value), span));
        Ok(())
    }

    fn parse_transform_directive(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        let kw = self.advance(); // 'transform'
        if !bp.transform.is_empty() {
            return Err(self.error_at(kw.start, "transform directive already declared"));
        }

        match self.peek_type() {
            TokenType::StringStart => {
                let (v, span) = self.parse_plain_string_literal()?;
                bp.transform.push(Spanned::new(v, span));
            }
            TokenType::LeftBracket => {
                self.advance(); // '['  (bumps grouping depth → newlines insignificant)
                while self.peek_type() != TokenType::RightBracket {
                    let (v, span) = self.parse_plain_string_literal()?;
                    bp.transform.push(Spanned::new(v, span));
                    if !self.match_token(TokenType::Comma) {
                        break;
                    }
                }
                self.expect(TokenType::RightBracket)?;
                if bp.transform.is_empty() {
                    return Err(self.error_at(kw.start, "expected at least one transform in list"));
                }
            }
            _ => {
                let tkn = self.peek();
                let start = tkn.start;
                let label = tkn.ty.display_label();
                return Err(self.error_at(
                    start,
                    format!("expected a string literal for transform, got {}", label),
                ));
            }
        }
        Ok(())
    }

    fn parse_metadata_block(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        let kw = self.advance(); // 'metadata'
        if bp.metadata.is_some() {
            return Err(self.error_at(kw.start, "top-level 'metadata' block already declared"));
        }
        bp.metadata = Some(self.parse_free_form_map_block()?);
        Ok(())
    }

    fn parse_variable_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'variable'
        let (name, name_span) = self.parse_element_name()?;
        self.expect(TokenType::Colon)?;
        let var_type = self.parse_variable_type()?;

        let mut variable = Variable {
            var_type,
            description: None,
            secret: None,
            default: None,
            allowed_values: vec![],
            span: Some(name_span),
        };
        self.parse_brace_block(|p| p.parse_variable_field(&mut variable))?;
        bp.variables.insert(name, variable, Some(name_span));
        Ok(())
    }

    /// A variable type: one of the four scalar keywords, or a custom element type.
    fn parse_variable_type(&mut self) -> Result<Spanned<VariableType>, ParseError> {
        match self.peek_type() {
            TokenType::KeywordString
            | TokenType::KeywordInteger
            | TokenType::KeywordFloat
            | TokenType::KeywordBoolean => {
                let t = self.advance();
                Ok(Spanned::new(
                    VariableType::from(t.value.clone()),
                    token_span(&t),
                ))
            }
            _ => {
                let (ty, span) = self.parse_element_type()?;
                Ok(Spanned::new(VariableType::from(ty), span))
            }
        }
    }

    fn parse_variable_field(&mut self, v: &mut Variable) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        match field.as_str() {
            "default" => v.default = Some(self.parse_scalar_literal()?),
            "description" => {
                // A variable description is a (possibly multi-line) string
                // literal with no interpolation, unlike the interpolated
                // descriptions on values, includes, and exports. A non-string
                // value such as an integer is rejected here.
                let (text, span) = self.collect_string_literal(true)?;
                v.description = Some(Scalar::new(ScalarValue::String(text), span));
            }
            "secret" => v.secret = Some(self.parse_bool_literal()?),
            "allowedValues" => {
                if matches!(v.var_type.value, VariableType::Boolean) {
                    return Err(self.error_at(
                        field_span.start,
                        "'allowedValues' is not valid on a boolean variable",
                    ));
                }
                v.allowed_values = self.parse_scalar_literal_array()?;
            }
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in variable declaration"),
                ));
            }
        }
        Ok(())
    }

    fn parse_value_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'value'
        let (name, name_span) = self.parse_element_name()?;
        self.expect(TokenType::Colon)?;
        let value_type = self.parse_value_type()?;

        let mut value = Value {
            value_type,
            value: MappingNode::None,
            description: None,
            secret: None,
            span: Some(name_span),
        };
        self.parse_brace_block(|p| p.parse_value_field(&mut value))?;
        bp.values.insert(name, value, Some(name_span));
        Ok(())
    }

    fn parse_value_type(&mut self) -> Result<Spanned<ValueType>, ParseError> {
        let value_type = match self.peek_type() {
            TokenType::KeywordString => ValueType::String,
            TokenType::KeywordInteger => ValueType::Integer,
            TokenType::KeywordFloat => ValueType::Float,
            TokenType::KeywordBoolean => ValueType::Boolean,
            TokenType::KeywordArray => ValueType::Array,
            TokenType::KeywordObject => ValueType::Object,
            other => {
                let start = self.peek().start;
                return Err(self.error_at(
                    start,
                    format!("expected a value type, got {}", other.display_label()),
                ));
            }
        };
        let t = self.advance();
        Ok(Spanned::new(value_type, token_span(&t)))
    }

    fn parse_value_field(&mut self, v: &mut Value) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        match field.as_str() {
            "value" => {
                let e = self.parse_expr()?;
                v.value = expr_to_mapping_node(e);
            }
            "description" => v.description = Some(self.parse_interpolated_string()?),
            "secret" => v.secret = Some(self.parse_bool_literal()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in value declaration"),
                ));
            }
        }
        Ok(())
    }

    fn parse_export_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'export'
        let (name, name_span) = self.parse_element_name()?;
        self.expect(TokenType::Colon)?;
        let export_type = self.parse_export_type()?;

        let mut export = Export {
            export_type,
            field: Scalar::untracked(ScalarValue::String(String::new())),
            description: None,
            span: Some(name_span),
        };
        self.parse_brace_block(|p| p.parse_export_field(&mut export))?;
        bp.exports.insert(name, export, Some(name_span));
        Ok(())
    }

    fn parse_export_type(&mut self) -> Result<Spanned<ExportType>, ParseError> {
        let export_type = match self.peek_type() {
            TokenType::KeywordString => ExportType::String,
            TokenType::KeywordInteger => ExportType::Integer,
            TokenType::KeywordFloat => ExportType::Float,
            TokenType::KeywordBoolean => ExportType::Boolean,
            TokenType::KeywordArray => ExportType::Array,
            TokenType::KeywordObject => ExportType::Object,
            other => {
                let start = self.peek().start;
                return Err(self.error_at(
                    start,
                    format!("expected an export type, got {}", other.display_label()),
                ));
            }
        };
        let t = self.advance();
        Ok(Spanned::new(export_type, token_span(&t)))
    }

    fn parse_export_field(&mut self, e: &mut Export) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        match field.as_str() {
            "field" => e.field = self.parse_export_field_value()?,
            "description" => e.description = Some(self.parse_interpolated_string()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in export declaration"),
                ));
            }
        }
        Ok(())
    }

    /// An export `field`: a string literal, or a bare reference path normalised
    /// to its canonical string form.
    fn parse_export_field_value(&mut self) -> Result<Scalar, ParseError> {
        if self.peek_type() == TokenType::StringStart {
            let (value, span) = self.collect_string_literal(false)?;
            return Ok(Scalar::new(ScalarValue::String(value), span));
        }

        let e = self.parse_reference_or_call()?;
        let pos = e.span().map(|s| s.start).unwrap_or(Position::new(1, 1));
        let path = match &e {
            Expr::Ref(sub) => substitution_to_path_string(sub),
            _ => None,
        };
        match (e, path) {
            (Expr::Ref(sub), Some(path)) => Ok(match sub.span {
                Some(span) => Scalar::new(ScalarValue::String(path), span),
                None => Scalar::untracked(ScalarValue::String(path)),
            }),
            _ => Err(self.error_at(
                pos,
                "export 'field' must be a reference path, not a function call or computed expression",
            )),
        }
    }

    pub(crate) fn parse_include_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'include'
        let (name, name_span) = self.parse_element_name()?;
        // positional path string (no '=')
        let path = self.parse_interpolated_string()?;

        let mut include = Include {
            path,
            variables: None,
            metadata: None,
            description: None,
            span: Some(name_span),
            fields_source_meta: BTreeMap::new(),
        };
        self.parse_brace_block(|p| p.parse_include_field(&mut include))?;
        bp.include.insert(name, include, Some(name_span));
        Ok(())
    }

    fn parse_include_field(&mut self, inc: &mut Include) -> Result<(), ParseError> {
        // Note: `variables { … }` / `metadata { … }` are keyword-introduced blocks
        // (no `=`); only `description =` uses an assignment.
        let (field, field_span) = self.parse_field_key()?;
        match field.as_str() {
            "description" => {
                self.expect(TokenType::Assign)?;
                inc.description = Some(self.parse_interpolated_string()?);
            }
            "variables" => inc.variables = Some(self.parse_free_form_map_block()?),
            "metadata" => inc.metadata = Some(self.parse_free_form_map_block()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in include declaration"),
                ))
            }
        }
        inc.fields_source_meta.insert(field, field_span);
        Ok(())
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
    fn test_version_is_required_and_unique() {
        assert!(crate::parse_string("variable x: string {}").is_err()); // missing version
        assert!(crate::parse_string("version \"2025-11-02\"\nversion \"2025-11-02\"").is_err());
        let bp = parse("version \"2025-11-02\"");
        assert_eq!(
            bp.version.unwrap().value,
            ScalarValue::String("2025-11-02".into())
        );
    }

    #[test]
    fn test_transform_single_and_list() {
        let single = parse("version \"2025-11-02\"\ntransform \"celerity-2027-02-01\"");
        assert_eq!(single.transform.len(), 1);

        let list = parse("version \"2025-11-02\"\ntransform [\"a/macros\", \"b/macros\"]");
        assert_eq!(list.transform.len(), 2);
        assert_eq!(list.transform[1].value, "b/macros");
    }

    #[test]
    fn test_variable_with_all_fields() {
        let bp = parse(
            "version \"2025-11-02\"\n\
variable region: string {\n\
    default = \"us-east-1\"\n\
    description = \"the region to deploy in\"\n\
    allowedValues = [\"us-east-1\", \"eu-west-1\"]\n\
}",
        );
        let v = bp.variables.get("region").unwrap();
        assert!(matches!(v.var_type.value, VariableType::String));
        assert_eq!(
            v.default.as_ref().unwrap().value,
            ScalarValue::String("us-east-1".into())
        );
        assert_eq!(v.allowed_values.len(), 2);
    }

    #[test]
    fn test_variable_custom_element_type() {
        let bp = parse("version \"2025-11-02\"\nvariable size: aws/ec2/instanceSize {}");
        assert!(matches!(
            &bp.variables.get("size").unwrap().var_type.value,
            VariableType::Custom(s) if s == "aws/ec2/instanceSize"
        ));
    }

    #[test]
    fn test_boolean_allowed_values_is_rejected() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nvariable b: boolean { allowedValues = [true] }",
        )
        .unwrap_err();
        assert!(err
            .to_string()
            .contains("'allowedValues' is not valid on a boolean"));
    }

    #[test]
    fn test_value_declaration() {
        let bp = parse(
            "version \"2025-11-02\"\nvalue isProd: boolean { value = variables.environment == \"prod\" }",
        );
        let v = bp.values.get("isProd").unwrap();
        assert!(matches!(v.value_type.value, ValueType::Boolean));
        assert!(!matches!(v.value, MappingNode::None));
    }

    #[test]
    fn test_export_string_and_reference_forms_normalise_equally() {
        let bp = parse(
            "version \"2025-11-02\"\n\
export bare: string { field = resources.fn.spec.functionArn }\n\
export quoted: string { field = \"resources.fn.spec.functionArn\" }",
        );
        let expected = ScalarValue::String("resources.fn.spec.functionArn".into());
        assert_eq!(bp.exports.get("bare").unwrap().field.value, expected);
        assert_eq!(bp.exports.get("quoted").unwrap().field.value, expected);
    }

    #[test]
    fn test_export_field_rejects_a_function_call() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nexport x: string { field = join(values.a, \"-\") }",
        )
        .unwrap_err();
        assert!(err.to_string().contains("must be a reference path"));
    }

    #[test]
    fn test_top_level_metadata() {
        let bp = parse("version \"2025-11-02\"\nmetadata { \"build.minify\" = false }");
        assert!(matches!(bp.metadata, Some(MappingNode::Fields { .. })));
    }

    #[test]
    fn test_unknown_field_is_reported() {
        let err = crate::parse_string("version \"2025-11-02\"\nvalue v: string { bogus = 1 }")
            .unwrap_err();
        assert!(err.to_string().contains("unknown field"));
    }

    #[test]
    fn test_include_declaration() {
        let bp = parse(
            r#"version "2025-11-02"
include coreInfra "core-infra.yaml" {
    description = "core infra"
    variables {
        databaseName = variables.databaseName
    }
    metadata {
        sourceType = "aws/s3"
    }
}"#,
        );
        let inc = bp.include.get("coreInfra").unwrap();
        assert!(inc.description.is_some());
        assert!(matches!(inc.variables, Some(MappingNode::Fields { .. })));
        assert!(inc.metadata.is_some());
    }

    #[test]
    fn test_multiple_errors_recover_and_collect() {
        // A nameless variable and a later unknown field both surface as
        // diagnostics, thanks to `synchronise` recovery between declarations.
        let errs = crate::parse_string(
            "version \"2025-11-02\"\nvariable : string {}\nvariable y: string { bogus = 1 }",
        )
        .unwrap_err();
        assert!(
            errs.children.len() >= 2,
            "expected >=2 diagnostics, got {}",
            errs.children.len()
        );
    }
}
