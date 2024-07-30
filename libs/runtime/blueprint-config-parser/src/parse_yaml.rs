use std::collections::HashMap;

use hashlink::LinkedHashMap;
use tracing::debug;

use crate::{
    blueprint::{
        BlueprintConfig, BlueprintLinkSelector, BlueprintResourceMetadata, BlueprintScalarValue,
        BlueprintVariable, CelerityApiAuth, CelerityApiAuthGuard, CelerityApiAuthGuardType,
        CelerityApiAuthGuardValueSource, CelerityApiBasePath, CelerityApiBasePathConfiguration,
        CelerityApiCors, CelerityApiCorsConfiguration, CelerityApiDomain,
        CelerityApiDomainSecurityPolicy, CelerityApiProtocol, CelerityApiSpec,
        CelerityConsumerSpec, CelerityHandlerSpec, CelerityResourceSpec, CelerityResourceType,
        CelerityScheduleSpec, DataStreamSourceConfiguration, DatabaseStreamSourceConfiguration,
        EventConfiguration, EventSourceConfiguration, EventSourceType,
        ObjectStorageEventSourceConfiguration, ObjectStorageEventType, RuntimeBlueprintResource,
        ValueSourceConfiguration, CELERITY_API_RESOURCE_TYPE, CELERITY_BLUEPRINT_V2023_04_20,
        CELERITY_CONSUMER_RESOURCE_TYPE, CELERITY_HANDLER_RESOURCE_TYPE,
        CELERITY_SCHEDULE_RESOURCE_TYPE,
    },
    parse::BlueprintParseError,
};

pub fn build_blueprint_config_from_yaml(
    yaml: &yaml_rust2::Yaml,
) -> Result<BlueprintConfig, BlueprintParseError> {
    let mut blueprint = BlueprintConfig::default();
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
                                _ => (),
                            }
                        }
                        _ => (),
                    }
                }
            }
        }
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a mapping for blueprint, found {:?}",
            yaml
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
    blueprint: &mut BlueprintConfig,
) -> Result<(), BlueprintParseError> {
    if version != CELERITY_BLUEPRINT_V2023_04_20 {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "expected version {}, found {}",
            CELERITY_BLUEPRINT_V2023_04_20, version
        )));
    }
    blueprint.version = version.to_string();
    Ok(())
}

fn validate_populate_variables(
    yaml_vars: &yaml_rust2::yaml::Hash,
    blueprint: &mut BlueprintConfig,
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
                            "expected a string for variable type, found {:?}",
                            value
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
                            "expected a string for variable description, found {:?}",
                            value,
                        )))?
                    }
                }
                "secret" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        blueprint_var.secret = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for variable secret field, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if blueprint_var.var_type.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "type must be provided in \\\"{}\\\" variable definition",
            var_name,
        )));
    }

    Ok(blueprint_var)
}

fn validate_populate_resources(
    yaml_resources: &yaml_rust2::yaml::Hash,
    blueprint: &mut BlueprintConfig,
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
                        debug!(
                            error = err.to_string(),
                            "skipping resource \\\"{}\\\" as it is either invalid \
                            or not a supported celerity runtime resource",
                            key_str,
                        );
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
) -> Result<RuntimeBlueprintResource, BlueprintParseError> {
    let mut blueprint_resource = RuntimeBlueprintResource::default();

    // Make sure the resource type is known before validating the spec.
    if let Some(resource_type_val) = value_map.get(&yaml_rust2::Yaml::String("type".to_string())) {
        if let yaml_rust2::Yaml::String(value_str) = resource_type_val {
            blueprint_resource.resource_type = validate_resource_type(value_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for resource type, found {:?}",
                resource_type_val
            )))?;
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "resource type must be defined for the \\\"{}\\\" resource definition",
            resource_name,
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
                            "expected a mapping for resource metadata, found {:?}",
                            value,
                        )))?
                    }
                }
                "linkSelector" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        blueprint_resource.link_selector = Some(validate_link_selector(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for resource link selector, found {:?}",
                            value
                        )))?
                    }
                }
                "description" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        blueprint_resource.description = Some(value_str.clone())
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for resource description, found {:?}",
                            value,
                        )))?
                    }
                }
                "spec" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        blueprint_resource.spec =
                            validate_resource_spec(&blueprint_resource.resource_type, value_map)?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for resource spec, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if let CelerityResourceSpec::NoSpec = blueprint_resource.spec {
        return Err(BlueprintParseError::YamlFormatError(format!(
            "resource spec must be defined for the \\\"{}\\\" resource definition",
            resource_name,
        )));
    }

    Ok(blueprint_resource)
}

