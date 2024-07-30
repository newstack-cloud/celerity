use insta::{assert_json_snapshot, with_settings};
use std::fs::read_to_string;

use celerity_blueprint_config_parser::blueprint::BlueprintConfig;

#[test_log::test]
fn parses_blueprint_config_from_yaml_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.yaml").unwrap();
    let blueprint_config = BlueprintConfig::from_yaml_str(doc_str.as_str()).unwrap();

    with_settings!({sort_maps => true}, {
        assert_json_snapshot!(blueprint_config);
    })
}

#[test_log::test]
fn parses_blueprint_config_from_json_string() {
    let doc_str: String = read_to_string("tests/data/fixtures/http-api.json").unwrap();
    let blueprint_config = BlueprintConfig::from_json_str(doc_str.as_str()).unwrap();

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
fn parses_http_api_blueprint_config_from_json_file() {
    let blueprint_config =
        BlueprintConfig::from_json_file("tests/data/fixtures/http-api.json").unwrap();

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
fn parses_websocket_api_blueprint_config_from_json_file() {
    let blueprint_config =
        BlueprintConfig::from_json_file("tests/data/fixtures/websocket-api.json").unwrap();

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
fn parses_combined_app_blueprint_config_from_json_file() {
    let blueprint_config =
        BlueprintConfig::from_json_file("tests/data/fixtures/combined-app.json").unwrap();

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
