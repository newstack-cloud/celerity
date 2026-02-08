use std::collections::HashMap;

use hashlink::LinkedHashMap;
use tracing::debug;

use crate::{
    blueprint::{
        BlueprintLinkSelector, BlueprintScalarValue, BlueprintVariable, CelerityResourceType,
        EventSourceType, BLUELINK_BLUEPRINT_V2025_11_02, CELERITY_API_RESOURCE_TYPE,
        CELERITY_CONSUMER_RESOURCE_TYPE, CELERITY_DATASTORE_RESOURCE_TYPE,
        CELERITY_HANDLER_CONFIG_RESOURCE_TYPE, CELERITY_HANDLER_RESOURCE_TYPE,
        CELERITY_SCHEDULE_RESOURCE_TYPE, CELERITY_VPC_RESOURCE_TYPE,
        CELERITY_WORKFLOW_RESOURCE_TYPE,
    },
    blueprint_with_subs::{
        is_string_with_substitutions_empty, BlueprintConfigWithSubs, BlueprintMetadataWithSubs,
        BlueprintResourceMetadataWithSubs, CelerityApiAuthGuardValueSourceWithSubs,
        CelerityApiAuthGuardWithSubs, CelerityApiAuthWithSubs,
        CelerityApiBasePathConfigurationWithSubs, CelerityApiBasePathWithSubs,
        CelerityApiCorsConfigurationWithSubs, CelerityApiCorsWithSubs, CelerityApiDomainWithSubs,
        CelerityApiSpecWithSubs, CelerityBucketSpecWithSubs, CelerityConfigSpecWithSubs,
        CelerityConsumerSpecWithSubs, CelerityDatastoreSpecWithSubs, CelerityHandlerSpecWithSubs,
        CelerityQueueSpecWithSubs, CelerityResourceSpecWithSubs, CelerityScheduleSpecWithSubs,
        CelerityTopicSpecWithSubs, CelerityVpcSpecWithSubs, DataStreamSourceConfigurationWithSubs,
        DatabaseStreamSourceConfigurationWithSubs, DatastoreFieldSchemaWithSubs,
        DatastoreIndexWithSubs, DatastoreKeysWithSubs, DatastoreTimeToLiveWithSubs,
        EventSourceConfigurationWithSubs, ExternalEventConfigurationWithSubs, MappingNode,
        ObjectStorageEventSourceConfigurationWithSubs, RuntimeBlueprintResourceWithSubs,
        SharedHandlerConfigWithSubs, StringOrSubstitution, StringOrSubstitutions,
        ValueSourceConfigurationWithSubs,
    },
    parse::BlueprintParseError,
    parse_substitutions::{parse_substitutions, ParseError},
    yaml_helpers::{
        extract_scalar_value, validate_array_of_strings, validate_mapping_node,
        validate_single_substitution,
    },
    yaml_workflow::validate_celerity_workflow_spec,
};

pub fn build_intermediate_blueprint_config_from_yaml(
    yaml: &yaml_rust2::Yaml,
) -> Result<BlueprintConfigWithSubs, BlueprintParseError> {
    let mut blueprint = BlueprintConfigWithSubs::default();
    match yaml {
        yaml_rust2::Yaml::Hash(hash) => {
            for (key, value) in hash {
                if let yaml_rust2::Yaml::String(key_str) = key {
                    match value {
                        yaml_rust2::Yaml::String(value_str) => {
                            let key_str = key_str.as_str();
                            let value_str = value_str.as_str();
                            match key_str {
                                "version" => validate_assign_version(value_str, &mut blueprint)?,
                                "transform" => {
                                    blueprint.transform = Some(Vec::from([value_str.to_string()]))
                                }
                                _ => (),
                            }
                        }
                        yaml_rust2::Yaml::Hash(value_map) => {
                            let key_str = key_str.as_str();
                            match key_str {
                                "variables" => {
                                    validate_populate_variables(value_map, &mut blueprint)?
                                }
                                "resources" => {
                                    validate_populate_resources(value_map, &mut blueprint)?
                                }
                                "metadata" => {
                                    validate_populate_blueprint_metadata(value_map, &mut blueprint)?
                                }
                                _ => (),
                            }
                        }
                        _ => (),
                    }
                }
            }
        }
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a mapping for blueprint, found {yaml:?}",
        )))?,
    };

    if blueprint.version.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "a blueprint version must be provided".to_string(),
        ));
    }

    if blueprint.resources.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "at least one resource must be provided for a blueprint".to_string(),
        ));
    }

    Ok(blueprint)
}

fn validate_assign_version(
    version: &str,
    blueprint: &mut BlueprintConfigWithSubs,
) -> Result<(), BlueprintParseError> {
    if version != BLUELINK_BLUEPRINT_V2025_11_02 {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "expected version {BLUELINK_BLUEPRINT_V2025_11_02}, found {version}",
        )));
    }

    blueprint.version = version.to_string();
    Ok(())
}

fn validate_populate_variables(
    yaml_vars: &yaml_rust2::yaml::Hash,
    blueprint: &mut BlueprintConfigWithSubs,
) -> Result<(), BlueprintParseError> {
    let mut vars = HashMap::new();
    for (key, value) in yaml_vars {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::Hash(value_map) = value {
                vars.insert(
                    key_str.clone(),
                    validate_variable_definition(key_str.as_str(), value_map)?,
                );
            }
        }
    }
    blueprint.variables = Some(vars);
    Ok(())
}

fn validate_variable_definition(
    var_name: &str,
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<BlueprintVariable, BlueprintParseError> {
    let mut blueprint_var = BlueprintVariable::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "type" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        blueprint_var.var_type = value_str.to_string();
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for variable type, found {value:?}",
                        )))?;
                    }
                }
                "allowedValues" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut allowed_values = Vec::new();
                        for item in value_arr {
                            let scalar_value = extract_scalar_value(item, "allowedValues")?;
                            if let Some(unwrapped_scalar) = scalar_value {
                                allowed_values.push(unwrapped_scalar);
                            }
                        }
                        blueprint_var.allowed_values = Some(allowed_values);
                    }
                }
                "default" => blueprint_var.default = extract_scalar_value(value, "default")?,
                "description" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        blueprint_var.description = Some(value_str.clone())
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for variable description, found {value:?}",
                        )))?
                    }
                }
                "secret" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        blueprint_var.secret = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for variable secret field, found {value:?}",
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if blueprint_var.var_type.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "type must be provided in \\\"{var_name}\\\" variable definition",
        )));
    }

    Ok(blueprint_var)
}

