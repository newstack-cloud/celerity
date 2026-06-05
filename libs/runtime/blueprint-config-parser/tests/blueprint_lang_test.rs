use celerity_blueprint_config_parser::blueprint::*;
use celerity_helpers::env::EnvVars;
use std::{collections::HashMap, env::VarError, sync::Arc};

struct MockEnvVars {
    vars: Arc<HashMap<String, String>>,
}

impl MockEnvVars {
    fn new() -> Self {
        Self {
            vars: Arc::new(HashMap::new()),
        }
    }

    fn with(pairs: &[(&str, &str)]) -> Self {
        let vars = pairs
            .iter()
            .map(|(key, value)| (key.to_string(), value.to_string()))
            .collect();
        Self {
            vars: Arc::new(vars),
        }
    }
}

impl EnvVars for MockEnvVars {
    fn var(&self, key: &str) -> Result<String, VarError> {
        self.vars.get(key).ok_or(VarError::NotPresent).cloned()
    }

    fn clone_env_vars(&self) -> Box<dyn EnvVars> {
        Box::new(MockEnvVars {
            vars: Arc::clone(&self.vars),
        })
    }
}

// A blueprint authored in the blueprint language must parse to the same
// `BlueprintConfig` as its JWCC equivalent, proving the reshape → `from_jsonc_str`
// pipeline end to end.
#[test_log::test]
fn parses_handler_matching_jsonc_twin() {
    let blueprint_lang = r#"version "2025-11-02"

resource getOrder: celerity/handler {
    metadata {
        displayName = "Get Order"
    }
    spec {
        handler = "handlers.get_order"
        runtime = "python3.12"
        memory = 1024
    }
}"#;

    let jsonc = r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "getOrder": {
            "type": "celerity/handler",
            "metadata": { "displayName": "Get Order" },
            "spec": {
                "handler": "handlers.get_order",
                "runtime": "python3.12",
                "memory": 1024
            }
        }
    }
}"#;

    let from_blueprint_lang =
        BlueprintConfig::from_blueprint_lang_str(blueprint_lang, Box::new(MockEnvVars::new()))
            .expect("blueprint-language parse should succeed");
    let from_jsonc = BlueprintConfig::from_jsonc_str(jsonc, Box::new(MockEnvVars::new()))
        .expect("jsonc parse should succeed");

    assert_eq!(from_blueprint_lang, from_jsonc);
}

// Scalar annotation values keep their JSON type (a bool stays a bool, an int an
// int) rather than being stringified to `"${true}"` by the flattening pass.
#[test_log::test]
fn preserves_scalar_annotation_types_matching_jsonc_twin() {
    let blueprint_lang = r#"version "2025-11-02"

resource getOrder: celerity/handler {
    metadata {
        displayName = "Get Order"
        annotations = {
            "celerity.handler.http" = true,
            "celerity.handler.http.timeout" = 30,
            "celerity.handler.http.method" = "GET"
        }
    }
    spec {
        handler = "handlers.get_order"
    }
}"#;

    let jsonc = r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "getOrder": {
            "type": "celerity/handler",
            "metadata": {
                "displayName": "Get Order",
                "annotations": {
                    "celerity.handler.http": true,
                    "celerity.handler.http.timeout": 30,
                    "celerity.handler.http.method": "GET"
                }
            },
            "spec": { "handler": "handlers.get_order" }
        }
    }
}"#;

    let from_blueprint_lang =
        BlueprintConfig::from_blueprint_lang_str(blueprint_lang, Box::new(MockEnvVars::new()))
            .expect("blueprint-language parse should succeed");
    let from_jsonc = BlueprintConfig::from_jsonc_str(jsonc, Box::new(MockEnvVars::new()))
        .expect("jsonc parse should succeed");

    assert_eq!(from_blueprint_lang, from_jsonc);
}

// A `variables.x` reference written in the blueprint language is flattened to raw text,
// re-parsed by the narrow scanner, and resolved from the environment — matching
// the JWCC equivalent that carries the same substitution.
#[test_log::test]
fn resolves_variable_substitution_matching_jsonc_equivalent() {
    let blueprint_lang = r#"version "2025-11-02"

variable region: string {
    default = "us-east-1"
}

resource getOrder: celerity/handler {
    spec {
        handler = "handlers.get_order"
        environmentVariables = {
            REGION = variables.region
        }
    }
}"#;

    let jsonc = r#"{
    "version": "2025-11-02",
    "transform": [],
    "variables": {
        "region": { "type": "string", "default": "us-east-1" }
    },
    "resources": {
        "getOrder": {
            "type": "celerity/handler",
            "spec": {
                "handler": "handlers.get_order",
                "environmentVariables": {
                    "REGION": "${variables.region}"
                }
            }
        }
    }
}"#;

    let env = || {
        Box::new(MockEnvVars::with(&[(
            "CELERITY_VARIABLE_region",
            "eu-west-1",
        )]))
    };

    let from_blueprint_lang = BlueprintConfig::from_blueprint_lang_str(blueprint_lang, env())
        .expect("blueprint-language parse should succeed");
    let from_jsonc =
        BlueprintConfig::from_jsonc_str(jsonc, env()).expect("jsonc parse should succeed");

    assert_eq!(from_blueprint_lang, from_jsonc);
}

