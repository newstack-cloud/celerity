//! Collapsing structured substitutions back to raw `${..}` source text.
//!
//! Backs the `SubstitutionMode::RawText` parse option: every parsed `${..}`
//! substitution in a `Blueprint` is replaced by the exact text it occupied in
//! the source, so consumers that resolve substitutions themselves receive
//! opaque strings instead of structured nodes.
//!
//! Operators, arrays, objects, and interpolations are desugared while parsing
//! (`a == b` becomes `eq(a, b)`, `[x]` becomes `list(x)`), so the original text
//! cannot be reproduced from the structured form, it is recovered by slicing
//! each substitution's source span.

use crate::mapping::MappingNode;
use crate::scalar::{Scalar, ScalarValue};
use crate::schema::{
    Blueprint, Condition, DataSource, DataSourceMetadata, Include, Metadata, NamedMap, Resource,
    Value,
};
use crate::source::{Position, Span};
use crate::substitution::{StringOrSubstitution, StringOrSubstitutions, SubstitutionKind};

/// Rewrites every substitution in `blueprint` to its raw `${..}` source text.
///
/// `source` must be the newline-normalised text the parser consumed (see
/// [`crate::parse_string_with_options`]), so substitution spans line up with it.
pub(crate) fn substitutions_to_text(blueprint: &mut Blueprint, source: &str) {
    flatten_opt_node(&mut blueprint.metadata, source);
    for value in blueprint.values.values.values_mut() {
        flatten_value(value, source);
    }
    for include in blueprint.include.values.values_mut() {
        flatten_include(include, source);
    }
    for resource in blueprint.resources.values.values_mut() {
        flatten_resource(resource, source);
    }
    for data_source in blueprint.data_sources.values.values_mut() {
        flatten_data_source(data_source, source);
    }
    for export in blueprint.exports.values.values_mut() {
        flatten_opt_sos(&mut export.description, source);
    }
    // Variables carry only plain scalars, so don't need flattening.
}

fn flatten_value(value: &mut Value, source: &str) {
    flatten_node(&mut value.value, source);
    flatten_opt_sos(&mut value.description, source);
}

fn flatten_include(include: &mut Include, source: &str) {
    flatten_sos(&mut include.path, source);
    flatten_opt_node(&mut include.variables, source);
    flatten_opt_node(&mut include.metadata, source);
    flatten_opt_sos(&mut include.description, source);
}

fn flatten_resource(resource: &mut Resource, source: &str) {
    flatten_node(&mut resource.spec, source);
    flatten_opt_sos(&mut resource.description, source);
    flatten_opt_sos(&mut resource.each, source);
    if let Some(metadata) = &mut resource.metadata {
        flatten_resource_metadata(metadata, source);
    }
    if let Some(condition) = &mut resource.condition {
        flatten_condition(condition, source);
    }
}

fn flatten_resource_metadata(metadata: &mut Metadata, source: &str) {
    flatten_opt_sos(&mut metadata.display_name, source);
    flatten_opt_sos_map(&mut metadata.annotations, source);
    flatten_opt_node(&mut metadata.custom, source);
}

fn flatten_data_source(data_source: &mut DataSource, source: &str) {
    flatten_opt_sos(&mut data_source.description, source);
    if let Some(metadata) = &mut data_source.metadata {
        flatten_data_source_metadata(metadata, source);
    }
    for filter in &mut data_source.filters {
        for search in &mut filter.search {
            flatten_sos(search, source);
        }
    }
    for export in data_source.exports.exports.values.values_mut() {
        flatten_opt_sos(&mut export.description, source);
    }
}

fn flatten_data_source_metadata(metadata: &mut DataSourceMetadata, source: &str) {
    flatten_opt_sos(&mut metadata.display_name, source);
    flatten_opt_sos_map(&mut metadata.annotations, source);
    flatten_opt_node(&mut metadata.custom, source);
}