fn validate_populate_resources(
    yaml_resources: &yaml_rust2::yaml::Hash,
    blueprint: &mut BlueprintConfigWithSubs,
) -> Result<(), BlueprintParseError> {
    let mut resources = HashMap::new();
    for (key, value) in yaml_resources {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::Hash(value_map) = value {
                match validate_resource_definition(key_str.as_str(), value_map) {
                    Ok(blueprint_resource) => {
                        resources.insert(key_str.clone(), blueprint_resource);
                    }
                    Err(err) => {
                        if let BlueprintParseError::UnsupportedResourceType(_) = err {
                            debug!(
                                error = err.to_string(),
                                "skipping resource \\\"{}\\\" as it is \
                                not a supported celerity runtime resource",
                                key_str,
                            );
                        } else {
                            return Err(err);
                        }
                    }
                }
            }
        }
    }
    blueprint.resources = resources;
    Ok(())
}

fn validate_resource_definition(
    resource_name: &str,
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<RuntimeBlueprintResourceWithSubs, BlueprintParseError> {
    let mut blueprint_resource = RuntimeBlueprintResourceWithSubs::default();

    // Make sure the resource type is known before validating the spec.
    if let Some(resource_type_val) = value_map.get(&yaml_rust2::Yaml::String("type".to_string())) {
        if let yaml_rust2::Yaml::String(value_str) = resource_type_val {
            blueprint_resource.resource_type = validate_resource_type(value_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for resource type, found {resource_type_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "resource type must be defined for the \\\"{resource_name}\\\" resource definition",
        )))?;
    }

    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "metadata" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        blueprint_resource.metadata = validate_resource_metadata(value_map)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for resource metadata, found {value:?}",
                        )))?
                    }
                }
                "linkSelector" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        blueprint_resource.link_selector = Some(validate_link_selector(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for resource link selector, found {value:?}",
                        )))?
                    }
                }
                "description" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        blueprint_resource.description = Some(value_str.clone())
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for resource description, found {value:?}",
                        )))?
                    }
                }
                "spec" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        blueprint_resource.spec =
                            validate_resource_spec(&blueprint_resource.resource_type, value_map)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for resource spec, found {value:?}",
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if let CelerityResourceSpecWithSubs::NoSpec = blueprint_resource.spec {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "resource spec must be defined for the \\\"{resource_name}\\\" resource definition",
        )));
    }

    Ok(blueprint_resource)
}

fn validate_resource_type(
    resource_type: &str,
) -> Result<CelerityResourceType, BlueprintParseError> {
    match resource_type {
        CELERITY_API_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityApi),
        CELERITY_CONSUMER_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityConsumer),
        CELERITY_DATASTORE_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityDatastore),
        CELERITY_SCHEDULE_RESOURCE_TYPE => Ok(CelerityResourceType::CeleritySchedule),
        CELERITY_HANDLER_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityHandler),
        CELERITY_HANDLER_CONFIG_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityHandlerConfig),
        CELERITY_VPC_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityVpc),
        CELERITY_WORKFLOW_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityWorkflow),
        _ => Err(BlueprintParseError::UnsupportedResourceType(
            resource_type.to_string(),
        )),
    }
}

fn validate_resource_spec(
    resource_type: &CelerityResourceType,
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityResourceSpecWithSubs, BlueprintParseError> {
    match resource_type {
        CelerityResourceType::CelerityApi => Ok(CelerityResourceSpecWithSubs::Api(
            validate_celerity_api_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityConsumer => Ok(CelerityResourceSpecWithSubs::Consumer(
            validate_celerity_consumer_spec(spec_map)?,
        )),
        CelerityResourceType::CeleritySchedule => Ok(CelerityResourceSpecWithSubs::Schedule(
            validate_celerity_schedule_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityHandler => Ok(CelerityResourceSpecWithSubs::Handler(
            validate_celerity_handler_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityHandlerConfig => Ok(
            CelerityResourceSpecWithSubs::HandlerConfig(validate_shared_handler_config(spec_map)?),
        ),
        CelerityResourceType::CelerityWorkflow => Ok(CelerityResourceSpecWithSubs::Workflow(
            validate_celerity_workflow_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityConfig => Ok(CelerityResourceSpecWithSubs::Config(
            validate_celerity_config_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityBucket => Ok(CelerityResourceSpecWithSubs::Bucket(
            validate_celerity_bucket_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityTopic => Ok(CelerityResourceSpecWithSubs::Topic(
            validate_celerity_topic_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityQueue => Ok(CelerityResourceSpecWithSubs::Queue(
            validate_celerity_queue_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityVpc => Ok(CelerityResourceSpecWithSubs::Vpc(
            validate_celerity_vpc_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityDatastore => Ok(CelerityResourceSpecWithSubs::Datastore(
            validate_celerity_datastore_spec(spec_map)?,
        )),
    }
}

fn validate_celerity_handler_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityHandlerSpecWithSubs, BlueprintParseError> {
    let mut celerity_handler_spec = CelerityHandlerSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "handlerName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.handler_name =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for handlerName, found {value:?}",
                        )))?
                    }
                }
                "codeLocation" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.code_location =
                            Some(parse_substitutions::<ParseError>(value_str)?)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for codeLocation, found {value:?}",
                        )))?
                    }
                }
                "handler" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.handler =
                            parse_substitutions::<ParseError>(value_str)?
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for handler, found {value:?}",
                        )))?
                    }
                }
                "runtime" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.runtime =
                            Some(parse_substitutions::<ParseError>(value_str)?)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for runtime, found {value:?}",
                        )))?
                    }
                }
                "memory" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_handler_spec.memory =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.memory = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for memory, found {value:?}",
                        )))?
                    }
                }
                "timeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_handler_spec.timeout =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.timeout = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for timeout, found {value:?}",
                        )))?
                    }
                }
                "tracingEnabled" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_handler_spec.tracing_enabled =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.tracing_enabled = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for tracingEnabled, found {value:?}",
                        )))?
                    }
                }
                "environmentVariables" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_handler_spec.environment_variables =
                            Some(validate_map_of_strings(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for environmentVariables, found {value:?}",
                        )))?
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_handler_spec)
}

