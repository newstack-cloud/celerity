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
fn parses_vpc_with_name_only() {
    let yaml = r#"
version: 2025-11-02
resources:
    myVpc:
        type: celerity/vpc
        metadata:
            displayName: "My VPC"
        spec:
            name: "production-vpc"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok(), "Failed to parse VPC: {:?}", result.err());

    let config = result.unwrap();
    assert_eq!(config.resources.len(), 1);

    let vpc_resource = config
        .resources
        .get("myVpc")
        .expect("VPC resource not found");
    assert_eq!(vpc_resource.metadata.display_name, "My VPC");

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.name, "production-vpc");
        assert_eq!(vpc_spec.preset, None);
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_standard_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    myVpc:
        type: celerity/vpc
        metadata:
            displayName: "Standard VPC"
        spec:
            name: "standard-vpc"
            preset: "standard"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok(), "Failed to parse VPC: {:?}", result.err());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("myVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.name, "standard-vpc");
        assert_eq!(vpc_spec.preset, Some("standard".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_public_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    publicVpc:
        type: celerity/vpc
        metadata:
            displayName: "Public VPC"
        spec:
            name: "public-vpc"
            preset: "public"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("publicVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.preset, Some("public".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_isolated_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    isolatedVpc:
        type: celerity/vpc
        metadata:
            displayName: "Isolated VPC"
        spec:
            name: "isolated-vpc"
            preset: "isolated"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("isolatedVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.preset, Some("isolated".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_light_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    lightVpc:
        type: celerity/vpc
        metadata:
            displayName: "Light VPC"
        spec:
            name: "light-vpc"
            preset: "light"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("lightVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.preset, Some("light".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_light_public_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    lightPublicVpc:
        type: celerity/vpc
        metadata:
            displayName: "Light Public VPC"
        spec:
            name: "light-public-vpc"
            preset: "light-public"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("lightPublicVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.preset, Some("light-public".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn rejects_vpc_with_invalid_preset() {
    let yaml = r#"
version: 2025-11-02
resources:
    invalidVpc:
        type: celerity/vpc
        metadata:
            displayName: "Invalid VPC"
        spec:
            name: "invalid-vpc"
            preset: "invalid-preset"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("Invalid VPC preset") || err_string.contains("invalid-preset"),
        "Expected error about invalid preset, got: {}",
        err_string
    );
}

#[test_log::test]
fn rejects_vpc_without_name() {
    let yaml = r#"
version: 2025-11-02
resources:
    noNameVpc:
        type: celerity/vpc
        metadata:
            displayName: "No Name VPC"
        spec:
            preset: "standard"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("name") || err_string.contains("requires"),
        "Expected error about missing name field, got: {}",
        err_string
    );
}

#[test_log::test]
fn parses_vpc_with_variable_substitution_in_name() {
    let yaml = r#"
version: 2025-11-02
variables:
    vpcName:
        type: string
        default: "my-dynamic-vpc"
resources:
    dynamicVpc:
        type: celerity/vpc
        metadata:
            displayName: "Dynamic VPC"
        spec:
            name: "${variables.vpcName}"
    "#;

    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_vpcName".to_string(),
            "my-dynamic-vpc".to_string(),
        )])),
    };
    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(env));
    assert!(result.is_ok(), "Failed to parse VPC: {:?}", result.err());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("dynamicVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.name, "my-dynamic-vpc");
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn parses_vpc_with_variable_substitution_in_preset() {
    let yaml = r#"
version: 2025-11-02
variables:
    vpcPreset:
        type: string
        default: "isolated"
resources:
    dynamicVpc:
        type: celerity/vpc
        metadata:
            displayName: "Dynamic Preset VPC"
        spec:
            name: "my-vpc"
            preset: "${variables.vpcPreset}"
    "#;

    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_vpcPreset".to_string(),
            "isolated".to_string(),
        )])),
    };
    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(env));
    assert!(result.is_ok(), "Failed to parse VPC: {:?}", result.err());

    let config = result.unwrap();
    let vpc_resource = config.resources.get("dynamicVpc").unwrap();

    if let CelerityResourceSpec::Vpc(vpc_spec) = &vpc_resource.spec {
        assert_eq!(vpc_spec.name, "my-vpc");
        assert_eq!(vpc_spec.preset, Some("isolated".to_string()));
    } else {
        panic!("Expected VPC resource spec");
    }
}

#[test_log::test]
fn rejects_vpc_with_unsupported_field() {
    let yaml = r#"
version: 2025-11-02
resources:
    badVpc:
        type: celerity/vpc
        metadata:
            displayName: "Bad VPC"
        spec:
            name: "my-vpc"
            unsupportedField: "value"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("Unsupported") || err_string.contains("unsupportedField"),
        "Expected error about unsupported field, got: {}",
        err_string
    );
}
