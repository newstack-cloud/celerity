use std::{collections::HashMap, env::VarError, fmt, str::FromStr};

use celerity_helpers::env::EnvVars;

use crate::{
    blueprint::{
        BlueprintConfig, BlueprintMetadata, BlueprintResourceMetadata, BlueprintScalarValue,
        CelerityApiAuth, CelerityApiAuthGuard, CelerityApiAuthGuardType,
        CelerityApiAuthGuardValueSource, CelerityApiBasePath, CelerityApiBasePathConfiguration,
        CelerityApiCors, CelerityApiCorsConfiguration, CelerityApiDomain,
        CelerityApiDomainSecurityPolicy, CelerityApiProtocol, CelerityApiSpec, CelerityBucketSpec,
        CelerityConfigSpec, CelerityConsumerSpec, CelerityHandlerSpec, CelerityQueueSpec,
        CelerityResourceSpec, CelerityScheduleSpec, CelerityTopicSpec, CelerityWorkflowCatchConfig,
        CelerityWorkflowCondition, CelerityWorkflowDecisionRule, CelerityWorkflowFailureConfig,
        CelerityWorkflowParallelBranch, CelerityWorkflowRetryConfig, CelerityWorkflowSpec,
        CelerityWorkflowState, CelerityWorkflowStateType, CelerityWorkflowWaitConfig,
        DataStreamSourceConfiguration, DatabaseStreamSourceConfiguration, EventSourceConfiguration,
        ExternalEventConfiguration, ObjectStorageEventSourceConfiguration, ObjectStorageEventType,
        ResolvedMappingNode, RuntimeBlueprintResource, SharedHandlerConfig,
        ValueSourceConfiguration, WebSocketAuthStrategy, WebSocketConfiguration,
    },
    blueprint_with_subs::{
        BlueprintConfigWithSubs, BlueprintMetadataWithSubs, BlueprintResourceMetadataWithSubs,
        CelerityApiAuthGuardValueSourceWithSubs, CelerityApiAuthGuardWithSubs,
        CelerityApiAuthWithSubs, CelerityApiBasePathConfigurationWithSubs,
        CelerityApiBasePathWithSubs, CelerityApiCorsConfigurationWithSubs, CelerityApiCorsWithSubs,
        CelerityApiDomainWithSubs, CelerityApiSpecWithSubs, CelerityBucketSpecWithSubs,
        CelerityConfigSpecWithSubs, CelerityConsumerSpecWithSubs, CelerityHandlerSpecWithSubs,
        CelerityQueueSpecWithSubs, CelerityResourceSpecWithSubs, CelerityScheduleSpecWithSubs,
        CelerityTopicSpecWithSubs, CelerityWorkflowCatchConfigWithSubs,
        CelerityWorkflowConditionWithSubs, CelerityWorkflowDecisionRuleWithSubs,
        CelerityWorkflowFailureConfigWithSubs, CelerityWorkflowParallelBranchWithSubs,
        CelerityWorkflowRetryConfigWithSubs, CelerityWorkflowSpecWithSubs,
        CelerityWorkflowStateWithSubs, CelerityWorkflowWaitConfigWithSubs,
        DataStreamSourceConfigurationWithSubs, DatabaseStreamSourceConfigurationWithSubs,
        EventSourceConfigurationWithSubs, ExternalEventConfigurationWithSubs, MappingNode,
        ObjectStorageEventSourceConfigurationWithSubs, RuntimeBlueprintResourceWithSubs,
        SharedHandlerConfigWithSubs, StringOrSubstitution, StringOrSubstitutions, Substitution,
        ValueSourceConfigurationWithSubs,
    },
};

#[derive(Debug)]
pub enum ResolveError {
    MissingVariable(VarError, String),
    InvalidSubstitution(Substitution),
    ParseError(String, String),
    ValueMustBeScalar(String),
    ValueMustBeInt(String),
    ValueMustBeBool(String),
    ValueMustBeFloat(String),
    MaxResolveDepthExceeded(usize),
}

impl fmt::Display for ResolveError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            ResolveError::MissingVariable(variable_error, field) => write!(
                f,
                "blueprint substitution resolution failed: missing variable in field {}: {}",
                field, variable_error
            ),
            ResolveError::InvalidSubstitution(substitution) => write!(
                f,
                "blueprint substitution resolution failed: invalid substitution: {:?}",
                substitution
            ),
            ResolveError::ParseError(parse_error, field) => write!(
                f,
                "blueprint substitution resolution failed: parse error in field {}: {}",
                field, parse_error
            ),
            ResolveError::ValueMustBeScalar(field) => write!(
                f,
                "blueprint substitution resolution failed: value must be scalar in the {} field",
                field
            ),
            ResolveError::ValueMustBeInt(field) => write!(
                f,
                "blueprint substitution resolution failed: value must be an integer in the {} field",
                field
            ),
            ResolveError::ValueMustBeBool(field) => write!(
                f,
                "blueprint substitution resolution failed: value must be a boolean in the {} field",
                field
            ),
            ResolveError::ValueMustBeFloat(field) => write!(
                f,
                "blueprint substitution resolution failed: value must be a float in the {} field",
                field
            ),
            ResolveError::MaxResolveDepthExceeded(depth) => write!(
                f,
                "blueprint substitution resolution failed: maximum resolve depth of {} exceeded",
                depth
            ),
        }
    }
}

/// Resolve substitutions in a parsed blueprint configuration.
/// In the current implementation, only variable references are supported
/// in substitutions.
pub fn resolve_blueprint_config_substitutions(
    blueprint_with_subs: BlueprintConfigWithSubs,
    env: Box<dyn EnvVars>,
) -> Result<BlueprintConfig, ResolveError> {
    let mut resolved_config = BlueprintConfig::default();
    resolved_config.version = blueprint_with_subs.version;
    resolved_config.transform = blueprint_with_subs.transform;
    resolved_config.variables = blueprint_with_subs.variables;
    resolved_config.resources =
        resolve_blueprint_config_resources(blueprint_with_subs.resources, env.clone())?;
    resolved_config.metadata =
        resolve_blueprint_config_metadata(blueprint_with_subs.metadata, env.clone())?;
    Ok(resolved_config)
}

fn resolve_blueprint_config_metadata(
    metadata: Option<BlueprintMetadataWithSubs>,
    env: Box<dyn EnvVars>,
) -> Result<Option<BlueprintMetadata>, ResolveError> {
    match metadata {
        Some(metadata) => {
            let shared_handler_config = resolve_optional_shared_handler_config(
                metadata.shared_handler_config,
                env.clone(),
                &field_path(&["metadata", "sharedHandlerConfig"]),
            )?;
            Ok(Some(BlueprintMetadata {
                shared_handler_config,
            }))
        }
        None => Ok(None),
    }
}