fn validate_celerity_schedule_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityScheduleSpecWithSubs, BlueprintParseError> {
    let mut celerity_schedule_spec = CelerityScheduleSpecWithSubs::default();
    if let Some(schedule_val) = spec_map.get(&yaml_rust2::Yaml::String("schedule".to_string())) {
        if let yaml_rust2::Yaml::String(schedule_str) = schedule_val {
            celerity_schedule_spec.schedule = parse_substitutions::<ParseError>(schedule_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for schedule, found {schedule_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a schedule field for schedule configuration".to_string(),
        ))?;
    }
    Ok(celerity_schedule_spec)
}

fn validate_celerity_consumer_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityConsumerSpecWithSubs, BlueprintParseError> {
    let mut celerity_consumer_spec = CelerityConsumerSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "sourceId" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.source_id =
                            Some(parse_substitutions::<ParseError>(value_str)?)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for sourceId, found {value:?}",
                        )))?
                    }
                }
                "batchSize" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.batch_size =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.batch_size = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for batchSize, found {value:?}",
                        )))?
                    }
                }
                "visibilityTimeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.visibility_timeout =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.visibility_timeout =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "integer",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for visibilityTimeout, found {value:?}",
                        )))?
                    }
                }
                "waitTimeSeconds" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.wait_time_seconds =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.wait_time_seconds =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "integer",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for waitTimeSeconds, found {value:?}",
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_consumer_spec.partial_failures =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.partial_failures =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "boolean",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for partialFailures, found {value:?}",
                        )))?
                    }
                }
                "routingKey" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.routing_key =
                            Some(parse_substitutions::<ParseError>(value_str)?)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for routingKey, found {value:?}",
                        )))?
                    }
                }
                "externalEvents" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_consumer_spec.external_events =
                            Some(validate_consumer_external_events_config_map(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for events, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_consumer_spec)
}

fn validate_consumer_external_events_config_map(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, ExternalEventConfigurationWithSubs>, BlueprintParseError> {
    let mut events = HashMap::new();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::Hash(value_map) = value {
                events.insert(
                    key_str.clone(),
                    validate_consumer_external_event_config(value_map)?,
                );
            }
        }
    }
    Ok(events)
}

fn validate_consumer_external_event_config(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<ExternalEventConfigurationWithSubs, BlueprintParseError> {
    let mut event_config = ExternalEventConfigurationWithSubs::default();

    // Make sure the event source type is known before validating the
    // source configuration.
    if let Some(source_type_val) =
        value_map.get(&yaml_rust2::Yaml::String("sourceType".to_string()))
    {
        if let yaml_rust2::Yaml::String(value_str) = source_type_val {
            event_config.source_type = validate_event_source_type(value_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for event source type, found {source_type_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a sourceType field for event configuration".to_string(),
        ))?;
    }

    if let Some(source_config) =
        value_map.get(&yaml_rust2::Yaml::String("sourceConfiguration".to_string()))
    {
        if let yaml_rust2::Yaml::Hash(source_config_map) = source_config {
            match event_config.source_type {
                EventSourceType::ObjectStorage => {
                    event_config.source_configuration =
                        EventSourceConfigurationWithSubs::ObjectStorage(
                            validate_event_source_object_storage_config(source_config_map)?,
                        )
                }
                EventSourceType::DatabaseStream => {
                    event_config.source_configuration =
                        EventSourceConfigurationWithSubs::DatabaseStream(
                            validate_event_source_database_stream_config(source_config_map)?,
                        )
                }
                EventSourceType::DataStream => {
                    event_config.source_configuration = EventSourceConfigurationWithSubs::DataStream(
                        validate_event_source_data_stream_config(source_config_map)?,
                    )
                }
            }
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for sourceConfiguration, found {source_config:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a sourceConfiguration field for event configuration".to_string(),
        ))?;
    }

    Ok(event_config)
}

fn validate_event_source_object_storage_config(
    source_config_map: &yaml_rust2::yaml::Hash,
) -> Result<ObjectStorageEventSourceConfigurationWithSubs, BlueprintParseError> {
    let mut object_storage_config = ObjectStorageEventSourceConfigurationWithSubs::default();
    if let Some(bucket_val) = source_config_map.get(&yaml_rust2::Yaml::String("bucket".to_string()))
    {
        if let yaml_rust2::Yaml::String(bucket_str) = bucket_val {
            object_storage_config.bucket = parse_substitutions::<ParseError>(bucket_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for bucket, found {bucket_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a bucket field for object storage event source configuration".to_string(),
        ))?;
    }

    if let Some(events_val) = source_config_map.get(&yaml_rust2::Yaml::String("events".to_string()))
    {
        if let yaml_rust2::Yaml::Array(events_arr) = events_val {
            object_storage_config.events = parse_substitutions_array(events_arr)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected an array for object source events, found {events_val:?}",
            )))?;
        }
    }
    Ok(object_storage_config)
}

fn parse_substitutions_array(
    events_arr: &yaml_rust2::yaml::Array,
) -> Result<Vec<StringOrSubstitutions>, BlueprintParseError> {
    let mut object_storage_events = Vec::new();
    for event_type in events_arr {
        if let yaml_rust2::Yaml::String(event_str) = event_type {
            object_storage_events.push(parse_substitutions::<ParseError>(event_str)?);
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for object storage source event, found {event_type:?}",
            )))?;
        }
    }
    Ok(object_storage_events)
}

fn validate_event_source_database_stream_config(
    source_config_map: &yaml_rust2::yaml::Hash,
) -> Result<DatabaseStreamSourceConfigurationWithSubs, BlueprintParseError> {
    let mut database_stream_config = DatabaseStreamSourceConfigurationWithSubs::default();

    if let Some(db_stream_id_val) =
        source_config_map.get(&yaml_rust2::Yaml::String("dbStreamId".to_string()))
    {
        if let yaml_rust2::Yaml::String(db_stream_id) = db_stream_id_val {
            database_stream_config.db_stream_id = parse_substitutions::<ParseError>(db_stream_id)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for dbStreamId, found {db_stream_id_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a dbStreamId field for database stream event source configuration"
                .to_string(),
        ))?;
    }

    for (key, value) in source_config_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "batchSize" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        database_stream_config.batch_size =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        database_stream_config.batch_size = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for batchSize, found {value:?}",
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        database_stream_config.partial_failures =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        database_stream_config.partial_failures =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "boolean",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for partialFailures, found {value:?}",                            
                        )))?
                    }
                }
                "startFromBeginning" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        database_stream_config.start_from_beginning =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        database_stream_config.start_from_beginning =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "boolean",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for startFromBeginning, found {value:?}",
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    Ok(database_stream_config)
}

