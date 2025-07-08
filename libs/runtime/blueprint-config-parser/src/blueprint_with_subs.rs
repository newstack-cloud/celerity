use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use crate::blueprint::{
    BlueprintLinkSelector, BlueprintScalarValue, BlueprintVariable, CelerityResourceType,
    EventSourceType,
};

/// This is a struct that holds an intermediary representation of
/// the blueprint configuration that contains ${..} substitutions.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct BlueprintConfigWithSubs {
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_version")]
    pub version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_optional_string_or_vec")]
    pub transform: Option<Vec<String>>,
    // Variable definitions can not have substitutions, so we can use the
    // final config type here.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub variables: Option<HashMap<String, BlueprintVariable>>,
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_resource_map")]
    pub resources: HashMap<String, RuntimeBlueprintResourceWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<BlueprintMetadataWithSubs>,
}

#[derive(Serialize, Debug, PartialEq)]
pub struct RuntimeBlueprintResourceWithSubs {
    #[serde(rename = "type")]
    pub resource_type: CelerityResourceType,
    pub metadata: BlueprintResourceMetadataWithSubs,
    pub link_selector: Option<BlueprintLinkSelector>,
    pub description: Option<String>,
    pub spec: CelerityResourceSpecWithSubs,
}

impl Default for RuntimeBlueprintResourceWithSubs {
    fn default() -> Self {
        Self {
            resource_type: CelerityResourceType::CelerityHandler,
            metadata: BlueprintResourceMetadataWithSubs::default(),
            link_selector: None,
            description: None,
            spec: CelerityResourceSpecWithSubs::NoSpec,
        }
    }
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct BlueprintResourceMetadataWithSubs {
    #[serde(rename = "displayName")]
    pub display_name: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations: Option<HashMap<String, MappingNode>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct BlueprintMetadataWithSubs {
    #[serde(rename = "sharedHandlerConfig")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub shared_handler_config: Option<SharedHandlerConfigWithSubs>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct SharedHandlerConfigWithSubs {
    #[serde(rename = "codeLocation")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code_location: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tracingEnabled")]
    pub tracing_enabled: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "environmentVariables")]
    pub environment_variables: Option<HashMap<String, StringOrSubstitutions>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityResourceSpecWithSubs {
    Handler(CelerityHandlerSpecWithSubs),
    Api(CelerityApiSpecWithSubs),
    Consumer(CelerityConsumerSpecWithSubs),
    Schedule(CelerityScheduleSpecWithSubs),
    HandlerConfig(SharedHandlerConfigWithSubs),
    Workflow(CelerityWorkflowSpecWithSubs),
    Config(CelerityConfigSpecWithSubs),
    Bucket(CelerityBucketSpecWithSubs),
    Topic(CelerityTopicSpecWithSubs),
    Queue(CelerityQueueSpecWithSubs),
    NoSpec,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityHandlerSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "handlerName")]
    pub handler_name: Option<StringOrSubstitutions>,
    #[serde(rename = "codeLocation")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code_location: Option<StringOrSubstitutions>,
    pub handler: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tracingEnabled")]
    pub tracing_enabled: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "environmentVariables")]
    pub environment_variables: Option<HashMap<String, StringOrSubstitutions>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiSpecWithSubs {
    pub protocols: Vec<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cors: Option<CelerityApiCorsWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub domain: Option<CelerityApiDomainWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub auth: Option<CelerityApiAuthWithSubs>,
    #[serde(rename = "tracingEnabled")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tracing_enabled: Option<MappingNode>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiCorsWithSubs {
    Str(StringOrSubstitutions),
    CorsConfiguration(CelerityApiCorsConfigurationWithSubs),
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiCorsConfigurationWithSubs {
    #[serde(rename = "allowCredentials")]
    pub allow_credentials: Option<MappingNode>,
    #[serde(rename = "allowOrigins")]
    pub allow_origins: Option<Vec<StringOrSubstitutions>>,
    #[serde(rename = "allowMethods")]
    pub allow_methods: Option<Vec<StringOrSubstitutions>>,
    #[serde(rename = "allowHeaders")]
    pub allow_headers: Option<Vec<StringOrSubstitutions>>,
    #[serde(rename = "exposeHeaders")]
    pub expose_headers: Option<Vec<StringOrSubstitutions>>,
    #[serde(rename = "maxAge")]
    pub max_age: Option<MappingNode>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiDomainWithSubs {
    #[serde(rename = "domainName")]
    pub domain_name: StringOrSubstitutions,
    #[serde(rename = "basePaths")]
    pub base_paths: Vec<CelerityApiBasePathWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "normalizeBasePath")]
    pub normalize_base_path: Option<MappingNode>,
    #[serde(rename = "certificateId")]
    pub certificate_id: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "securityPolicy")]
    pub security_policy: Option<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiBasePathWithSubs {
    Str(StringOrSubstitutions),
    BasePathConfiguration(CelerityApiBasePathConfigurationWithSubs),
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiBasePathConfigurationWithSubs {
    pub protocol: MappingNode,
    #[serde(rename = "basePath")]
    pub base_path: StringOrSubstitutions,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiAuthWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "defaultGuard")]
    pub default_guard: Option<StringOrSubstitutions>,
    pub guards: HashMap<String, CelerityApiAuthGuardWithSubs>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiAuthGuardWithSubs {
    #[serde(rename = "type")]
    pub guard_type: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub issuer: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tokenSource")]
    pub token_source: Option<CelerityApiAuthGuardValueSourceWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audience: Option<Vec<StringOrSubstitutions>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiAuthGuardValueSourceWithSubs {
    Str(StringOrSubstitutions),
    ValueSourceConfiguration(ValueSourceConfigurationWithSubs),
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct ValueSourceConfigurationWithSubs {
    pub protocol: MappingNode,
    pub source: StringOrSubstitutions,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityConsumerSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "sourceId")]
    pub source_id: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "visibilityTimeout")]
    pub visibility_timeout: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "waitTimeSeconds")]
    pub wait_time_seconds: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "routingKey")]
    pub routing_key: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "externalEvents")]
    pub external_events: Option<HashMap<String, ExternalEventConfigurationWithSubs>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct ExternalEventConfigurationWithSubs {
    #[serde(rename = "sourceType")]
    pub source_type: EventSourceType,
    #[serde(rename = "sourceConfiguration")]
    pub source_configuration: EventSourceConfigurationWithSubs,
}

