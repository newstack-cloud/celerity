//! Behaviour of `SubstitutionMode::RawText`: parsing with `${..}` substitutions
//! kept as their exact source text instead of structured nodes.
//!
//! These exercise the public `parse_string_with_options` entry point (including
//! its newline normalisation), so they live here as parser-behaviour tests
//! rather than as unit tests of the internal flattening pass.

use celerity_blueprint_lang::mapping::MappingNode;
use celerity_blueprint_lang::scalar::{Scalar, ScalarValue};
use celerity_blueprint_lang::substitution::StringOrSubstitution;
use celerity_blueprint_lang::{
    parse_string_with_options, Blueprint, ParseOptions, SubstitutionMode,
};
use pretty_assertions::assert_eq;

/// Parses `source` in `RawText` mode, asserting success.
fn raw_text(source: &str) -> Blueprint {
    parse_string_with_options(
        source,
        ParseOptions {
            substitutions: SubstitutionMode::RawText,
        },
    )
    .unwrap_or_else(|e| panic!("parse failed:\n{e}"))
}

fn spec_field<'a>(blueprint: &'a Blueprint, resource: &str, field: &str) -> &'a MappingNode {
    match &blueprint.resources.get(resource).expect("resource").spec {
        MappingNode::Fields { fields, .. } => fields.get(field).expect("field"),
        other => panic!("expected a fields spec, got {other:?}"),
    }
}

fn spec_string<'a>(blueprint: &'a Blueprint, resource: &str, field: &str) -> &'a str {
    match spec_field(blueprint, resource, field) {
        MappingNode::Scalar(Scalar {
            value: ScalarValue::String(text),
            ..
        }) => text,
        other => panic!("expected a string scalar, got {other:?}"),
    }
}

#[test]
fn test_bare_reference_becomes_dollar_brace_text() {
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             spec { name = variables.bucketName }\n\
         }",
    );
    assert_eq!(
        spec_string(&blueprint, "store", "name"),
        "${variables.bucketName}"
    );
}

#[test]
fn test_interpolated_string_round_trips_verbatim() {
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             spec { name = \"${variables.env}-orders\" }\n\
         }",
    );
    assert_eq!(
        spec_string(&blueprint, "store", "name"),
        "${variables.env}-orders"
    );
}

#[test]
fn test_operator_keeps_original_source_not_desugared_form() {
    // Proves the text is sliced from source rather than rendered from the
    // desugared structured substitution, which would read `${and(...)}`.
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             spec { enabled = variables.a && variables.b }\n\
         }",
    );
    assert_eq!(
        spec_string(&blueprint, "store", "enabled"),
        "${variables.a && variables.b}"
    );
}

#[test]
fn test_non_substitution_scalars_are_left_untouched() {
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             spec { count = 1024 }\n\
         }",
    );
    assert!(matches!(
        spec_field(&blueprint, "store", "count"),
        MappingNode::Scalar(Scalar {
            value: ScalarValue::Int(1024),
            ..
        })
    ));
}

#[test]
fn test_nested_fields_are_flattened_recursively() {
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             spec { tags = { team = variables.team } }\n\
         }",
    );
    match spec_field(&blueprint, "store", "tags") {
        MappingNode::Fields { fields, .. } => assert!(matches!(
            fields.get("team"),
            Some(MappingNode::Scalar(Scalar {
                value: ScalarValue::String(text),
                ..
            })) if text == "${variables.team}"
        )),
        other => panic!("expected nested fields, got {other:?}"),
    }
}

#[test]
fn test_string_or_substitutions_field_collapses_to_one_literal_part() {
    let blueprint = raw_text(
        "version \"2025-11-02\"\n\
         resource store: celerity/bucket {\n\
             description = \"${variables.env} bucket\"\n\
             spec {}\n\
         }",
    );
    let description = blueprint
        .resources
        .get("store")
        .unwrap()
        .description
        .as_ref()
        .expect("description");
    assert_eq!(description.values.len(), 1);
    assert!(matches!(
        &description.values[0],
        StringOrSubstitution::String { value, .. } if value == "${variables.env} bucket"
    ));
}

#[test]
fn test_crlf_line_endings_are_normalised_before_slicing() {
    // The entry point normalises `\r\n` to `\n` before parsing and slicing. A
    // substitution spanning a CRLF boundary must come back with no stray
    // carriage return; without normalisation the slice would retain the `\r`
    // from the original line ending.
    let blueprint = raw_text(
        "version \"2025-11-02\"\r\n\
         resource store: celerity/bucket {\r\n\
             spec {\r\n\
                 enabled = variables.a &&\r\n\
                 variables.b\r\n\
             }\r\n\
         }",
    );
    let text = spec_string(&blueprint, "store", "enabled");
    assert!(
        !text.contains('\r'),
        "unexpected carriage return in {text:?}"
    );
    assert_eq!(text, "${variables.a &&\nvariables.b}");
}