fn validate_resource_type(
    resource_type: &String,
) -> Result<CelerityResourceType, BlueprintParseError> {
    match resource_type.as_str() {
        CELERITY_API_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityApi),
        CELERITY_CONSUMER_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityConsumer),
        CELERITY_SCHEDULE_RESOURCE_TYPE => Ok(CelerityResourceType::CeleritySchedule),
        CELERITY_HANDLER_RESOURCE_TYPE => Ok(CelerityResourceType::CelerityHandler),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported resource type, found {}",
            resource_type
        ))),
    }
}

fn validate_resource_spec(
    resource_type: &CelerityResourceType,
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityResourceSpec, BlueprintParseError> {
    match resource_type {
        CelerityResourceType::CelerityApi => Ok(CelerityResourceSpec::Api(
            validate_celerity_api_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityConsumer => Ok(CelerityResourceSpec::Consumer(
            validate_celerity_consumer_spec(spec_map)?,
        )),
        CelerityResourceType::CeleritySchedule => Ok(CelerityResourceSpec::Schedule(
            validate_celerity_schedule_spec(spec_map)?,
        )),
        CelerityResourceType::CelerityHandler => Ok(CelerityResourceSpec::Handler(
            validate_celerity_handler_spec(spec_map)?,
        )),
    }
}

fn validate_celerity_handler_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityHandlerSpec, BlueprintParseError> {
    let mut celerity_handler_spec = CelerityHandlerSpec::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "handlerName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.handler_name = Some(value_str.clone())
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for handlerName, found {:?}",
                            value,
                        )))?
                    }
                }
                "codeLocation" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.code_location = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for codeLocation, found {:?}",
                            value,
                        )))?
                    }
                }
                "handler" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.handler = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for handler, found {:?}",
                            value,
                        )))?
                    }
                }
                "runtime" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_handler_spec.runtime = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for runtime, found {:?}",
                            value,
                        )))?
                    }
                }
                "memory" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_handler_spec.memory = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for memory, found {:?}",
                            value,
                        )))?
                    }
                }
                "timeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_handler_spec.timeout = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for timeout, found {:?}",
                            value,
                        )))?
                    }
                }
                "tracingEnabled" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_handler_spec.tracing_enabled = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for tracingEnabled, found {:?}",
                            value,
                        )))?
                    }
                }
                "environmentVariables" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_handler_spec.environment_variables =
                            Some(validate_map_of_strings(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for environmentVariables, found {:?}",
                            value,
                        )))?
                    }
                }
                "events" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_handler_spec.events =
                            Some(validate_handler_events_config_map(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for events, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_handler_spec)
}

fn validate_handler_events_config_map(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, EventConfiguration>, BlueprintParseError> {
    let mut events = HashMap::new();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::Hash(value_map) = value {
                events.insert(key_str.clone(), validate_handler_event_config(value_map)?);
            }
        }
    }
    Ok(events)
}