impl Default for ExternalEventConfigurationWithSubs {
    fn default() -> Self {
        Self {
            source_type: EventSourceType::ObjectStorage, // or another sensible default
            source_configuration: EventSourceConfigurationWithSubs::ObjectStorage(
                ObjectStorageEventSourceConfigurationWithSubs::default(),
            ),
        }
    }
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum EventSourceConfigurationWithSubs {
    ObjectStorage(ObjectStorageEventSourceConfigurationWithSubs),
    DatabaseStream(DatabaseStreamSourceConfigurationWithSubs),
    DataStream(DataStreamSourceConfigurationWithSubs),
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct ObjectStorageEventSourceConfigurationWithSubs {
    pub bucket: StringOrSubstitutions,
    pub events: Vec<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct DatabaseStreamSourceConfigurationWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<MappingNode>,
    #[serde(rename = "dbStreamId")]
    pub db_stream_id: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "startFromBeginning")]
    pub start_from_beginning: Option<MappingNode>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct DataStreamSourceConfigurationWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<MappingNode>,
    #[serde(rename = "dataStreamId")]
    pub data_stream_id: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "startFromBeginning")]
    pub start_from_beginning: Option<MappingNode>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityScheduleSpecWithSubs {
    pub schedule: StringOrSubstitutions,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Clone, Default)]
pub struct CelerityWorkflowSpecWithSubs {
    #[serde(rename = "startAt")]
    pub start_at: StringOrSubstitutions,
    pub states: HashMap<String, CelerityWorkflowStateWithSubs>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Clone, Default)]