fn validate_event_source_data_stream_config(
    source_config_map: &yaml_rust2::yaml::Hash,
) -> Result<DataStreamSourceConfigurationWithSubs, BlueprintParseError> {
    let mut data_stream_config = DataStreamSourceConfigurationWithSubs::default();

    if let Some(data_stream_id_val) =
        source_config_map.get(&yaml_rust2::Yaml::String("dataStreamId".to_string()))
    {
        if let yaml_rust2::Yaml::String(data_stream_id) = data_stream_id_val {
            data_stream_config.data_stream_id = parse_substitutions::<ParseError>(data_stream_id)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for dataStreamId, found {data_stream_id_val:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a dataStreamId field for database stream event source configuration"
                .to_string(),
        ))?;
    }

    for (key, value) in source_config_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "batchSize" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        data_stream_config.batch_size =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        data_stream_config.batch_size = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for batchSize, found {value:?}",
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        data_stream_config.partial_failures =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        data_stream_config.partial_failures = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for partialFailures, found {value:?}",
                        )))?
                    }
                }
                "startFromBeginning" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        data_stream_config.start_from_beginning =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        data_stream_config.start_from_beginning =
                            Some(MappingNode::SubstitutionStr(validate_single_substitution(
                                value_str, "boolean",
                            )?))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for startFromBeginning, found {value:?}",
                        )))?
                    }
                }
                _ => (),
            }
        }
    }
    Ok(data_stream_config)
}

// Substitutions are not supported for event source types as the type needs to be known
// to determine which configuration object to extract and validate from the yaml document.
fn validate_event_source_type(
    source_type: &String,
) -> Result<EventSourceType, BlueprintParseError> {
    match source_type.as_str() {
        "objectStorage" => Ok(EventSourceType::ObjectStorage),
        "dbStream" => Ok(EventSourceType::DatabaseStream),
        "dataStream" => Ok(EventSourceType::DataStream),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported event source type, found {source_type}",
        ))),
    }
}

fn validate_map_of_strings(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, StringOrSubstitutions>, BlueprintParseError> {
    let mut map = HashMap::new();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::String(value_str) = value {
                map.insert(
                    key_str.clone(),
                    parse_substitutions::<ParseError>(value_str)?,
                );
            } else {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a string for environment variable value, found {value:?}",
                )))?;
            }
        }
    }
    Ok(map)
}

fn validate_celerity_api_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiSpecWithSubs, BlueprintParseError> {
    let mut celerity_api_spec = CelerityApiSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocols" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut protocols = Vec::new();
                        for item in value_arr {
                            let protocol_opt = validate_api_protocol(item)?;
                            if let Some(protocol) = protocol_opt {
                                protocols.push(protocol);
                            }
                        }
                        celerity_api_spec.protocols = protocols;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for api protocols, found {value:?}",
                        )))?;
                    }
                }
                "cors" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_api_spec.cors =
                            Some(CelerityApiCorsWithSubs::Str(parse_substitutions::<
                                ParseError,
                            >(
                                value_str
                            )?))
                    } else if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.cors = Some(CelerityApiCorsWithSubs::CorsConfiguration(
                            validate_celerity_api_cors_config(value_map)?,
                        ))
                    }
                }
                "domain" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.domain = Some(validate_celerity_api_domain(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for domain configuration, found {value:?}",
                        )))?;
                    }
                }
                "auth" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.auth = Some(validate_celerity_api_auth(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for auth, found {value:?}",
                        )))?;
                    }
                }
                "tracingEnabled" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_api_spec.tracing_enabled =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_api_spec.tracing_enabled = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for tracingEnabled, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }

    if celerity_api_spec.protocols.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "at least one protocol must be provided for the api spec".to_string(),
        ));
    }

    Ok(celerity_api_spec)
}

fn validate_celerity_api_cors_config(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiCorsConfigurationWithSubs, BlueprintParseError> {
    let mut cors_config = CelerityApiCorsConfigurationWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "allowCredentials" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        cors_config.allow_credentials =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        cors_config.allow_credentials = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for allowCredentials, found {value:?}",
                        )))?;
                    }
                }
                "allowOrigins" => {
                    cors_config.allow_origins = validate_cors_item_array(value, "allow_origins")?
                }
                "allowMethods" => {
                    cors_config.allow_methods = validate_cors_item_array(value, "allow_methods")?
                }
                "allowHeaders" => {
                    cors_config.allow_headers = validate_cors_item_array(value, "allow_headers")?
                }
                "exposeHeaders" => {
                    cors_config.expose_headers = validate_cors_item_array(value, "expose_headers")?
                }
                "maxAge" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        cors_config.max_age =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        cors_config.max_age = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for maxAge, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(cors_config)
}

fn validate_cors_item_array(
    value: &yaml_rust2::Yaml,
    field: &str,
) -> Result<Option<Vec<StringOrSubstitutions>>, BlueprintParseError> {
    let mut values = Vec::new();
    if let yaml_rust2::Yaml::Array(value_arr) = value {
        for item in value_arr {
            if let yaml_rust2::Yaml::String(value_str) = item {
                values.push(parse_substitutions::<ParseError>(value_str)?);
            } else {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a string for {field}, found {item:?}",
                )))?;
            }
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "expected an array for {field}, found {value:?}",
        )))?;
    }
    Ok(Some(values))
}

fn validate_celerity_api_domain(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiDomainWithSubs, BlueprintParseError> {
    let mut domain = CelerityApiDomainWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "domainName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.domain_name = parse_substitutions::<ParseError>(value_str)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for domain name, found {value:?}",
                        )))?;
                    }
                }
                "basePaths" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut base_paths = Vec::new();
                        for item in value_arr {
                            if let yaml_rust2::Yaml::String(value_str) = item {
                                base_paths.push(CelerityApiBasePathWithSubs::Str(
                                    parse_substitutions::<ParseError>(value_str)?,
                                ));
                            } else if let yaml_rust2::Yaml::Hash(value_map) = item {
                                base_paths.push(
                                    CelerityApiBasePathWithSubs::BasePathConfiguration(
                                        validate_celerity_api_base_path_config(value_map)?,
                                    ),
                                );
                            } else {
                                Err(BlueprintParseError::YamlFormatError(format!(
                                    "expected a string or mapping for base path, found {item:?}",
                                )))?;
                            }
                        }
                        domain.base_paths = base_paths;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for base paths, found {value:?}",
                        )))?;
                    }
                }
                "normalizeBasePath" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        domain.normalize_base_path =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.normalize_base_path = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for normalizeBasePath, found {value:?}",
                        )))?;
                    }
                }
                "certificateId" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.certificate_id = parse_substitutions::<ParseError>(value_str)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for certificateId, found {value:?}",
                        )))?;
                    }
                }
                "securityPolicy" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.security_policy =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for securityPolicy, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }

    if is_string_with_substitutions_empty(&domain.domain_name) {
        return Err(BlueprintParseError::YamlFormatError(
            "domainName must be defined for the domain configuration".to_string(),
        ));
    }

    if domain.base_paths.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "at least one basePath must be defined for the domain configuration".to_string(),
        ));
    }

    if is_string_with_substitutions_empty(&domain.certificate_id) {
        return Err(BlueprintParseError::YamlFormatError(
            "certificateId must be defined for the domain configuration".to_string(),
        ));
    }

    Ok(domain)
}

