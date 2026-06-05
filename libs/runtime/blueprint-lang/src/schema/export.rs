//! Export declarations.
//!
//! Ports the export schema from the Go implementation.
//! An export's `field` is a pure reference path,
//! carried in the canonical schema as a plain string scalar (a bare reference
//! is normalised to the same string a quoted path would produce).

use serde::{Deserialize, Serialize};

use crate::scalar::Scalar;
use crate::source::{Span, Spanned};
use crate::substitution::StringOrSubstitutions;

/// A value exposed from this blueprint as a publicly accessible attribute.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Export {
    #[serde(rename = "type")]
    pub export_type: Spanned<ExportType>,
    pub field: Scalar,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// The declared type of an export: a scalar type, or one of the two composite
/// types. Serialised as its bare lowercase type string.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ExportType {
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
    fn test_export_round_trips() {
        let export = Export {
            export_type: Spanned::untracked(ExportType::String),
            field: Scalar::untracked("resources.saveOrder.spec.functionArn"),
            description: None,
            span: None,
        };
        let json = serde_json::to_string(&export).unwrap();
        let back: Export = serde_json::from_str(&json).unwrap();
        assert_eq!(export, back);
    }
}
