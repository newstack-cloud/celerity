//! Resource sub-constructs: `metadata`, `spec`, `select by label`, and the
//! `foreach` inline statement.

use super::core::Parser;
use crate::errors::ParseError;
use crate::expr_transform::{
    expr_to_condition, expr_to_string_or_substitutions, object_to_string_map,
};
use crate::parser::core::merge_end;
use crate::parser::expr::Expr;
use crate::scalar::{Scalar, ScalarValue};
use crate::schema::{LinkSelector, Metadata, RemovalPolicy, Resource};
use crate::source::{Position, Span, Spanned};
use crate::substitution::{Substitution, SubstitutionKind};
use crate::{Blueprint, TokenType};
use std::collections::BTreeMap;

impl Parser {
    pub(crate) fn parse_resource_decl(&mut self, bp: &mut Blueprint) -> Result<(), ParseError> {
        self.advance(); // 'resource'
        let (name, name_span) = self.parse_element_name()?;
        self.expect(TokenType::Colon)?;
        let (type_str, type_span) = self.parse_element_type()?;

        let mut resource = Resource {
            res_type: Spanned::new(type_str, type_span),
            description: None,
            metadata: None,
            depends_on: vec![],
            condition: None,
            each: None,
            link_selector: None,
            removal_policy: None,
            spec: crate::mapping::MappingNode::None,
            span: Some(name_span),
            fields_source_meta: BTreeMap::new(),
        };
        self.parse_brace_block(|p| p.parse_resource_decl_entry(&mut resource))?;

        // `spec` is required; an empty `spec {}` parses to Fields, so only a
        // never-parsed spec stays None.
        if matches!(resource.spec, crate::mapping::MappingNode::None) {
            return Err(self.error_at(
                name_span.start,
                format!("resource {name:?} is missing the required 'spec {{ ... }}' block"),
            ));
        }
        bp.resources.insert(name, resource, Some(name_span));
        Ok(())
    }

    fn parse_resource_decl_entry(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        match self.peek_type() {
            TokenType::KeywordMetadata => self.parse_resource_metadata_block(r),
            TokenType::KeywordSelect => self.parse_select_statement(r),
            TokenType::KeywordSpec => self.parse_spec_block(r),
            TokenType::KeywordForeach => self.parse_foreach_statement(r),
            TokenType::Identifier => self.parse_resource_field_assignment(r),
            _ => {
                let t = self.peek();
                let (start, label) = (t.start, t.ty.display_label());
                Err(self.error_at(start, format!(
                    "expected 'metadata', 'select', 'spec', 'foreach', or a field assignment in resource, got {label}")))
            }
        }
    }

    fn parse_resource_metadata_block(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        let mut meta = Metadata::default();
        let span =
            self.parse_metadata_keyword_block(|p| p.parse_resource_metadata_field(&mut meta))?;
        meta.span = Some(span);
        r.fields_source_meta.insert("metadata".into(), span);
        r.metadata = Some(meta);
        Ok(())
    }