fn validate_handler_event_config(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<EventConfiguration, BlueprintParseError> {
    let mut event_config = EventConfiguration::default();

    // Make sure the event source type is known before validating the
    // source configuration.
    if let Some(source_type_val) =
        value_map.get(&yaml_rust2::Yaml::String("sourceType".to_string()))
    {
        if let yaml_rust2::Yaml::String(value_str) = source_type_val {
            event_config.source_type = validate_event_source_type(value_str)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for event source type, found {:?}",
                source_type_val
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
                    event_config.source_configuration = EventSourceConfiguration::ObjectStorage(
                        validate_event_source_object_storage_config(source_config_map)?,
                    )
                }
                EventSourceType::DatabaseStream => {
                    event_config.source_configuration = EventSourceConfiguration::DatabaseStream(
                        validate_event_source_database_stream_config(source_config_map)?,
                    )
                }
                EventSourceType::DataStream => {
                    event_config.source_configuration = EventSourceConfiguration::DataStream(
                        validate_event_source_data_stream_config(source_config_map)?,
                    )
                }
            }
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for sourceConfiguration, found {:?}",
                source_config
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
) -> Result<ObjectStorageEventSourceConfiguration, BlueprintParseError> {
    let mut object_storage_config = ObjectStorageEventSourceConfiguration::default();
    if let Some(bucket_val) = source_config_map.get(&yaml_rust2::Yaml::String("bucket".to_string()))
    {
        if let yaml_rust2::Yaml::String(bucket_str) = bucket_val {
            object_storage_config.bucket = bucket_str.clone();
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for bucket, found {:?}",
                bucket_val
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
            object_storage_config.events = validate_object_storage_events(events_arr)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected an array for object source events, found {:?}",
                events_val
            )))?;
        }
    }
    Ok(object_storage_config)
}

fn validate_object_storage_events(
    events_arr: &yaml_rust2::yaml::Array,
) -> Result<Vec<ObjectStorageEventType>, BlueprintParseError> {
    let mut object_storage_events = Vec::new();
    for event_type in events_arr {
        if let yaml_rust2::Yaml::String(event_str) = event_type {
            match event_str.as_str() {
                "created" => object_storage_events.push(ObjectStorageEventType::ObjectCreated),
                "deleted" => object_storage_events.push(ObjectStorageEventType::ObjectDeleted),
                "metadataUpdated" => {
                    object_storage_events.push(ObjectStorageEventType::ObjectMetadataUpdated)
                }
                _ => Err(BlueprintParseError::YamlFormatError(format!(
                    "expected \\\"created\\\", \\\"deleted\\\" or \\\"metadataUpdated\\\" 
                        for object storage source event, found {:?}",
                    event_type
                )))?,
            }
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected \\\"created\\\", \\\"deleted\\\" or \\\"metadataUpdated\\\" for object storage source event, found {:?}",
                event_type
            )))?;
        }
    }
    Ok(object_storage_events)
}

fn validate_event_source_database_stream_config(
    source_config_map: &yaml_rust2::yaml::Hash,
) -> Result<DatabaseStreamSourceConfiguration, BlueprintParseError> {
    let mut database_stream_config = DatabaseStreamSourceConfiguration::default();

    if let Some(db_stream_id_val) =
        source_config_map.get(&yaml_rust2::Yaml::String("dbStreamId".to_string()))
    {
        if let yaml_rust2::Yaml::String(db_stream_id) = db_stream_id_val {
            database_stream_config.db_stream_id = db_stream_id.clone();
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for dbStreamId, found {:?}",
                db_stream_id_val
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
                        database_stream_config.batch_size = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for batchSize, found {:?}",
                            value,
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        database_stream_config.partial_failures = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for partialFailures, found {:?}",
                            value,
                        )))?
                    }
                }
                "startFromBeginning" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        database_stream_config.start_from_beginning = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for startFromBeginning, found {:?}",
                            value,
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
) -> Result<DataStreamSourceConfiguration, BlueprintParseError> {
    let mut data_stream_config = DataStreamSourceConfiguration::default();

    if let Some(data_stream_id_val) =
        source_config_map.get(&yaml_rust2::Yaml::String("dataStreamId".to_string()))
    {
        if let yaml_rust2::Yaml::String(data_stream_id) = data_stream_id_val {
            data_stream_config.data_stream_id = data_stream_id.clone();
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for dataStreamId, found {:?}",
                data_stream_id_val
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
                        data_stream_config.batch_size = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for batchSize, found {:?}",
                            value,
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        data_stream_config.partial_failures = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for partialFailures, found {:?}",
                            value,
                        )))?
                    }
                }
                "startFromBeginning" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        data_stream_config.start_from_beginning = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for startFromBeginning, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }
    Ok(data_stream_config)
}

