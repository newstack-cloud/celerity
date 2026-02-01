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
fn parses_datastore_with_partition_key_only() {
    let yaml = r#"
version: 2025-11-02
resources:
    userStore:
        type: celerity/datastore
        metadata:
            displayName: "User Store"
        spec:
            keys:
                partitionKey: "userId"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    assert_eq!(config.resources.len(), 1);

    let datastore_resource = config
        .resources
        .get("userStore")
        .expect("Datastore resource not found");
    assert_eq!(datastore_resource.metadata.display_name, "User Store");

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(datastore_spec.keys.partition_key, "userId");
        assert_eq!(datastore_spec.keys.sort_key, None);
        assert_eq!(datastore_spec.name, None);
        assert_eq!(datastore_spec.schema, None);
        assert_eq!(datastore_spec.indexes, None);
        assert_eq!(datastore_spec.time_to_live, None);
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_partition_and_sort_keys() {
    let yaml = r#"
version: 2025-11-02
resources:
    eventStore:
        type: celerity/datastore
        metadata:
            displayName: "Event Store"
        spec:
            keys:
                partitionKey: "userId"
                sortKey: "timestamp"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("eventStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(datastore_spec.keys.partition_key, "userId");
        assert_eq!(datastore_spec.keys.sort_key, Some("timestamp".to_string()));
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_name() {
    let yaml = r#"
version: 2025-11-02
resources:
    namedStore:
        type: celerity/datastore
        metadata:
            displayName: "Named Store"
        spec:
            name: "production-datastore"
            keys:
                partitionKey: "id"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_ok());

    let config = result.unwrap();
    let datastore_resource = config.resources.get("namedStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(
            datastore_spec.name,
            Some("production-datastore".to_string())
        );
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_simple_schema() {
    let yaml = r#"
version: 2025-11-02
resources:
    userStore:
        type: celerity/datastore
        metadata:
            displayName: "User Store"
        spec:
            keys:
                partitionKey: "userId"
            schema:
                userId:
                    type: "string"
                    description: "Unique user identifier"
                email:
                    type: "string"
                    description: "User email address"
                age:
                    type: "number"
                    nullable: true
                isActive:
                    type: "boolean"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("userStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        let schema = datastore_spec.schema.as_ref().expect("Schema not found");

        // Check userId field
        let user_id_field = schema.get("userId").expect("userId field not found");
        assert_eq!(user_id_field.field_type, "string");
        assert_eq!(
            user_id_field.description,
            Some("Unique user identifier".to_string())
        );

        // Check email field
        let email_field = schema.get("email").expect("email field not found");
        assert_eq!(email_field.field_type, "string");

        // Check age field
        let age_field = schema.get("age").expect("age field not found");
        assert_eq!(age_field.field_type, "number");
        assert_eq!(age_field.nullable, Some(true));

        // Check isActive field
        let is_active_field = schema.get("isActive").expect("isActive field not found");
        assert_eq!(is_active_field.field_type, "boolean");
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_nested_object_schema() {
    let yaml = r#"
version: 2025-11-02
resources:
    profileStore:
        type: celerity/datastore
        metadata:
            displayName: "Profile Store"
        spec:
            keys:
                partitionKey: "profileId"
            schema:
                profileId:
                    type: "string"
                address:
                    type: "object"
                    fields:
                        street:
                            type: "string"
                        city:
                            type: "string"
                        zipCode:
                            type: "string"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("profileStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        let schema = datastore_spec.schema.as_ref().expect("Schema not found");

        let address_field = schema.get("address").expect("address field not found");
        assert_eq!(address_field.field_type, "object");

        let nested_fields = address_field
            .fields
            .as_ref()
            .expect("Nested fields not found");

        assert_eq!(nested_fields.get("street").unwrap().field_type, "string");
        assert_eq!(nested_fields.get("city").unwrap().field_type, "string");
        assert_eq!(nested_fields.get("zipCode").unwrap().field_type, "string");
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_array_schema() {
    let yaml = r#"
version: 2025-11-02
resources:
    tagStore:
        type: celerity/datastore
        metadata:
            displayName: "Tag Store"
        spec:
            keys:
                partitionKey: "itemId"
            schema:
                itemId:
                    type: "string"
                tags:
                    type: "array"
                    items:
                        type: "string"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("tagStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        let schema = datastore_spec.schema.as_ref().expect("Schema not found");

        let tags_field = schema.get("tags").expect("tags field not found");
        assert_eq!(tags_field.field_type, "array");

        let items_schema = tags_field.items.as_ref().expect("Items schema not found");
        assert_eq!(items_schema.field_type, "string");
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_indexes() {
    let yaml = r#"
version: 2025-11-02
resources:
    userStore:
        type: celerity/datastore
        metadata:
            displayName: "User Store"
        spec:
            keys:
                partitionKey: "userId"
            indexes:
                - name: "emailIndex"
                  fields: ["email"]
                - name: "statusIndex"
                  fields: ["status", "createdAt"]
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("userStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        let indexes = datastore_spec.indexes.as_ref().expect("Indexes not found");
        assert_eq!(indexes.len(), 2);

        // Check first index
        assert_eq!(indexes[0].name, "emailIndex");
        assert_eq!(indexes[0].fields, vec!["email"]);

        // Check second index
        assert_eq!(indexes[1].name, "statusIndex");
        assert_eq!(indexes[1].fields, vec!["status", "createdAt"]);
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_ttl() {
    let yaml = r#"
version: 2025-11-02
resources:
    sessionStore:
        type: celerity/datastore
        metadata:
            displayName: "Session Store"
        spec:
            keys:
                partitionKey: "sessionId"
            timeToLive:
                fieldName: "expiresAt"
                enabled: true
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("sessionStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        let ttl = datastore_spec.time_to_live.as_ref().expect("TTL not found");
        assert_eq!(ttl.field_name, "expiresAt");
        assert!(ttl.enabled);
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_all_features() {
    let yaml = r#"
version: 2025-11-02
resources:
    completeStore:
        type: celerity/datastore
        metadata:
            displayName: "Complete Store"
        spec:
            name: "complete-datastore"
            keys:
                partitionKey: "userId"
                sortKey: "timestamp"
            schema:
                userId:
                    type: "string"
                timestamp:
                    type: "number"
                data:
                    type: "object"
                    fields:
                        value:
                            type: "string"
            indexes:
                - name: "userIndex"
                  fields: ["userId"]
            timeToLive:
                fieldName: "expiresAt"
                enabled: true
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("completeStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(datastore_spec.name, Some("complete-datastore".to_string()));
        assert_eq!(datastore_spec.keys.partition_key, "userId");
        assert_eq!(datastore_spec.keys.sort_key, Some("timestamp".to_string()));
        assert!(datastore_spec.schema.is_some());
        assert!(datastore_spec.indexes.is_some());
        assert!(datastore_spec.time_to_live.is_some());
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn rejects_datastore_without_keys() {
    let yaml = r#"
version: 2025-11-02
resources:
    badStore:
        type: celerity/datastore
        metadata:
            displayName: "Bad Store"
        spec:
            name: "no-keys-store"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("keys") || err_string.contains("requires"),
        "Expected error about missing keys, got: {}",
        err_string
    );
}

#[test_log::test]
fn rejects_datastore_without_partition_key() {
    let yaml = r#"
version: 2025-11-02
resources:
    badStore:
        type: celerity/datastore
        metadata:
            displayName: "Bad Store"
        spec:
            keys:
                sortKey: "timestamp"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("partitionKey") || err_string.contains("requires"),
        "Expected error about missing partitionKey, got: {}",
        err_string
    );
}

#[test_log::test]
fn rejects_datastore_with_invalid_field_type() {
    let yaml = r#"
version: 2025-11-02
resources:
    badStore:
        type: celerity/datastore
        metadata:
            displayName: "Bad Store"
        spec:
            keys:
                partitionKey: "id"
            schema:
                field1:
                    type: "invalid-type"
    "#;

    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(MockEnvVars::new()));
    assert!(result.is_err());

    let err = result.unwrap_err();
    let err_string = err.to_string();
    assert!(
        err_string.contains("Invalid") || err_string.contains("type"),
        "Expected error about invalid field type, got: {}",
        err_string
    );
}

#[test_log::test]
fn parses_datastore_with_variable_substitution_in_keys() {
    let yaml = r#"
version: 2025-11-02
variables:
    partKey:
        type: string
        default: "userId"
    sortKeyField:
        type: string
        default: "timestamp"
resources:
    dynamicStore:
        type: celerity/datastore
        metadata:
            displayName: "Dynamic Store"
        spec:
            keys:
                partitionKey: "${variables.partKey}"
                sortKey: "${variables.sortKeyField}"
    "#;

    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([
            (
                "CELERITY_VARIABLE_partKey".to_string(),
                "userId".to_string(),
            ),
            (
                "CELERITY_VARIABLE_sortKeyField".to_string(),
                "timestamp".to_string(),
            ),
        ])),
    };
    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(env));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("dynamicStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(datastore_spec.keys.partition_key, "userId");
        assert_eq!(datastore_spec.keys.sort_key, Some("timestamp".to_string()));
    } else {
        panic!("Expected Datastore resource spec");
    }
}

#[test_log::test]
fn parses_datastore_with_variable_substitution_in_name() {
    let yaml = r#"
version: 2025-11-02
variables:
    storeName:
        type: string
        default: "production-store"
resources:
    dynamicStore:
        type: celerity/datastore
        metadata:
            displayName: "Dynamic Store"
        spec:
            name: "${variables.storeName}"
            keys:
                partitionKey: "id"
    "#;

    let env = MockEnvVars {
        vars: Arc::new(HashMap::from([(
            "CELERITY_VARIABLE_storeName".to_string(),
            "production-store".to_string(),
        )])),
    };
    let result = BlueprintConfig::from_yaml_str(yaml, Box::new(env));
    assert!(
        result.is_ok(),
        "Failed to parse datastore: {:?}",
        result.err()
    );

    let config = result.unwrap();
    let datastore_resource = config.resources.get("dynamicStore").unwrap();

    if let CelerityResourceSpec::Datastore(datastore_spec) = &datastore_resource.spec {
        assert_eq!(datastore_spec.name, Some("production-store".to_string()));
    } else {
        panic!("Expected Datastore resource spec");
    }
}
