use std::collections::HashMap;

use serde::{Deserialize, Serialize};

/// The default version for the Celerity blueprint configuration.
pub const CELERITY_BLUEPRINT_V2023_04_20: &str = "2023-04-20";

/// The resource type identifier for a Celerity API.
pub const CELERITY_API_RESOURCE_TYPE: &str = "celerity/api";

/// The resource type identifier for a Celerity Consumer.
pub const CELERITY_CONSUMER_RESOURCE_TYPE: &str = "celerity/consumer";

/// The resource type identifier for a Celerity Schedule.
pub const CELERITY_SCHEDULE_RESOURCE_TYPE: &str = "celerity/schedule";

/// The resource type identifier for a Celerity Handler.
pub const CELERITY_HANDLER_RESOURCE_TYPE: &str = "celerity/handler";

/// The resource type identifier a Celerity Handler Config (shared config).
pub const CELERITY_HANDLER_CONFIG_RESOURCE_TYPE: &str = "celerity/handlerConfig";

/// This is a struct that holds the configuration
/// for the Celerity runtime in the form a blueprint.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct BlueprintConfig {
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_version")]
    pub version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_optional_string_or_vec")]
    pub transform: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub variables: Option<HashMap<String, BlueprintVariable>>,
    #[serde(deserialize_with = "crate::parse_helpers::deserialize_resource_map")]
    pub resources: HashMap<String, RuntimeBlueprintResource>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<BlueprintMetadata>,
}

impl Default for BlueprintConfig {
    fn default() -> Self {
        BlueprintConfig {
            version: "".to_string(),
            transform: None,
            variables: None,
            resources: HashMap::new(),
            metadata: None,
        }
    }
}

/// This is a struct that holds a variable
/// in the blueprint configuration.
/// In the runtime, variables can be sourced
/// from the runtime environment and used in the evaluation
/// of the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct BlueprintVariable {
    #[serde(rename = "type")]
    pub var_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "allowedValues")]
    pub allowed_values: Option<Vec<BlueprintScalarValue>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub default: Option<BlueprintScalarValue>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub secret: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

impl Default for BlueprintVariable {
    fn default() -> Self {
        BlueprintVariable {
            var_type: "".to_string(),
            allowed_values: None,
            default: None,
            secret: None,
            description: None,
        }
    }
}

/// This is a struct that holds a value
/// for a scalar that can be multiple types
/// in the blueprint configuration.
/// This is used to define allowed and default values
/// for a variable.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum BlueprintScalarValue {
    Str(String),
    Int(i64),
    Float(f64),
    Bool(bool),
}

impl ToString for BlueprintScalarValue {
    fn to_string(&self) -> String {
        match self {
            BlueprintScalarValue::Str(val) => val.to_string(),
            BlueprintScalarValue::Int(val) => val.to_string(),
            BlueprintScalarValue::Float(val) => val.to_string(),
            BlueprintScalarValue::Bool(val) => val.to_string(),
        }
    }
}

/// This is a struct that holds the configuration
/// for a resource in the blueprint configuration.
/// This is a type specific to the runtime configuration
/// that is focused on the Celerity-specific resources
/// that are used in the blueprint configuration.
/// Resource types that are not recognised by the runtime
/// will be ignored.
#[derive(Serialize, Debug, PartialEq)]
pub struct RuntimeBlueprintResource {
    #[serde(rename = "type")]
    pub resource_type: CelerityResourceType,
    pub metadata: BlueprintResourceMetadata,
    pub spec: CelerityResourceSpec,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "linkSelector")]
    pub link_selector: Option<BlueprintLinkSelector>,
}

impl Default for RuntimeBlueprintResource {
    fn default() -> Self {
        RuntimeBlueprintResource {
            resource_type: CelerityResourceType::CelerityHandler,
            metadata: BlueprintResourceMetadata {
                display_name: "".to_string(),
                annotations: None,
                labels: None,
            },
            link_selector: None,
            description: None,
            spec: CelerityResourceSpec::NoSpec,
        }
    }
}