// A comprehensive blueprint loaded from a `.bp` file resolves to the same
// `BlueprintConfig` as its JWCC fixture equivalent: variables (with allowedValues,
// secret, default), an API with nested cors/domain/auth and a `variables.x`
// reference, a handler with typed annotations and environment-variable
// references, a link selector, and a transform directive.
#[test_log::test]
fn parses_http_api_blueprint_file_matching_jsonc_fixture() {
    let env = || {
        Box::new(MockEnvVars::with(&[
            ("CELERITY_VARIABLE_secretStoreId", "secret-store-id"),
            ("CELERITY_VARIABLE_certificateId", "certificate-id"),
            ("CELERITY_VARIABLE_logLevel", "DEBUG"),
            ("CELERITY_VARIABLE_paymentApiSecret", "payment-api-secret"),
        ]))
    };

    let from_blueprint_lang =
        BlueprintConfig::from_blueprint_lang_file("tests/data/fixtures/http-api.bp", env())
            .expect("blueprint-language file should parse");
    let from_jsonc = BlueprintConfig::from_jsonc_file("tests/data/fixtures/http-api.jsonc", env())
        .expect("jsonc fixture should parse");

    assert_eq!(from_blueprint_lang, from_jsonc);
}

/// Asserts a blueprint-language source resolves to the same `BlueprintConfig` as
/// its JWCC equivalent (with an empty environment).
fn assert_matches_jsonc_equivalent(blueprint_lang: &str, jsonc: &str) {
    let from_blueprint_lang =
        BlueprintConfig::from_blueprint_lang_str(blueprint_lang, Box::new(MockEnvVars::new()))
            .expect("blueprint-language parse should succeed");
    let from_jsonc = BlueprintConfig::from_jsonc_str(jsonc, Box::new(MockEnvVars::new()))
        .expect("jsonc parse should succeed");
    assert_eq!(from_blueprint_lang, from_jsonc);
}

// A resource metadata block without a display name is now valid (displayName is
// optional in the narrow model).
#[test_log::test]
fn allows_resource_metadata_without_display_name() {
    assert_matches_jsonc_equivalent(
        r#"version "2025-11-02"

resource getOrder: celerity/handler {
    metadata {
        labels = {
            application = "orders"
        }
    }
    spec {
        handler = "handlers.get_order"
    }
}"#,
        r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "getOrder": {
            "type": "celerity/handler",
            "metadata": { "labels": { "application": "orders" } },
            "spec": { "handler": "handlers.get_order" }
        }
    }
}"#,
    );
}

// An API spec exercises arrays, nested objects, and typed scalars (bool/int) in
// the reshaped spec.
#[test_log::test]
fn parses_api_resource_matching_jsonc_equivalent() {
    assert_matches_jsonc_equivalent(
        r#"version "2025-11-02"

resource ordersApi: celerity/api {
    spec {
        protocols = ["http"]
        cors = {
            allowCredentials = true,
            allowOrigins = ["https://example.com"],
            allowMethods = ["GET", "POST"],
            maxAge = 3600
        }
        tracingEnabled = true
    }
}"#,
        r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "ordersApi": {
            "type": "celerity/api",
            "spec": {
                "protocols": ["http"],
                "cors": {
                    "allowCredentials": true,
                    "allowOrigins": ["https://example.com"],
                    "allowMethods": ["GET", "POST"],
                    "maxAge": 3600
                },
                "tracingEnabled": true
            }
        }
    }
}"#,
    );
}

// A schedule spec carries a static `input` object that must reshape to a nested
// JSON object.
#[test_log::test]
fn parses_schedule_resource_matching_jsonc_equivalent() {
    assert_matches_jsonc_equivalent(
        r#"version "2025-11-02"

resource dailyReport: celerity/schedule {
    spec {
        schedule = "cron(0 12 * * ? *)"
        input = {
            report = "daily",
            retries = 3
        }
    }
}"#,
        r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "dailyReport": {
            "type": "celerity/schedule",
            "spec": {
                "schedule": "cron(0 12 * * ? *)",
                "input": { "report": "daily", "retries": 3 }
            }
        }
    }
}"#,
    );
}

// A workflow spec exercises the deeply nested `states` map of nested objects.
#[test_log::test]
fn parses_workflow_resource_matching_jsonc_equivalent() {
    assert_matches_jsonc_equivalent(
        r#"version "2025-11-02"

resource processDocument: celerity/workflow {
    spec {
        startAt = "fetchDocument"
        states = {
            fetchDocument = {
                type = "executeStep",
                next = "scanDocument"
            },
            scanDocument = {
                type = "executeStep",
                end = true
            }
        }
    }
}"#,
        r#"{
    "version": "2025-11-02",
    "transform": [],
    "resources": {
        "processDocument": {
            "type": "celerity/workflow",
            "spec": {
                "startAt": "fetchDocument",
                "states": {
                    "fetchDocument": { "type": "executeStep", "next": "scanDocument" },
                    "scanDocument": { "type": "executeStep", "end": true }
                }
            }
        }
    }
}"#,
    );
}
