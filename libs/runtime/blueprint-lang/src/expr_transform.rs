//! Transformation of the transient [`crate::parser::expr`] AST into the
//! canonical substitution / mapping / scalar types (array → `list`, object →
//! `object`, interpolation → `StringOrSubstitutions`, etc.).

use hashlink::LinkedHashMap;
use std::collections::BTreeMap;

use crate::errors::ParseError;
use crate::mapping::MappingNode;
use crate::parser::expr::{CallArg, Expr, InterpPart, ObjectField};
use crate::parser::strings::join_plain_parts;
use crate::scalar::{Scalar, ScalarValue};
use crate::schema::{Condition, NamedMap};
use crate::source::Span;
use crate::substitution::{
    function_names, StringOrSubstitution, StringOrSubstitutions, Substitution,
    SubstitutionFunction, SubstitutionFunctionArg, SubstitutionFunctionName, SubstitutionKind,
};

/// Transforms a transient expression into a canonical `MappingNode`, the shape
/// used by provider-defined field values (resource specs, `value` values,
/// custom metadata, free-form maps).
pub(crate) fn expr_to_mapping_node(e: Expr) -> MappingNode {
    match e {
        Expr::Scalar(scalar) => MappingNode::Scalar(scalar),
        Expr::None(_) => MappingNode::None,
        Expr::Array { elems, span } => MappingNode::Items {
            items: elems.into_iter().map(expr_to_mapping_node).collect(),
            span,
        },
        Expr::Object { entries, span } => object_to_fields_node(entries, span),
        Expr::Ref(sub) => substitution_to_mapping_node(sub),
        Expr::Interpolation { parts, span } => {
            MappingNode::StringWithSubstitutions(StringOrSubstitutions {
                values: interpolation_parts_to_values(parts),
                span,
            })
        }
        call_or_op @ (Expr::Call { .. } | Expr::Op { .. }) => {
            substitution_to_mapping_node(expr_to_substitution(call_or_op))
        }
    }
}

fn object_to_fields_node(entries: Vec<ObjectField>, span: Option<Span>) -> MappingNode {
    let mut fields = LinkedHashMap::new();
    let mut fields_span = BTreeMap::new();
    for entry in entries {
        if let Some(s) = entry.span {
            fields_span.insert(entry.key.clone(), s);
        }
        fields.insert(entry.key, expr_to_mapping_node(entry.value));
    }
    MappingNode::Fields {
        fields,
        fields_span,
        span,
    }
}

/// A single substitution becomes a one-part `StringWithSubstitutions` node.
fn substitution_to_mapping_node(sub: Substitution) -> MappingNode {
    let span = sub.span;
    MappingNode::StringWithSubstitutions(StringOrSubstitutions {
        values: vec![StringOrSubstitution::Substitution(sub)],
        span,
    })
}

pub(crate) fn expr_to_substitution(e: Expr) -> Substitution {
    match e {
        Expr::Ref(sub) => sub,
        Expr::Scalar(scalar) => scalar_to_substitution(scalar),
        Expr::None(span) => Substitution {
            kind: SubstitutionKind::None,
            span,
        },
        Expr::Call {
            name,
            args,
            path,
            span,
        } => function_substitution(name, call_args_to_fn_args(args), path, span),
        Expr::Op {
            fn_name,
            args,
            span,
        } => function_substitution(fn_name, positional_fn_args(args), vec![], span),
        Expr::Array { elems, span } => function_substitution(
            SubstitutionFunctionName::new(function_names::LIST),
            positional_fn_args(elems),
            vec![],
            span,
        ),
        Expr::Object { entries, span } => function_substitution(
            SubstitutionFunctionName::new(function_names::OBJECT),
            object_entries_to_fn_args(entries),
            vec![],
            span,
        ),
        Expr::Interpolation { parts, span } => interpolation_to_substitution(parts, span),
    }
}

