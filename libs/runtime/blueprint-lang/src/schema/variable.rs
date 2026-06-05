//! Variable declarations.
//!
//! Ports variable schema from the Go implementation.
//! A variable's `description`, `secret`, `default`,
//! and `allowedValues` are plain scalars (not substitutions) — variables cannot
//! reference other elements.

use serde::{Deserialize, Serialize};

use crate::scalar::Scalar;
use crate::source::{Span, Spanned};

/// A dynamic input to the blueprint, supplied at deployment time.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Variable {
    #[serde(rename = "type")]
    pub var_type: Spanned<VariableType>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub secret: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub allowed_values: Vec<Scalar>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// The type of a variable: one of the four scalar types or a provider-supplied
/// custom type (e.g. `aws/ec2/instanceSize`). Serialised as its bare type
/// string. Open set, so it is not a closed enum.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum VariableType {
    String,
    Integer,
    Float,
    Boolean,
    Custom(String),
}

impl VariableType {
    /// The canonical type string for this variable type.
    pub fn as_str(&self) -> &str {
        match self {
            VariableType::String => "string",
            VariableType::Integer => "integer",
            VariableType::Float => "float",
            VariableType::Boolean => "boolean",
            VariableType::Custom(name) => name.as_str(),
        }
    }

    /// Reports whether this is one of the four built-in scalar types.
    pub fn is_builtin(&self) -> bool {
        !matches!(self, VariableType::Custom(_))
    }
}

impl From<String> for VariableType {
    fn from(value: String) -> Self {
        match value.as_str() {
            "string" => VariableType::String,
            "integer" => VariableType::Integer,
            "float" => VariableType::Float,
            "boolean" => VariableType::Boolean,
            _ => VariableType::Custom(value),
        }
    }
}

impl From<VariableType> for String {
    fn from(value: VariableType) -> Self {
        match value {
            VariableType::Custom(name) => name,
            builtin => builtin.as_str().to_string(),
        }
    }
}

// Serialise as the bare type string (e.g. `"string"`, `"aws/ec2/instanceSize"`)
// rather than an enum wrapper, matching the canonical schema.
impl Serialize for VariableType {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(self.as_str())
    }
}

impl<'de> Deserialize<'de> for VariableType {
    fn deserialize<D: serde::Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let value = String::deserialize(deserializer)?;
        Ok(VariableType::from(value))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_variable_type_serialises_as_bare_string() {
        assert_eq!(
            serde_json::to_string(&VariableType::String).unwrap(),
            "\"string\""
        );
        assert_eq!(
            serde_json::to_string(&VariableType::Custom("aws/ec2/instanceSize".into())).unwrap(),
            "\"aws/ec2/instanceSize\""
        );
    }

    #[test]
    fn test_variable_type_round_trips_builtin_and_custom() {
        for value in [
            VariableType::String,
            VariableType::Integer,
            VariableType::Float,
            VariableType::Boolean,
            VariableType::Custom("aws/ec2/instanceSize".into()),
        ] {
            let json = serde_json::to_string(&value).unwrap();
            let back: VariableType = serde_json::from_str(&json).unwrap();
            assert_eq!(value, back);
        }
        assert!(VariableType::String.is_builtin());
        assert!(!VariableType::Custom("x/y".into()).is_builtin());
    }

    #[test]
    fn test_variable_round_trips() {
        let variable = Variable {
            var_type: Spanned::untracked(VariableType::String),
            description: Some(Scalar::untracked("the region")),
            secret: Some(Scalar::untracked(false)),
            default: Some(Scalar::untracked("us-east-1")),
            allowed_values: vec![
                Scalar::untracked("us-east-1"),
                Scalar::untracked("eu-west-1"),
            ],
            span: None,
        };
        let json = serde_json::to_string(&variable).unwrap();
        let back: Variable = serde_json::from_str(&json).unwrap();
        assert_eq!(variable, back);
    }
}
