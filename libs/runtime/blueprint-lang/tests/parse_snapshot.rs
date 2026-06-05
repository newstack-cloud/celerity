//! AST and error-diagnostic parity with the Go reference suite.
//!
//! Each `.bp` fixture under `tests/fixtures/` is parsed and its full
//! `Blueprint` AST with spans included is captured as an `insta` snapshot.

use celerity_blueprint_lang::parse_string;

/// Reads a fixture's source from `tests/fixtures/<name>.bp` (cwd is the crate
/// root when tests run).
fn fixture(name: &str) -> String {
    let path = format!("tests/fixtures/{name}.bp");
    std::fs::read_to_string(&path).unwrap_or_else(|e| panic!("reading {path}: {e}"))
}

/// Parses a fixture, asserting success, and snapshots the `Debug` of the
/// resulting `Blueprint` (spans included) under the fixture's name.
fn snapshot_fixture(name: &str) {
    let src = fixture(name);
    let bp =
        parse_string(&src).unwrap_or_else(|e| panic!("fixture {name:?} failed to parse:\n{e}"));
    insta::assert_snapshot!(name, format!("{bp:#?}"));
}

/// Parses each fixture in `names`, snapshotting each AST.
fn snapshot_fixtures(names: &[&str]) {
    for name in names {
        snapshot_fixture(name);
    }
}

/// Parses `src`, asserting it fails, and snapshots the rendered diagnostic.
fn snapshot_error(name: &str, src: &str) {
    match parse_string(src) {
        Ok(_) => panic!("error case {name:?} parsed successfully, expected a diagnostic"),
        Err(errs) => insta::assert_snapshot!(name, errs.to_string()),
    }
}

// --- success fixtures ----------

#[test]
fn parses_directive_fixtures() {
    snapshot_fixtures(&["version-only", "transform-single", "transform-multiple"]);
}

#[test]
fn parses_variable_fixtures() {
    snapshot_fixtures(&[
        "variable-string",
        "variable-integer",
        "variable-float",
        "variable-boolean",
        "variable-secret",
        "variable-element-types",
        "variable-empty-block",
        "variable-separators",
        "variable-quoted-name",
        "variable-multiple",
        "variable-multiline-description",
    ]);
}

#[test]
fn parses_value_fixtures() {
    snapshot_fixtures(&[
        "value-string",
        "value-array",
        "value-object",
        "value-expression",
        "value-call",
    ]);
}

#[test]
fn parses_export_fixtures() {
    snapshot_fixtures(&[
        "export-bare-ref",
        "export-string-form",
        "export-each-type",
        "export-with-description",
    ]);
}

#[test]
fn parses_include_fixtures() {
    snapshot_fixtures(&["include-minimal", "include-with-description", "include-all"]);
}

#[test]
fn parses_metadata_fixtures() {
    snapshot_fixtures(&["metadata-dotted-keys", "metadata-nested"]);
}

#[test]
fn parses_data_fixtures() {
    snapshot_fixtures(&[
        "data-minimal",
        "data-metadata",
        "data-filter-equality",
        "data-filter-comparison",
        "data-filter-collection",
        "data-filter-text",
        "data-export-forms",
        "data-description",
    ]);
}

#[test]
fn parses_resource_fixtures() {
    snapshot_fixtures(&[
        "resource-minimal",
        "resource-condition-bare",
        "resource-condition-object",
        "resource-metadata-full",
        "resource-select",
        "resource-select-exclude",
        "resource-dependson",
        "resource-removal-policy",
        "resource-foreach",
        "resource-spec-complex",
    ]);
}

#[test]
fn parses_expression_fixtures() {
    snapshot_fixtures(&[
        "expr-precedence",
        "expr-multiline-ops",
        "expr-function-call",
        "expr-multiline-string",
        "expr-resource-path",
        "expr-resource-quoted-accessor",
        "expr-none-literal",
    ]);
}

#[test]
fn parses_lexical_fixtures() {
    snapshot_fixtures(&["comments-basic"]);
}

// --- error cases ----------

#[test]
fn reports_directive_errors() {
    snapshot_error(
        "duplicate-version",
        "version \"2025-11-02\"\nversion \"2025-11-02\"",
    );
    snapshot_error(
        "transform-interpolation",
        "transform \"${variables.region}\"",
    );
    snapshot_error(
        "empty-transform-list",
        "version \"2025-11-02\"\ntransform []",
    );
    snapshot_error("unexpected-top-level-token", "notAKeyword");
    snapshot_error("multiline-version", "version \"\"\"\n2025-11-02\n\"\"\"");
    snapshot_error("transform-not-a-string", "transform 123");
    snapshot_error("missing-version", "variable region: string {}");
}