fn validate_event_source_type(
    source_type: &String,
) -> Result<EventSourceType, BlueprintParseError> {
    match source_type.as_str() {
        "objectStorage" => Ok(EventSourceType::ObjectStorage),
        "databaseStream" => Ok(EventSourceType::DatabaseStream),
        "dataStream" => Ok(EventSourceType::DataStream),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported event source type, found {}",
            source_type
        ))),
    }
}

fn validate_map_of_strings(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, String>, BlueprintParseError> {
    let mut map = HashMap::new();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            if let yaml_rust2::Yaml::String(value_str) = value {
                map.insert(key_str.clone(), value_str.clone());
            } else {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a string for environment variable value, found {:?}",
                    value,
                )))?
            }
        }
    }
    Ok(map)
}

fn validate_celerity_schedule_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityScheduleSpec, BlueprintParseError> {
    let mut celerity_schedule_spec = CelerityScheduleSpec::default();
    if let Some(schedule_val) = spec_map.get(&yaml_rust2::Yaml::String("schedule".to_string())) {
        if let yaml_rust2::Yaml::String(schedule_str) = schedule_val {
            celerity_schedule_spec.schedule = schedule_str.clone();
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for schedule, found {:?}",
                schedule_val
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
) -> Result<CelerityConsumerSpec, BlueprintParseError> {
    let mut celerity_consumer_spec = CelerityConsumerSpec::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "sourceId" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_consumer_spec.source_id = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for sourceId, found {:?}",
                            value,
                        )))?
                    }
                }
                "batchSize" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.batch_size = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for batchSize, found {:?}",
                            value,
                        )))?
                    }
                }
                "visibilityTimeout" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.visibility_timeout = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for visibilityTimeout, found {:?}",
                            value,
                        )))?
                    }
                }
                "waitTimeSeconds" => {
                    if let yaml_rust2::Yaml::Integer(value_int) = value {
                        celerity_consumer_spec.wait_time_seconds = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for waitTimeSeconds, found {:?}",
                            value,
                        )))?
                    }
                }
                "partialFailures" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_consumer_spec.partial_failures = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for partialFailures, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }
    Ok(celerity_consumer_spec)
}

fn validate_celerity_api_spec(
    spec_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiSpec, BlueprintParseError> {
    let mut celerity_api_spec = CelerityApiSpec::default();
    for (key, value) in spec_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocols" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut protocols = Vec::new();
                        for item in value_arr {
                            if let yaml_rust2::Yaml::String(protocol_str) = item {
                                protocols.push(validate_api_protocol(protocol_str)?);
                            }
                        }
                        celerity_api_spec.protocols = protocols;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for api protocols, found {:?}",
                            value,
                        )))?
                    }
                }
                "cors" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        celerity_api_spec.cors = Some(CelerityApiCors::Str(value_str.clone()))
                    } else if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.cors = Some(CelerityApiCors::CorsConfiguration(
                            validate_celerity_api_cors_config(value_map)?,
                        ))
                    }
                }
                "domain" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.domain = Some(validate_celerity_api_domain(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for domain configuration, found {:?}",
                            value
                        )))?
                    }
                }
                "auth" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        celerity_api_spec.auth = Some(validate_celerity_api_auth(value_map)?);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a mapping for auth, found {:?}",
                            value,
                        )))?
                    }
                }
                "tracingEnabled" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        celerity_api_spec.tracing_enabled = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for tracingEnabled, found {:?}",
                            value,
                        )))?
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
) -> Result<CelerityApiCorsConfiguration, BlueprintParseError> {
    let mut cors_config = CelerityApiCorsConfiguration::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "allowCredentials" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        cors_config.allow_credentials = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for allow_credentials, found {:?}",
                            value,
                        )))?
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
                        cors_config.max_age = Some(*value_int)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an integer for maxAge, found {:?}",
                            value,
                        )))?
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
) -> Result<Option<Vec<String>>, BlueprintParseError> {
    let mut values = Vec::new();
    if let yaml_rust2::Yaml::Array(value_arr) = value {
        for item in value_arr {
            if let yaml_rust2::Yaml::String(value_str) = item {
                values.push(value_str.clone());
            } else {
                Err(BlueprintParseError::YamlFormatError(format!(
                    "expected a string for {}, found {:?}",
                    field, item,
                )))?
            }
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(format!(
            "expected an array for {}, found {:?}",
            field, value,
        )))?
    }
    Ok(Some(values))
}

