use insta::{assert_json_snapshot, with_settings};
use std::fs::read_to_string;

use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};

#[test_log::test]
fn parses_blueprint_config_from_yaml_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.yaml").unwrap();
    let blueprint_config = BlueprintConfig::from_yaml_str(doc_str.as_str()).unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_blueprint_config_from_jsonc_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.jsonc").unwrap();
    let blueprint_config = BlueprintConfig::from_jsonc_str(doc_str.as_str()).unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_http_api_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/http-api.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_http_api_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/http-api.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_shared_handler_config_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/shared-handler-config.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_shared_handler_config_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/shared-handler-config.jsonc")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_handler_config_resources_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/handler-config-resource-types.yaml")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_handler_config_resources_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/handler-config-resource-types.jsonc")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/websocket-api.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/websocket-api.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_with_ws_protocol_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/websocket-api-with-ws-config.yaml")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_websocket_api_blueprint_config_with_ws_protocol_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/websocket-api-with-ws-config.jsonc")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_combined_app_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/combined-app.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_combined_app_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/combined-app.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_hybrid_api_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/hybrid-api.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_hybrid_api_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/hybrid-api.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_schedule_app_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/schedule-app.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_schedule_app_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/schedule-app.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_workflow_app_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/workflow-app.yaml").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_workflow_app_blueprint_config_from_jsonc_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/workflow-app.jsonc").unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn produces_expected_error_for_invalid_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-blueprint.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a mapping for blueprint, found \
        Array([String(\"Array of strings\"), String(\"Is not a valid blueprint\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-blueprint.jsonc");

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
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/missing-version.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "a blueprint version must be provided"
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_version_in_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/missing-version.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains("missing field `version`")
    ));
}

#[test_log::test]
fn produces_expected_error_for_no_resources_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/no-resources.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "at least one resource must be provided for a blueprint"
    ));
}

#[test_log::test]
fn produces_expected_error_for_no_resources_in_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/no-resources.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::ValidationError(msg)) if msg == "at least one resource must be provided for a blueprint"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_version_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-version.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected version \
        2025-05-12, found unsupported-2020-03-10"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_version_in_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-version.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid value: string \"unsupported-2020-03-10\", expected 2025-05-12"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_type_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-variable-type.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a string for variable \
        type, found Real(\"304493.231\")"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_type_in_json_blueprint_config() {
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-variable-type.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: floating point `304493.231`, expected a string"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_description_in_yaml_blueprint_config() {
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-variable-description.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a string for \
        variable description, found Array([String(\"Invalid description, array not expected.\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_variable_description_in_json_blueprint_config() {
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-variable-description.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: sequence, expected a string"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_secret_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-secret.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "expected a boolean for variable secret field, \
        found String(\"Invalid secret value, boolean expected\")"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_secret_in_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-secret.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err)) if err.to_string().contains(
            "invalid type: string \"Invalid secret value, boolean expected\", expected a boolean"
        )
    ));
}

#[test_log::test]
fn produces_expected_error_for_empty_variable_type_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/empty-variable-type.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg)) if msg == "type must be provided in \\\"secretStoreId\\\" variable definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_empty_variable_type_in_json_blueprint_config() {
    let result = BlueprintConfig::from_jsonc_file("tests/data/fixtures/empty-variable-type.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::ValidationError(msg))
        if msg == "type must be provided in \\\"secretStoreId\\\" variable definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_type_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-resource-type.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "expected a string for resource type, found Array([String(\"invalid/type in array\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_type_in_json_blueprint_config() {
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-resource-type.jsonc");
    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err))
        if err.to_string().contains("invalid data type provided for resource type")
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_resource_type_in_yaml_blueprint_config() {
    let result = BlueprintConfig::from_yaml_file("tests/data/fixtures/missing-resource-type.yaml");
    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "resource type must be defined for the \\\"getOrderHandler\\\" resource definition"
    ));
}

#[test_log::test]
fn produces_expected_error_for_missing_resource_type_in_json_blueprint_config() {
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/missing-resource-type.jsonc");
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
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/unsupported-resource-type.yaml")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn skips_parsing_resource_for_unsupported_resource_type_in_json_blueprint_config() {
    let blueprint_config =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/unsupported-resource-type.jsonc")
            .unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_metadata_in_yaml_blueprint_config() {
    let result =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/invalid-resource-metadata.yaml");

    assert!(matches!(
        result,
        Err(BlueprintParseError::YamlFormatError(msg))
        if msg == "expected a mapping for resource metadata, found Array([String(\"Array not expected here\")])"
    ));
}

#[test_log::test]
fn produces_expected_error_for_invalid_resource_metadata_in_json_blueprint_config() {
    let result =
        BlueprintConfig::from_jsonc_file("tests/data/fixtures/invalid-resource-metadata.jsonc");

    assert!(matches!(
        result,
        Err(BlueprintParseError::JsonError(err))
        if err.to_string().contains(
            "expected struct BlueprintResourceMetadata"
        )
    ));
}
