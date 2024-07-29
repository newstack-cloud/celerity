use std::collections::HashMap;

use celerity_blueprint_config_parser::blueprint::BlueprintConfig;

#[test_log::test]
fn parses_blueprint_config_from_yaml_file() {
    let blueprint_config =
        BlueprintConfig::from_yaml_file("tests/data/fixtures/http-api.yaml").unwrap();

    // assert_eq!(
    //     blueprint_config,
    //     BlueprintConfig {
    //         version: "2023-04-20".to_string(),
    //         transform: None,
    //         variables: None,
    //         resources: HashMap::new(),
    //     }
    // );
    assert_eq!(blueprint_config.version, "2023-04-20".to_string());
}