fn resolve_optional_shared_handler_config(
    shared_handler_config_with_subs_opt: Option<SharedHandlerConfigWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<SharedHandlerConfig>, ResolveError> {
    match shared_handler_config_with_subs_opt {
        Some(shared_handler_config_with_subs) => Ok(Some(resolve_shared_handler_config(
            shared_handler_config_with_subs,
            env.clone(),
            field,
        )?)),
        None => Ok(None),
    }
}

fn resolve_shared_handler_config(
    shared_handler_config_with_subs: SharedHandlerConfigWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<SharedHandlerConfig, ResolveError> {
    let mut shared_handler_config = SharedHandlerConfig::default();
    shared_handler_config.code_location = resolve_optional_string_or_substitutions(
        shared_handler_config_with_subs.code_location,
        env.clone(),
        &field_path(&[field, "codeLocation"]),
    )?;
    shared_handler_config.runtime = resolve_optional_string_or_substitutions(
        shared_handler_config_with_subs.runtime,
        env.clone(),
        &field_path(&[field, "runtime"]),
    )?;
    shared_handler_config.memory = resolve_optional_mapping_node_to_int(
        shared_handler_config_with_subs.memory,
        env.clone(),
        &field_path(&[field, "memory"]),
    )?;
    shared_handler_config.timeout = resolve_optional_mapping_node_to_int(
        shared_handler_config_with_subs.timeout,
        env.clone(),
        &field_path(&[field, "timeout"]),
    )?;
    shared_handler_config.tracing_enabled = resolve_optional_mapping_node_to_bool(
        shared_handler_config_with_subs.tracing_enabled,
        env.clone(),
        &field_path(&[field, "tracingEnabled"]),
    )?;
    shared_handler_config.environment_variables = resolve_optional_string_or_subs_map(
        shared_handler_config_with_subs.environment_variables,
        env.clone(),
        &field_path(&[field, "environmentVariables"]),
    )?;
    Ok(shared_handler_config)
}

fn resolve_blueprint_config_resources(
    resources: HashMap<String, RuntimeBlueprintResourceWithSubs>,
    env: Box<dyn EnvVars>,
) -> Result<HashMap<String, RuntimeBlueprintResource>, ResolveError> {
    let mut resolved_resources = HashMap::new();
    for (resource_name, resource) in resources {
        let resolved_resource =
            resolve_blueprint_config_resource(resource, env.clone(), &resource_name)?;
        match resolved_resource.spec {
            CelerityResourceSpec::NoSpec => {
                // Do nothing for resources that don't match
                //one of the supported Celerity resource types.
            }
            _ => {
                resolved_resources.insert(resource_name.clone(), resolved_resource);
            }
        }
    }
    Ok(resolved_resources)
}

fn resolve_blueprint_config_resource(
    resource_with_subs: RuntimeBlueprintResourceWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<RuntimeBlueprintResource, ResolveError> {
    let mut resolved_resource = RuntimeBlueprintResource::default();
    resolved_resource.resource_type = resource_with_subs.resource_type;
    resolved_resource.description = resource_with_subs.description;
    resolved_resource.link_selector = resource_with_subs.link_selector;
    resolved_resource.metadata =
        resolve_resource_metadata(resource_with_subs.metadata, env.clone(), resource_name)?;
    resolved_resource.spec =
        resolve_resource_spec(resource_with_subs.spec, env.clone(), resource_name)?;
    Ok(resolved_resource)
}

fn resolve_resource_spec(
    spec: CelerityResourceSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityResourceSpec, ResolveError> {
    match spec {
        CelerityResourceSpecWithSubs::Handler(handler_spec) => Ok(CelerityResourceSpec::Handler(
            resolve_handler_spec(handler_spec, env.clone(), resource_name)?,
        )),
        CelerityResourceSpecWithSubs::Api(api_spec) => Ok(CelerityResourceSpec::Api(
            resolve_api_spec(api_spec, env.clone(), resource_name)?,
        )),
        CelerityResourceSpecWithSubs::Consumer(consumer_spec) => {
            Ok(CelerityResourceSpec::Consumer(resolve_consumer_spec(
                consumer_spec,
                env.clone(),
                resource_name,
            )?))
        }
        CelerityResourceSpecWithSubs::Schedule(schedule_spec) => {
            Ok(CelerityResourceSpec::Schedule(resolve_schedule_spec(
                schedule_spec,
                env.clone(),
                resource_name,
            )?))
        }
        CelerityResourceSpecWithSubs::HandlerConfig(handler_config_spec) => Ok(
            CelerityResourceSpec::HandlerConfig(resolve_shared_handler_config(
                handler_config_spec,
                env.clone(),
                &resource_spec_field_path(resource_name, &[]),
            )?),
        ),
        CelerityResourceSpecWithSubs::Workflow(workflow_spec) => {
            Ok(CelerityResourceSpec::Workflow(resolve_workflow_spec(
                workflow_spec,
                env.clone(),
                resource_name,
            )?))
        }
        CelerityResourceSpecWithSubs::Config(config_spec) => Ok(CelerityResourceSpec::Config(
            resolve_config_spec(config_spec, env.clone(), resource_name)?,
        )),
        CelerityResourceSpecWithSubs::Bucket(bucket_spec) => Ok(CelerityResourceSpec::Bucket(
            resolve_bucket_spec(bucket_spec, env.clone(), resource_name)?,
        )),
        CelerityResourceSpecWithSubs::Topic(topic_spec) => Ok(CelerityResourceSpec::Topic(
            resolve_topic_spec(topic_spec, env.clone(), resource_name)?,
        )),
        CelerityResourceSpecWithSubs::Queue(queue_spec) => Ok(CelerityResourceSpec::Queue(
            resolve_queue_spec(queue_spec, env.clone(), resource_name)?,
        )),
        _ => Ok(CelerityResourceSpec::NoSpec),
    }
}

fn resolve_handler_spec(
    spec: CelerityHandlerSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityHandlerSpec, ResolveError> {
    let mut resolved_spec = CelerityHandlerSpec::default();
    resolved_spec.handler_name = resolve_optional_string_or_substitutions(
        spec.handler_name,
        env.clone(),
        &resource_spec_field_path(resource_name, &["handlerName"]),
    )?;
    resolved_spec.code_location = resolve_optional_string_or_substitutions(
        spec.code_location,
        env.clone(),
        &resource_spec_field_path(resource_name, &["codeLocation"]),
    )?;
    resolved_spec.handler = resolve_string_or_substitutions_to_string(
        spec.handler,
        env.clone(),
        &resource_spec_field_path(resource_name, &["handler"]),
    )?;
    resolved_spec.runtime = resolve_optional_string_or_substitutions(
        spec.runtime,
        env.clone(),
        &resource_spec_field_path(resource_name, &["runtime"]),
    )?;
    resolved_spec.memory = resolve_optional_mapping_node_to_int(
        spec.memory,
        env.clone(),
        &resource_spec_field_path(resource_name, &["memory"]),
    )?;
    resolved_spec.timeout = resolve_optional_mapping_node_to_int(
        spec.timeout,
        env.clone(),
        &resource_spec_field_path(resource_name, &["timeout"]),
    )?;
    resolved_spec.tracing_enabled = resolve_optional_mapping_node_to_bool(
        spec.tracing_enabled,
        env.clone(),
        &resource_spec_field_path(resource_name, &["tracingEnabled"]),
    )?;
    resolved_spec.environment_variables = resolve_optional_string_or_subs_map(
        spec.environment_variables,
        env.clone(),
        &resource_spec_field_path(resource_name, &["environmentVariables"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_api_spec(
    spec_with_subs: CelerityApiSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityApiSpec, ResolveError> {
    let mut resolved_spec: CelerityApiSpec = CelerityApiSpec::default();
    resolved_spec.protocols = resolve_api_protocols(
        spec_with_subs.protocols,
        env.clone(),
        &resource_spec_field_path(resource_name, &["protocols"]),
    )?;
    resolved_spec.cors = resolve_api_cors(
        spec_with_subs.cors,
        env.clone(),
        &resource_spec_field_path(resource_name, &["cors"]),
    )?;
    resolved_spec.domain = resolve_api_domain_config(
        spec_with_subs.domain,
        env.clone(),
        &resource_spec_field_path(resource_name, &["domain"]),
    )?;
    resolved_spec.auth = resolve_api_auth(
        spec_with_subs.auth,
        env.clone(),
        &resource_spec_field_path(resource_name, &["auth"]),
    )?;
    resolved_spec.tracing_enabled = resolve_optional_mapping_node_to_bool(
        spec_with_subs.tracing_enabled,
        env.clone(),
        &resource_spec_field_path(resource_name, &["tracingEnabled"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_consumer_spec(
    spec_with_subs: CelerityConsumerSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityConsumerSpec, ResolveError> {
    let mut resolved_spec = CelerityConsumerSpec::default();
    resolved_spec.source_id = resolve_optional_string_or_substitutions(
        spec_with_subs.source_id,
        env.clone(),
        &resource_spec_field_path(resource_name, &["sourceId"]),
    )?;
    resolved_spec.batch_size = resolve_optional_mapping_node_to_int(
        spec_with_subs.batch_size,
        env.clone(),
        &resource_spec_field_path(resource_name, &["batchSize"]),
    )?;
    resolved_spec.visibility_timeout = resolve_optional_mapping_node_to_int(
        spec_with_subs.visibility_timeout,
        env.clone(),
        &resource_spec_field_path(resource_name, &["visibilityTimeout"]),
    )?;
    resolved_spec.wait_time_seconds = resolve_optional_mapping_node_to_int(
        spec_with_subs.wait_time_seconds,
        env.clone(),
        &resource_spec_field_path(resource_name, &["waitTimeSeconds"]),
    )?;
    resolved_spec.partial_failures = resolve_optional_mapping_node_to_bool(
        spec_with_subs.partial_failures,
        env.clone(),
        &resource_spec_field_path(resource_name, &["partialFailures"]),
    )?;
    resolved_spec.routing_key = resolve_optional_string_or_substitutions(
        spec_with_subs.routing_key,
        env.clone(),
        &resource_spec_field_path(resource_name, &["routingKey"]),
    )?;
    resolved_spec.external_events = resolve_optional_external_events(
        spec_with_subs.external_events,
        env.clone(),
        &resource_spec_field_path(resource_name, &["externalEvents"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_schedule_spec(
    spec_with_subs: CelerityScheduleSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityScheduleSpec, ResolveError> {
    let mut resolved_spec = CelerityScheduleSpec::default();
    resolved_spec.schedule = resolve_string_or_substitutions_to_string(
        spec_with_subs.schedule,
        env.clone(),
        &resource_spec_field_path(resource_name, &["scheduleName"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_config_spec(
    config_spec_with_subs: CelerityConfigSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityConfigSpec, ResolveError> {
    let mut resolved_spec = CelerityConfigSpec::default();
    resolved_spec.name = resolve_optional_string_or_substitutions(
        config_spec_with_subs.name,
        env.clone(),
        &resource_spec_field_path(resource_name, &["name"]),
    )?;
    resolved_spec.plaintext = resolve_optional_string_or_subs_list(
        config_spec_with_subs.plaintext,
        env.clone(),
        &resource_spec_field_path(resource_name, &["plaintext"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_bucket_spec(
    spec_with_subs: CelerityBucketSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityBucketSpec, ResolveError> {
    let mut resolved_spec = CelerityBucketSpec::default();
    resolved_spec.name = resolve_optional_string_or_substitutions(
        spec_with_subs.name,
        env.clone(),
        &resource_spec_field_path(resource_name, &["name"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_topic_spec(
    spec_with_subs: CelerityTopicSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityTopicSpec, ResolveError> {
    let mut resolved_spec = CelerityTopicSpec::default();
    resolved_spec.name = resolve_optional_string_or_substitutions(
        spec_with_subs.name,
        env.clone(),
        &resource_spec_field_path(resource_name, &["name"]),
    )?;
    resolved_spec.fifo = resolve_optional_mapping_node_to_bool(
        spec_with_subs.fifo,
        env.clone(),
        &resource_spec_field_path(resource_name, &["fifo"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_queue_spec(
    spec_with_subs: CelerityQueueSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityQueueSpec, ResolveError> {
    let mut resolved_spec = CelerityQueueSpec::default();
    resolved_spec.name = resolve_optional_string_or_substitutions(
        spec_with_subs.name,
        env.clone(),
        &resource_spec_field_path(resource_name, &["name"]),
    )?;
    resolved_spec.fifo = resolve_optional_mapping_node_to_bool(
        spec_with_subs.fifo,
        env.clone(),
        &resource_spec_field_path(resource_name, &["fifo"]),
    )?;
    resolved_spec.visibility_timeout = resolve_optional_mapping_node_to_int(
        spec_with_subs.visibility_timeout,
        env.clone(),
        &resource_spec_field_path(resource_name, &["visibilityTimeout"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_workflow_spec(
    spec_with_subs: CelerityWorkflowSpecWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<CelerityWorkflowSpec, ResolveError> {
    let mut resolved_spec = CelerityWorkflowSpec::default();
    resolved_spec.start_at = resolve_string_or_substitutions_to_string(
        spec_with_subs.start_at,
        env.clone(),
        &resource_spec_field_path(resource_name, &["startAt"]),
    )?;
    resolved_spec.states = resolve_workflow_states(
        spec_with_subs.states,
        env.clone(),
        &resource_spec_field_path(resource_name, &["states"]),
    )?;
    Ok(resolved_spec)
}

fn resolve_workflow_states(
    states_with_subs: HashMap<String, CelerityWorkflowStateWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<HashMap<String, CelerityWorkflowState>, ResolveError> {
    let mut resolved_states = HashMap::new();
    for (state_name, state_with_subs) in states_with_subs {
        let resolved_state = resolve_workflow_state(
            state_with_subs,
            env.clone(),
            &field_path(&[field, &state_name]),
        )?;
        resolved_states.insert(state_name, resolved_state);
    }
    Ok(resolved_states)
}

fn resolve_workflow_state(
    state_with_subs: CelerityWorkflowStateWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowState, ResolveError> {
    let mut state = CelerityWorkflowState::default();
    state.state_type = resolve_workflow_state_type(
        state_with_subs.state_type,
        env.clone(),
        &field_path(&[field, "type"]),
    )?;
    state.input_path = resolve_optional_string_or_substitutions(
        state_with_subs.input_path,
        env.clone(),
        &field_path(&[field, "inputPath"]),
    )?;
    state.result_path = resolve_optional_string_or_substitutions(
        state_with_subs.result_path,
        env.clone(),
        &field_path(&[field, "resultPath"]),
    )?;
    state.output_path = resolve_optional_string_or_substitutions(
        state_with_subs.output_path,
        env.clone(),
        &field_path(&[field, "outputPath"]),
    )?;
    let payload_template_mapping = resolve_optional_mapping_node(
        state_with_subs.payload_template.map(MappingNode::Mapping),
        env.clone(),
        &field_path(&[field, "payloadTemplate"]),
    )?;
    state.payload_template = match payload_template_mapping {
        Some(ResolvedMappingNode::Mapping(mapping)) => Some(mapping),
        _ => None,
    };
    state.next = resolve_optional_string_or_substitutions(
        state_with_subs.next,
        env.clone(),
        &field_path(&[field, "next"]),
    )?;
    state.end = resolve_optional_mapping_node_to_bool(
        state_with_subs.end,
        env.clone(),
        &field_path(&[field, "end"]),
    )?;
    state.decisions = resolve_optional_workflow_decisions(
        state_with_subs.decisions,
        env.clone(),
        &field_path(&[field, "decisions"]),
    )?;
    state.description = resolve_optional_string_or_substitutions(
        state_with_subs.description,
        env.clone(),
        &field_path(&[field, "description"]),
    )?;
    state.result = resolve_optional_mapping_node(
        state_with_subs.result,
        env.clone(),
        &field_path(&[field, "result"]),
    )?;
    state.timeout = resolve_optional_mapping_node_to_int(
        state_with_subs.timeout,
        env.clone(),
        &field_path(&[field, "timeout"]),
    )?;
    state.wait_config = resolve_optional_workflow_wait_config(
        state_with_subs.wait_config,
        env.clone(),
        &field_path(&[field, "waitConfig"]),
    )?;
    state.failure_config = resolve_optional_workflow_failure_config(
        state_with_subs.failure_config,
        env.clone(),
        &field_path(&[field, "failureConfig"]),
    )?;
    state.parallel_branches = resolve_optional_workflow_parallel_branches(
        state_with_subs.parallel_branches,
        env.clone(),
        &field_path(&[field, "parallelBranches"]),
    )?;
    state.retry = resolve_optional_workflow_retry_configs(
        state_with_subs.retry,
        env.clone(),
        &field_path(&[field, "retry"]),
    )?;
    state.catch = resolve_optional_workflow_catch_configs(
        state_with_subs.catch,
        env.clone(),
        &field_path(&[field, "catch"]),
    )?;
    Ok(state)
}

fn resolve_optional_workflow_wait_config(
    wait_config_with_subs: Option<CelerityWorkflowWaitConfigWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityWorkflowWaitConfig>, ResolveError> {
    match wait_config_with_subs {
        Some(wait_config_with_subs) => {
            let resolved_wait_config =
                resolve_workflow_wait_config(wait_config_with_subs, env.clone(), field)?;
            Ok(Some(resolved_wait_config))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_wait_config(
    wait_config_with_subs: CelerityWorkflowWaitConfigWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowWaitConfig, ResolveError> {
    let mut wait_config = CelerityWorkflowWaitConfig::default();
    wait_config.seconds = resolve_optional_string_or_substitutions(
        wait_config_with_subs.seconds,
        env.clone(),
        &field_path(&[field, "seconds"]),
    )?;
    wait_config.timestamp = resolve_optional_string_or_substitutions(
        wait_config_with_subs.timestamp,
        env.clone(),
        &field_path(&[field, "timestamp"]),
    )?;
    Ok(wait_config)
}

fn resolve_optional_workflow_failure_config(
    failure_config_with_subs: Option<CelerityWorkflowFailureConfigWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityWorkflowFailureConfig>, ResolveError> {
    match failure_config_with_subs {
        Some(failure_config_with_subs) => {
            let resolved_failure_config =
                resolve_workflow_failure_config(failure_config_with_subs, env.clone(), field)?;
            Ok(Some(resolved_failure_config))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_failure_config(
    failure_config_with_subs: CelerityWorkflowFailureConfigWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowFailureConfig, ResolveError> {
    let mut failure_config = CelerityWorkflowFailureConfig::default();
    failure_config.error = resolve_optional_string_or_substitutions(
        failure_config_with_subs.error,
        env.clone(),
        &field_path(&[field, "error"]),
    )?;
    failure_config.cause = resolve_optional_string_or_substitutions(
        failure_config_with_subs.cause,
        env.clone(),
        &field_path(&[field, "cause"]),
    )?;
    Ok(failure_config)
}

fn resolve_optional_workflow_parallel_branches(
    parallel_branches_with_subs: Option<Vec<CelerityWorkflowParallelBranchWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<Vec<CelerityWorkflowParallelBranch>>, ResolveError> {
    match parallel_branches_with_subs {
        Some(parallel_branches_with_subs) => {
            let mut resolved_parallel_branches = Vec::new();
            for parallel_branch_with_subs in parallel_branches_with_subs {
                let resolved_parallel_branch = resolve_workflow_parallel_branch(
                    parallel_branch_with_subs,
                    env.clone(),
                    field,
                )?;
                resolved_parallel_branches.push(resolved_parallel_branch);
            }
            Ok(Some(resolved_parallel_branches))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_parallel_branch(
    parallel_branch_with_subs: CelerityWorkflowParallelBranchWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowParallelBranch, ResolveError> {
    let mut parallel_branch = CelerityWorkflowParallelBranch::default();
    parallel_branch.start_at = resolve_string_or_substitutions_to_string(
        parallel_branch_with_subs.start_at,
        env.clone(),
        &field_path(&[field, "startAt"]),
    )?;
    parallel_branch.states = resolve_workflow_states(
        parallel_branch_with_subs.states,
        env.clone(),
        &field_path(&[field, "states"]),
    )?;
    Ok(parallel_branch)
}

fn resolve_optional_workflow_retry_configs(
    retry_configs_with_subs: Option<Vec<CelerityWorkflowRetryConfigWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<Vec<CelerityWorkflowRetryConfig>>, ResolveError> {
    match retry_configs_with_subs {
        Some(retry_configs_with_subs) => {
            let mut resolved_retry_configs = Vec::new();
            for retry_config_with_subs in retry_configs_with_subs {
                let resolved_retry_config =
                    resolve_workflow_retry_config(retry_config_with_subs, env.clone(), field)?;
                resolved_retry_configs.push(resolved_retry_config);
            }
            Ok(Some(resolved_retry_configs))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_retry_config(
    retry_config_with_subs: CelerityWorkflowRetryConfigWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowRetryConfig, ResolveError> {
    let mut retry_config = CelerityWorkflowRetryConfig::default();
    retry_config.match_errors = resolve_string_or_subs_list(
        retry_config_with_subs.match_errors,
        env.clone(),
        &field_path(&[field, "matchErrors"]),
    )?;
    retry_config.interval = resolve_optional_mapping_node_to_int(
        retry_config_with_subs.interval,
        env.clone(),
        &field_path(&[field, "interval"]),
    )?;
    retry_config.max_attempts = resolve_optional_mapping_node_to_int(
        retry_config_with_subs.max_attempts,
        env.clone(),
        &field_path(&[field, "maxAttempts"]),
    )?;
    retry_config.max_delay = resolve_optional_mapping_node_to_int(
        retry_config_with_subs.max_delay,
        env.clone(),
        &field_path(&[field, "maxDelay"]),
    )?;
    retry_config.jitter = resolve_optional_mapping_node_to_bool(
        retry_config_with_subs.jitter,
        env.clone(),
        &field_path(&[field, "jitter"]),
    )?;
    retry_config.backoff_rate = resolve_optional_mapping_node_to_float(
        retry_config_with_subs.backoff_rate,
        env.clone(),
        &field_path(&[field, "backoffRate"]),
    )?;
    Ok(retry_config)
}

fn resolve_optional_workflow_catch_configs(
    catch_configs_with_subs: Option<Vec<CelerityWorkflowCatchConfigWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<Vec<CelerityWorkflowCatchConfig>>, ResolveError> {
    match catch_configs_with_subs {
        Some(catch_configs_with_subs) => {
            let mut resolved_catch_configs = Vec::new();
            for catch_config_with_subs in catch_configs_with_subs {
                let resolved_catch_config =
                    resolve_workflow_catch_config(catch_config_with_subs, env.clone(), field)?;
                resolved_catch_configs.push(resolved_catch_config);
            }
            Ok(Some(resolved_catch_configs))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_catch_config(
    catch_config_with_subs: CelerityWorkflowCatchConfigWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowCatchConfig, ResolveError> {
    let mut catch_config = CelerityWorkflowCatchConfig::default();
    catch_config.match_errors = resolve_string_or_subs_list(
        catch_config_with_subs.match_errors,
        env.clone(),
        &field_path(&[field, "matchErrors"]),
    )?;
    catch_config.next = resolve_string_or_substitutions_to_string(
        catch_config_with_subs.next,
        env.clone(),
        &field_path(&[field, "next"]),
    )?;
    catch_config.result_path = resolve_optional_string_or_substitutions(
        catch_config_with_subs.result_path,
        env.clone(),
        &field_path(&[field, "resultPath"]),
    )?;
    Ok(catch_config)
}

fn resolve_workflow_state_type(
    state_type_with_subs: StringOrSubstitutions,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityWorkflowStateType, ResolveError> {
    let state_type_str =
        resolve_string_or_substitutions_to_string(state_type_with_subs, env, field)?;
    match state_type_str.as_str() {
        "executeStep" => Ok(CelerityWorkflowStateType::ExecuteStep),
        "pass" => Ok(CelerityWorkflowStateType::Pass),
        "parallel" => Ok(CelerityWorkflowStateType::Parallel),
        "wait" => Ok(CelerityWorkflowStateType::Wait),
        "decision" => Ok(CelerityWorkflowStateType::Decision),
        "failure" => Ok(CelerityWorkflowStateType::Failure),
        "success" => Ok(CelerityWorkflowStateType::Success),
        _ => Ok(CelerityWorkflowStateType::Unknown),
    }
}

fn resolve_optional_workflow_decisions(
    decisions_with_subs: Option<Vec<CelerityWorkflowDecisionRuleWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<Vec<CelerityWorkflowDecisionRule>>, ResolveError> {
    match decisions_with_subs {
        Some(decisions_with_subs) => {
            let mut resolved_decisions = Vec::new();
            for decision_with_subs in decisions_with_subs {
                let resolved_decision =
                    resolve_workflow_decision(decision_with_subs, env.clone(), field, 0)?;
                resolved_decisions.push(resolved_decision);
            }
            Ok(Some(resolved_decisions))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_decision(
    decision_with_subs: CelerityWorkflowDecisionRuleWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<CelerityWorkflowDecisionRule, ResolveError> {
    if depth > MAX_RESOLVE_DEPTH {
        return Err(ResolveError::MaxResolveDepthExceeded(depth));
    }

    let mut decision = CelerityWorkflowDecisionRule::default();
    decision.condition = resolve_optional_workflow_condition(
        decision_with_subs.condition,
        env.clone(),
        field,
        depth + 1,
    )?;
    decision.and = resolve_optional_workflow_conditions(
        decision_with_subs.and,
        env.clone(),
        &field_path(&[field, "and"]),
        depth + 1,
    )?;
    decision.or = resolve_optional_workflow_conditions(
        decision_with_subs.or,
        env.clone(),
        &field_path(&[field, "or"]),
        depth + 1,
    )?;
    decision.not = resolve_optional_workflow_condition(
        decision_with_subs.not,
        env.clone(),
        &field_path(&[field, "not"]),
        depth + 1,
    )?;
    decision.next = resolve_string_or_substitutions_to_string(
        decision_with_subs.next,
        env.clone(),
        &field_path(&[field, "next"]),
    )?;
    Ok(decision)
}

fn resolve_optional_workflow_conditions(
    conditions_with_subs: Option<Vec<CelerityWorkflowConditionWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<Option<Vec<CelerityWorkflowCondition>>, ResolveError> {
    match conditions_with_subs {
        Some(conditions_with_subs) => {
            let mut resolved_conditions = Vec::new();
            for condition_with_subs in conditions_with_subs {
                let resolved_condition =
                    resolve_workflow_condition(condition_with_subs, env.clone(), field, depth + 1)?;
                resolved_conditions.push(resolved_condition);
            }
            Ok(Some(resolved_conditions))
        }
        None => Ok(None),
    }
}

fn resolve_optional_workflow_condition(
    condition_with_subs: Option<CelerityWorkflowConditionWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<Option<CelerityWorkflowCondition>, ResolveError> {
    match condition_with_subs {
        Some(condition_with_subs) => {
            let resolved_condition =
                resolve_workflow_condition(condition_with_subs, env.clone(), field, depth + 1)?;
            Ok(Some(resolved_condition))
        }
        None => Ok(None),
    }
}

fn resolve_workflow_condition(
    condition_with_subs: CelerityWorkflowConditionWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<CelerityWorkflowCondition, ResolveError> {
    let mut condition = CelerityWorkflowCondition::default();
    condition.inputs = resolve_mapping_node_list(
        condition_with_subs.inputs,
        env.clone(),
        &field_path(&[field, "inputs"]),
        depth + 1,
    )?;
    condition.function = resolve_string_or_substitutions_to_string(
        condition_with_subs.function,
        env.clone(),
        &field_path(&[field, "function"]),
    )?;
    Ok(condition)
}

fn resolve_optional_external_events(
    external_events_with_subs_opt: Option<HashMap<String, ExternalEventConfigurationWithSubs>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<HashMap<String, ExternalEventConfiguration>>, ResolveError> {
    match external_events_with_subs_opt {
        Some(external_events_with_subs) => {
            let mut resolved_external_events = HashMap::new();
            for (event_name, event_with_subs) in external_events_with_subs {
                let resolved_event = resolve_external_event_config(
                    event_with_subs,
                    env.clone(),
                    &field_path(&[field, &event_name]),
                )?;
                resolved_external_events.insert(event_name, resolved_event);
            }
            Ok(Some(resolved_external_events))
        }
        None => Ok(None),
    }
}

fn resolve_external_event_config(
    event_with_subs: ExternalEventConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<ExternalEventConfiguration, ResolveError> {
    let mut event = ExternalEventConfiguration::default();
    event.source_type = event_with_subs.source_type;
    event.source_configuration = resolve_event_source_configuration(
        event_with_subs.source_configuration,
        env.clone(),
        &field_path(&[field, "sourceConfiguration"]),
    )?;
    Ok(event)
}

fn resolve_event_source_configuration(
    source_configuration_with_subs: EventSourceConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<EventSourceConfiguration, ResolveError> {
    match source_configuration_with_subs {
        EventSourceConfigurationWithSubs::ObjectStorage(object_storage_with_subs) => {
            resolve_object_storage_event_source_configuration(
                object_storage_with_subs,
                env.clone(),
                field,
            )
        }
        EventSourceConfigurationWithSubs::DatabaseStream(database_stream_with_subs) => {
            resolve_database_stream_event_source_configuration(
                database_stream_with_subs,
                env.clone(),
                field,
            )
        }
        EventSourceConfigurationWithSubs::DataStream(data_stream_with_subs) => {
            resolve_data_stream_event_source_configuration(
                data_stream_with_subs,
                env.clone(),
                field,
            )
        }
    }
}

fn resolve_object_storage_event_source_configuration(
    object_storage_with_subs: ObjectStorageEventSourceConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<EventSourceConfiguration, ResolveError> {
    let mut object_storage = ObjectStorageEventSourceConfiguration::default();
    object_storage.bucket = resolve_string_or_substitutions_to_string(
        object_storage_with_subs.bucket,
        env.clone(),
        &field_path(&[field, "bucket"]),
    )?;
    object_storage.events = resolve_object_storage_events(
        object_storage_with_subs.events,
        env.clone(),
        &field_path(&[field, "events"]),
    )?;
    Ok(EventSourceConfiguration::ObjectStorage(object_storage))
}

fn resolve_data_stream_event_source_configuration(
    data_stream_with_subs: DataStreamSourceConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<EventSourceConfiguration, ResolveError> {
    let mut data_stream = DataStreamSourceConfiguration::default();
    data_stream.batch_size = resolve_optional_mapping_node_to_int(
        data_stream_with_subs.batch_size,
        env.clone(),
        &field_path(&[field, "batchSize"]),
    )?;
    data_stream.data_stream_id = resolve_string_or_substitutions_to_string(
        data_stream_with_subs.data_stream_id,
        env.clone(),
        &field_path(&[field, "dataStreamId"]),
    )?;
    data_stream.partial_failures = resolve_optional_mapping_node_to_bool(
        data_stream_with_subs.partial_failures,
        env.clone(),
        &field_path(&[field, "partialFailures"]),
    )?;
    data_stream.start_from_beginning = resolve_optional_mapping_node_to_bool(
        data_stream_with_subs.start_from_beginning,
        env.clone(),
        &field_path(&[field, "startFromBeginning"]),
    )?;
    Ok(EventSourceConfiguration::DataStream(data_stream))
}

fn resolve_database_stream_event_source_configuration(
    database_stream_with_subs: DatabaseStreamSourceConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<EventSourceConfiguration, ResolveError> {
    let mut database_stream = DatabaseStreamSourceConfiguration::default();
    database_stream.batch_size = resolve_optional_mapping_node_to_int(
        database_stream_with_subs.batch_size,
        env.clone(),
        &field_path(&[field, "batchSize"]),
    )?;
    database_stream.db_stream_id = resolve_string_or_substitutions_to_string(
        database_stream_with_subs.db_stream_id,
        env.clone(),
        &field_path(&[field, "dbStreamId"]),
    )?;
    database_stream.partial_failures = resolve_optional_mapping_node_to_bool(
        database_stream_with_subs.partial_failures,
        env.clone(),
        &field_path(&[field, "partialFailures"]),
    )?;
    database_stream.start_from_beginning = resolve_optional_mapping_node_to_bool(
        database_stream_with_subs.start_from_beginning,
        env.clone(),
        &field_path(&[field, "startFromBeginning"]),
    )?;
    Ok(EventSourceConfiguration::DatabaseStream(database_stream))
}

fn resolve_object_storage_events(
    events_with_subs: Vec<StringOrSubstitutions>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Vec<ObjectStorageEventType>, ResolveError> {
    let mut resolved_events = Vec::new();
    for event_with_subs in events_with_subs {
        let resolved_event =
            resolve_object_storage_event_type(event_with_subs, env.clone(), field)?;
        resolved_events.push(resolved_event);
    }
    Ok(resolved_events)
}

fn resolve_object_storage_event_type(
    event_type_with_subs: StringOrSubstitutions,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<ObjectStorageEventType, ResolveError> {
    let event_type_str =
        resolve_string_or_substitutions_to_string(event_type_with_subs, env, field)?;
    match event_type_str.as_str() {
        "created" => Ok(ObjectStorageEventType::ObjectCreated),
        "deleted" => Ok(ObjectStorageEventType::ObjectDeleted),
        "metadataUpdated" => Ok(ObjectStorageEventType::ObjectMetadataUpdated),
        _ => Err(ResolveError::ParseError(
            format!("unsupported object storage event type: {}", event_type_str),
            field.to_string(),
        )),
    }
}

fn resolve_api_auth(
    auth_with_subs_opt: Option<CelerityApiAuthWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityApiAuth>, ResolveError> {
    match auth_with_subs_opt {
        Some(auth_with_subs) => {
            let mut auth = CelerityApiAuth::default();
            auth.default_guard = resolve_optional_string_or_substitutions(
                auth_with_subs.default_guard,
                env.clone(),
                &field_path(&[field, "defaultGuard"]),
            )?;
            auth.guards = resolve_api_auth_guards(
                auth_with_subs.guards,
                env.clone(),
                &field_path(&[field, "guards"]),
            )?;
            Ok(Some(auth))
        }
        None => Ok(None),
    }
}

fn resolve_api_auth_guards(
    guards_with_subs: HashMap<String, CelerityApiAuthGuardWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<HashMap<String, CelerityApiAuthGuard>, ResolveError> {
    let mut resolved_guards = HashMap::new();
    for (guard_name, guard_with_subs) in guards_with_subs {
        let resolved_guard = resolve_api_auth_guard(guard_with_subs, env.clone(), field)?;
        resolved_guards.insert(guard_name, resolved_guard);
    }
    Ok(resolved_guards)
}

fn resolve_api_auth_guard(
    guard_with_subs: CelerityApiAuthGuardWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityApiAuthGuard, ResolveError> {
    let mut guard = CelerityApiAuthGuard::default();
    guard.guard_type = resolve_api_auth_guard_type(guard_with_subs.guard_type, env.clone(), field)?;
    guard.issuer = resolve_optional_string_or_substitutions(
        guard_with_subs.issuer,
        env.clone(),
        &field_path(&[field, "issuer"]),
    )?;
    guard.token_source = resolve_optional_api_auth_guard_value_source(
        guard_with_subs.token_source,
        env.clone(),
        &field_path(&[field, "tokenSource"]),
    )?;
    guard.audience = resolve_optional_string_or_subs_list(
        guard_with_subs.audience,
        env.clone(),
        &field_path(&[field, "audience"]),
    )?;
    Ok(guard)
}

fn resolve_api_auth_guard_type(
    guard_type_with_subs: StringOrSubstitutions,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityApiAuthGuardType, ResolveError> {
    let guard_type_str =
        resolve_string_or_substitutions_to_string(guard_type_with_subs, env, field)?;
    match guard_type_str.as_str() {
        "jwt" => Ok(CelerityApiAuthGuardType::Jwt),
        "custom" => Ok(CelerityApiAuthGuardType::Custom),
        _ => Err(ResolveError::ParseError(
            format!("unsupported guard type: {}", guard_type_str),
            field.to_string(),
        )),
    }
}

fn resolve_optional_api_auth_guard_value_source(
    value_source_with_subs_opt: Option<CelerityApiAuthGuardValueSourceWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityApiAuthGuardValueSource>, ResolveError> {
    match value_source_with_subs_opt {
        Some(value_source_with_subs) => match value_source_with_subs {
            CelerityApiAuthGuardValueSourceWithSubs::Str(string_or_subs) => {
                let value_source =
                    resolve_string_or_substitutions_to_string(string_or_subs, env, field)?;
                Ok(Some(CelerityApiAuthGuardValueSource::Str(value_source)))
            }
            CelerityApiAuthGuardValueSourceWithSubs::ValueSourceConfiguration(
                value_source_config_with_subs,
            ) => {
                let value_source_config = resolve_api_auth_guard_value_source_config(
                    value_source_config_with_subs,
                    env,
                    field,
                )?;
                Ok(Some(
                    CelerityApiAuthGuardValueSource::ValueSourceConfiguration(value_source_config),
                ))
            }
        },
        None => Ok(None),
    }
}

fn resolve_api_auth_guard_value_source_config(
    value_source_config_with_subs: ValueSourceConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<ValueSourceConfiguration, ResolveError> {
    let mut value_source_config = ValueSourceConfiguration::default();
    value_source_config.protocol = resolve_api_protocol(
        value_source_config_with_subs.protocol,
        env.clone(),
        &field_path(&[field, "protocol"]),
    )?;
    value_source_config.source = resolve_string_or_substitutions_to_string(
        value_source_config_with_subs.source,
        env.clone(),
        &field_path(&[field, "source"]),
    )?;
    Ok(value_source_config)
}

fn resolve_api_domain_config(
    domain_config_with_subs_opt: Option<CelerityApiDomainWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityApiDomain>, ResolveError> {
    match domain_config_with_subs_opt {
        Some(domain_config_with_subs) => {
            let mut domain_config = CelerityApiDomain::default();
            domain_config.domain_name = resolve_string_or_substitutions_to_string(
                domain_config_with_subs.domain_name,
                env.clone(),
                &field_path(&[field, "domain_name"]),
            )?;
            domain_config.base_paths = resolve_celerity_api_base_paths(
                domain_config_with_subs.base_paths,
                env.clone(),
                &field_path(&[field, "basePaths"]),
            )?;
            domain_config.normalize_base_path = resolve_optional_mapping_node_to_bool(
                domain_config_with_subs.normalize_base_path,
                env.clone(),
                &field_path(&[field, "normalizeBasePath"]),
            )?;
            domain_config.certificate_id = resolve_string_or_substitutions_to_string(
                domain_config_with_subs.certificate_id,
                env.clone(),
                &field_path(&[field, "certificateId"]),
            )?;
            domain_config.security_policy = resolve_security_policy(
                domain_config_with_subs.security_policy,
                env.clone(),
                &field_path(&[field, "securityPolicy"]),
            )?;
            Ok(Some(domain_config))
        }
        None => Ok(None),
    }
}

fn resolve_security_policy(
    security_policy_string_with_subs: Option<StringOrSubstitutions>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityApiDomainSecurityPolicy>, ResolveError> {
    match security_policy_string_with_subs {
        Some(string_or_subs) => {
            let string_value =
                resolve_string_or_substitutions_to_string(string_or_subs, env.clone(), field)?;
            match string_value.as_str() {
                "TLS_1_0" => Ok(Some(CelerityApiDomainSecurityPolicy::Tls1_0)),
                "TLS_1_2" => Ok(Some(CelerityApiDomainSecurityPolicy::Tls1_2)),
                _ => Err(ResolveError::ParseError(
                    format!("unsupported security policy: {}", string_value),
                    field.to_string(),
                )),
            }
        }
        None => Ok(None),
    }
}

fn resolve_celerity_api_base_paths(
    base_paths_with_subs: Vec<CelerityApiBasePathWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Vec<CelerityApiBasePath>, ResolveError> {
    let mut resolved_base_paths = Vec::new();
    for base_path_with_subs in base_paths_with_subs {
        match base_path_with_subs {
            CelerityApiBasePathWithSubs::Str(string_or_subs) => {
                let base_path =
                    resolve_string_or_substitutions_to_string(string_or_subs, env.clone(), field)?;
                resolved_base_paths.push(CelerityApiBasePath::Str(base_path));
            }
            CelerityApiBasePathWithSubs::BasePathConfiguration(base_path_config_with_subs) => {
                let base_path_config =
                    resolve_api_base_path_config(base_path_config_with_subs, env.clone(), field)?;
                resolved_base_paths
                    .push(CelerityApiBasePath::BasePathConfiguration(base_path_config));
            }
        }
    }
    Ok(resolved_base_paths)
}

fn resolve_api_base_path_config(
    base_path_config_with_subs: CelerityApiBasePathConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityApiBasePathConfiguration, ResolveError> {
    let mut base_path_config = CelerityApiBasePathConfiguration::default();
    base_path_config.protocol = resolve_api_protocol(
        base_path_config_with_subs.protocol,
        env.clone(),
        &field_path(&[field, "protocol"]),
    )?;
    base_path_config.base_path = resolve_string_or_substitutions_to_string(
        base_path_config_with_subs.base_path,
        env.clone(),
        &field_path(&[field, "basePath"]),
    )?;
    Ok(base_path_config)
}

fn resolve_api_cors(
    cors_with_subs_opt: Option<CelerityApiCorsWithSubs>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<CelerityApiCors>, ResolveError> {
    match cors_with_subs_opt {
        Some(cors_with_subs) => match cors_with_subs {
            CelerityApiCorsWithSubs::CorsConfiguration(cors_config_with_subs) => {
                Ok(Some(CelerityApiCors::CorsConfiguration(
                    resolve_api_cors_config(cors_config_with_subs, env, field)?,
                )))
            }
            CelerityApiCorsWithSubs::Str(string_or_subs) => Ok(Some(CelerityApiCors::Str(
                resolve_string_or_substitutions_to_string(string_or_subs, env, field)?,
            ))),
        },
        None => Ok(None),
    }
}

fn resolve_api_cors_config(
    cors_config_with_subs: CelerityApiCorsConfigurationWithSubs,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityApiCorsConfiguration, ResolveError> {
    let mut cors_config = CelerityApiCorsConfiguration::default();
    cors_config.allow_credentials = resolve_optional_mapping_node_to_bool(
        cors_config_with_subs.allow_credentials,
        env.clone(),
        &field_path(&[field, "allowCredentials"]),
    )?;
    cors_config.allow_origins = resolve_optional_string_or_subs_list(
        cors_config_with_subs.allow_origins,
        env.clone(),
        &field_path(&[field, "allowOrigins"]),
    )?;
    cors_config.allow_methods = resolve_optional_string_or_subs_list(
        cors_config_with_subs.allow_methods,
        env.clone(),
        &field_path(&[field, "allowMethods"]),
    )?;
    cors_config.allow_headers = resolve_optional_string_or_subs_list(
        cors_config_with_subs.allow_headers,
        env.clone(),
        &field_path(&[field, "allowHeaders"]),
    )?;
    cors_config.expose_headers = resolve_optional_string_or_subs_list(
        cors_config_with_subs.expose_headers,
        env.clone(),
        &field_path(&[field, "exposeHeaders"]),
    )?;
    cors_config.max_age = resolve_optional_mapping_node_to_int(
        cors_config_with_subs.max_age,
        env.clone(),
        &field_path(&[field, "maxAge"]),
    )?;
    Ok(cors_config)
}

fn resolve_api_protocols(
    protocols_with_subs: Vec<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Vec<CelerityApiProtocol>, ResolveError> {
    let mut resolved_protocols = Vec::new();
    for protocol in protocols_with_subs {
        resolved_protocols.push(resolve_api_protocol(protocol, env.clone(), field)?);
    }
    Ok(resolved_protocols)
}

fn resolve_api_protocol(
    protocol: MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<CelerityApiProtocol, ResolveError> {
    match protocol {
        MappingNode::Scalar(BlueprintScalarValue::Str(protocol_str)) => {
            match protocol_str.as_str() {
                "http" => Ok(CelerityApiProtocol::Http),
                "websocket" => Ok(CelerityApiProtocol::WebSocket),
                _ => Err(ResolveError::ParseError(
                    format!("unsupported protocol: {}", protocol_str),
                    field.to_string(),
                )),
            }
        }
        MappingNode::SubstitutionStr(string_or_substitutions) => {
            let protocol_str =
                resolve_string_or_substitutions_to_string(string_or_substitutions, env, field)?;
            match protocol_str.as_str() {
                "http" => Ok(CelerityApiProtocol::Http),
                "websocket" => Ok(CelerityApiProtocol::WebSocket),
                _ => Err(ResolveError::ParseError(
                    format!("unsupported protocol: {}", protocol_str),
                    field.to_string(),
                )),
            }
        }
        MappingNode::Mapping(config_map) => match config_map.get("websocketConfig") {
            Some(websocket_config_node) => Ok(CelerityApiProtocol::WebSocketConfig(
                resolve_websocket_config(
                    websocket_config_node,
                    env.clone(),
                    &field_path(&[field, "websocketConfig"]),
                )?,
            )),
            None => {
                return Err(ResolveError::ParseError(
                    "missing websocketConfig field".to_string(),
                    field.to_string(),
                ));
            }
        },
        _ => Err(ResolveError::ParseError(
            "protocol must be a string or WebSocket configuration object".to_string(),
            field.to_string(),
        )),
    }
}

fn resolve_websocket_config(
    websocket_config_node: &MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<WebSocketConfiguration, ResolveError> {
    let mut websocket_config = WebSocketConfiguration::default();
    match websocket_config_node {
        MappingNode::Mapping(ws_config_map) => {
            if let Some(route_key_node) = ws_config_map.get("routeKey") {
                websocket_config.route_key = Some(resolve_mapping_node_to_string(
                    route_key_node.clone(),
                    env.clone(),
                    &field_path(&[field, "routeKey"]),
                )?)
            }
            if let Some(auth_strategy_node) = ws_config_map.get("authStrategy") {
                websocket_config.auth_strategy = Some(resolve_websocket_auth_strategy(
                    auth_strategy_node,
                    env.clone(),
                    &field_path(&[field, "authStrategy"]),
                )?)
            }
        }
        _ => {
            return Err(ResolveError::ParseError(
                "websocketConfig must be a mapping".to_string(),
                field.to_string(),
            ));
        }
    }

    Ok(websocket_config)
}

fn resolve_websocket_auth_strategy(
    auth_strategy_node: &MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<WebSocketAuthStrategy, ResolveError> {
    match auth_strategy_node {
        MappingNode::Scalar(BlueprintScalarValue::Str(auth_strategy_str)) => {
            match auth_strategy_str.as_str() {
                "authMessage" => Ok(WebSocketAuthStrategy::AuthMessage),
                "connect" => Ok(WebSocketAuthStrategy::Connect),
                _ => Err(ResolveError::ParseError(
                    format!("unsupported auth strategy: {}", auth_strategy_str),
                    field.to_string(),
                )),
            }
        }
        MappingNode::SubstitutionStr(string_or_substitutions) => {
            let auth_strategy_str = resolve_string_or_substitutions_to_string(
                string_or_substitutions.clone(),
                env,
                field,
            )?;
            match auth_strategy_str.as_str() {
                "authMessage" => Ok(WebSocketAuthStrategy::AuthMessage),
                "connect" => Ok(WebSocketAuthStrategy::Connect),
                _ => Err(ResolveError::ParseError(
                    format!("unsupported auth strategy: {}", auth_strategy_str),
                    field.to_string(),
                )),
            }
        }
        _ => Err(ResolveError::ParseError(
            "authStrategy must be a string".to_string(),
            field.to_string(),
        )),
    }
}

fn resolve_resource_metadata(
    metadata: BlueprintResourceMetadataWithSubs,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<BlueprintResourceMetadata, ResolveError> {
    let mut resolved_metadata = BlueprintResourceMetadata::default();
    resolved_metadata.display_name = resolve_string_or_substitutions_to_string(
        metadata.display_name,
        env.clone(),
        &resource_metadata_field_path(resource_name, &["displayName"]),
    )?;
    resolved_metadata.annotations =
        resolve_annotations(metadata.annotations, env.clone(), resource_name)?;
    resolved_metadata.labels = metadata.labels;
    Ok(resolved_metadata)
}

fn resolve_annotations(
    annotations: Option<HashMap<String, MappingNode>>,
    env: Box<dyn EnvVars>,
    resource_name: &str,
) -> Result<Option<HashMap<String, BlueprintScalarValue>>, ResolveError> {
    match annotations {
        Some(unwrapped_annotations) => {
            let mut resolved_annotations = HashMap::new();
            for (key, value) in unwrapped_annotations {
                resolved_annotations.insert(
                    key.clone(),
                    resolve_mapping_node_to_scalar_value(
                        value,
                        env.clone(),
                        &resource_metadata_field_path(resource_name, &["annotations", &key]),
                    )?,
                );
            }
            Ok(Some(resolved_annotations))
        }
        None => Ok(None),
    }
}

fn resolve_optional_string_or_subs_list(
    string_or_subs_list: Option<Vec<StringOrSubstitutions>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<Vec<String>>, ResolveError> {
    match string_or_subs_list {
        Some(unwrapped_list) => resolve_string_or_subs_list(unwrapped_list, env, field).map(Some),
        None => Ok(None),
    }
}

fn resolve_string_or_subs_list(
    string_or_subs_list: Vec<StringOrSubstitutions>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Vec<String>, ResolveError> {
    let mut resolved_list = Vec::new();
    for string_or_subs in string_or_subs_list {
        resolved_list.push(resolve_string_or_substitutions_to_string(
            string_or_subs,
            env.clone(),
            field,
        )?);
    }
    Ok(resolved_list)
}

fn resolve_optional_mapping_node_to_int(
    mapping_node: Option<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<i64>, ResolveError> {
    match mapping_node {
        Some(mapping_node) => {
            let scalar_value = resolve_mapping_node_to_scalar_value(mapping_node, env, field)?;
            match scalar_value {
                BlueprintScalarValue::Int(int) => Ok(Some(int)),
                _ => Err(ResolveError::ValueMustBeInt(field.to_string())),
            }
        }
        None => Ok(None),
    }
}

fn resolve_optional_mapping_node_to_float(
    mapping_node: Option<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<f64>, ResolveError> {
    match mapping_node {
        Some(mapping_node) => {
            let scalar_value = resolve_mapping_node_to_scalar_value(mapping_node, env, field)?;
            match scalar_value {
                BlueprintScalarValue::Float(float) => Ok(Some(float)),
                _ => Err(ResolveError::ValueMustBeFloat(field.to_string())),
            }
        }
        None => Ok(None),
    }
}

fn resolve_optional_mapping_node_to_bool(
    mapping_node: Option<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<bool>, ResolveError> {
    match mapping_node {
        Some(mapping_node) => {
            let scalar_value = resolve_mapping_node_to_scalar_value(mapping_node, env, field)?;
            match scalar_value {
                BlueprintScalarValue::Bool(bool) => Ok(Some(bool)),
                _ => Err(ResolveError::ValueMustBeBool(field.to_string())),
            }
        }
        None => Ok(None),
    }
}

fn resolve_mapping_node_to_scalar_value(
    mapping_node: MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<BlueprintScalarValue, ResolveError> {
    match mapping_node {
        MappingNode::Scalar(BlueprintScalarValue::Str(str)) => Ok(BlueprintScalarValue::Str(str)),
        MappingNode::Scalar(BlueprintScalarValue::Int(int)) => Ok(BlueprintScalarValue::Int(int)),
        MappingNode::Scalar(BlueprintScalarValue::Float(float)) => {
            Ok(BlueprintScalarValue::Float(float))
        }
        MappingNode::Scalar(BlueprintScalarValue::Bool(bool)) => {
            Ok(BlueprintScalarValue::Bool(bool))
        }
        MappingNode::SubstitutionStr(string_or_substitutions) => {
            resolve_scalar_value_from_string_or_substitutions(string_or_substitutions, env, field)
        }
        _ => Err(ResolveError::ValueMustBeScalar(field.to_string())),
    }
}

fn resolve_optional_mapping_node(
    mapping_node: Option<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<ResolvedMappingNode>, ResolveError> {
    match mapping_node {
        Some(mapping_node) => resolve_mapping_node(mapping_node, env, field, 0).map(Some),
        None => Ok(None),
    }
}

fn resolve_mapping_node_list(
    mapping_node_list: Vec<MappingNode>,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<Vec<ResolvedMappingNode>, ResolveError> {
    let mut resolved_list = Vec::new();
    for mapping_node in mapping_node_list {
        resolved_list.push(resolve_mapping_node(
            mapping_node,
            env.clone(),
            field,
            depth + 1,
        )?);
    }
    Ok(resolved_list)
}

// The maximum depth of nested mapping nodes to resolve
// as a safety measure to prevent infinite recursion or performance-degrading
// recursive resolution of mapping nodes.
const MAX_RESOLVE_DEPTH: usize = 20;

fn resolve_mapping_node(
    mapping_node: MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
    depth: usize,
) -> Result<ResolvedMappingNode, ResolveError> {
    if depth > MAX_RESOLVE_DEPTH {
        return Err(ResolveError::MaxResolveDepthExceeded(depth));
    }

    match mapping_node {
        MappingNode::Scalar(scalar) => Ok(ResolvedMappingNode::Scalar(scalar)),
        MappingNode::Mapping(mapping) => {
            let mut resolved_map = HashMap::new();
            for (key, value) in mapping {
                resolved_map.insert(
                    key,
                    resolve_mapping_node(value, env.clone(), field, depth + 1)?,
                );
            }
            Ok(ResolvedMappingNode::Mapping(resolved_map))
        }
        MappingNode::Sequence(sequence) => {
            let mut resolved_sequence = Vec::new();
            for value in sequence {
                resolved_sequence.push(resolve_mapping_node(value, env.clone(), field, depth + 1)?);
            }
            Ok(ResolvedMappingNode::Sequence(resolved_sequence))
        }
        MappingNode::SubstitutionStr(string_or_substitutions) => {
            resolve_scalar_value_from_string_or_substitutions(string_or_substitutions, env, field)
                .map(ResolvedMappingNode::Scalar)
        }
        MappingNode::Null => Ok(ResolvedMappingNode::Null),
    }
}

fn resolve_mapping_node_to_string(
    mapping_node: MappingNode,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<String, ResolveError> {
    let scalar_value = resolve_mapping_node_to_scalar_value(mapping_node, env, field)?;
    match scalar_value {
        BlueprintScalarValue::Str(str) => Ok(str),
        _ => Err(ResolveError::ValueMustBeScalar(field.to_string())),
    }
}

fn resolve_scalar_value_from_string_or_substitutions(
    string_or_substitutions: StringOrSubstitutions,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<BlueprintScalarValue, ResolveError> {
    if string_or_substitutions.values.len() > 1 {
        Ok(BlueprintScalarValue::Str(
            resolve_string_or_substitutions_to_string(string_or_substitutions, env.clone(), field)?,
        ))
    } else {
        let value = string_or_substitutions.values[0].clone();
        match value {
            StringOrSubstitution::StringValue(str) => Ok(BlueprintScalarValue::Str(str)),
            StringOrSubstitution::SubstitutionValue(substitution) => {
                if let Ok(int_value) =
                    resolve_substitution::<i64>(&substitution, env.clone(), field)
                {
                    Ok(BlueprintScalarValue::Int(int_value))
                } else if let Ok(float_value) =
                    resolve_substitution::<f64>(&substitution, env.clone(), field)
                {
                    Ok(BlueprintScalarValue::Float(float_value))
                } else if let Ok(bool_value) =
                    resolve_substitution::<bool>(&substitution, env.clone(), field)
                {
                    Ok(BlueprintScalarValue::Bool(bool_value))
                } else {
                    Ok(BlueprintScalarValue::Str(resolve_substitution::<String>(
                        &substitution,
                        env.clone(),
                        field,
                    )?))
                }
            }
        }
    }
}

fn resolve_optional_string_or_subs_map(
    string_or_subs_map: Option<HashMap<String, StringOrSubstitutions>>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<HashMap<String, String>>, ResolveError> {
    match string_or_subs_map {
        Some(unwrapped_map) => {
            let mut resolved_map = HashMap::new();
            for (key, value) in unwrapped_map {
                resolved_map.insert(
                    key.clone(),
                    resolve_string_or_substitutions_to_string(
                        value,
                        env.clone(),
                        &field_path(&[field, &key]),
                    )?,
                );
            }
            Ok(Some(resolved_map))
        }
        None => Ok(None),
    }
}

fn resolve_optional_string_or_substitutions(
    string_or_substitutions: Option<StringOrSubstitutions>,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Option<String>, ResolveError> {
    match string_or_substitutions {
        Some(string_or_substitutions) => Ok(Some(resolve_string_or_substitutions_to_string(
            string_or_substitutions,
            env.clone(),
            field,
        )?)),
        None => Ok(None),
    }
}

fn resolve_string_or_substitutions_to_string(
    string_or_substitutions: StringOrSubstitutions,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<String, ResolveError> {
    let mut resolved_string = String::new();
    for string_or_sub in string_or_substitutions.values.iter() {
        resolved_string.push_str(&resolve_string_or_substitution_to_string(
            string_or_sub.clone(),
            env.clone(),
            field,
        )?);
    }
    Ok(resolved_string)
}

fn resolve_string_or_substitution_to_string(
    string_or_substitution: StringOrSubstitution,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<String, ResolveError> {
    match string_or_substitution {
        StringOrSubstitution::StringValue(str) => Ok(str),
        StringOrSubstitution::SubstitutionValue(substitution) => {
            Ok(resolve_substitution(&substitution, env.clone(), field)?)
        }
    }
}

fn resolve_substitution<Target>(
    substitution: &Substitution,
    env: Box<dyn EnvVars>,
    field: &str,
) -> Result<Target, ResolveError>
where
    Target: FromStr,
    <Target as FromStr>::Err: fmt::Display,
{
    match substitution {
        Substitution::VariableReference(variable_reference) => {
            let env_var_name = format!("CELERITY_VARIABLE_{}", variable_reference.variable_name);
            let env_var_value = env
                .var(&env_var_name)
                .map_err(|e| ResolveError::MissingVariable(e, field.to_string()))?;
            Ok(env_var_value
                .to_string()
                .parse::<Target>()
                .map_err(|e| ResolveError::ParseError(e.to_string(), field.to_string()))?)
        }
    }
}

fn resource_spec_field_path(resource_name: &str, keys: &[&str]) -> String {
    if keys.is_empty() {
        format!("resources.{}.spec", resource_name)
    } else {
        format!("resources.{}.spec.{}", resource_name, keys.join("."))
    }
}

fn resource_metadata_field_path(resource_name: &str, keys: &[&str]) -> String {
    format!("resources.{}.metadata.{}", resource_name, keys.join("."))
}

fn field_path(keys: &[&str]) -> String {
    format!("{}", keys.join("."))
}
