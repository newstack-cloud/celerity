//! Scalar values: the leaf primitives of the blueprint schema.

use std::fmt;

use serde::{Deserialize, Serialize};

use crate::source::Span;

/// A primitive scalar value. Serialised untagged so it round-trips as the bare
/// scalar (`"x"`, `5`, `1.5`, `true`) rather than an enum wrapper.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum ScalarValue {
    String(String),
    Int(i64),
    Float(f64),
    Bool(bool),
}

impl ScalarValue {
    /// The canonical type name of this scalar (`string`, `integer`, `float`, or
    /// `boolean`).
    pub fn type_name(&self) -> &'static str {
        match self {
            ScalarValue::String(_) => "string",
            ScalarValue::Int(_) => "integer",
            ScalarValue::Float(_) => "float",
            ScalarValue::Bool(_) => "boolean",
        }
    }
}

impl fmt::Display for ScalarValue {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            ScalarValue::String(value) => f.write_str(value),
            ScalarValue::Int(value) => write!(f, "{value}"),
            ScalarValue::Bool(value) => write!(f, "{value}"),
            ScalarValue::Float(value) => {
                // Keep a decimal point so a float renders as `1.0`, not `1`,
                // matching the blueprint float literal form.
                let rendered = format!("{value}");
                if rendered.contains('.') || rendered.contains(['e', 'E', 'n', 'i']) {
                    f.write_str(&rendered)
                } else {
                    write!(f, "{rendered}.0")
                }
            }
        }
    }
}

impl From<&str> for ScalarValue {
    fn from(value: &str) -> Self {
        ScalarValue::String(value.to_string())
    }
}

impl From<String> for ScalarValue {
    fn from(value: String) -> Self {
        ScalarValue::String(value)
    }
}

impl From<i64> for ScalarValue {
    fn from(value: i64) -> Self {
        ScalarValue::Int(value)
    }
}

impl From<f64> for ScalarValue {
    fn from(value: f64) -> Self {
        ScalarValue::Float(value)
    }
}

impl From<bool> for ScalarValue {
    fn from(value: bool) -> Self {
        ScalarValue::Bool(value)
    }
}

/// A scalar value together with its optional source span. Schema fields that
/// hold a scalar use this so that diagnostics and language-server features can
/// point at the literal in the source.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Scalar {
    pub value: ScalarValue,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

impl Scalar {
    /// Creates a scalar carrying a source span.
    pub fn new(value: impl Into<ScalarValue>, span: Span) -> Self {
        Self {
            value: value.into(),
            span: Some(span),
        }
    }

    /// Creates a scalar with no source span (useful in tests and synthesised
    /// values).
    pub fn untracked(value: impl Into<ScalarValue>) -> Self {
        Self {
            value: value.into(),
            span: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::source::Position;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_scalar_value_round_trips_untagged() {
        let cases = [
            ScalarValue::String("hello".into()),
            ScalarValue::Int(5432),
            ScalarValue::Int(-1),
            ScalarValue::Float(3.5),
            ScalarValue::Bool(true),
        ];
        for case in cases {
            let json = serde_json::to_string(&case).unwrap();
            let back: ScalarValue = serde_json::from_str(&json).unwrap();
            assert_eq!(case, back, "round-trip failed for {json}");
        }
        // Untagged: the scalar serialises bare, not wrapped in a variant tag.
        assert_eq!(serde_json::to_string(&ScalarValue::Int(5)).unwrap(), "5");
        assert_eq!(
            serde_json::to_string(&ScalarValue::String("x".into())).unwrap(),
            "\"x\""
        );
    }

    #[test]
    fn test_scalar_value_display() {
        assert_eq!(ScalarValue::String("hi".into()).to_string(), "hi");
        assert_eq!(ScalarValue::Int(120).to_string(), "120");
        assert_eq!(ScalarValue::Bool(false).to_string(), "false");
        // A whole-number float keeps its decimal point.
        assert_eq!(ScalarValue::Float(1.0).to_string(), "1.0");
        assert_eq!(ScalarValue::Float(3.5).to_string(), "3.5");
    }

    #[test]
    fn test_scalar_span_is_skipped_when_absent() {
        let untracked = Scalar::untracked(42_i64);
        assert_eq!(
            serde_json::to_string(&untracked).unwrap(),
            r#"{"value":42}"#
        );

        let tracked = Scalar::new("x", Span::at(Position::new(1, 1)));
        let json = serde_json::to_string(&tracked).unwrap();
        let back: Scalar = serde_json::from_str(&json).unwrap();
        assert_eq!(tracked, back);
    }
}
