//! The general blueprint schema: the canonical, format-independent model that
//! the blueprint language parses into (the same model YAML and JWCC map to).

use std::collections::BTreeMap;

use hashlink::LinkedHashMap;
use serde::{Deserialize, Serialize};

use crate::source::Span;

pub mod blueprint;
pub mod data_source;
pub mod export;
pub mod include;
pub mod resource;
pub mod value;
pub mod variable;

pub use blueprint::Blueprint;
pub use data_source::{
    DataSource, DataSourceExport, DataSourceExportMap, DataSourceFieldType, DataSourceMetadata,
    Filter, FilterOperator,
};
pub use export::{Export, ExportType};
pub use include::Include;
pub use resource::{Condition, LinkSelector, Metadata, RemovalPolicy, Resource};
pub use value::{Value, ValueType};
pub use variable::{Variable, VariableType};

/// An ordered, name-keyed collection of declarations, with the source span of
/// each key.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct NamedMap<T> {
    pub values: LinkedHashMap<String, T>,
    /// Source span of each declaration's name.
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub source_meta: BTreeMap<String, Span>,
}

impl<T> NamedMap<T> {
    /// Creates an empty map.
    pub fn new() -> Self {
        Self {
            values: LinkedHashMap::new(),
            source_meta: BTreeMap::new(),
        }
    }

    /// Inserts a declaration, recording the source span of its name.
    pub fn insert(&mut self, name: impl Into<String>, value: T, span: Option<Span>) {
        let name = name.into();
        if let Some(span) = span {
            self.source_meta.insert(name.clone(), span);
        }
        self.values.insert(name, value);
    }

    /// Returns the declaration with the given name, if present.
    pub fn get(&self, name: &str) -> Option<&T> {
        self.values.get(name)
    }

    /// Reports whether the map holds no declarations.
    pub fn is_empty(&self) -> bool {
        self.values.is_empty()
    }

    /// The number of declarations.
    pub fn len(&self) -> usize {
        self.values.len()
    }
}

impl<T> Default for NamedMap<T> {
    fn default() -> Self {
        Self::new()
    }
}