fn validate_celerity_api_domain(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiDomain, BlueprintParseError> {
    let mut domain = CelerityApiDomain::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "domainName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.domain_name = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for domain name, found {:?}",
                            value,
                        )))?
                    }
                }
                "basePaths" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut base_paths = Vec::new();
                        for item in value_arr {
                            if let yaml_rust2::Yaml::String(value_str) = item {
                                base_paths.push(CelerityApiBasePath::Str(value_str.clone()));
                            } else if let yaml_rust2::Yaml::Hash(value_map) = item {
                                base_paths.push(CelerityApiBasePath::BasePathConfiguration(
                                    validate_celerity_api_base_path_config(value_map)?,
                                ));
                            } else {
                                Err(BlueprintParseError::YamlFormatError(format!(
                                    "expected a string or mapping for base path, found {:?}",
                                    item,
                                )))?
                            }
                        }
                        domain.base_paths = base_paths;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for base paths, found {:?}",
                            value,
                        )))?
                    }
                }
                "normalizeBasePath" => {
                    if let yaml_rust2::Yaml::Boolean(value_bool) = value {
                        domain.normalize_base_path = Some(*value_bool)
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a boolean for normalizeBasePath, found {:?}",
                            value,
                        )))?
                    }
                }
                "certificateId" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        domain.certificate_id = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for certificateId, found {:?}",
                            value,
                        )))?
                    }
                }
                "securityPolicy" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        match value_str.as_str() {
                            "TLS_1_0" => {
                                domain.security_policy =
                                    Some(CelerityApiDomainSecurityPolicy::Tls1_0)
                            }
                            "TLS_1_2" => {
                                domain.security_policy =
                                    Some(CelerityApiDomainSecurityPolicy::Tls1_2)
                            }
                            _ => (),
                        }
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for securityPolicy, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if domain.domain_name.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "domainName must be defined for the domain configuration".to_string(),
        ));
    }

    if domain.base_paths.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "at least one basePath must be defined for the domain configuration".to_string(),
        ));
    }

    if domain.certificate_id.is_empty() {
        return Err(BlueprintParseError::YamlFormatError(
            "certificateId must be defined for the domain configuration".to_string(),
        ));
    }

    Ok(domain)
}

fn validate_api_protocol(
    protocol_str: &String,
) -> Result<CelerityApiProtocol, BlueprintParseError> {
    match protocol_str.as_str() {
        "http" => Ok(CelerityApiProtocol::Http),
        "websocket" => Ok(CelerityApiProtocol::WebSocket),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported api protocol (\\\"http\\\" or \\\"websocket\\\"), found {}",
            protocol_str
        ))),
    }
}