fn flatten_condition(condition: &mut Condition, source: &str) {
    match condition {
        Condition::Expr(parts) => flatten_sos(parts, source),
        Condition::And(conditions) | Condition::Or(conditions) => {
            for condition in conditions {
                flatten_condition(condition, source);
            }
        }
        Condition::Not(condition) => flatten_condition(condition, source),
    }
}

fn flatten_opt_node(node: &mut Option<MappingNode>, source: &str) {
    if let Some(node) = node {
        flatten_node(node, source);
    }
}

fn flatten_node(node: &mut MappingNode, source: &str) {
    match node {
        MappingNode::StringWithSubstitutions(parts) => {
            let text = render(parts, source);
            let span = parts.span;
            *node = MappingNode::Scalar(Scalar {
                value: ScalarValue::String(text),
                span,
            });
        }
        MappingNode::Fields { fields, .. } => {
            for field in fields.values_mut() {
                flatten_node(field, source);
            }
        }
        MappingNode::Items { items, .. } => {
            for item in items {
                flatten_node(item, source);
            }
        }
        MappingNode::Scalar(_) | MappingNode::None => {}
    }
}

fn flatten_opt_sos(parts: &mut Option<StringOrSubstitutions>, source: &str) {
    if let Some(parts) = parts {
        flatten_sos(parts, source);
    }
}

fn flatten_opt_sos_map(map: &mut Option<NamedMap<StringOrSubstitutions>>, source: &str) {
    if let Some(map) = map {
        for parts in map.values.values_mut() {
            flatten_sos(parts, source);
        }
    }
}

fn flatten_sos(parts: &mut StringOrSubstitutions, source: &str) {
    // A lone scalar literal (e.g. a boolean annotation value) is kept structured
    // so the consumer can recover its type rather than receiving a stringified
    // form; genuine substitutions and mixed strings collapse to text.
    if let [StringOrSubstitution::Substitution(substitution)] = parts.values.as_slice() {
        if is_scalar_literal(&substitution.kind) {
            return;
        }
    }
    let text = render(parts, source);
    parts.values = vec![StringOrSubstitution::String {
        value: text,
        span: parts.span,
    }];
}

/// Reconstructs the source text of a string-or-substitutions sequence: literal
/// runs verbatim, a scalar-literal substitution as its bare value, and any other
/// substitution as `${` + its exact source slice + `}`.
fn render(parts: &StringOrSubstitutions, source: &str) -> String {
    let mut text = String::new();
    for part in &parts.values {
        match part {
            StringOrSubstitution::String { value, .. } => text.push_str(value),
            StringOrSubstitution::Substitution(substitution) => {
                if is_scalar_literal(&substitution.kind) {
                    // A scalar literal is not a real substitution; emit its bare
                    // value without the `${..}` wrapper.
                    text.push_str(slice(source, substitution.span));
                } else {
                    text.push_str("${");
                    text.push_str(slice(source, substitution.span));
                    text.push('}');
                }
            }
        }
    }
    text
}

/// Reports whether a substitution is a bare scalar literal (an `int`, `float`,
/// or `bool` value that the canonical model wraps in a substitution when it
/// appears in a string-or-substitutions position).
fn is_scalar_literal(kind: &SubstitutionKind) -> bool {
    matches!(
        kind,
        SubstitutionKind::Int(_) | SubstitutionKind::Float(_) | SubstitutionKind::Bool(_)
    )
}

fn slice(source: &str, span: Option<Span>) -> &str {
    let Some(Span {
        start,
        end: Some(end),
    }) = span
    else {
        return "";
    };
    &source[offset(source, start)..offset(source, end)]
}

/// Converts a 1-indexed line/column position to a byte offset. Columns count
/// characters (the lexer is a rune scanner), so the conversion stays UTF-8 safe.
fn offset(source: &str, position: Position) -> usize {
    let mut line = 1;
    let mut column = 1;
    for (byte, character) in source.char_indices() {
        if line == position.line && column == position.column {
            return byte;
        }
        if character == '\n' {
            line += 1;
            column = 1;
        } else {
            column += 1;
        }
    }
    source.len()
}
