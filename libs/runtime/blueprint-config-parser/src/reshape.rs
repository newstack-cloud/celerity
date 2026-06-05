//! Reshaping `celerity_blueprint_lang::Blueprint` into the narrow
//! JWCC JSON the runtime's intermediate model deserializes.
//!
//! The blueprint must already have had its substitutions flattened to raw
//! `${..}` text (parsed with `SubstitutionMode::RawText`), so every spec value
//! is a plain scalar/mapping/sequence and every string-or-substitutions field is
//! a single literal string.
//!
//! Two things force an explicit reshape rather than re-serialising the blueprint
//! types: the narrow model uses camelCase field names (e.g. `allowedValues`) and
//! bare scalars (not the canonical `{value, span}` wrapper); and a resource's
//! `type` key must be emitted before `spec`, because the narrow resource
//! deserializer reads `spec` using the already-seen `type`. Resource objects are
//! therefore ordered `Serialize` structs (serde emits struct fields in
//! declaration order), while order-insensitive parts are built as `Value`s.

use std::collections::BTreeMap;

use celerity_blueprint_lang::mapping::MappingNode;
use celerity_blueprint_lang::scalar::{Scalar, ScalarValue};
use celerity_blueprint_lang::schema::{LinkSelector, Metadata, NamedMap, Resource, Variable};
use celerity_blueprint_lang::substitution::{
    StringOrSubstitution, StringOrSubstitutions, SubstitutionKind,
};
use celerity_blueprint_lang::Blueprint;
use serde::Serialize;
use serde_json::{Map, Value};

const CELERITY_RESOURCE_PREFIX: &str = "celerity/";

/// Reshapes a (substitution-flattened) blueprint into a JWCC JSON
/// string the runtime's `from_jsonc_str` can parse.
pub(crate) fn to_jsonc_string(blueprint: &Blueprint) -> Result<String, serde_json::Error> {
    serde_json::to_string(&reshape_blueprint(blueprint))
}

#[derive(Serialize)]
struct ReshapedBlueprint {
    #[serde(skip_serializing_if = "Option::is_none")]
    version: Option<String>,
    // `transform` and `resources` are required by the intermediate JWCC model
    // (their custom deserializers have no default), so they are always emitted.
    // An empty `transform` becomes `[]`.
    transform: Vec<String>,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    variables: BTreeMap<String, Value>,
    resources: BTreeMap<String, ReshapedResource>,
    #[serde(skip_serializing_if = "Option::is_none")]
    metadata: Option<Value>,
}

/// Field order is significant: serde serialises struct fields in declaration
/// order, so `type` is emitted before `spec` as the narrow resource
/// deserializer requires.
#[derive(Serialize)]
struct ReshapedResource {
    #[serde(rename = "type")]
    resource_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    metadata: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    description: Option<String>,
    #[serde(rename = "linkSelector", skip_serializing_if = "Option::is_none")]
    link_selector: Option<Value>,
    spec: Value,
}

fn reshape_blueprint(blueprint: &Blueprint) -> ReshapedBlueprint {
    let mut variables = BTreeMap::new();
    for (name, variable) in &blueprint.variables.values {
        variables.insert(name.clone(), reshape_variable(variable));
    }

    // Only `celerity/*` resources are interpreted by the runtime; everything
    // else (and data sources, includes, exports) is skipped, mirroring the
    // YAML/JWCC paths.
    let mut resources = BTreeMap::new();
    for (name, resource) in &blueprint.resources.values {
        if resource
            .res_type
            .value
            .starts_with(CELERITY_RESOURCE_PREFIX)
        {
            resources.insert(name.clone(), reshape_resource(resource));
        }
    }

    ReshapedBlueprint {
        version: blueprint.version.as_ref().and_then(scalar_string),
        transform: blueprint
            .transform
            .iter()
            .map(|t| t.value.clone())
            .collect(),
        variables,
        resources,
        metadata: blueprint.metadata.as_ref().map(mapping_node_to_json),
    }
}

fn reshape_resource(resource: &Resource) -> ReshapedResource {
    ReshapedResource {
        resource_type: resource.res_type.value.clone(),
        metadata: resource.metadata.as_ref().map(reshape_metadata),
        description: resource.description.as_ref().and_then(sos_string),
        link_selector: resource.link_selector.as_ref().map(reshape_link_selector),
        spec: mapping_node_to_json(&resource.spec),
    }
}