fn validate_celerity_api_auth(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<CelerityApiAuth, BlueprintParseError> {
    let mut auth = CelerityApiAuth::default();

    if let Some(guards) = value_map.get(&yaml_rust2::Yaml::String("guards".to_string())) {
        if let yaml_rust2::Yaml::Hash(value_map) = guards {
            auth.guards = validate_celerity_api_auth_guards(value_map)?;
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a mapping for guards, found {:?}",
                guards,
            )))?
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a guards field for auth configuration".to_string(),
        ))?
    }

    if let Some(default_guard) =
        value_map.get(&yaml_rust2::Yaml::String("defaultGuard".to_string()))
    {
        if let yaml_rust2::Yaml::String(value_str) = default_guard {
            auth.default_guard = Some(value_str.clone())
        } else {
            Err(BlueprintParseError::YamlFormatError(format!(
                "expected a string for defaultGuard, found {:?}",
                default_guard,
            )))?
        }
    }

    Ok(auth)
}

fn validate_celerity_api_auth_guards(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<HashMap<String, CelerityApiAuthGuard>, BlueprintParseError> {
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
) -> Result<CelerityApiAuthGuard, BlueprintParseError> {
    let mut guard = CelerityApiAuthGuard::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "type" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.guard_type =
                            validate_celerity_api_auth_guard_type(value_str.clone())?;
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for type, found {:?}",
                            value,
                        )))?
                    }
                }
                "issuer" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.issuer = Some(value_str.clone())
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for issuer, found {:?}",
                            value,
                        )))?
                    }
                }
                "tokenSource" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        guard.token_source =
                            Some(CelerityApiAuthGuardValueSource::ValueSourceConfiguration(
                                validate_celerity_api_auth_value_source_config(value_map, "token")?,
                            ))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.token_source =
                            Some(CelerityApiAuthGuardValueSource::Str(value_str.clone()))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string or mapping for token source, found {:?}",
                            value,
                        )))?
                    }
                }
                "audience" => {
                    if let yaml_rust2::Yaml::Array(value_arr) = value {
                        let mut audiences = Vec::new();
                        for item in value_arr {
                            if let yaml_rust2::Yaml::String(value_str) = item {
                                audiences.push(value_str.clone());
                            } else {
                                Err(BlueprintParseError::YamlFormatError(format!(
                                    "expected a string for audience, found {:?}",
                                    item,
                                )))?
                            }
                        }
                        guard.audience = Some(audiences);
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected an array for audience, found {:?}",
                            value,
                        )))?
                    }
                }
                "apiKeySource" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        guard.api_key_source =
                            Some(CelerityApiAuthGuardValueSource::ValueSourceConfiguration(
                                validate_celerity_api_auth_value_source_config(
                                    value_map, "apiKey",
                                )?,
                            ))
                    } else if let yaml_rust2::Yaml::String(value_str) = value {
                        guard.api_key_source =
                            Some(CelerityApiAuthGuardValueSource::Str(value_str.clone()))
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string or mapping for apiKey source, found {:?}",
                            value,
                        )))?
                    }
                }
                _ => (),
            }
        }
    }

    if guard.guard_type == CelerityApiAuthGuardType::NoGuardType {
        return Err(BlueprintParseError::YamlFormatError(
            "type must be defined for an auth guard".to_string(),
        ));
    }

    Ok(guard)
}