    fn parse_resource_metadata_field(&mut self, m: &mut Metadata) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_key()?;
        match field.as_str() {
            "displayName" => m.display_name = Some(self.parse_display_name_field()?),
            "labels" => {
                let entries = self.parse_object_literal_assignment("labels")?;
                m.labels = Some(object_to_string_map(entries)?);
            }
            "annotations" => m.annotations = Some(self.parse_annotations_field()?),
            "custom" => m.custom = Some(self.parse_custom_field()?),
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in resource metadata"),
                ))
            }
        }
        m.fields_source_meta.insert(field, field_span);
        Ok(())
    }

    fn parse_select_statement(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        let open = self.advance(); // 'select'
        self.expect(TokenType::KeywordBy)?;
        self.expect(TokenType::KeywordLabel)?;

        let mut selector = LinkSelector::default();
        let block = self.parse_brace_block(|p| p.parse_select_by_label_entry(&mut selector))?;
        let span = Span::new(open.start, block.end.unwrap_or(block.start));
        selector.span = Some(span);
        r.fields_source_meta.insert("linkSelector".into(), span);
        r.link_selector = Some(selector);
        Ok(())
    }

    fn parse_select_by_label_entry(&mut self, s: &mut LinkSelector) -> Result<(), ParseError> {
        let (key, key_span) = self.parse_object_key()?;
        self.expect(TokenType::Assign)?;

        if key == "exclude" {
            let e = self.parse_expr()?;
            s.exclude = expr_to_resource_name_list(e, "exclude")?;
            return Ok(());
        }
        // A label value must be a string literal.
        if self.peek_type() != TokenType::StringStart {
            let t = self.peek();
            let (start, label) = (t.start, t.ty.display_label());
            return Err(self.error_at(
                start,
                format!("label value for {key:?} must be a string literal, got {label}"),
            ));
        }
        let (value, _) = self.collect_string_literal(false)?;
        s.by_label.insert(key, value, Some(key_span));
        Ok(())
    }

    fn parse_spec_block(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        let open = self.advance(); // 'spec'
        r.spec = self.parse_free_form_map_block()?;
        let end = r.spec.span().and_then(|s| s.end).unwrap_or(open.end);
        r.fields_source_meta
            .insert("spec".into(), Span::new(open.start, end));
        Ok(())
    }

    fn parse_foreach_statement(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        let open = self.advance(); // 'foreach'
        let e = self.parse_expr()?;
        let end = e.span().and_then(|s| s.end).unwrap_or(open.end);
        r.each = Some(expr_to_string_or_substitutions(e));
        r.fields_source_meta
            .insert("each".into(), Span::new(open.start, end));
        Ok(())
    }

    fn parse_resource_field_assignment(&mut self, r: &mut Resource) -> Result<(), ParseError> {
        let (field, field_span) = self.parse_field_assignment()?;
        // The match yields the value's end position, so fields_source_meta can
        // cover the whole `field = value` span.
        let value_end: Option<Position> = match field.as_str() {
            "description" => {
                let sos = self.parse_interpolated_string()?;
                let end = sos.span.and_then(|s| s.end);
                r.description = Some(sos);
                end
            }
            "condition" => {
                let e = self.parse_expr()?;
                let end = e.span().and_then(|s| s.end);
                r.condition = Some(expr_to_condition(e)?);
                end
            }
            "dependsOn" => {
                let e = self.parse_expr()?;
                r.depends_on = expr_to_resource_name_list(e, "dependsOn")?;
                r.depends_on.last().and_then(|s| s.span).and_then(|s| s.end)
            }
            "removalPolicy" => {
                let policy = self.parse_removal_policy_value()?;
                let end = policy.span.and_then(|s| s.end);
                r.removal_policy = Some(policy);
                end
            }
            other => {
                return Err(self.error_at(
                    field_span.start,
                    format!("unknown field {other:?} in resource declaration"),
                ))
            }
        };
        r.fields_source_meta
            .insert(field, merge_end(field_span, value_end));
        Ok(())
    }

    fn parse_removal_policy_value(&mut self) -> Result<Spanned<RemovalPolicy>, ParseError> {
        if self.peek_type() != TokenType::StringStart {
            let t = self.peek();
            let (start, label) = (t.start, t.ty.display_label());
            return Err(self.error_at(
                start,
                format!("removalPolicy must be a literal string, got {label}"),
            ));
        }
        let (value, span) = self.collect_string_literal(false)?;
        let policy = match value.as_str() {
            "delete" => RemovalPolicy::Delete,
            "retain" => RemovalPolicy::Retain,
            _ => {
                return Err(self.error_at(
                    span.start,
                    format!("removalPolicy must be \"delete\" or \"retain\", got {value:?}"),
                ))
            }
        };
        Ok(Spanned::new(policy, span))
    }
}

/// A single resource name or an array of them (for `dependsOn` / `exclude`).
/// Each element is a string literal or a bare single-segment resource reference.
fn expr_to_resource_name_list(e: Expr, field: &str) -> Result<Vec<Spanned<String>>, ParseError> {
    match e {
        Expr::Array { elems, .. } => elems
            .into_iter()
            .map(|el| extract_resource_name(el, field))
            .collect(),
        other => Ok(vec![extract_resource_name(other, field)?]),
    }
}

fn extract_resource_name(e: Expr, field: &str) -> Result<Spanned<String>, ParseError> {
    match e {
        Expr::Scalar(Scalar {
            value: ScalarValue::String(s),
            span,
        }) => Ok(Spanned { value: s, span }),
        Expr::Ref(Substitution {
            kind: SubstitutionKind::ResourceProperty(rp),
            span,
        }) if rp.path.is_empty() && rp.resource_each_template_index.is_none() => Ok(Spanned {
            value: rp.resource_name,
            span,
        }),
        other => Err(ParseError {
            message: format!(
                "entries in {field} must be either a string literal or a bare resource name"
            ),
            span: other.span(),
        }),
    }
}

