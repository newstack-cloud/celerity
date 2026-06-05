//! Resource declarations.
//!
//! Ports the resource schema from the Go implementation.
//! The resource is the central declaration: most other elements exist to feed and
//! expose data through resources.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::mapping::MappingNode;
use crate::schema::NamedMap;
use crate::source::{Span, Spanned};
use crate::substitution::StringOrSubstitutions;

/// A resource provisioned by one of the configured providers.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Resource {
    #[serde(rename = "type")]
    pub res_type: Spanned<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub metadata: Option<Metadata>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub depends_on: Vec<Spanned<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub condition: Option<Condition>,
    /// The `foreach` iteration expression, transformed to the canonical `each`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub each: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub link_selector: Option<LinkSelector>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub removal_policy: Option<Spanned<RemovalPolicy>>,
    pub spec: MappingNode,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
    /// Source span of each field key.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub fields_source_meta: BTreeMap<String, Span>,
}

/// Identification and linkage for a resource (`displayName`, `labels`,
/// `annotations`, `custom`).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct Metadata {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub display_name: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub annotations: Option<NamedMap<StringOrSubstitutions>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub labels: Option<NamedMap<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub custom: Option<MappingNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub fields_source_meta: BTreeMap<String, Span>,
}

/// The implicit link selector for a resource (`select by label`).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct LinkSelector {
    pub by_label: NamedMap<String>,
    /// Resource names to exclude from the selection.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub exclude: Vec<Spanned<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// Controls the fate of the underlying provider resource on removal.
/// Serialised as its bare lowercase string.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RemovalPolicy {
    Delete,
    Retain,
}

/// A resource condition: a boolean expression, or a structured `and`/`or`/`not`
/// combination of further conditions.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Condition {
    And(Vec<Condition>),
    Or(Vec<Condition>),
    Not(Box<Condition>),
    /// A bare boolean expression (the natural blueprint-language form).
    Expr(StringOrSubstitutions),
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::substitution::{
        StringOrSubstitution, Substitution, SubstitutionKind, SubstitutionResourceProperty,
    };
    use pretty_assertions::assert_eq;

    fn literal(text: &str) -> StringOrSubstitutions {
        StringOrSubstitutions::new(vec![StringOrSubstitution::String {
            value: text.to_string(),
            span: None,
        }])
    }

    fn expr_resource(name: &str) -> StringOrSubstitutions {
        StringOrSubstitutions::new(vec![StringOrSubstitution::Substitution(
            Substitution::untracked(SubstitutionKind::ResourceProperty(
                SubstitutionResourceProperty {
                    resource_name: name.to_string(),
                    resource_each_template_index: None,
                    path: vec![],
                },
            )),
        )])
    }

    #[test]
    fn test_removal_policy_serialises_lowercase() {
        assert_eq!(
            serde_json::to_string(&RemovalPolicy::Retain).unwrap(),
            "\"retain\""
        );
    }

    #[test]
    fn test_condition_round_trips() {
        let condition = Condition::And(vec![
            Condition::Expr(expr_resource("a")),
            Condition::Not(Box::new(Condition::Expr(expr_resource("b")))),
        ]);
        let json = serde_json::to_string(&condition).unwrap();
        let back: Condition = serde_json::from_str(&json).unwrap();
        assert_eq!(condition, back);
    }

    #[test]
    fn test_resource_round_trips() {
        let mut by_label = NamedMap::new();
        by_label.insert("service", "ordersApi".to_string(), None);

        let resource = Resource {
            res_type: Spanned::untracked("aws/lambda/function".to_string()),
            description: Some(literal("Saves an order")),
            metadata: Some(Metadata {
                display_name: Some(literal("Save Order Function")),
                ..Metadata::default()
            }),
            depends_on: vec![Spanned::untracked("auditLogStream".to_string())],
            condition: None,
            each: None,
            link_selector: Some(LinkSelector {
                by_label,
                exclude: vec![Spanned::untracked("testOrdersTable".to_string())],
                span: None,
            }),
            removal_policy: Some(Spanned::untracked(RemovalPolicy::Retain)),
            spec: MappingNode::scalar("placeholder"),
            span: None,
            fields_source_meta: BTreeMap::new(),
        };

        let json = serde_json::to_string(&resource).unwrap();
        let back: Resource = serde_json::from_str(&json).unwrap();
        assert_eq!(resource, back);
    }
}
