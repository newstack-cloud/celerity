//! Arbitrary-shape values: the tree used for resource specs, metadata, and any
//! field whose schema is provider-defined rather than fixed.

use std::collections::BTreeMap;

use hashlink::LinkedHashMap;
use serde::{Deserialize, Serialize};

use crate::scalar::{Scalar, ScalarValue};
use crate::source::Span;
use crate::substitution::StringOrSubstitutions;

/// A node in an arbitrary value tree.
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub enum MappingNode {
    /// A scalar leaf.
    Scalar(Scalar),
    /// An object: ordered field names to child nodes.
    Fields {
        fields: LinkedHashMap<String, MappingNode>,
        /// Source span of each field key.
        #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
        fields_span: BTreeMap<String, Span>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        span: Option<Span>,
    },
    /// An array of child nodes.
    Items {
        items: Vec<MappingNode>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        span: Option<Span>,
    },
    /// A string that may interpolate `${..}` substitutions.
    StringWithSubstitutions(StringOrSubstitutions),
    /// An absent value (`none`), or an otherwise empty node.
    #[default]
    None,
}

impl MappingNode {
    /// Creates a scalar node with no source span.
    pub fn scalar(value: impl Into<ScalarValue>) -> Self {
        MappingNode::Scalar(Scalar::untracked(value))
    }

    /// Creates an object node from its fields, with no field or node spans.
    pub fn fields(fields: LinkedHashMap<String, MappingNode>) -> Self {
        MappingNode::Fields {
            fields,
            fields_span: BTreeMap::new(),
            span: None,
        }
    }

    /// Creates an array node from its items, with no node span.
    pub fn items(items: Vec<MappingNode>) -> Self {
        MappingNode::Items { items, span: None }
    }

    /// The source span of this node, if tracked.
    pub fn span(&self) -> Option<Span> {
        match self {
            MappingNode::Scalar(scalar) => scalar.span,
            MappingNode::StringWithSubstitutions(parts) => parts.span,
            MappingNode::Fields { span, .. } | MappingNode::Items { span, .. } => *span,
            MappingNode::None => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    fn obj(entries: Vec<(&str, MappingNode)>) -> MappingNode {
        let mut fields = LinkedHashMap::new();
        for (key, node) in entries {
            fields.insert(key.to_string(), node);
        }
        MappingNode::fields(fields)
    }

    #[test]
    fn test_nested_mapping_node_round_trips() {
        let node = obj(vec![
            ("name", MappingNode::scalar("orders")),
            ("count", MappingNode::scalar(3_i64)),
            (
                "tags",
                MappingNode::items(vec![MappingNode::scalar("a"), MappingNode::scalar("b")]),
            ),
            ("nested", obj(vec![("enabled", MappingNode::scalar(true))])),
        ]);

        let json = serde_json::to_string(&node).unwrap();
        let back: MappingNode = serde_json::from_str(&json).unwrap();
        assert_eq!(node, back);
    }

    #[test]
    fn test_object_field_order_is_preserved() {
        let node = obj(vec![
            ("z", MappingNode::scalar(1_i64)),
            ("a", MappingNode::scalar(2_i64)),
            ("m", MappingNode::scalar(3_i64)),
        ]);
        let MappingNode::Fields { fields, .. } = &node else {
            panic!("expected fields");
        };
        let keys: Vec<&String> = fields.keys().collect();
        assert_eq!(keys, vec!["z", "a", "m"]);
    }

    #[test]
    fn test_default_is_none() {
        assert_eq!(MappingNode::default(), MappingNode::None);
        assert_eq!(MappingNode::None.span(), None);
    }
}