fn validate_api_protocol(
    protocol_item: &yaml_rust2::Yaml,
) -> Result<Option<MappingNode>, BlueprintParseError> {
    if let yaml_rust2::Yaml::String(protocol_str) = protocol_item {
        match protocol_str.as_str() {
            "http" => Ok(Some(MappingNode::Scalar(BlueprintScalarValue::Str("http".to_string())))),
            "websocket" => Ok(Some(MappingNode::Scalar(BlueprintScalarValue::Str("websocket".to_string())))),
            _ => Err(BlueprintParseError::YamlFormatError(format!(
                "expected a supported api protocol (\\\"http\\\" or \\\"websocket\\\" or websocket configuration object), found {protocol_str}",
            ))),
        }
    } else if let yaml_rust2::Yaml::Hash(protocol_map) = protocol_item {
        if let Some(config_item) =
            protocol_map.get(&yaml_rust2::Yaml::String("websocketConfig".to_string()))
        {
            if let yaml_rust2::Yaml::Hash(config_map) = config_item {
                let websocket_config = validate_websocket_config(config_map)?;
                Ok(Some(MappingNode::Mapping(HashMap::from([(
                    "websocketConfig".to_string(),
                    websocket_config,
                )]))))
            } else {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a mapping for websocket configuration, found {config_item:?}",
                )))
            }
        } else {
            Err(BlueprintParseError::YamlFormatError(
                "expected a websocket configuration object for api protocol".to_string(),
            ))
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "expected a string or websocket configuration object for api protocol, found {protocol_item:?}",
        )))
    }
}

fn validate_websocket_config(
    websocket_map: &yaml_rust2::yaml::Hash,
) -> Result<MappingNode, BlueprintParseError> {
    let mut websocket_config_map = HashMap::new();
    for (key, value) in websocket_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "routeKey" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        websocket_config_map.insert(
                            "routeKey".to_string(),
                            MappingNode::SubstitutionStr(parse_substitutions::<ParseError>(
                                value_str,
                            )?),
                        );
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for routeKey, found {value:?}",
                        )))?;
                    }
                }
                "authStrategy" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        websocket_config_map.insert(
                            "authStrategy".to_string(),
                            MappingNode::SubstitutionStr(parse_substitutions::<ParseError>(
                                value_str,
                            )?),
                        );
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for authStrategy, found {value:?}",
                        )))?;
                    }
                }
                "authGuard" => match value {
                    yaml_rust2::Yaml::String(value_str) => {
                        websocket_config_map.insert(
                            "authGuard".to_string(),
                            MappingNode::Sequence(vec![MappingNode::SubstitutionStr(
                                parse_substitutions::<ParseError>(value_str)?,
                            )]),
                        );
                    }
                    yaml_rust2::Yaml::Array(arr) => {
                        let mut items = Vec::new();
                        for item in arr {
                            if let yaml_rust2::Yaml::String(s) = item {
                                items.push(MappingNode::SubstitutionStr(
                                    parse_substitutions::<ParseError>(s)?,
                                ));
                            } else {
                                Err(BlueprintParseError::YamlFormatError(format!(
                                    "expected a string in authGuard array, found {item:?}",
                                )))?;
                            }
                        }
                        websocket_config_map.insert(
                            "authGuard".to_string(),
                            MappingNode::Sequence(items),
                        );
                    }
                    _ => {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string or array for authGuard, found {value:?}",
                        )))?;
                    }
                },
                _ => (),
            }
        }
    }
    Ok(MappingNode::Mapping(websocket_config_map))
}

fn validate_celerity_api_auth(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiAuthWithSubs, BlueprintParseError> {
    let mut auth = CelerityApiAuthWithSubs::default();

    if let Some(guards) = value_map.get(&yaml_rust2::Yaml::String("guards".to_string())) {
        if let yaml_rust2::Yaml::Hash(value_map) = guards {
            auth.guards = validate_celerity_api_auth_guards(value_map)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for guards, found {guards:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a guards field for auth configuration".to_string(),
        ))?;
    }

    if let Some(default_guard) =
        value_map.get(&yaml_rust2::Yaml::String("defaultGuard".to_string()))
    {
        match default_guard {
            yaml_rust2::Yaml::String(value_str) => {
                auth.default_guard =
                    Some(vec![parse_substitutions::<ParseError>(value_str)?]);
            }
            yaml_rust2::Yaml::Array(arr) => {
                let mut guard_names = Vec::new();
                for item in arr {
                    if let yaml_rust2::Yaml::String(s) = item {
                        guard_names.push(parse_substitutions::<ParseError>(s)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string in defaultGuard array, found {item:?}",
                        )))?;
                    }
                }
                auth.default_guard = Some(guard_names);
            }
            _ => {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a string or array for defaultGuard, found {default_guard:?}",
                )))?;
            }
        }
    }

    Ok(auth)
}

fn validate_celerity_api_auth_guards(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, CelerityApiAuthGuardWithSubs>, BlueprintParseError> {
    let mut guards = HashMap::new();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::Hash(value_map) = value {
                guards.insert(
                    key_str.clone(),
                    validate_celerity_api_auth_guard(value_map)?,
                );
            }
        }
    }
    Ok(guards)
}

fn validate_celerity_api_auth_guard(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiAuthGuardWithSubs, BlueprintParseError> {
    let mut guard = CelerityApiAuthGuardWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "type" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.guard_type =
                            validate_celerity_api_auth_guard_type(value_str.clone())?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for type, found {value:?}",
                        )))?;
                    }
                }
                "issuer" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.issuer = Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for issuer, found {value:?}",
                        )))?;
                    }
                }
                "tokenSource" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        guard.token_source = Some(
                            CelerityApiAuthGuardValueSourceWithSubs::ValueSourceConfiguration(
                                validate_celerity_api_auth_value_source_configs(
                                    value_arr, "token",
                                )?,
                            ),
                        )
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.token_source = Some(CelerityApiAuthGuardValueSourceWithSubs::Str(
                            parse_substitutions::<ParseError>(value_str)?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string or array for token source, found {value:?}",
                        )))?;
                    }
                }
                "audience" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut audiences = Vec::new();
                        for item in value_arr {
                            if let yaml_rust2::Yaml::String(value_str) = item {
                                audiences.push(parse_substitutions::<ParseError>(value_str)?);
                            } else {
                                Err(BlueprintParseError::YamlFormatError(format!(
                                    "expected a string for audience, found {item:?}",
                                )))?;
                            }
                        }
                        guard.audience = Some(audiences);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for audience, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }

    if is_string_with_substitutions_empty(&guard.guard_type) {
        return Err(BlueprintParseError::YamlFormatError(
            "type must be defined for an auth guard".to_string(),
        ));
    }

    Ok(guard)
}