fn validate_celerity_api_auth_value_source_config(
    value_map: &yaml_rust2::yaml::Hash,
    context: &str,
) -> Result<ValueSourceConfiguration, BlueprintParseError> {
    let mut value_source_config = ValueSourceConfiguration::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocol" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        match val_str.as_str() {
                            "http" => value_source_config.protocol = CelerityApiProtocol::Http,
                            "websocket" => {
                                value_source_config.protocol = CelerityApiProtocol::WebSocket
                            }
                            _ => Err(BlueprintParseError::YamlFormatError(format!(
                                "expected \\\"http\\\" or \\\"websocket\\\" for \\\"{}\\\" value source protocol, found {:?}",
                                context,
                                value,
                            )))?,
                        }
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected \\\"http\\\" or \\\"websocket\\\" for \\\"{}\\\" value source protocol, found {:?}",
                            context,
                            value,
                        )))?
                    }
                }
                "source" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        value_source_config.source = val_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for \\\"{}\\\" value source, found {:?}",
                            context, value,
                        )))?
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
) -> Result<CelerityApiBasePathConfiguration, BlueprintParseError> {
    let mut base_path_config = CelerityApiBasePathConfiguration::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "protocol" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        match val_str.as_str() {
                            "http" => base_path_config.protocol = CelerityApiProtocol::Http,
                            "websocket" => {
                                base_path_config.protocol = CelerityApiProtocol::WebSocket
                            }
                            _ => Err(BlueprintParseError::YamlFormatError(format!(
                                "expected \\\"http\\\" or \\\"websocket\\\" for base path protocol, found {:?}",
                                value,
                            )))?,
                        }
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected \\\"http\\\" or \\\"websocket\\\" for base path protocol, found {:?}",
                            value,
                        )))?
                    }
                }
                "basePath" => {
                    if let yaml_rust2::Yaml::String(val_str) = value {
                        base_path_config.base_path = val_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for base path, found {:?}",
                            value,
                        )))?
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
) -> Result<CelerityApiAuthGuardType, BlueprintParseError> {
    match guard_type.as_str() {
        "jwt" => Ok(CelerityApiAuthGuardType::Jwt),
        "apiKey" => Ok(CelerityApiAuthGuardType::ApiKey),
        "custom" => Ok(CelerityApiAuthGuardType::Custom),
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a supported guard type (\\\"jwt\\\", 
                \\\"apiKey\\\", \\\"custom\\\"), found {}",
            guard_type
        )))?,
    }
}

fn validate_resource_metadata(
    value_map: &yaml_rust2::yaml::Hash,
) -> Result<BlueprintResourceMetadata, BlueprintParseError> {
    let mut resource_metadata = BlueprintResourceMetadata::default();
    for (key, value) in value_map {
        if let yaml_rust2::Yaml::String(key_str) = key {
            match key_str.as_str() {
                "displayName" => {
                    if let yaml_rust2::Yaml::String(value_str) = value {
                        resource_metadata.display_name = value_str.clone()
                    } else {
                        Err(BlueprintParseError::YamlFormatError(format!(
                            "expected a string for resource display name, found {:?}",
                            value,
                        )))?
                    }
                }
                "annotations" => {
                    if let yaml_rust2::Yaml::Hash(value_map) = value {
                        let mut annotations = HashMap::new();
                        for (key, value) in value_map {
                            if let yaml_rust2::Yaml::String(key_str) = key {
                                let key_str = key_str.clone();
                                let scalar_value = extract_scalar_value(value, "annotations")?;
                                if let Some(unwrapped_scalar) = scalar_value {
                                    annotations.insert(key_str, unwrapped_scalar);
                                }
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
                                        "expected a string for label value, found {:?}",
                                        value,
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

    if resource_metadata.display_name.is_empty() {
        Err(BlueprintParseError::YamlFormatError(
            "expected a display name for resource metadata".to_string(),
        ))?
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
                "expected a mapping for byLabel link selector, found {:?}",
                by_label_value
            )))?
        }
    } else {
        Err(BlueprintParseError::YamlFormatError(
            "expected a byLabel field for link selector".to_string(),
        ))?
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

fn extract_scalar_value(
    value: &yaml_rust2::Yaml,
    field: &str,
) -> Result<Option<BlueprintScalarValue>, BlueprintParseError> {
    match value {
        yaml_rust2::Yaml::Integer(value_int) => Ok(Some(BlueprintScalarValue::Int(*value_int))),
        yaml_rust2::Yaml::Real(value_int) => {
            Ok(Some(BlueprintScalarValue::Float(value_int.parse()?)))
        }
        yaml_rust2::Yaml::Boolean(value_bool) => Ok(Some(BlueprintScalarValue::Bool(*value_bool))),
        yaml_rust2::Yaml::String(value_str) => {
            Ok(Some(BlueprintScalarValue::Str(value_str.clone())))
        }
        _ => Err(BlueprintParseError::YamlFormatError(format!(
            "expected a scalar value for {}, found {:?}",
            field, value
        ))),
    }
}