/// This is an enum that holds the types of resources
/// that are recognised by the Celerity runtime.
/// The runtime will only process resources that are
/// of these types and ignore any other types.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum CelerityResourceType {
    #[serde(rename = "celerity/handler")]
    CelerityHandler,
    #[serde(rename = "celerity/api")]
    CelerityApi,
    #[serde(rename = "celerity/consumer")]
    CelerityConsumer,
    #[serde(rename = "celerity/schedule")]
    CeleritySchedule,
    #[serde(rename = "celerity/handlerConfig")]
    CelerityHandlerConfig,
}

/// This holds the metadata
/// for a resource in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct BlueprintResourceMetadata {
    #[serde(rename = "displayName")]
    pub display_name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub annotations: Option<HashMap<String, BlueprintScalarValue>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
}

impl Default for BlueprintResourceMetadata {
    fn default() -> Self {
        BlueprintResourceMetadata {
            display_name: "".to_string(),
            annotations: None,
            labels: None,
        }
    }
}

/// This holds the configuration
/// for a link selector in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct BlueprintLinkSelector {
    #[serde(rename = "byLabel")]
    pub by_label: HashMap<String, String>,
}

/// This holds the specification
/// for a resource in the blueprint configuration.
/// This is specific to resource types recognised
/// by the Celerity runtime.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityResourceSpec {
    Handler(CelerityHandlerSpec),
    Api(CelerityApiSpec),
    Consumer(CelerityConsumerSpec),
    Schedule(CelerityScheduleSpec),
    HandlerConfig(SharedHandlerConfig),
    NoSpec,
}

/// This holds the specification
/// for a handler resource in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityHandlerSpec {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "handlerName")]
    pub handler_name: Option<String>,
    #[serde(rename = "codeLocation")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code_location: Option<String>,
    pub handler: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tracingEnabled")]
    pub tracing_enabled: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "environmentVariables")]
    pub environment_variables: Option<HashMap<String, String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub events: Option<HashMap<String, EventConfiguration>>,
}

impl Default for CelerityHandlerSpec {
    fn default() -> Self {
        CelerityHandlerSpec {
            handler_name: None,
            code_location: None,
            handler: "".to_string(),
            runtime: None,
            memory: None,
            timeout: None,
            tracing_enabled: None,
            environment_variables: None,
            events: None,
        }
    }
}

/// This holds the specification
/// for an API resource in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiSpec {
    pub protocols: Vec<CelerityApiProtocol>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cors: Option<CelerityApiCors>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub domain: Option<CelerityApiDomain>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub auth: Option<CelerityApiAuth>,
    #[serde(rename = "tracingEnabled")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tracing_enabled: Option<bool>,
}

/// This holds the specification
/// for a consumer resource in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityConsumerSpec {
    #[serde(rename = "sourceId")]
    pub source_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "visibilityTimeout")]
    pub visibility_timeout: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "waitTimeSeconds")]
    pub wait_time_seconds: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<bool>,
}

impl Default for CelerityConsumerSpec {
    fn default() -> Self {
        CelerityConsumerSpec {
            source_id: "".to_string(),
            batch_size: None,
            visibility_timeout: None,
            wait_time_seconds: None,
            partial_failures: None,
        }
    }
}

/// This holds the specification
/// for a schedule resource in the blueprint configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityScheduleSpec {
    pub schedule: String,
}

impl Default for CelerityScheduleSpec {
    fn default() -> Self {
        CelerityScheduleSpec {
            schedule: "".to_string(),
        }
    }
}

/// A protocol that an API resource can support.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum CelerityApiProtocol {
    #[serde(rename = "http")]
    Http,
    #[serde(rename = "websocket")]
    WebSocket,
}

/// CORS configuration for a Celerity API resource which can be
/// a string or detailed configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiCors {
    Str(String),
    CorsConfiguration(CelerityApiCorsConfiguration),
}

