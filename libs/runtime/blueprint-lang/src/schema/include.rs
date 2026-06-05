//! Include declarations.
//!
//! Ports the include schema from the Go implementation.
//! An include pulls in another blueprint as a child:
//! `path` locates the child, `variables` binds its inputs, and `metadata`
//! carries source-resolution context for include resolver plugins.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::mapping::MappingNode;
use crate::source::Span;
use crate::substitution::StringOrSubstitutions;

/// An included child blueprint.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Include {
    pub path: StringOrSubstitutions,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub variables: Option<MappingNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub metadata: Option<MappingNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
    /// Source span of each field key.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub fields_source_meta: BTreeMap<String, Span>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::substitution::StringOrSubstitution;
    use pretty_assertions::assert_eq;

    fn literal(text: &str) -> StringOrSubstitutions {
        StringOrSubstitutions::new(vec![StringOrSubstitution::String {
            value: text.to_string(),
            span: None,
        }])
    }

    #[test]
    fn test_include_round_trips() {
        let mut variables = hashlink::LinkedHashMap::new();
        variables.insert("databaseName".to_string(), MappingNode::scalar("orders"));

        let include = Include {
            path: literal("core-infra.yaml"),
            variables: Some(MappingNode::fields(variables)),
            metadata: None,
            description: Some(literal("core infra for the Orders API")),
            span: None,
            fields_source_meta: BTreeMap::new(),
        };
        let json = serde_json::to_string(&include).unwrap();
        let back: Include = serde_json::from_str(&json).unwrap();
        assert_eq!(include, back);
    }
}
