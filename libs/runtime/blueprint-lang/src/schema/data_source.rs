//! Data source declarations.
//!
//! Ports the data source schema from the Go implementation.
//! A data source reads an external resource and
//! exposes selected fields: `filters` select the resource, `exports` declare the
//! fields to expose (or all of them, via `export_all`).

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::scalar::Scalar;
use crate::schema::NamedMap;
use crate::source::{Span, Spanned};
use crate::substitution::StringOrSubstitutions;

/// A data source: a read of an external resource exposing selected fields.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DataSource {
    #[serde(rename = "type")]
    pub ds_type: Spanned<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub metadata: Option<DataSourceMetadata>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub filters: Vec<Filter>,
    pub exports: DataSourceExportMap,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
    /// Source span of each field key.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub fields_source_meta: BTreeMap<String, Span>,
}

/// Identification and context for a data source (`displayName`, `annotations`,
/// `custom`).
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct DataSourceMetadata {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub display_name: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub annotations: Option<NamedMap<StringOrSubstitutions>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub custom: Option<crate::mapping::MappingNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub fields_source_meta: BTreeMap<String, Span>,
}

/// A single `filter <field> <operator> <search>` statement.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Filter {
    pub field: Scalar,
    pub operator: Spanned<FilterOperator>,
    /// The search value(s). A single scalar for most operators; multiple values
    /// for `in` / `not in` and array-to-array comparisons.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub search: Vec<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// A data source filter operator. The canonical (YAML/JWCC) spelling is used as
/// the serialised form; note that equality is `=` here, not `==`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum FilterOperator {
    #[serde(rename = "=")]
    Equals,
    #[serde(rename = "!=")]
    NotEquals,
    #[serde(rename = "in")]
    In,
    #[serde(rename = "not in")]
    NotIn,
    #[serde(rename = "has key")]
    HasKey,
    #[serde(rename = "not has key")]
    NotHasKey,
    #[serde(rename = "contains")]
    Contains,
    #[serde(rename = "not contains")]
    NotContains,
    #[serde(rename = "starts with")]
    StartsWith,
    #[serde(rename = "not starts with")]
    NotStartsWith,
    #[serde(rename = "ends with")]
    EndsWith,
    #[serde(rename = "not ends with")]
    NotEndsWith,
    #[serde(rename = ">")]
    GreaterThan,
    #[serde(rename = "<")]
    LessThan,
    #[serde(rename = ">=")]
    GreaterThanOrEqual,
    #[serde(rename = "<=")]
    LessThanOrEqual,
}

impl FilterOperator {
    /// The canonical operator string (`=`, `not in`, `has key`, ...).
    pub fn as_str(&self) -> &'static str {
        match self {
            FilterOperator::Equals => "=",
            FilterOperator::NotEquals => "!=",
            FilterOperator::In => "in",
            FilterOperator::NotIn => "not in",
            FilterOperator::HasKey => "has key",
            FilterOperator::NotHasKey => "not has key",
            FilterOperator::Contains => "contains",
            FilterOperator::NotContains => "not contains",
            FilterOperator::StartsWith => "starts with",
            FilterOperator::NotStartsWith => "not starts with",
            FilterOperator::EndsWith => "ends with",
            FilterOperator::NotEndsWith => "not ends with",
            FilterOperator::GreaterThan => ">",
            FilterOperator::LessThan => "<",
            FilterOperator::GreaterThanOrEqual => ">=",
            FilterOperator::LessThanOrEqual => "<=",
        }
    }
}

/// A single exported field of a data source.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DataSourceExport {
    #[serde(rename = "type")]
    pub field_type: Spanned<DataSourceFieldType>,
    /// The source field name, when the export is renamed with `as`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub alias_for: Option<Scalar>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// The type of an exported data source field. Arrays may only hold primitives,
/// so there is no `object`. Serialised as its bare lowercase type string.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DataSourceFieldType {
    String,
    Integer,
    Float,
    Boolean,
    Array,
}

/// The exports of a data source: named field exports plus an `export_all` flag
/// set when the blueprint uses `export *`.
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct DataSourceExportMap {
    #[serde(default, skip_serializing_if = "NamedMap::is_empty")]
    pub exports: NamedMap<DataSourceExport>,
    #[serde(default, skip_serializing_if = "is_false")]
    pub export_all: bool,
}

/// Serde skip predicate: true when the bool is `false`, so a default
/// `export_all = false` is omitted from the serialised form.
fn is_false(value: &bool) -> bool {
    !value
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
    fn test_filter_operator_serialises_canonically() {
        assert_eq!(
            serde_json::to_string(&FilterOperator::NotIn).unwrap(),
            "\"not in\""
        );
        assert_eq!(
            serde_json::to_string(&FilterOperator::Equals).unwrap(),
            "\"=\""
        );
        assert_eq!(FilterOperator::HasKey.as_str(), "has key");
    }

    #[test]
    fn test_data_source_round_trips() {
        let mut exports = NamedMap::new();
        exports.insert(
            "subnets",
            DataSourceExport {
                field_type: Spanned::untracked(DataSourceFieldType::Array),
                alias_for: None,
                description: None,
                span: None,
            },
            None,
        );

        let data_source = DataSource {
            ds_type: Spanned::untracked("aws/vpc".to_string()),
            metadata: None,
            filters: vec![Filter {
                field: Scalar::untracked("state"),
                operator: Spanned::untracked(FilterOperator::Equals),
                search: vec![literal("available")],
                span: None,
            }],
            exports: DataSourceExportMap {
                exports,
                export_all: false,
            },
            description: None,
            span: None,
            fields_source_meta: BTreeMap::new(),
        };

        let json = serde_json::to_string(&data_source).unwrap();
        let back: DataSource = serde_json::from_str(&json).unwrap();
        assert_eq!(data_source, back);
    }

    #[test]
    fn test_export_all_round_trips() {
        let exports = DataSourceExportMap {
            exports: NamedMap::new(),
            export_all: true,
        };
        let json = serde_json::to_string(&exports).unwrap();
        let back: DataSourceExportMap = serde_json::from_str(&json).unwrap();
        assert_eq!(exports, back);
        assert!(back.export_all);
    }
}
