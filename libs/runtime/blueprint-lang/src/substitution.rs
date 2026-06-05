//! The substitution model: the reference / function-call / operator expression
//! language shared with the YAML and JWCC formats, the contents of a `${..}`
//! placeholder.

use serde::{Deserialize, Serialize};

use crate::source::Span;

/// A value that is either literal text or a `${..}` substitution. A string
/// field that mixes the two (e.g. `"${values.prefix}-orders"`) is a sequence of
/// these parts.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum StringOrSubstitution {
    /// A literal run of text, with the span it occupies in the source.
    String {
        value: String,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        span: Option<Span>,
    },
    /// An interpolated `${..}` substitution (which carries its own span).
    Substitution(Substitution),
}

/// A sequence of literal-text and substitution parts. A field whose value is a
/// bare expression transforms to a single-part sequence; an interpolated string
/// transforms to one part per literal run and `${..}`.
#[derive(Debug, Clone, PartialEq, Default, Serialize, Deserialize)]
pub struct StringOrSubstitutions {
    pub values: Vec<StringOrSubstitution>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

impl StringOrSubstitutions {
    /// Creates a sequence from its parts, with no overall span.
    pub fn new(values: Vec<StringOrSubstitution>) -> Self {
        Self { values, span: None }
    }
}

/// A parsed `${..}` substitution: a [`SubstitutionKind`] plus the span of the
/// whole substitution.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Substitution {
    pub kind: SubstitutionKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

impl Substitution {
    /// Creates a substitution carrying a source span.
    pub fn new(kind: SubstitutionKind, span: Span) -> Self {
        Self {
            kind,
            span: Some(span),
        }
    }

    /// Creates a substitution with no source span (useful in tests and
    /// synthesised values).
    pub fn untracked(kind: SubstitutionKind) -> Self {
        Self { kind, span: None }
    }
}

/// The different forms a substitution can take. Mirrors the union in
/// Go implementation. The literal variants (`String`/`Int`/`Float`/
/// `Bool`/`None`) carry substitution-internal literal values; the reference and
/// function variants carry their own structured payloads.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum SubstitutionKind {
    Function(SubstitutionFunction),
    Variable(SubstitutionVariable),
    ValueReference(SubstitutionValueReference),
    ElemReference(SubstitutionElemReference),
    ElemIndexReference,
    DataSourceProperty(SubstitutionDataSourceProperty),
    ResourceProperty(SubstitutionResourceProperty),
    Child(SubstitutionChild),
    String(String),
    Int(i64),
    Float(f64),
    Bool(bool),
    None,
}

/// `variables.<name>` - resolves to a scalar; no further accessors permitted.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionVariable {
    pub variable_name: String,
}

/// `values.<name>[.path]` - a static or computed value with an accessor path.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionValueReference {
    pub value_name: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub path: Vec<SubstitutionPathItem>,
}

/// `elem[.path]` - the current iteration element inside a `foreach`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionElemReference {
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub path: Vec<SubstitutionPathItem>,
}

/// `datasources.<name>.<field>[index]` - an exported data-source field, with an
/// optional index into a primitive array.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionDataSourceProperty {
    pub data_source_name: String,
    pub field_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub primitive_arr_index: Option<i64>,
}

/// `resources.<name>[index].path` (or the bare `<name>...` form) - a property of
/// a resource, with an optional `each` template index.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionResourceProperty {
    pub resource_name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resource_each_template_index: Option<i64>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub path: Vec<SubstitutionPathItem>,
}

/// `children.<name>.<output>[.path]` - an output of an included child blueprint.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionChild {
    pub child_name: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub path: Vec<SubstitutionPathItem>,
}

/// `name(args)[.path]` - a function call, optionally followed by accessors that
/// drill into an array/object return value.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionFunction {
    pub name: SubstitutionFunctionName,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub arguments: Vec<SubstitutionFunctionArg>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub path: Vec<SubstitutionPathItem>,
}

/// A single function-call argument. `name` is set only for a named argument
/// (`name = expr`), which is meaningful solely for the `object` core function.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SubstitutionFunctionArg {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    pub value: Box<Substitution>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub span: Option<Span>,
}

/// A single accessor in a reference path: either a named field (`.name` /
/// `["name"]`) or an array index (`[n]` / `[]`).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum SubstitutionPathItem {
    Field {
        name: String,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        span: Option<Span>,
    },
    Index {
        index: i64,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        span: Option<Span>,
    },
}

impl SubstitutionPathItem {
    /// The source span of this path item, if tracked.
    pub fn span(&self) -> Option<Span> {
        match self {
            SubstitutionPathItem::Field { span, .. } | SubstitutionPathItem::Index { span, .. } => {
                *span
            }
        }
    }
}

/// The name of a function callable in a substitution. A new type over `String`
/// because the set is open: providers contribute their own functions alongside
/// the core set.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SubstitutionFunctionName(pub String);

impl SubstitutionFunctionName {
    /// Creates a function name from any string-like value.
    pub fn new(name: impl Into<String>) -> Self {
        Self(name.into())
    }