fn validate_celerity_api_auth_value_source_configs(
    value_arr: &yaml_rust2::yaml::Array,
    context: &str,
) -> Result<Vec<ValueSourceConfigurationWithSubs>, BlueprintParseError> {
    let mut value_source_configs = Vec::new();
    for item in value_arr {
        if let yaml_rust2::Yaml::Hash(value_map) = item {
            let value_source_config =
                validate_celerity_api_auth_value_source_config(value_map, context)?;
            value_source_configs.push(value_source_config);
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for \\\"{context}\\\" value source, found {item:?}",
            )))?;
        }
    }

    Ok(value_source_configs)
}

fn validate_celerity_api_auth_value_source_config(
    value_map: &yaml_rust2::yaml::Hash,
    context: &str,
) -> Result<ValueSourceConfigurationWithSubs, BlueprintParseError> {
    let mut value_source_config = ValueSourceConfigurationWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocol" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        match val_str.as_str() {
                            "http" => value_source_config.protocol = MappingNode::Scalar(BlueprintScalarValue::Str("http".to_string())),
                            "websocket" => {
                                value_source_config.protocol = MappingNode::Scalar(BlueprintScalarValue::Str("websocket".to_string()))
                            }
                            _ => Err(BlueprintParseError::YamlFormatError(format!(
                                "expected \\\"http\\\" or \\\"websocket\\\" for \\\"{context}\\\" value source protocol, found {value:?}",
                            )))?,
                        }
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected \\\"http\\\" or \\\"websocket\\\" for \\\"{context}\\\" value source protocol, found {value:?}",
                        )))?;
                    }
                }
                "source" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        value_source_config.source = parse_substitutions::<ParseError>(val_str)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for \\\"{context}\\\" value source, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(value_source_config)
}

fn validate_celerity_api_base_path_config(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiBasePathConfigurationWithSubs, BlueprintParseError> {
    let mut base_path_config = CelerityApiBasePathConfigurationWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocol" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        match val_str.as_str() {
                            "http" => base_path_config.protocol = MappingNode::Scalar(BlueprintScalarValue::Str("http".to_string())),
                            "websocket" => {
                                base_path_config.protocol = MappingNode::Scalar(BlueprintScalarValue::Str("websocket".to_string()))
                            }
                            _ => Err(BlueprintParseError::YamlFormatError(format!(
                                "expected \\\"http\\\" or \\\"websocket\\\" for base path protocol, found {value:?}",
                            )))?,
                        }
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected \\\"http\\\" or \\\"websocket\\\" for base path protocol, found {value:?}",
                        )))?;
                    }
                }
                "basePath" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        base_path_config.base_path = parse_substitutions::<ParseError>(val_str)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for base path, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(base_path_config)
}

fn validate_celerity_api_auth_guard_type(
    guard_type: String,
) -> Result<StringOrSubstitutions, BlueprintParseError> {
    match guard_type.as_str() {
        "jwt" => Ok(StringOrSubstitutions {
            values: vec![StringOrSubstitution::StringValue("jwt".to_string())],
        }),
        "custom" => Ok(StringOrSubstitutions {
            values: vec![StringOrSubstitution::StringValue("custom".to_string())],
        }),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported guard type (\\\"jwt\\\", \\\"custom\\\"), found {guard_type}",
        ))),
    }
}

fn validate_resource_metadata(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<BlueprintResourceMetadataWithSubs, BlueprintParseError> {
    let mut resource_metadata = BlueprintResourceMetadataWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "displayName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        resource_metadata.display_name =
                            parse_substitutions::<ParseError>(value_str)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for resource display name, found {value:?}",
                        )))?;
                    }
                }
                "annotations" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        let mut annotations = HashMap::new();
                        for (key, value) in value_map {
                            if let yaml_rust2::Yaml::String(key_str) = key {
                                let key_str = key_str.clone();
                                let node_value = validate_mapping_node(value, "annotations")?;
                                annotations.insert(key_str, node_value);
                            }
                        }
                        resource_metadata.annotations = Some(annotations);
                    }
                }
                "labels" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        let mut labels = HashMap::new();
                        for (key, value) in value_map {
                            if let yaml_rust2::Yaml::String(key_str) = key {
                                let key_str = key_str.clone();
                                match value {
                                    yaml_rust2::Yaml::String(value_str) => {
                                        labels.insert(key_str, value_str.clone());
                                    }
                                    _ => Err(BlueprintParseError::YamlFormatError(format!(
                                        "expected a string for label value, found {value:?}",
                                    )))?,
                                }
                            }
                        }
                        resource_metadata.labels = Some(labels);
                    }
                }
                _ => (),
            }
        }
    }

    if is_string_with_substitutions_empty(&resource_metadata.display_name) {
        Err(BlueprintParseError::YamlFormatError(
            "expected a display name for resource metadata".to_string(),
        ))?;
    }
    Ok(resource_metadata)
}

fn validate_link_selector(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<BlueprintLinkSelector, BlueprintParseError> {
    let mut link_selector = BlueprintLinkSelector::default();
    let by_label = value_map.get(&yaml_rust2::Yaml::String("byLabel".to_string()));
    if let Some(by_label_value) = by_label {
        if let yaml_rust2::Yaml::Hash(by_label_map) = by_label_value {
            populate_by_label_selectors(&mut link_selector, by_label_map);
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for byLabel link selector, found {by_label_value:?}",
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a byLabel field for link selector".to_string(),
        ))?;
    }
    Ok(link_selector)
}

fn populate_by_label_selectors(
    link_selector: &mut BlueprintLinkSelector,
    by_label_map: &LinkedHashMap<yaml_rust2::Yaml, yaml_rust2::Yaml>,
) {
    for (key, value) in by_label_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            let key_str = key_str.clone();
            if let yaml_rust2::Yaml::String(value_str) = value {
                link_selector.by_label.insert(key_str, value_str.clone());
            }
        }
    }
}

fn validate_celerity_config_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityConfigSpecWithSubs, BlueprintParseError> {
    let mut celerity_config_spec = CelerityConfigSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "name" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_config_spec.name =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for name, found {value:?}",
                        )))?;
                    }
                }
                "plaintext" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        celerity_config_spec.plaintext =
                            Some(validate_array_of_strings(value_arr, "plaintext")?)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for plaintext, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_config_spec)
}

fn validate_celerity_bucket_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityBucketSpecWithSubs, BlueprintParseError> {
    let mut celerity_bucket_spec = CelerityBucketSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if key_str.as_str() == "name" {
                if let yaml_rust2::Yaml::String(value_str) = value {
                    celerity_bucket_spec.name = Some(parse_substitutions::<ParseError>(value_str)?);
                } else {
                    Err(BlueprintParseError::YamlFormatError(format!(
                        "expected a string for name, found {value:?}",
                    )))?;
                }
            }
        }
    }
    Ok(celerity_bucket_spec)
}