fn reshape_variable(variable: &Variable) -> Value {
    let mut object = Map::new();
    object.insert(
        "type".to_string(),
        Value::String(variable.var_type.value.as_str().to_string()),
    );
    if let Some(description) = &variable.description {
        object.insert("description".to_string(), scalar_to_json(description));
    }
    if let Some(default) = &variable.default {
        object.insert("default".to_string(), scalar_to_json(default));
    }
    if !variable.allowed_values.is_empty() {
        let values = variable.allowed_values.iter().map(scalar_to_json).collect();
        object.insert("allowedValues".to_string(), Value::Array(values));
    }
    if let Some(secret) = &variable.secret {
        object.insert("secret".to_string(), scalar_to_json(secret));
    }
    Value::Object(object)
}

fn reshape_metadata(metadata: &Metadata) -> Value {
    let mut object = Map::new();
    if let Some(display_name) = metadata.display_name.as_ref().and_then(sos_string) {
        object.insert("displayName".to_string(), Value::String(display_name));
    }
    if let Some(labels) = &metadata.labels {
        object.insert("labels".to_string(), string_map_to_json(labels));
    }
    if let Some(annotations) = &metadata.annotations {
        object.insert("annotations".to_string(), sos_map_to_json(annotations));
    }
    // The canonical `custom` field has no narrow equivalent and is dropped.
    Value::Object(object)
}

fn reshape_link_selector(link_selector: &LinkSelector) -> Value {
    let mut object = Map::new();
    object.insert(
        "byLabel".to_string(),
        string_map_to_json(&link_selector.by_label),
    );
    Value::Object(object)
}

fn mapping_node_to_json(node: &MappingNode) -> Value {
    match node {
        MappingNode::Scalar(scalar) => scalar_to_json(scalar),
        MappingNode::Fields { fields, .. } => Value::Object(
            fields
                .iter()
                .map(|(key, value)| (key.clone(), mapping_node_to_json(value)))
                .collect(),
        ),
        MappingNode::Items { items, .. } => {
            Value::Array(items.iter().map(mapping_node_to_json).collect())
        }
        MappingNode::StringWithSubstitutions(parts) => {
            Value::String(sos_string(parts).unwrap_or_default())
        }
        MappingNode::None => Value::Null,
    }
}

fn scalar_to_json(scalar: &Scalar) -> Value {
    match &scalar.value {
        ScalarValue::String(value) => Value::String(value.clone()),
        ScalarValue::Int(value) => Value::Number((*value).into()),
        ScalarValue::Float(value) => serde_json::Number::from_f64(*value)
            .map(Value::Number)
            .unwrap_or(Value::Null),
        ScalarValue::Bool(value) => Value::Bool(*value),
    }
}

fn scalar_string(scalar: &Scalar) -> Option<String> {
    match &scalar.value {
        ScalarValue::String(value) => Some(value.clone()),
        _ => None,
    }
}

fn string_map_to_json(map: &NamedMap<String>) -> Value {
    Value::Object(
        map.values
            .iter()
            .map(|(key, value)| (key.clone(), Value::String(value.clone())))
            .collect(),
    )
}

fn sos_map_to_json(map: &NamedMap<StringOrSubstitutions>) -> Value {
    Value::Object(
        map.values
            .iter()
            .map(|(key, value)| (key.clone(), sos_to_json(value)))
            .collect(),
    )
}

/// Converts a (flattened) string-or-substitutions value to JSON. A lone scalar
/// literal becomes a typed JSON scalar (so e.g. a boolean annotation stays a
/// boolean); anything else becomes a string.
fn sos_to_json(parts: &StringOrSubstitutions) -> Value {
    if let [StringOrSubstitution::Substitution(substitution)] = parts.values.as_slice() {
        if let Some(value) = scalar_kind_to_json(&substitution.kind) {
            return value;
        }
    }
    Value::String(sos_string(parts).unwrap_or_default())
}

fn scalar_kind_to_json(kind: &SubstitutionKind) -> Option<Value> {
    match kind {
        SubstitutionKind::Int(value) => Some(Value::Number((*value).into())),
        SubstitutionKind::Float(value) => serde_json::Number::from_f64(*value).map(Value::Number),
        SubstitutionKind::Bool(value) => Some(Value::Bool(*value)),
        _ => None,
    }
}

/// Joins the literal parts of a (flattened) string-or-substitutions value.
/// Returns `None` if a structured substitution remains, which should not happen
/// for a `RawText`-flattened blueprint.
fn sos_string(parts: &StringOrSubstitutions) -> Option<String> {
    let mut text = String::new();
    for part in &parts.values {
        match part {
            StringOrSubstitution::String { value, .. } => text.push_str(value),
            StringOrSubstitution::Substitution(_) => return None,
        }
    }
    Some(text)
}