/// Detailed CORS configuration
/// for a Celerity API resource.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiCorsConfiguration {
    #[serde(rename = "allowCredentials")]
    pub allow_credentials: Option<bool>,
    #[serde(rename = "allowOrigins")]
    pub allow_origins: Option<Vec<String>>,
    #[serde(rename = "allowMethods")]
    pub allow_methods: Option<Vec<String>>,
    #[serde(rename = "allowHeaders")]
    pub allow_headers: Option<Vec<String>>,
    #[serde(rename = "exposeHeaders")]
    pub expose_headers: Option<Vec<String>>,
    #[serde(rename = "maxAge")]
    pub max_age: Option<i64>,
}

/// Domain configuration for a Celerity API resource.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiDomain {
    #[serde(rename = "domainName")]
    pub domain_name: String,
    #[serde(rename = "basePaths")]
    pub base_paths: Vec<CelerityApiBasePath>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "normalizeBasePath")]
    pub normalize_base_path: Option<bool>,
    #[serde(rename = "certificateId")]
    pub certificate_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "securityPolicy")]
    pub security_policy: Option<CelerityApiDomainSecurityPolicy>,
}

/// Base path configuration for a Celerity API resource which can be
/// a string or detailed configuration.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiBasePath {
    Str(String),
    BasePathConfiguration(CelerityApiBasePathConfiguration),
}

/// Base path configuration for a Celerity API resource.
/// This allows you to configure a base path for a specific
/// protocol for an API.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityApiBasePathConfiguration {
    pub protocol: CelerityApiProtocol,
    #[serde(rename = "basePath")]
    pub base_path: String,
}

impl Default for CelerityApiBasePathConfiguration {
    fn default() -> Self {
        CelerityApiBasePathConfiguration {
            protocol: CelerityApiProtocol::Http,
            base_path: "".to_string(),
        }
    }
}

/// Security policy for a Celerity API domain.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum CelerityApiDomainSecurityPolicy {
    #[serde(rename = "TLS_1_0")]
    Tls1_0,
    #[serde(rename = "TLS_1_2")]
    Tls1_2,
}

/// Authentication configuration for a Celerity API resource.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct CelerityApiAuth {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "defaultGuard")]
    pub default_guard: Option<String>,
    pub guards: HashMap<String, CelerityApiAuthGuard>,
}

/// Guard configuration that provides access control
/// for a Celerity API resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct CelerityApiAuthGuard {
    #[serde(rename = "type")]
    pub guard_type: CelerityApiAuthGuardType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub issuer: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tokenSource")]
    pub token_source: Option<CelerityApiAuthGuardValueSource>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audience: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "apiKeySource")]
    pub api_key_source: Option<CelerityApiAuthGuardValueSource>,
}

impl Default for CelerityApiAuthGuard {
    fn default() -> Self {
        CelerityApiAuthGuard {
            guard_type: CelerityApiAuthGuardType::NoGuardType,
            issuer: None,
            token_source: None,
            audience: None,
            api_key_source: None,
        }
    }
}

/// Auth guard type for authorization configuration
/// in a Celerity API resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum CelerityApiAuthGuardType {
    #[serde(rename = "jwt")]
    Jwt,
    #[serde(rename = "apiKey")]
    ApiKey,
    #[serde(rename = "custom")]
    Custom,
    #[serde(rename = "noGuardType")]
    NoGuardType,
}

/// Value source for authorization configuration
/// in a Celerity API resource.
/// A value would be an API key or JWT.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum CelerityApiAuthGuardValueSource {
    Str(String),
    ValueSourceConfiguration(ValueSourceConfiguration),
}

/// Value source configuration for extracting a value
/// from a request or message.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct ValueSourceConfiguration {
    pub protocol: CelerityApiProtocol,
    pub source: String,
}

impl Default for ValueSourceConfiguration {
    fn default() -> Self {
        ValueSourceConfiguration {
            protocol: CelerityApiProtocol::Http,
            source: "".to_string(),
        }
    }
}