#[test]
fn reports_name_and_type_errors() {
    snapshot_error("reserved-word-as-name", "variable resource: string {}");
    snapshot_error("invalid-quoted-name", "variable \"with space\": string {}");
    snapshot_error("invalid-type-segment", "variable instance: aws-x/ec2 {}");
    snapshot_error("element-type-single-segment", "variable instance: aws {}");
}

#[test]
fn reports_variable_errors() {
    snapshot_error(
        "variable-unknown-field",
        "variable region: string { foo = \"bar\" }",
    );
    snapshot_error(
        "variable-missing-assign",
        "variable region: string { default \"us-east-1\" }",
    );
    snapshot_error(
        "variable-non-scalar-default",
        "variable region: string { default = [\"a\"] }",
    );
    snapshot_error(
        "variable-allowedvalues-not-array",
        "variable region: string { allowedValues = \"a\" }",
    );
    snapshot_error(
        "variable-unterminated-block",
        "variable region: string { default = \"us-east-1\"",
    );
    snapshot_error(
        "variable-secret-not-bool",
        "variable region: string { secret = \"yes\" }",
    );
    snapshot_error(
        "variable-description-not-string",
        "version \"2025-11-02\"\nvariable region: string { description = 42 }",
    );
    snapshot_error(
        "variable-boolean-allowedvalues",
        "version \"2025-11-02\"\nvariable flag: boolean { allowedValues = [true, false] }",
    );
}

#[test]
fn reports_value_errors() {
    snapshot_error("value-unknown-field", "value x: string { foo = \"bar\" }");
    snapshot_error("value-bad-type", "value x: notAType { value = \"x\" }");
}

#[test]
fn reports_export_errors() {
    snapshot_error("export-unknown-field", "export x: string { foo = \"bar\" }");
    snapshot_error(
        "export-fn-call",
        "export x: string { field = jsonencode(variables.x) }",
    );
    snapshot_error(
        "export-bad-type",
        "export x: notAType { field = variables.x }",
    );
}

#[test]
fn reports_include_errors() {
    snapshot_error("include-missing-path", "include child {}");
    snapshot_error(
        "include-unknown-field",
        "include child \"a.yaml\" { foo = \"bar\" }",
    );
    snapshot_error("include-unterminated", "include child \"a.yaml\" {");
}

#[test]
fn reports_metadata_errors() {
    snapshot_error("metadata-missing-assign", "metadata { foo \"bar\" }");
    snapshot_error(
        "metadata-duplicate",
        "version \"2025-11-02\"\nmetadata { a = 1 }\nmetadata { b = 2 }",
    );
}

#[test]
fn reports_data_errors() {
    snapshot_error(
        "data-export-type-object",
        "data n: aws/vpc { filter \"x\" == \"y\"\nexport id: object }",
    );
    snapshot_error(
        "data-not-before-eq",
        "data n: aws/vpc { filter \"x\" not == \"y\"\nexport id: string }",
    );
    snapshot_error(
        "data-has-without-key",
        "data n: aws/vpc { filter \"x\" has \"y\"\nexport id: string }",
    );
    snapshot_error("data-unknown-field", "data n: aws/vpc { foo = \"bar\" }");
}

#[test]
fn reports_resource_errors() {
    snapshot_error(
        "resource-condition-two-keys",
        "resource r: aws/x { condition = { and = [variables.x], or = [variables.y] }\nspec {} }",
    );
    snapshot_error(
        "resource-labels-non-string",
        "resource r: aws/x { metadata { labels = { service = 42 } }\nspec {} }",
    );
    snapshot_error(
        "resource-removal-policy-non-literal",
        "resource r: aws/x { removalPolicy = variables.policy\nspec {} }",
    );
    snapshot_error(
        "resource-removal-policy-invalid",
        "version \"2025-11-02\"\nresource r: aws/x { removalPolicy = \"destroy\"\nspec {} }",
    );
    snapshot_error(
        "resource-spec-missing",
        "version \"2025-11-02\"\nresource r: aws/x { description = \"desc\" }",
    );
    snapshot_error(
        "resource-select-missing-by",
        "resource r: aws/x { select { service = \"ordersApi\" }\nspec {} }",
    );
    snapshot_error(
        "resource-unknown-field",
        "resource r: aws/x { foo = \"bar\"\nspec {} }",
    );
}

#[test]
fn reports_expression_errors() {
    snapshot_error(
        "expr-dangling-op",
        "value x: boolean { value = variables.a && }",
    );
    snapshot_error(
        "expr-unterminated-call",
        "value x: object { value = object(",
    );
}