fn function_substitution(
    name: SubstitutionFunctionName,
    arguments: Vec<SubstitutionFunctionArg>,
    path: Vec<crate::substitution::SubstitutionPathItem>,
    span: Option<Span>,
) -> Substitution {
    Substitution {
        kind: SubstitutionKind::Function(SubstitutionFunction {
            name,
            arguments,
            path,
        }),
        span,
    }
}

fn scalar_to_substitution(scalar: Scalar) -> Substitution {
    let kind = match scalar.value {
        ScalarValue::String(s) => SubstitutionKind::String(s),
        ScalarValue::Int(i) => SubstitutionKind::Int(i),
        ScalarValue::Float(f) => SubstitutionKind::Float(f),
        ScalarValue::Bool(b) => SubstitutionKind::Bool(b),
    };
    Substitution {
        kind,
        span: scalar.span,
    }
}

fn call_args_to_fn_args(args: Vec<CallArg>) -> Vec<SubstitutionFunctionArg> {
    args.into_iter()
        .map(|a| SubstitutionFunctionArg {
            name: a.name,
            value: Box::new(expr_to_substitution(a.value)),
            span: a.span,
        })
        .collect()
}

fn positional_fn_args(exprs: Vec<Expr>) -> Vec<SubstitutionFunctionArg> {
    exprs
        .into_iter()
        .map(|e| SubstitutionFunctionArg {
            name: None,
            value: Box::new(expr_to_substitution(e)),
            span: None,
        })
        .collect()
}

fn object_entries_to_fn_args(entries: Vec<ObjectField>) -> Vec<SubstitutionFunctionArg> {
    entries
        .into_iter()
        .map(|entry| SubstitutionFunctionArg {
            name: Some(entry.key),
            value: Box::new(expr_to_substitution(entry.value)),
            span: entry.span,
        })
        .collect()
}

fn interpolation_parts_to_values(parts: Vec<InterpPart>) -> Vec<StringOrSubstitution> {
    parts
        .into_iter()
        .map(|part| match part {
            InterpPart::String { value, span } => StringOrSubstitution::String { value, span },
            InterpPart::Sub { value, .. } => {
                StringOrSubstitution::Substitution(expr_to_substitution(value))
            }
        })
        .collect()
}

/// Inside a substitution position, an interpolated string desugars to
/// `join(list(parts...), "")`; a plain (all-literal) string is just a string.
/// For example, `${trimprefix("orders-${variables.environment}-queue", "orders-")}`
/// desugars to `${trimprefix(join(list("orders-", variables.environment, "-queue"), ""), "orders-")}`.
fn interpolation_to_substitution(parts: Vec<InterpPart>, span: Option<Span>) -> Substitution {
    if let Some(text) = join_plain_parts(&parts) {
        return Substitution {
            kind: SubstitutionKind::String(text),
            span,
        };
    }

    let list = function_substitution(
        SubstitutionFunctionName::new(function_names::LIST),
        parts
            .into_iter()
            .map(|p| SubstitutionFunctionArg {
                name: None,
                value: Box::new(interpolation_part_to_substitution(p)),
                span: None,
            })
            .collect(),
        vec![],
        None,
    );

    let empty = Substitution {
        kind: SubstitutionKind::String(String::new()),
        span: None,
    };

    function_substitution(
        SubstitutionFunctionName::new(function_names::JOIN),
        vec![
            SubstitutionFunctionArg {
                name: None,
                value: Box::new(list),
                span: None,
            },
            SubstitutionFunctionArg {
                name: None,
                value: Box::new(empty),
                span: None,
            },
        ],
        vec![],
        span,
    )
}

fn interpolation_part_to_substitution(part: InterpPart) -> Substitution {
    match part {
        InterpPart::String { value, span } => Substitution {
            kind: SubstitutionKind::String(value),
            span,
        },
        InterpPart::Sub { value, .. } => expr_to_substitution(value),
    }
}

