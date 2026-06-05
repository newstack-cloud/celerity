//! Value declarations.
//!
//! Ports the value schema from the Go implementation.
//! A value's `value` field is an arbitrary mapping
//! node (it can carry scalars, arrays, objects, or substitutions); its
//! `description` may interpolate, while `secret` is a plain scalar.

use serde::{Deserialize, Serialize};

use crate::mapping::MappingNode;
use crate::scalar::Scalar;
use crate::source::{Span, Spanned};
use crate::substitution::StringOrSubstitutions;

/// A static or computed value, reusable across the blueprint.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Value {
    #[serde(rename = "type")]
    pub value_type: Spanned<ValueType>,
    pub value: MappingNode,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub secret: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// The declared type of a value: a scalar type, or one of the two composite
/// types. Serialised as its bare lowercase type string.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ValueType {
    String,
    Integer,
    Float,
    Boolean,
    Array,
    Object,
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_value_type_serialises_lowercase() {
        assert_eq!(
            serde_json::to_string(&ValueType::Object).unwrap(),
            "\"object\""
        );
        assert_eq!(
            serde_json::to_string(&ValueType::Boolean).unwrap(),
            "\"boolean\""
        );
    }

    #[test]
    fn test_value_round_trips() {
        let value = Value {
            value_type: Spanned::untracked(ValueType::Array),
            value: MappingNode::items(vec![MappingNode::scalar("a"), MappingNode::scalar("b")]),
            description: None,
            secret: Some(Scalar::untracked(true)),
            span: None,
        };
        let json = serde_json::to_string(&value).unwrap();
        let back: Value = serde_json::from_str(&json).unwrap();
        assert_eq!(value, back);
    }
}