pub struct CelerityWorkflowStateWithSubs {
    #[serde(rename = "type")]
    pub state_type: StringOrSubstitutions,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<StringOrSubstitutions>,
    #[serde(rename = "inputPath")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_path: Option<StringOrSubstitutions>,
    #[serde(rename = "resultPath")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result_path: Option<StringOrSubstitutions>,
    #[serde(rename = "outputPath")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_path: Option<StringOrSubstitutions>,
    #[serde(rename = "payloadTemplate")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub payload_template: Option<HashMap<String, MappingNode>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub next: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub end: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub decisions: Option<Vec<CelerityWorkflowDecisionRuleWithSubs>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<MappingNode>,
    #[serde(rename = "waitConfig")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub wait_config: Option<CelerityWorkflowWaitConfigWithSubs>,
    #[serde(rename = "failureConfig")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub failure_config: Option<CelerityWorkflowFailureConfigWithSubs>,
    #[serde(rename = "parallelBranches")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parallel_branches: Option<Vec<CelerityWorkflowParallelBranchWithSubs>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub retry: Option<Vec<CelerityWorkflowRetryConfigWithSubs>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub catch: Option<Vec<CelerityWorkflowCatchConfigWithSubs>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowDecisionRuleWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub and: Option<Vec<CelerityWorkflowConditionWithSubs>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub or: Option<Vec<CelerityWorkflowConditionWithSubs>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub not: Option<CelerityWorkflowConditionWithSubs>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub condition: Option<CelerityWorkflowConditionWithSubs>,
    pub next: StringOrSubstitutions,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowConditionWithSubs {
    pub inputs: Vec<MappingNode>,
    pub function: StringOrSubstitutions,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowWaitConfigWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub seconds: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timestamp: Option<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowFailureConfigWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cause: Option<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowParallelBranchWithSubs {
    #[serde(rename = "startAt")]
    pub start_at: StringOrSubstitutions,
    pub states: HashMap<String, CelerityWorkflowStateWithSubs>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowRetryConfigWithSubs {
    #[serde(rename = "matchErrors")]
    pub match_errors: Vec<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub interval: Option<MappingNode>,
    #[serde(rename = "maxAttempts")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_attempts: Option<MappingNode>,
    #[serde(rename = "maxDelay")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_delay: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub jitter: Option<MappingNode>,
    #[serde(rename = "backoffRate")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub backoff_rate: Option<MappingNode>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default, Clone)]
pub struct CelerityWorkflowCatchConfigWithSubs {
    #[serde(rename = "matchErrors")]
    pub match_errors: Vec<StringOrSubstitutions>,
    pub next: StringOrSubstitutions,
    #[serde(rename = "resultPath")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub result_path: Option<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityConfigSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plaintext: Option<Vec<StringOrSubstitutions>>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityBucketSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<StringOrSubstitutions>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityTopicSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub fifo: Option<MappingNode>,
}

impl Default for CelerityTopicSpecWithSubs {
    fn default() -> Self {
        CelerityTopicSpecWithSubs {
            name: None,
            fifo: Some(MappingNode::Scalar(BlueprintScalarValue::Bool(false))),
        }
    }
}

#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityQueueSpecWithSubs {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<StringOrSubstitutions>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub fifo: Option<MappingNode>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "visibilityTimeout")]
    pub visibility_timeout: Option<MappingNode>,
}

impl Default for CelerityQueueSpecWithSubs {
    fn default() -> Self {
        CelerityQueueSpecWithSubs {
            name: None,
            fifo: Some(MappingNode::Scalar(BlueprintScalarValue::Bool(false))),
            visibility_timeout: None,
        }
    }
}

/// A mapping node is used for user-defined complex
/// structures that can not be known at compile time.
/// An example use case is a payload template in a workflow
/// state.
#[derive(Serialize, Debug, PartialEq, Clone, Default)]
#[serde(untagged)]
pub enum MappingNode {
    Scalar(BlueprintScalarValue),
    Mapping(HashMap<String, MappingNode>),
    Sequence(Vec<MappingNode>),
    SubstitutionStr(StringOrSubstitutions),
    #[default]
    Null,
}

#[derive(Serialize, Debug, PartialEq, Clone, Default)]
pub struct StringOrSubstitutions {
    pub values: Vec<StringOrSubstitution>,
}

/// A string or substitution value is used for values that can be a string
/// or a `${..}` substitution.
#[derive(Serialize, Debug, PartialEq, Clone)]
pub enum StringOrSubstitution {
    StringValue(String),
    SubstitutionValue(Substitution),
}

/// Checks if a `StringOrSubstitutions` value is empty or contains
/// a single empty string value.
pub fn is_string_with_substitutions_empty(value: &StringOrSubstitutions) -> bool {
    value.values.is_empty()
        || (value.values.len() == 1
            && matches!(&value.values[0], StringOrSubstitution::StringValue(s) if s.is_empty()))
}

/// This is the parsed contents of a `${..}` substitution
/// as per the Bluelink Blueprint specification:
/// https://bluelink.dev/docs/blueprint/specification#references--substitutions
///
/// For the runtime, only variable references are supported.
#[derive(Serialize, Debug, PartialEq, Clone)]
pub enum Substitution {
    VariableReference(SubstitutionVariableReference),
}

#[derive(Serialize, Debug, PartialEq, Clone)]
pub struct SubstitutionVariableReference {
    pub variable_name: String,
}