/// Configuration for a cloud service event source
/// for a handler resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct EventConfiguration {
    #[serde(rename = "sourceType")]
    pub source_type: EventSourceType,
    #[serde(rename = "sourceConfiguration")]
    pub source_configuration: EventSourceConfiguration,
}

impl Default for EventConfiguration {
    fn default() -> Self {
        EventConfiguration {
            source_type: EventSourceType::ObjectStorage,
            source_configuration: EventSourceConfiguration::ObjectStorage(
                ObjectStorageEventSourceConfiguration {
                    bucket: "".to_string(),
                    events: vec![],
                },
            ),
        }
    }
}

/// The type of event source for a handler resource.
/// This can be an object storage, database stream
/// or a data stream.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum EventSourceType {
    #[serde(rename = "objectStorage")]
    ObjectStorage,
    #[serde(rename = "dbStream")]
    DatabaseStream,
    #[serde(rename = "dataStream")]
    DataStream,
}

/// Configuration for an event source for a handler resource.
/// This can be a configuration for an object storage,
/// database stream or a data stream.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
#[serde(untagged)]
pub enum EventSourceConfiguration {
    ObjectStorage(ObjectStorageEventSourceConfiguration),
    DatabaseStream(DatabaseStreamSourceConfiguration),
    DataStream(DataStreamSourceConfiguration),
}

/// Configuration for an object storage event source
/// for a handler resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct ObjectStorageEventSourceConfiguration {
    pub bucket: String,
    pub events: Vec<ObjectStorageEventType>,
}

impl Default for ObjectStorageEventSourceConfiguration {
    fn default() -> Self {
        ObjectStorageEventSourceConfiguration {
            bucket: "".to_string(),
            events: vec![],
        }
    }
}

/// Event types for an object storage event source.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub enum ObjectStorageEventType {
    #[serde(rename = "created")]
    ObjectCreated,
    #[serde(rename = "deleted")]
    ObjectDeleted,
    #[serde(rename = "metadataUpdated")]
    ObjectMetadataUpdated,
}

/// Configuration for a database stream event source
/// for a handler resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct DatabaseStreamSourceConfiguration {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<i64>,
    #[serde(rename = "dbStreamId")]
    pub db_stream_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "startFromBeginning")]
    pub start_from_beginning: Option<bool>,
}

impl Default for DatabaseStreamSourceConfiguration {
    fn default() -> Self {
        DatabaseStreamSourceConfiguration {
            batch_size: None,
            db_stream_id: "".to_string(),
            partial_failures: None,
            start_from_beginning: None,
        }
    }
}

/// Configuration for a data stream event source
/// for a handler resource.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct DataStreamSourceConfiguration {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "batchSize")]
    pub batch_size: Option<i64>,
    #[serde(rename = "dataStreamId")]
    pub data_stream_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "partialFailures")]
    pub partial_failures: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "startFromBeginning")]
    pub start_from_beginning: Option<bool>,
}

impl Default for DataStreamSourceConfiguration {
    fn default() -> Self {
        DataStreamSourceConfiguration {
            batch_size: None,
            data_stream_id: "".to_string(),
            partial_failures: None,
            start_from_beginning: None,
        }
    }
}

/// Metadata for a blueprint.
/// For the purpose of the runtime, this is strongly
/// typed to expect an optional `sharedHandlerConfig`
/// object that provides shared defaults for all handlers
/// declared in a blueprint.
#[derive(Serialize, Deserialize, Debug, PartialEq)]
pub struct BlueprintMetadata {
    #[serde(rename = "sharedHandlerConfig")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub shared_handler_config: Option<SharedHandlerConfig>,
}

/// Provides shared defaults
/// for all handlers declared in a blueprint.
#[derive(Serialize, Deserialize, Debug, PartialEq, Default)]
pub struct SharedHandlerConfig {
    #[serde(rename = "codeLocation")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub code_location: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub runtime: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub memory: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "tracingEnabled")]
    pub tracing_enabled: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "environmentVariables")]
    pub environment_variables: Option<HashMap<String, String>>,
}