fn validate_celerity_topic_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityTopicSpecWithSubs, BlueprintParseError> {
    let mut celerity_topic_spec = CelerityTopicSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "name" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_topic_spec.name =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for name, found {value:?}",
                        )))?;
                    }
                }
                "fifo" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_topic_spec.fifo =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_topic_spec.fifo = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for fifo, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_topic_spec)
}

fn validate_celerity_queue_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityQueueSpecWithSubs, BlueprintParseError> {
    let mut celerity_queue_spec = CelerityQueueSpecWithSubs::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "name" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_queue_spec.name =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for name, found {value:?}",
                        )))?;
                    }
                }
                "fifo" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_queue_spec.fifo =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_queue_spec.fifo = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for fifo, found {value:?}",
                        )))?;
                    }
                }
                "visibilityTimeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_queue_spec.visibility_timeout =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_queue_spec.visibility_timeout = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for visibilityTimeout, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_queue_spec)
}

fn validate_celerity_vpc_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityVpcSpecWithSubs, BlueprintParseError> {
    let mut celerity_vpc_spec = CelerityVpcSpecWithSubs::default();
    let mut has_name = false;

    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "name" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_vpc_spec.name = parse_substitutions::<ParseError>(value_str)?;
                        has_name = true;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "VPC name must be a string".to_string(),
                        ));
                    }
                }
                "preset" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_vpc_spec.preset =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "VPC preset must be a string".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for VPC spec: {}",
                        key_str
                    )));
                }
            }
        }
    }

    if !has_name {
        return Err(BlueprintParseError::YamlFormatError(
            "VPC spec requires 'name' field".to_string(),
        ));
    }

    Ok(celerity_vpc_spec)
}

fn validate_celerity_datastore_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityDatastoreSpecWithSubs, BlueprintParseError> {
    let mut datastore_spec = CelerityDatastoreSpecWithSubs::default();
    let mut has_keys = false;

    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "keys" => {
                    if let yaml_rust2::Yaml::Hash(keys_map) = value {
                        datastore_spec.keys = parse_datastore_keys(keys_map)?;
                        has_keys = true;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore keys must be an object".to_string(),
                        ));
                    }
                }
                "name" => {
                    if let yaml_rust2::Yaml::String(name_str) = value {
                        datastore_spec.name = Some(parse_substitutions::<ParseError>(name_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore name must be a string".to_string(),
                        ));
                    }
                }
                "schema" => {
                    if let yaml_rust2::Yaml::Hash(schema_map) = value {
                        let mut schema = HashMap::new();
                        for (field_key, field_value) in schema_map {
                            if let yaml_rust2::Yaml::String(field_name) = field_key {
                                if let yaml_rust2::Yaml::Hash(field_def) = field_value {
                                    schema.insert(
                                        field_name.clone(),
                                        parse_datastore_field_schema(field_def)?,
                                    );
                                } else {
                                    return Err(BlueprintParseError::YamlFormatError(format!(
                                        "Schema field '{}' must be an object",
                                        field_name
                                    )));
                                }
                            }
                        }
                        datastore_spec.schema = Some(schema);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore schema must be an object".to_string(),
                        ));
                    }
                }
                "indexes" => {
                    if let yaml_rust2::Yaml::Array(indexes_array) = value {
                        let mut indexes = Vec::new();
                        for index_value in indexes_array {
                            if let yaml_rust2::Yaml::Hash(index_map) = index_value {
                                indexes.push(parse_datastore_index(index_map)?);
                            } else {
                                return Err(BlueprintParseError::YamlFormatError(
                                    "Each index must be an object".to_string(),
                                ));
                            }
                        }
                        datastore_spec.indexes = Some(indexes);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore indexes must be an array".to_string(),
                        ));
                    }
                }
                "timeToLive" => {
                    if let yaml_rust2::Yaml::Hash(ttl_map) = value {
                        datastore_spec.time_to_live = Some(parse_datastore_ttl(ttl_map)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore timeToLive must be an object".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for Datastore spec: {}",
                        key_str
                    )));
                }
            }
        }
    }

    if !has_keys {
        return Err(BlueprintParseError::YamlFormatError(
            "Datastore spec requires 'keys' field".to_string(),
        ));
    }

    Ok(datastore_spec)
}

fn parse_datastore_keys(
    keys_map: &yaml_rust2::yaml::Hash,
) -> Result<DatastoreKeysWithSubs, BlueprintParseError> {
    let mut keys = DatastoreKeysWithSubs::default();
    let mut has_partition_key = false;

    for (key, value) in keys_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "partitionKey" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        keys.partition_key = parse_substitutions::<ParseError>(value_str)?;
                        has_partition_key = true;
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore partitionKey must be a string".to_string(),
                        ));
                    }
                }
                "sortKey" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        keys.sort_key = Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Datastore sortKey must be a string".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for Datastore keys: {}",
                        key_str
                    )));
                }
            }
        }
    }

    if !has_partition_key {
        return Err(BlueprintParseError::YamlFormatError(
            "Datastore keys require 'partitionKey' field".to_string(),
        ));
    }

    Ok(keys)
}