/// For schema positions the canonical model expresses as "a string with optional
/// substitutions" (filter search values, condition string form, dependsOn, …).
pub(crate) fn expr_to_string_or_substitutions(e: Expr) -> StringOrSubstitutions {
    match e {
        // A plain string scalar is a single literal part.
        Expr::Scalar(Scalar {
            value: ScalarValue::String(s),
            span,
        }) => StringOrSubstitutions {
            values: vec![StringOrSubstitution::String { value: s, span }],
            span,
        },
        Expr::Interpolation { parts, span } => StringOrSubstitutions {
            values: interpolation_parts_to_values(parts),
            span,
        },
        // Everything else (non-string scalars, refs, calls, ops, arrays, objects)
        // becomes a single substitution part.
        other => {
            let span = other.span();
            StringOrSubstitutions {
                values: vec![StringOrSubstitution::Substitution(expr_to_substitution(
                    other,
                ))],
                span,
            }
        }
    }
}

/// A resource condition: a bare boolean expression (string form), or an object
/// literal with exactly one of `and` / `or` / `not` (structural form).
pub(crate) fn expr_to_condition(e: Expr) -> Result<Condition, ParseError> {
    match e {
        Expr::Object { entries, span } => object_to_condition(entries, span),
        other => Ok(Condition::Expr(expr_to_string_or_substitutions(other))),
    }
}

fn object_to_condition(
    entries: Vec<ObjectField>,
    span: Option<Span>,
) -> Result<Condition, ParseError> {
    if entries.len() != 1 {
        return Err(ParseError {
            message: "condition object must have exactly one of 'and', 'or', or 'not'".into(),
            span,
        });
    }

    let entry = entries.into_iter().next().expect("len == 1");
    match entry.key.as_str() {
        "and" => Ok(Condition::And(condition_list(entry.value)?)),
        "or" => Ok(Condition::Or(condition_list(entry.value)?)),
        "not" => Ok(Condition::Not(Box::new(expr_to_condition(entry.value)?))),
        unknown => Err(ParseError {
            message: format!(
                "unknown condition object field {unknown:?}: expected 'and', 'or', or 'not'"
            ),
            span: entry.span,
        }),
    }
}

fn condition_list(e: Expr) -> Result<Vec<Condition>, ParseError> {
    match e {
        Expr::Array { elems, .. } => elems.into_iter().map(expr_to_condition).collect(),
        other => Err(ParseError {
            message: "'and' and 'or' condition entries must be an array".into(),
            span: other.span(),
        }),
    }
}

/// `labels` / `byLabel`: every value must be a plain string scalar.
pub(crate) fn object_to_string_map(
    entries: Vec<ObjectField>,
) -> Result<NamedMap<String>, ParseError> {
    let mut map = NamedMap::new();
    for entry in entries {
        let value = match entry.value {
            Expr::Scalar(Scalar {
                value: ScalarValue::String(s),
                ..
            }) => s,
            other => {
                return Err(ParseError {
                    message: format!("expected a string value for key {:?}", entry.key),
                    span: other.span(),
                })
            }
        };
        map.insert(entry.key, value, entry.span);
    }
    Ok(map)
}