#[cfg(test)]
mod tests {
    use crate::mapping::MappingNode;
    use crate::schema::{Condition, RemovalPolicy};
    use pretty_assertions::assert_eq;

    fn parse(src: &str) -> crate::Blueprint {
        crate::parse_string(src).unwrap_or_else(|e| panic!("parse failed:\n{e}"))
    }

    #[test]
    fn test_resource_full() {
        let bp = parse(
            r#"version "2025-11-02"
resource saveOrder: aws/lambda/function {
    description = "Saves an order"
    metadata {
        displayName = "Save Order"
        labels = { service = "ordersApi" }
        annotations = { "aws.populateEnvVars" = true }
        custom = { ui = { x = 1 } }
    }
    select by label {
        service = "ordersApi"
        exclude = [testTable]
    }
    foreach values.buckets
    condition = variables.env == "prod"
    dependsOn = ["queueDrainer", schemaInit]
    removalPolicy = "retain"
    spec {
        handler = "save_order.handler"
        memorySize = if(values.isProd, 1024, 512)
    }
}"#,
        );
        let r = bp.resources.get("saveOrder").unwrap();
        assert_eq!(r.res_type.value, "aws/lambda/function");
        assert!(r.description.is_some());

        let m = r.metadata.as_ref().unwrap();
        assert!(m.display_name.is_some());
        assert_eq!(
            m.labels
                .as_ref()
                .unwrap()
                .get("service")
                .map(String::as_str),
            Some("ordersApi")
        );
        assert!(m.annotations.is_some() && m.custom.is_some());

        let sel = r.link_selector.as_ref().unwrap();
        assert_eq!(sel.by_label.len(), 1);
        assert_eq!(sel.exclude.len(), 1); // testTable

        assert!(r.each.is_some()); // foreach
        assert!(matches!(r.condition, Some(Condition::Expr(_))));
        assert_eq!(r.depends_on.len(), 2); // string + bare ref
        assert!(matches!(
            r.removal_policy.as_ref().unwrap().value,
            RemovalPolicy::Retain
        ));
        assert!(matches!(r.spec, MappingNode::Fields { .. }));
    }

    #[test]
    fn test_condition_object_form() {
        let bp = parse(
            r#"version "2025-11-02"
resource r: x/y {
    condition = { and = [variables.a, variables.b] }
    spec { name = "x" }
}"#,
        );
        assert!(matches!(
            bp.resources.get("r").unwrap().condition,
            Some(Condition::And(_))
        ));
    }

    #[test]
    fn test_empty_spec_is_allowed() {
        // An empty `spec {}` is a present (empty) Fields node, not missing.
        let bp = parse("version \"2025-11-02\"\nresource r: x/y { spec {} }");
        assert!(matches!(
            bp.resources.get("r").unwrap().spec,
            MappingNode::Fields { .. }
        ));
    }

    #[test]
    fn test_missing_spec_is_rejected() {
        let err =
            crate::parse_string("version \"2025-11-02\"\nresource r: x/y { description = \"x\" }")
                .unwrap_err();
        assert!(err.to_string().contains("missing the required 'spec"));
    }

    #[test]
    fn test_bad_removal_policy_is_rejected() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nresource r: x/y { removalPolicy = \"keep\"\nspec { a = 1 } }",
        )
        .unwrap_err();
        let msg = err.to_string();
        assert!(msg.contains("delete") && msg.contains("retain"));
    }

    #[test]
    fn test_label_value_must_be_a_string() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nresource r: x/y { select by label { service = 1 }\nspec { a = 1 } }",
        )
        .unwrap_err();
        assert!(err.to_string().contains("must be a string literal"));
    }

    #[test]
    fn test_depends_on_rejects_computed_entries() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nresource r: x/y { dependsOn = [join(a, b)]\nspec { a = 1 } }",
        )
        .unwrap_err();
        assert!(err
            .to_string()
            .contains("string literal or a bare resource name"));
    }

    #[test]
    fn test_unknown_resource_field_is_reported() {
        let err = crate::parse_string(
            "version \"2025-11-02\"\nresource r: x/y { bogus = 1\nspec { a = 1 } }",
        )
        .unwrap_err();
        assert!(err.to_string().contains("unknown field"));
    }
}