fn parse_datastore_field_schema(
    field_map: &yaml_rust2::yaml::Hash,
) -> Result<DatastoreFieldSchemaWithSubs, BlueprintParseError> {
    let mut field_type: Option<StringOrSubstitutions> = None;
    let mut description: Option<StringOrSubstitutions> = None;
    let mut nullable: Option<MappingNode> = None;
    let mut fields: Option<HashMap<String, DatastoreFieldSchemaWithSubs>> = None;
    let mut items: Option<Box<DatastoreFieldSchemaWithSubs>> = None;

    for (key, value) in field_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "type" => {
                    if let yaml_rust2::Yaml::String(type_str) = value {
                        field_type = Some(parse_substitutions::<ParseError>(type_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Schema field type must be a string".to_string(),
                        ));
                    }
                }
                "description" => {
                    if let yaml_rust2::Yaml::String(desc_str) = value {
                        description = Some(parse_substitutions::<ParseError>(desc_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Schema field description must be a string".to_string(),
                        ));
                    }
                }
                "nullable" => {
                    if let yaml_rust2::Yaml::Boolean(bool_val) = value {
                        nullable = Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*bool_val)));
                    } else if let yaml_rust2::Yaml::String(bool_str) = value {
                        nullable = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(bool_str, "boolean")?,
                        ));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Schema field nullable must be a boolean".to_string(),
                        ));
                    }
                }
                "fields" => {
                    if let yaml_rust2::Yaml::Hash(nested_fields_map) = value {
                        let mut nested_fields = HashMap::new();
                        for (nested_key, nested_value) in nested_fields_map {
                            if let yaml_rust2::Yaml::String(nested_name) = nested_key {
                                if let yaml_rust2::Yaml::Hash(nested_def) = nested_value {
                                    nested_fields.insert(
                                        nested_name.clone(),
                                        parse_datastore_field_schema(nested_def)?,
                                    );
                                } else {
                                    return Err(BlueprintParseError::YamlFormatError(format!(
                                        "Nested field '{}' must be an object",
                                        nested_name
                                    )));
                                }
                            }
                        }
                        fields = Some(nested_fields);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Schema field 'fields' must be an object".to_string(),
                        ));
                    }
                }
                "items" => {
                    if let yaml_rust2::Yaml::Hash(items_map) = value {
                        items = Some(Box::new(parse_datastore_field_schema(items_map)?));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Schema field 'items' must be an object".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for schema field: {}",
                        key_str
                    )));
                }
            }
        }
    }

    let field_type = field_type.ok_or_else(|| {
        BlueprintParseError::YamlFormatError("Schema field requires 'type' field".to_string())
    })?;

    Ok(DatastoreFieldSchemaWithSubs {
        field_type,
        description,
        nullable,
        fields,
        items,
    })
}

fn parse_datastore_index(
    index_map: &yaml_rust2::yaml::Hash,
) -> Result<DatastoreIndexWithSubs, BlueprintParseError> {
    let mut name: Option<StringOrSubstitutions> = None;
    let mut fields: Option<Vec<StringOrSubstitutions>> = None;

    for (key, value) in index_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "name" => {
                    if let yaml_rust2::Yaml::String(name_str) = value {
                        name = Some(parse_substitutions::<ParseError>(name_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Index name must be a string".to_string(),
                        ));
                    }
                }
                "fields" => {
                    if let yaml_rust2::Yaml::Array(fields_array) = value {
                        let mut field_names = Vec::new();
                        for field_value in fields_array {
                            if let yaml_rust2::Yaml::String(field_str) = field_value {
                                field_names.push(parse_substitutions::<ParseError>(field_str)?);
                            } else {
                                return Err(BlueprintParseError::YamlFormatError(
                                    "Index field names must be strings".to_string(),
                                ));
                            }
                        }
                        fields = Some(field_names);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "Index fields must be an array".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for index: {}",
                        key_str
                    )));
                }
            }
        }
    }

    let name = name.ok_or_else(|| {
        BlueprintParseError::YamlFormatError("Index requires 'name' field".to_string())
    })?;

    let fields = fields.ok_or_else(|| {
        BlueprintParseError::YamlFormatError("Index requires 'fields' field".to_string())
    })?;

    Ok(DatastoreIndexWithSubs { name, fields })
}

fn parse_datastore_ttl(
    ttl_map: &yaml_rust2::yaml::Hash,
) -> Result<DatastoreTimeToLiveWithSubs, BlueprintParseError> {
    let mut field_name: Option<StringOrSubstitutions> = None;
    let mut enabled: Option<MappingNode> = None;

    for (key, value) in ttl_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "fieldName" => {
                    if let yaml_rust2::Yaml::String(name_str) = value {
                        field_name = Some(parse_substitutions::<ParseError>(name_str)?);
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "TTL fieldName must be a string".to_string(),
                        ));
                    }
                }
                "enabled" => {
                    if let yaml_rust2::Yaml::Boolean(bool_val) = value {
                        enabled = Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*bool_val)));
                    } else if let yaml_rust2::Yaml::String(bool_str) = value {
                        enabled = Some(MappingNode::SubstitutionStr(validate_single_substitution(
                            bool_str, "boolean",
                        )?));
                    } else {
                        return Err(BlueprintParseError::YamlFormatError(
                            "TTL enabled must be a boolean".to_string(),
                        ));
                    }
                }
                _ => {
                    return Err(BlueprintParseError::YamlFormatError(format!(
                        "Unsupported key for timeToLive: {}",
                        key_str
                    )));
                }
            }
        }
    }

    let field_name = field_name.ok_or_else(|| {
        BlueprintParseError::YamlFormatError("TTL requires 'fieldName' field".to_string())
    })?;

    let enabled = enabled.ok_or_else(|| {
        BlueprintParseError::YamlFormatError("TTL requires 'enabled' field".to_string())
    })?;

    Ok(DatastoreTimeToLiveWithSubs {
        field_name,
        enabled,
    })
}

fn validate_populate_blueprint_metadata(
    value_map: &yaml_rust2::yaml::Hash,
    blueprint: &mut BlueprintConfigWithSubs,
) -> Result<(), BlueprintParseError> {
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if key_str.as_str() == "sharedHandlerConfig" {
                if let yaml_rust2::Yaml::Hash(value_map) = value {
                    blueprint.metadata = Some(BlueprintMetadataWithSubs {
                        shared_handler_config: Some(validate_shared_handler_config(value_map)?),
                    });
                }
            }
        }
    }
    Ok(())
}

fn validate_shared_handler_config(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<SharedHandlerConfigWithSubs, BlueprintParseError> {
    let mut shared_handler_config = SharedHandlerConfigWithSubs::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "codeLocation" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        shared_handler_config.code_location =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for codeLocation, found {value:?}",
                        )))?;
                    }
                }
                "runtime" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        shared_handler_config.runtime =
                            Some(parse_substitutions::<ParseError>(value_str)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for runtime, found {value:?}",
                        )))?;
                    }
                }
                "memory" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        shared_handler_config.memory =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)));
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        shared_handler_config.memory = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ));
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for memory, found {value:?}",
                        )))?;
                    }
                }
                "timeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        shared_handler_config.timeout =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Int(*value_int)));
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        shared_handler_config.timeout = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "integer")?,
                        ));
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer or ${{..}} substitution for timeout, found {value:?}",
                        )))?;
                    }
                }
                "tracingEnabled" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        shared_handler_config.tracing_enabled =
                            Some(MappingNode::Scalar(BlueprintScalarValue::Bool(*value_bool)));
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        shared_handler_config.tracing_enabled = Some(MappingNode::SubstitutionStr(
                            validate_single_substitution(value_str, "boolean")?,
                        ));
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean or ${{..}} substitution for tracingEnabled, found {value:?}",
                        )))?;
                    }
                }
                "environmentVariables" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        shared_handler_config.environment_variables =
                            Some(validate_map_of_strings(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for environmentVariables, found {value:?}",
                        )))?;
                    }
                }
                _ => (),
            }
        }
    }
    Ok(shared_handler_config)
}
