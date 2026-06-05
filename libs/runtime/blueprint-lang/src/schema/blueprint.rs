//! The root [`Blueprint`] schema type.

use serde::{Deserialize, Serialize};

use crate::mapping::MappingNode;
use crate::scalar::Scalar;
use crate::schema::data_source::DataSource;
use crate::schema::export::Export;
use crate::schema::include::Include;
use crate::schema::resource::Resource;
use crate::schema::value::Value;
use crate::schema::variable::Variable;
use crate::schema::NamedMap;
use crate::source::{Span, Spanned};

/// A blueprint specification loaded into memory from blueprint-language source.
///
/// `version` is `None` only while a blueprint is being built up during parsing;
/// a fully parsed blueprint always carries a version (the parser reports an
/// error otherwise).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct Blueprint {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub transform: Vec<Spanned<String>>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub variables: NamedMap<Variable>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub values: NamedMap<Value>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub include: NamedMap<Include>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub resources: NamedMap<Resource>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub data_sources: NamedMap<DataSource>,
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub exports: NamedMap<Export>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub metadata: Option<MappingNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::schema::value::ValueType;
    use crate::schema::variable::VariableType;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_blueprint_assembles_and_round_trips() {
        let mut blueprint = Blueprint {
            version: Some(Scalar::untracked("2025-11-02")),
            transform: vec![Spanned::untracked("celerity-2027-02-01".to_string())],
            ..Blueprint::default()
        };

        blueprint.variables.insert(
            "region",
            Variable {
                var_type: Spanned::untracked(VariableType::String),
                description: None,
                secret: None,
                default: Some(Scalar::untracked("us-east-1")),
                allowed_values: vec![],
                span: None,
            },
            None,
        );

        blueprint.values.insert(
            "isProd",
            Value {
                value_type: Spanned::untracked(ValueType::Boolean),
                value: MappingNode::scalar(true),
                description: None,
                secret: None,
                span: None,
            },
            None,
        );

        let json = serde_json::to_string(&blueprint).unwrap();
        let back: Blueprint = serde_json::from_str(&json).unwrap();
        assert_eq!(blueprint, back);
        assert_eq!(blueprint.variables.len(), 1);
        assert_eq!(blueprint.values.len(), 1);
    }

    #[test]
    fn test_empty_blueprint_serialises_compactly() {
        // A default blueprint omits every empty collection and absent field.
        let blueprint = Blueprint::default();
        assert_eq!(serde_json::to_string(&blueprint).unwrap(), "{}");
    }
}