/// `annotations`: each value is a string-with-substitutions.
pub(crate) fn object_to_string_or_substitutions_map(
    entries: Vec<ObjectField>,
) -> NamedMap<StringOrSubstitutions> {
    let mut map = NamedMap::new();
    for entry in entries {
        map.insert(
            entry.key,
            expr_to_string_or_substitutions(entry.value),
            entry.span,
        );
    }
    map
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::parser::core::Parser;
    use pretty_assertions::assert_eq;

    fn parse(src: &str) -> Expr {
        Parser::new(src)
            .parse_expr()
            .unwrap_or_else(|e| panic!("parse failed for {src:?}: {}", e.message))
    }

    fn mapping(src: &str) -> MappingNode {
        expr_to_mapping_node(parse(src))
    }

    fn sub(src: &str) -> Substitution {
        expr_to_substitution(parse(src))
    }

    fn sos(src: &str) -> StringOrSubstitutions {
        expr_to_string_or_substitutions(parse(src))
    }

    fn object_entries(src: &str) -> Vec<ObjectField> {
        match parse(src) {
            Expr::Object { entries, .. } => entries,
            other => panic!("expected an object literal, got {other:?}"),
        }
    }

    /// Asserts a substitution is a function call and returns its function.
    fn as_function(sub: Substitution) -> SubstitutionFunction {
        match sub.kind {
            SubstitutionKind::Function(f) => f,
            other => panic!("expected a function substitution, got {other:?}"),
        }
    }

    #[test]
    fn test_scalar_and_none_to_mapping_node() {
        assert!(matches!(
            mapping("5"),
            MappingNode::Scalar(Scalar {
                value: ScalarValue::Int(5),
                ..
            })
        ));
        // `none` has no scalar form in our model, so it becomes the None node.
        assert!(matches!(mapping("none"), MappingNode::None));
    }

    #[test]
    fn test_ref_and_op_to_mapping_node_wrap_as_substitution() {
        assert!(matches!(
            mapping("variables.x"),
            MappingNode::StringWithSubstitutions(_)
        ));
        match mapping("variables.a == variables.b") {
            MappingNode::StringWithSubstitutions(parts) => assert!(matches!(
                &parts.values[0],
                StringOrSubstitution::Substitution(Substitution {
                    kind: SubstitutionKind::Function(f),
                    ..
                }) if f.name.as_str() == "eq"
            )),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_array_and_object_to_mapping_node() {
        assert!(matches!(mapping("[1, 2]"), MappingNode::Items { items, .. } if items.len() == 2));
        match mapping("{ a = 1, b = true }") {
            MappingNode::Fields { fields, .. } => assert_eq!(fields.len(), 2),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_interpolation_to_mapping_node_keeps_parts() {
        match mapping(r#""a-${variables.x}""#) {
            MappingNode::StringWithSubstitutions(parts) => assert_eq!(parts.values.len(), 2),
            other => panic!("{other:?}"),
        }
    }

    #[test]
    fn test_scalar_to_substitution_carries_value() {
        assert!(matches!(sub("5").kind, SubstitutionKind::Int(5)));
        assert!(matches!(sub("true").kind, SubstitutionKind::Bool(true)));
        assert!(matches!(sub(r#""hi""#).kind, SubstitutionKind::String(s) if s == "hi"));
        assert!(matches!(sub("none").kind, SubstitutionKind::None));
    }

    #[test]
    fn test_reference_passes_through_unchanged() {
        assert!(matches!(
            sub("variables.region").kind,
            SubstitutionKind::Variable(_)
        ));
    }

    #[test]
    fn test_function_call_to_substitution() {
        let f = as_function(sub("max(variables.a, variables.b)"));
        assert_eq!(f.name.as_str(), "max");
        assert_eq!(f.arguments.len(), 2);
        assert!(f.arguments.iter().all(|a| a.name.is_none()));
    }

    #[test]
    fn test_array_and_object_desugar_to_list_and_object_functions() {
        let list = as_function(sub("[1, 2]"));
        assert_eq!(list.name.as_str(), "list");
        assert_eq!(list.arguments.len(), 2);

        let object = as_function(sub("{ a = variables.x }"));
        assert_eq!(object.name.as_str(), "object");
        assert_eq!(object.arguments[0].name.as_deref(), Some("a"));
    }

    #[test]
    fn test_operator_desugars_to_function() {
        let f = as_function(sub("variables.a && variables.b"));
        assert_eq!(f.name.as_str(), "and");
        assert_eq!(f.arguments.len(), 2);
    }

    #[test]
    fn test_plain_interpolation_string_to_substitution_is_a_string() {
        // An all-literal interpolation collapses to a plain string substitution.
        assert!(matches!(
            sub("\"\"\"\n    hello\n    \"\"\"").kind,
            SubstitutionKind::String(s) if s == "hello"
        ));
    }

    #[test]
    fn test_interpolation_to_substitution_desugars_to_join_of_list() {
        let join = as_function(sub(r#""a-${variables.x}""#));
        assert_eq!(join.name.as_str(), "join");
        assert_eq!(join.arguments.len(), 2);
        // First arg is list(parts...), second is the empty separator string.
        let list = as_function((*join.arguments[0].value).clone());
        assert_eq!(list.name.as_str(), "list");
        assert!(matches!(
            join.arguments[1].value.kind,
            SubstitutionKind::String(ref s) if s.is_empty()
        ));
    }

    #[test]
    fn test_plain_string_to_sos_is_a_literal_part() {
        let parts = sos(r#""hi""#);
        assert_eq!(parts.values.len(), 1);
        assert!(
            matches!(&parts.values[0], StringOrSubstitution::String { value, .. } if value == "hi")
        );
    }

    #[test]
    fn test_interpolation_to_sos_keeps_literal_and_sub_parts() {
        let parts = sos(r#""a-${variables.x}""#);
        assert_eq!(parts.values.len(), 2);
        assert!(
            matches!(&parts.values[0], StringOrSubstitution::String { value, .. } if value == "a-")
        );
        assert!(matches!(
            &parts.values[1],
            StringOrSubstitution::Substitution(_)
        ));
    }

    #[test]
    fn test_non_string_expr_to_sos_becomes_a_single_substitution() {
        // A bare int / reference becomes a one-part substitution sequence.
        let parts = sos("5");
        assert_eq!(parts.values.len(), 1);
        assert!(matches!(
            &parts.values[0],
            StringOrSubstitution::Substitution(Substitution {
                kind: SubstitutionKind::Int(5),
                ..
            })
        ));
        assert!(matches!(
            &sos("variables.x").values[0],
            StringOrSubstitution::Substitution(_)
        ));
    }

    #[test]
    fn test_condition_string_and_structural_forms() {
        assert!(matches!(
            expr_to_condition(parse("values.isProd")).unwrap(),
            Condition::Expr(_)
        ));
        match expr_to_condition(parse("{ and = [values.a, values.b] }")).unwrap() {
            Condition::And(conditions) => assert_eq!(conditions.len(), 2),
            other => panic!("{other:?}"),
        }
        assert!(matches!(
            expr_to_condition(parse("{ not = values.a }")).unwrap(),
            Condition::Not(_)
        ));
    }

    #[test]
    fn test_condition_object_validation_errors() {
        // More than one key, an unknown key, and a non-array and/or are rejected.
        assert!(expr_to_condition(parse("{ and = [], or = [] }")).is_err());
        assert!(expr_to_condition(parse("{ xyz = values.a }")).is_err());
        assert!(expr_to_condition(parse("{ and = values.a }")).is_err());
    }

    #[test]
    fn test_object_to_string_map_requires_string_values() {
        let map =
            object_to_string_map(object_entries(r#"{ service = "ordersApi", tier = "web" }"#))
                .unwrap();
        assert_eq!(map.len(), 2);
        assert_eq!(map.get("service").map(String::as_str), Some("ordersApi"));
        // A non-string value is rejected.
        assert!(object_to_string_map(object_entries("{ service = 1 }")).is_err());
    }

    #[test]
    fn test_object_to_string_or_substitutions_map() {
        let map = object_to_string_or_substitutions_map(object_entries(
            r#"{ "aws.populateEnvVars" = true, region = variables.region }"#,
        ));
        assert_eq!(map.len(), 2);
        assert!(map.get("region").is_some());
    }
}