    /// The function name as a string slice.
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl From<&str> for SubstitutionFunctionName {
    fn from(name: &str) -> Self {
        Self(name.to_string())
    }
}

/// The well-known core function names that the blueprint-language operators
/// desugar to, plus the literal-collection builders used when transforming
/// array and object literals.
pub mod function_names {
    pub const EQ: &str = "eq";
    pub const NOT: &str = "not";
    pub const AND: &str = "and";
    pub const OR: &str = "or";
    pub const GT: &str = "gt";
    pub const LT: &str = "lt";
    pub const GE: &str = "ge";
    pub const LE: &str = "le";
    pub const LIST: &str = "list";
    pub const OBJECT: &str = "object";
    pub const JOIN: &str = "join";
}

/// Renders a *reference* substitution to its canonical path string
/// (`resources.x.spec.y`, `variables["a.b"]`). Returns `None` for function or
/// literal substitutions, which are not addressable reference paths — useful for
/// fields such as an export's `field`, which must be a pure reference path.
pub fn substitution_to_path_string(sub: &Substitution) -> Option<String> {
    let mut out = String::new();
    match &sub.kind {
        SubstitutionKind::Variable(v) => {
            out.push_str("variables");
            push_name(&mut out, &v.variable_name);
        }
        SubstitutionKind::ValueReference(v) => {
            out.push_str("values");
            push_name(&mut out, &v.value_name);
            push_path(&mut out, &v.path);
        }
        SubstitutionKind::ResourceProperty(r) => {
            out.push_str("resources");
            push_name(&mut out, &r.resource_name);
            if let Some(index) = r.resource_each_template_index {
                out.push_str(&format!("[{index}]"));
            }
            push_path(&mut out, &r.path);
        }
        SubstitutionKind::DataSourceProperty(d) => {
            out.push_str("datasources");
            push_name(&mut out, &d.data_source_name);
            push_name(&mut out, &d.field_name);
            if let Some(index) = d.primitive_arr_index {
                out.push_str(&format!("[{index}]"));
            }
        }
        SubstitutionKind::Child(c) => {
            out.push_str("children");
            push_name(&mut out, &c.child_name);
            push_path(&mut out, &c.path);
        }
        SubstitutionKind::ElemReference(e) => {
            out.push_str("elem");
            push_path(&mut out, &e.path);
        }
        SubstitutionKind::ElemIndexReference => out.push('i'),
        // Function and literal substitutions are not reference paths.
        _ => return None,
    }
    Some(out)
}

/// Appends a name accessor: `.name` for a bare name, `["name"]` otherwise.
fn push_name(out: &mut String, name: &str) {
    if is_bare_name(name) {
        out.push('.');
        out.push_str(name);
    } else {
        out.push_str(&format!("[{name:?}]"));
    }
}

fn push_path(out: &mut String, path: &[SubstitutionPathItem]) {
    for item in path {
        match item {
            SubstitutionPathItem::Field { name, .. } => push_name(out, name),
            SubstitutionPathItem::Index { index, .. } => out.push_str(&format!("[{index}]")),
        }
    }
}

fn is_bare_name(name: &str) -> bool {
    let mut chars = name.chars();
    matches!(chars.next(), Some(c) if c.is_ascii_alphabetic() || c == '_')
        && chars.all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-')
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn test_resource_property_round_trips() {
        let sub = Substitution::untracked(SubstitutionKind::ResourceProperty(
            SubstitutionResourceProperty {
                resource_name: "saveOrder".into(),
                resource_each_template_index: Some(0),
                path: vec![
                    SubstitutionPathItem::Field {
                        name: "spec".into(),
                        span: None,
                    },
                    SubstitutionPathItem::Field {
                        name: "functionArn".into(),
                        span: None,
                    },
                ],
            },
        ));
        let json = serde_json::to_string(&sub).unwrap();
        let back: Substitution = serde_json::from_str(&json).unwrap();
        assert_eq!(sub, back);
    }

    #[test]
    fn test_function_with_args_round_trips() {
        let sub = Substitution::untracked(SubstitutionKind::Function(SubstitutionFunction {
            name: SubstitutionFunctionName::from(function_names::OBJECT),
            arguments: vec![SubstitutionFunctionArg {
                name: Some("host".into()),
                value: Box::new(Substitution::untracked(SubstitutionKind::String(
                    "api.example.com".into(),
                ))),
                span: None,
            }],
            path: vec![SubstitutionPathItem::Field {
                name: "host".into(),
                span: None,
            }],
        }));
        let json = serde_json::to_string(&sub).unwrap();
        let back: Substitution = serde_json::from_str(&json).unwrap();
        assert_eq!(sub, back);
    }

    #[test]
    fn test_string_or_substitutions_mixes_literal_and_substitution() {
        let parts = StringOrSubstitutions::new(vec![
            StringOrSubstitution::Substitution(Substitution::untracked(
                SubstitutionKind::Variable(SubstitutionVariable {
                    variable_name: "prefix".into(),
                }),
            )),
            StringOrSubstitution::String {
                value: "-orders".into(),
                span: None,
            },
        ]);
        let json = serde_json::to_string(&parts).unwrap();
        let back: StringOrSubstitutions = serde_json::from_str(&json).unwrap();
        assert_eq!(parts, back);
        assert_eq!(parts.values.len(), 2);
    }

    #[test]
    fn test_path_item_span_accessor() {
        let item = SubstitutionPathItem::Index {
            index: 3,
            span: None,
        };
        assert_eq!(item.span(), None);
    }
}
