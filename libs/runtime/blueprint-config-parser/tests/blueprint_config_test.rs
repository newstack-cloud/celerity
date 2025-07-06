use celerity_helpers::env::EnvVars;
use insta::{assert_json_snapshot, with_settings};
use std::{collections::HashMap, env::VarError, fs::read_to_string, sync::Arc};

use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};

struct MockEnvVars {
    vars: Arc<HashMap<String, String>>,
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

#[test_log::test]
fn parses_blueprint_config_from_yaml_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.yaml").unwrap();
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_yaml_str(doc_str.as_str(), Box::new(env)).unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_blueprint_config_from_jsonc_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.jsonc").unwrap();
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config =
        BlueprintConfig::from_jsonc_str(doc_str.as_str(), Box::new(env)).unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_shared_handler_config_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/shared-handler-config.yaml",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_shared_handler_config_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/shared-handler-config.jsonc",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_handler_config_resources_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/handler-config-resource-types.yaml",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_handler_config_resources_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/handler-config-resource-types.jsonc",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_certificateId".to_string(),
            "certificate-id".to_string(),
        )])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/websocket-api.yaml", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_certificateId".to_string(),
            "certificate-id".to_string(),
        )])),
    };
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/websocket-api.jsonc", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_with_ws_protocol_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_certificateId".to_string(),
            "certificate-id".to_string(),
        )])),
    };
    let blueprint_config = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/websocket-api-with-ws-config.yaml",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_with_ws_protocol_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_certificateId".to_string(),
            "certificate-id".to_string(),
        )])),
    };
    let blueprint_config = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/websocket-api-with-ws-config.jsonc",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_combined_app_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_orderEventsSourceId".to_string(),
                "order-events-source-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_invoiceBucket".to_string(),
                "invoice-bucket".to_string(),
            ),
            (
                "CELERITY_VARIABLE_orderDBStreamId".to_string(),
                "order-db-stream-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_invoiceEventStreamId".to_string(),
                "invoice-event-stream-id".to_string(),
            ),
        ])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/combined-app.yaml", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_combined_app_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_orderEventsSourceId".to_string(),
                "order-events-source-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_invoiceBucket".to_string(),
                "invoice-bucket".to_string(),
            ),
            (
                "CELERITY_VARIABLE_orderDBStreamId".to_string(),
                "order-db-stream-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_invoiceEventStreamId".to_string(),
                "invoice-event-stream-id".to_string(),
            ),
        ])),
    };
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/combined-app.jsonc", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_hybrid_api_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
        ])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/hybrid-api.yaml", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_hybrid_api_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
        ])),
    };
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/hybrid-api.jsonc", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_schedule_app_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/schedule-app.yaml", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_schedule_app_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/schedule-app.jsonc", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_workflow_app_blueprint_config_from_yaml_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/workflow-app.yaml", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_workflow_app_blueprint_config_from_jsonc_file() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/workflow-app.jsonc", Box::new(env))
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn produces_expected_error_for_invalid_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/invalid-blueprint.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a mapping for blueprint, found \
        Array([String(\"Array of strings\"), String(\"Is not a valid blueprint\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-blueprint.jsonc",
        Box::new(env),
    );

    // serde takes a bottom up approach, so will try to parse the innermost value first,
    // therefore the error message will be for a failure to match against a blueprint version.
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid value: string \"Array of strings\", expected 2025-05-12"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_version_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/missing-version.yaml", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "a blueprint version must be provided"
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_version_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/missing-version.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains("missing field `version`")
    ));
}

#[test_log::test]
fn produces_expected_error_for_no_resources_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/no-resources.yaml", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "at least one resource must be provided for a blueprint"
    ));
}

#[test_log::test]
fn produces_expected_error_for_no_resources_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/no-resources.jsonc", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::ValidationError(msg)) if msg == "at least one resource must be provided for a blueprint"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_version_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-version.yaml", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected version \
        2025-05-12, found unsupported-2020-03-10"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_version_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-version.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid value: string \"unsupported-2020-03-10\", expected 2025-05-12"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_type_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/invalid-variable-type.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a string for variable \
        type, found Real(\"304493.231\")"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_type_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-variable-type.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: floating point `304493.231`, expected a string"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_description_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/invalid-variable-description.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a string for \
        variable description, found Array([String(\"Invalid description, array not expected.\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_description_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-variable-description.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: sequence, expected a string"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_secret_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-secret.yaml", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a boolean for variable secret field, \
        found String(\"Invalid secret value, boolean expected\")"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_secret_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-secret.jsonc", Box::new(env));
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: string \"Invalid secret value, boolean expected\", expected a boolean"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_empty_variable_type_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
        ])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/empty-variable-type.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "type must be provided in \\\"secretStoreId\\\" variable definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_empty_variable_type_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
        ])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/empty-variable-type.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::ValidationError(msg))
        if msg == "type must be provided in \\\"secretStoreId\\\" variable definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_type_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/invalid-resource-type.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "expected a string for resource type, found Array([String(\"invalid/type in array\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_type_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-resource-type.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err))
        if err.to_string().contains("invalid data type provided for resource type")
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_resource_type_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/missing-resource-type.yaml",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "resource type must be defined for the \\\"getOrderHandler\\\" resource definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_resource_type_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/missing-resource-type.jsonc",
        Box::new(env),
    );
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err))
        if err.to_string().contains(
            "spec must come after type in resource, type is either defined after spec or is missing"
        )
    ));
}

#[test_log::test]
fn skips_parsing_resource_for_unsupported_resource_type_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/unsupported-resource-type.yaml",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn skips_parsing_resource_for_unsupported_resource_type_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_secretStoreId".to_string(),
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId".to_string(),
                "certificate-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_logLevel".to_string(),
                "DEBUG".to_string(),
            ),
            (
                "CELERITY_VARIABLE_paymentApiSecret".to_string(),
                "payment-api-secret".to_string(),
            ),
        ])),
    };
    let blueprint_config = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/unsupported-resource-type.jsonc",
        Box::new(env),
    )
    .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_metadata_in_yaml_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_yaml_file(
        "tests/data/fixtures/invalid-resource-metadata.yaml",
        Box::new(env),
    );

    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "expected a mapping for resource metadata, found Array([String(\"Array not expected here\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_metadata_in_json_blueprint_config() {
    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([])),
    };
    let result = BlueprintConfig::from_jsonc_file(
        "tests/data/fixtures/invalid-resource-metadata.jsonc",
        Box::new(env),
    );

    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err))
        if err.to_string().contains(
            "expected struct BlueprintResourceMetadata"
        )
    ));
}
