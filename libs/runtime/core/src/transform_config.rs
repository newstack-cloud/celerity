use std::{cmp::min, collections::HashMap};

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintMetadata, CelerityApiAuth, CelerityApiAuthGuardType,
    CelerityApiBasePath, CelerityApiCors, CelerityApiProtocol, CelerityConsumerSpec,
    CelerityHandlerSpec, CelerityResourceSpec, CelerityResourceType, EventSourceConfiguration,
    ExternalEventConfiguration, RuntimeBlueprintResource, WebSocketAuthStrategy,
};
use tracing::warn;

use crate::{
    blueprint_helpers::{find_resources_linking_to, select_resources, ResourceWithName},
    config::{
        ApiConfig, ConsumerConfig, ConsumerSourceType, ConsumersConfig, CustomHandlerDefinition,
        CustomHandlersConfig, EventConfig, EventHandlerDefinition, EventTriggerConfig,
        EventsConfig, GuardHandlerDefinition, GuardsConfig, HttpConfig, HttpHandlerDefinition,
        RuntimeConfig, ScheduleConfig, SchedulesConfig, StreamConfig, StreamSourceType,
        WebSocketConfig, WebSocketHandlerDefinition,
    },
    consts::{
        CELERITY_CONSUMER_BUCKET_ANNOTATION_NAME, CELERITY_CONSUMER_BUCKET_EVENTS_ANNOTATION_NAME,
        CELERITY_CONSUMER_DATASTORE_ANNOTATION_NAME,
        CELERITY_CONSUMER_DATASTORE_START_ANNOTATION_NAME, CELERITY_CONSUMER_DLQ_ANNOTATION_NAME,
        CELERITY_CONSUMER_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME,
        CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME, CELERITY_CONSUMER_HANDLER_ROUTE_ANNOTATION_NAME,
        CELERITY_CONSUMER_QUEUE_ANNOTATION_NAME, CELERITY_HANDLER_GUARD_ANNOTATION_NAME,
        CELERITY_HANDLER_PUBLIC_ANNOTATION_NAME, CELERITY_HTTP_HANDLER_ANNOTATION_NAME,
        CELERITY_HTTP_METHOD_ANNOTATION_NAME, CELERITY_HTTP_PATH_ANNOTATION_NAME,
        CELERITY_QUEUE_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME, CELERITY_SCHEDULE_HANDLER_ANNOTATION_NAME,
        CELERITY_WS_HANDLER_ANNOTATION_NAME, CELERITY_WS_ROUTE_ANNOTATION_NAME,
        DEFAULT_HANDLER_TIMEOUT, DEFAULT_TRACING_ENABLED, DEFAULT_WEBSOCKET_API_AUTH_STRATEGY,
        DEFAULT_WEBSOCKET_API_ROUTE_KEY, MAX_HANDLER_TIMEOUT,
    },
    errors::ConfigError,
};

pub(crate) fn collect_api_config(
    blueprint_config: &BlueprintConfig,
    runtime_config: &RuntimeConfig,
) -> Result<(ApiConfig, Vec<String>), ConfigError> {
    let mut api_config = ApiConfig {
        http: None,
        websocket: None,
        guards: None,
        auth: None,
        cors: None,
        tracing_enabled: false,
    };

    let api = get_api_resource(blueprint_config, runtime_config)?;

    let target_handlers = select_resources(
        &api.link_selector,
        blueprint_config,
        CelerityResourceType::CelerityHandler,
    );

    let mut collected_handler_names: Vec<String> = Vec::new();

    let http_handlers = collect_http_handler_definitions(
        &target_handlers,
        blueprint_config,
        &mut collected_handler_names,
    )?;

    if !http_handlers.is_empty() {
        api_config.http = Some(HttpConfig {
            handlers: http_handlers,
            base_paths: vec![],
        });
    }

    let ws_handlers = collect_ws_handler_definitions(
        &target_handlers,
        blueprint_config,
        &api.spec,
        &mut collected_handler_names,
    )?;

    if !ws_handlers.is_empty() {
        api_config.websocket = create_websocket_config(&api.spec, ws_handlers)?;
    }

    api_config.tracing_enabled = resolve_api_tracing_enabled(&api.spec);
    api_config.auth = resolve_api_auth(&api.spec);
    api_config.cors = resolve_api_cors(&api.spec);
    api_config.guards = collect_custom_guard_definitions(&api_config.auth);

    Ok((api_config, collected_handler_names))
}

fn create_websocket_config(
    api_spec: &CelerityResourceSpec,
    handlers: Vec<WebSocketHandlerDefinition>,
) -> Result<Option<WebSocketConfig>, ConfigError> {
    let route_key = resolve_websocket_api_route_key(api_spec)?;
    let auth_strategy = resolve_websocket_auth_strategy(api_spec)?;
    let base_paths = resolve_base_paths(api_spec)?;
    let connection_auth_guard = resolve_websocket_connection_auth_guard(api_spec)?;

    Ok(Some(WebSocketConfig {
        handlers,
        route_key,
        base_paths,
        auth_strategy,
        connection_auth_guard,
    }))
}

fn get_api_resource<'a>(
    blueprint_config: &'a BlueprintConfig,
    runtime_config: &RuntimeConfig,
) -> Result<&'a RuntimeBlueprintResource, ConfigError> {
    let (_, api_resource) = blueprint_config
        .resources
        .iter()
        .find(
            |&(current_name, current)| match runtime_config.api_resource.as_ref() {
                // Find the API resource in the blueprint that
                // matches the name in the runtime config.
                Some(api_resource_name) => {
                    current_name == api_resource_name
                        && current.resource_type == CelerityResourceType::CelerityApi
                }
                // Fall back to using the first `celerity/api` resource in the blueprint.
                None => current.resource_type == CelerityResourceType::CelerityApi,
            },
        )
        .ok_or(ConfigError::ApiMissing)?;

    Ok(api_resource)
}

fn collect_http_handler_definitions(
    target_handlers: &Vec<ResourceWithName>,
    blueprint_config: &BlueprintConfig,
    collected_handler_names: &mut Vec<String>,
) -> Result<Vec<HttpHandlerDefinition>, ConfigError> {
    let mut http_handlers = Vec::new();

    for handler in target_handlers {
        if let Some(annotations) = &handler.resource.metadata.annotations {
            let http_enabled = annotations
                .get(CELERITY_HTTP_HANDLER_ANNOTATION_NAME)
                .map(|v| v.eq_ignore_ascii_case("true"))
                .unwrap_or(false);

            if http_enabled {
                check_handler_already_collected(&handler.name, collected_handler_names)?;

                // Get http-specific annotations and push to http handlers list.
                let method = annotations
                    .get(CELERITY_HTTP_METHOD_ANNOTATION_NAME)
                    .cloned()
                    .unwrap_or_else(|| "GET".to_string());
                let path = annotations
                    .get(CELERITY_HTTP_PATH_ANNOTATION_NAME)
                    .cloned()
                    .unwrap_or_else(|| "/".to_string());
                let auth_guard: Option<Vec<String>> = annotations
                    .get(CELERITY_HANDLER_GUARD_ANNOTATION_NAME)
                    .map(|v| {
                        v.split(',')
                            .map(|s| s.trim().to_string())
                            .filter(|s| !s.is_empty())
                            .collect()
                    });
                let public = annotations
                    .get(CELERITY_HANDLER_PUBLIC_ANNOTATION_NAME)
                    .map(|v| v.eq_ignore_ascii_case("true"))
                    .unwrap_or(false);

                collect_http_handler_definition(
                    handler,
                    HttpRouteInfo {
                        method,
                        path,
                        auth_guard,
                        public,
                    },
                    blueprint_config,
                    &mut http_handlers,
                    collected_handler_names,
                )?;
            }
        }
    }

    Ok(http_handlers)
}

struct HttpRouteInfo {
    method: String,
    path: String,
    auth_guard: Option<Vec<String>>,
    public: bool,
}

fn collect_http_handler_definition(
    handler: &ResourceWithName,
    route: HttpRouteInfo,
    blueprint_config: &BlueprintConfig,
    http_handlers: &mut Vec<HttpHandlerDefinition>,
    collected_handler_names: &mut Vec<String>,
) -> Result<(), ConfigError> {
    if let CelerityResourceSpec::Handler(handler_spec) = &handler.resource.spec {
        let handler_configs = select_resources(
            &handler.resource.link_selector,
            blueprint_config,
            CelerityResourceType::CelerityHandlerConfig,
        );
        let handler_definition = apply_http_handler_configurations(
            handler.name.clone(),
            handler_spec,
            handler_configs,
            blueprint_config.metadata.as_ref(),
            route,
        )?;
        http_handlers.push(handler_definition);
        collected_handler_names.push(handler.name.clone());
        Ok(())
    } else {
        Err(ConfigError::Api(format!(
            "handler {} is missing spec or resource is not a handler",
            handler.name
        )))
    }
}

fn collect_ws_handler_definitions(
    target_handlers: &Vec<ResourceWithName>,
    blueprint_config: &BlueprintConfig,
    api_spec: &CelerityResourceSpec,
    collected_handler_names: &mut Vec<String>,
) -> Result<Vec<WebSocketHandlerDefinition>, ConfigError> {
    let mut ws_handlers = Vec::new();

    for handler in target_handlers {
        if let Some(annotations) = &handler.resource.metadata.annotations {
            let ws_enabled = annotations
                .get(CELERITY_WS_HANDLER_ANNOTATION_NAME)
                .map(|v| v.eq_ignore_ascii_case("true"))
                .unwrap_or(false);

            if ws_enabled {
                check_handler_already_collected(&handler.name, collected_handler_names)?;

                // Get websocket-specific annotations and push to websocket handlers list.
                let route = annotations
                    .get(CELERITY_WS_ROUTE_ANNOTATION_NAME)
                    .cloned()
                    .unwrap_or_else(|| "$default".to_string());

                // Derive the message object property name to use
                // as the route key from the API spec.
                let route_key = resolve_websocket_api_route_key(api_spec)?;

                collect_websocket_handler_definition(
                    handler,
                    route,
                    route_key,
                    blueprint_config,
                    &mut ws_handlers,
                    collected_handler_names,
                )?;
            }
        }
    }
    Ok(ws_handlers)
}

fn resolve_websocket_api_route_key(api_spec: &CelerityResourceSpec) -> Result<String, ConfigError> {
    if let CelerityResourceSpec::Api(api_spec) = api_spec {
        let route_key_opt = api_spec
            .protocols
            .iter()
            .find(|protocol| matches!(protocol, CelerityApiProtocol::WebSocketConfig(_)))
            .map(|protocol| match protocol {
                CelerityApiProtocol::WebSocketConfig(config) => config
                    .route_key
                    .clone()
                    .unwrap_or_else(|| DEFAULT_WEBSOCKET_API_ROUTE_KEY.to_string()),
                _ => DEFAULT_WEBSOCKET_API_ROUTE_KEY.to_string(),
            });

        if let Some(route_key) = route_key_opt {
            Ok(route_key)
        } else {
            Ok(DEFAULT_WEBSOCKET_API_ROUTE_KEY.to_string())
        }
    } else {
        Err(ConfigError::Api(
            "Invalid API spec was provided when resolving WebSocket API route key".to_string(),
        ))
    }
}

fn resolve_websocket_auth_strategy(
    api_resource_spec: &CelerityResourceSpec,
) -> Result<WebSocketAuthStrategy, ConfigError> {
    if let CelerityResourceSpec::Api(api_spec) = api_resource_spec {
        let auth_strategy_opt = api_spec
            .protocols
            .iter()
            .find(|protocol| matches!(protocol, CelerityApiProtocol::WebSocketConfig(_)))
            .and_then(|protocol| match protocol {
                CelerityApiProtocol::WebSocketConfig(config) => config.auth_strategy.clone(),
                _ => None,
            });

        if let Some(auth_strategy) = auth_strategy_opt {
            Ok(auth_strategy)
        } else {
            Ok(DEFAULT_WEBSOCKET_API_AUTH_STRATEGY)
        }
    } else {
        Err(ConfigError::Api(
            "Invalid API spec was provided when resolving WebSocket API auth strategy".to_string(),
        ))
    }
}

fn resolve_base_paths(
    api_resource_spec: &CelerityResourceSpec,
) -> Result<Vec<CelerityApiBasePath>, ConfigError> {
    if let CelerityResourceSpec::Api(api_spec) = api_resource_spec {
        if let Some(domain_config) = api_spec.domain.as_ref() {
            Ok(domain_config.base_paths.clone())
        } else {
            Ok(vec![])
        }
    } else {
        Err(ConfigError::Api(
            "Invalid API spec was provided when resolving API base paths".to_string(),
        ))
    }
}

fn resolve_websocket_connection_auth_guard(
    api_resource_spec: &CelerityResourceSpec,
) -> Result<Option<Vec<String>>, ConfigError> {
    if let CelerityResourceSpec::Api(api_spec) = api_resource_spec {
        let connected_auth_guard_opt = api_spec
            .protocols
            .iter()
            .find(|protocol| matches!(protocol, CelerityApiProtocol::WebSocketConfig(_)))
            .and_then(|protocol| match protocol {
                CelerityApiProtocol::WebSocketConfig(config) => config.auth_guard.clone(),
                _ => None,
            });

        if let Some(connected_auth_guard) = connected_auth_guard_opt {
            Ok(Some(connected_auth_guard))
        } else {
            Ok(None)
        }
    } else {
        Err(ConfigError::Api(
            "Invalid API spec was provided when resolving WebSocket API connection auth guard"
                .to_string(),
        ))
    }
}

fn collect_websocket_handler_definition(
    handler: &ResourceWithName,
    route: String,
    route_key: String,
    blueprint_config: &BlueprintConfig,
    ws_handlers: &mut Vec<WebSocketHandlerDefinition>,
    collected_handler_names: &mut Vec<String>,
) -> Result<(), ConfigError> {
    if let CelerityResourceSpec::Handler(handler_spec) = &handler.resource.spec {
        let handler_configs = select_resources(
            &handler.resource.link_selector,
            blueprint_config,
            CelerityResourceType::CelerityHandlerConfig,
        );
        let handler_definition = apply_websocket_handler_configurations(
            handler.name.clone(),
            handler_spec,
            handler_configs,
            blueprint_config.metadata.as_ref(),
            route,
            route_key,
        )?;
        ws_handlers.push(handler_definition);
        collected_handler_names.push(handler.name.clone());
        Ok(())
    } else {
        Err(ConfigError::Api(format!(
            "handler {} is missing spec or resource is not a handler",
            handler.name
        )))
    }
}

fn check_handler_already_collected(
    handler_name: &String,
    collected_handler_names: &[String],
) -> Result<(), ConfigError> {
    if collected_handler_names.contains(handler_name) {
        return Err(ConfigError::Api(format!(
            "handler {handler_name} is configured for multiple kinds of applications, \
            a handler can only be configured for one kind of application \
            (e.g. HTTP, WebSocket, Queue Consumer etc.)",
        )));
    }
    Ok(())
}

fn apply_http_handler_configurations(
    handler_name: String,
    handler_spec: &CelerityHandlerSpec,
    handler_configs: Vec<ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
    route: HttpRouteInfo,
) -> Result<HttpHandlerDefinition, ConfigError> {
    let handler_definition = HttpHandlerDefinition {
        name: handler_name.clone(),
        handler: handler_spec.handler.clone(),
        path: to_axum_path(route.path),
        method: route.method,
        location: resolve_handler_location(
            handler_name,
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        )?,
        timeout: resolve_handler_timeout(handler_spec, handler_configs.first(), blueprint_metadata),
        tracing_enabled: resolve_tracing_enabled(
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        ),
        auth_guard: route.auth_guard,
        public: route.public,
    };

    Ok(handler_definition)
}

fn apply_websocket_handler_configurations(
    handler_name: String,
    handler_spec: &CelerityHandlerSpec,
    handler_configs: Vec<ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
    route: String,
    route_key: String,
) -> Result<WebSocketHandlerDefinition, ConfigError> {
    let handler_definition = WebSocketHandlerDefinition {
        name: handler_name.clone(),
        handler: handler_spec.handler.clone(),
        route,
        route_key,
        location: resolve_handler_location(
            handler_name,
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        )?,
        timeout: resolve_handler_timeout(handler_spec, handler_configs.first(), blueprint_metadata),
        tracing_enabled: resolve_tracing_enabled(
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        ),
    };

    Ok(handler_definition)
}

fn resolve_handler_location<'a>(
    handler_name: String,
    handler_spec: &'a CelerityHandlerSpec,
    handler_config: Option<&'a ResourceWithName>,
    blueprint_metadata: Option<&'a BlueprintMetadata>,
) -> Result<String, ConfigError> {
    let final_location = handler_spec.code_location.as_ref().or_else(|| {
        handler_config
            .and_then(|config| match &config.resource.spec {
                CelerityResourceSpec::HandlerConfig(handler_config) => {
                    handler_config.code_location.as_ref()
                }
                _ => None,
            })
            .or_else(|| {
                blueprint_metadata.and_then(|metadata| {
                    metadata
                        .shared_handler_config
                        .as_ref()
                        .and_then(|config| config.code_location.as_ref())
                })
            })
    });

    if let Some(location) = final_location {
        Ok(location.clone())
    } else {
        Err(ConfigError::Api(format!(
            "handler {handler_name} is missing code location, define it in the \
            handler spec or one of the supported handler config locations",
        )))
    }
}

fn resolve_handler_timeout(
    handler_spec: &CelerityHandlerSpec,
    handler_config: Option<&ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
) -> i64 {
    handler_spec
        .timeout
        .map(|timeout| min(timeout, MAX_HANDLER_TIMEOUT))
        .or_else(|| {
            handler_config
                .and_then(|config| match &config.resource.spec {
                    CelerityResourceSpec::HandlerConfig(handler_config) => handler_config.timeout,
                    _ => None,
                })
                .map(|timeout| min(timeout, MAX_HANDLER_TIMEOUT))
        })
        .or_else(|| {
            blueprint_metadata.and_then(|metadata| {
                metadata
                    .shared_handler_config
                    .as_ref()
                    .and_then(|config| config.timeout)
            })
        })
        // We can safely fallback to a reasonable default timeout when one is not supplied.
        .unwrap_or(DEFAULT_HANDLER_TIMEOUT)
}

fn resolve_tracing_enabled(
    handler_spec: &CelerityHandlerSpec,
    handler_config: Option<&ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
) -> bool {
    handler_spec
        .tracing_enabled
        .or_else(|| {
            handler_config
                .and_then(|config| match &config.resource.spec {
                    CelerityResourceSpec::HandlerConfig(handler_config) => {
                        handler_config.tracing_enabled
                    }
                    _ => None,
                })
                .or_else(|| {
                    blueprint_metadata.and_then(|metadata| {
                        metadata
                            .shared_handler_config
                            .as_ref()
                            .and_then(|config| config.tracing_enabled)
                    })
                })
        })
        .unwrap_or(DEFAULT_TRACING_ENABLED)
}

fn resolve_api_tracing_enabled(api_spec: &CelerityResourceSpec) -> bool {
    match api_spec {
        CelerityResourceSpec::Api(api_spec) => {
            api_spec.tracing_enabled.unwrap_or(DEFAULT_TRACING_ENABLED)
        }
        _ => DEFAULT_TRACING_ENABLED,
    }
}

fn collect_custom_guard_definitions(auth: &Option<CelerityApiAuth>) -> Option<GuardsConfig> {
    let auth = auth.as_ref()?;
    let handlers: Vec<GuardHandlerDefinition> = auth
        .guards
        .iter()
        .filter(|(_, guard)| guard.guard_type == CelerityApiAuthGuardType::Custom)
        .map(|(name, _)| GuardHandlerDefinition { name: name.clone() })
        .collect();

    if handlers.is_empty() {
        None
    } else {
        Some(GuardsConfig { handlers })
    }
}

fn resolve_api_auth(api_spec: &CelerityResourceSpec) -> Option<CelerityApiAuth> {
    match api_spec {
        CelerityResourceSpec::Api(api_spec) => api_spec.auth.clone(),
        _ => None,
    }
}

fn resolve_api_cors(api_spec: &CelerityResourceSpec) -> Option<CelerityApiCors> {
    match api_spec {
        CelerityResourceSpec::Api(api_spec) => api_spec.cors.clone(),
        _ => None,
    }
}

// Converts a Celerity path to an Axum path.
// As of Axum v0.7, path segments now follow the same syntax
// as the Celerity paths with {capture} syntax.
// Celerity wildcards are of the form `/{param+}`.
// Axum wildcards are of the form `/{*param}`.
fn to_axum_path(celerity_path: String) -> String {
    celerity_path
        .split('/')
        .map(|part| {
            if part.starts_with('{') && part.ends_with('}') {
                let inner = &part[1..part.len() - 1];
                if let Some(stripped) = inner.strip_suffix('+') {
                    format!("{{*{stripped}}}")
                } else {
                    part.to_string()
                }
            } else {
                part.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("/")
}

/// Checks whether a resource matches the provided app filter.
/// The filter matches against either the `celerity.app` annotation
/// on the resource or the resource name itself.
/// Returns true if no filter is set (all resources match).
fn matches_app_filter(
    resource_name: &str,
    resource: &RuntimeBlueprintResource,
    app_filter: &Option<String>,
) -> bool {
    match app_filter {
        None => true,
        Some(filter) => {
            let matches_annotation = resource
                .metadata
                .annotations
                .as_ref()
                .and_then(|a| a.get("celerity.app"))
                .map(|v| v == filter)
                .unwrap_or(false);
            matches_annotation || resource_name == filter
        }
    }
}

/// Describes how a consumer's message/event source was resolved.
#[derive(Debug)]
enum ConsumerSourceResolution<'a> {
    /// sourceId was explicitly set in the consumer spec.
    ExplicitSourceId(String),
    /// externalEvents mapping is present.
    ExternalEvents(&'a HashMap<String, ExternalEventConfiguration>),
    /// Consumer is linked from a celerity/queue resource.
    LinkedQueue { queue_resource_name: String },
    /// Consumer is linked from a celerity/datastore resource.
    LinkedDatastore { datastore_resource_name: String },
    /// Consumer is linked from a celerity/bucket resource.
    LinkedBucket { bucket_resource_name: String },
    /// No source detected — this is a configuration error.
    NoSource,
}

/// Resolves the message/event source for a consumer resource.
/// A consumer can have exactly one source, determined by one of three
/// mutually exclusive paths: explicit sourceId, externalEvents, or
/// a linked-from resource (queue, datastore, bucket).
fn resolve_consumer_source<'a>(
    consumer_name: &str,
    consumer_resource: &RuntimeBlueprintResource,
    consumer_spec: &'a CelerityConsumerSpec,
    blueprint_config: &'a BlueprintConfig,
) -> Result<ConsumerSourceResolution<'a>, ConfigError> {
    // Path 1: Explicit sourceId
    if let Some(source_id) = &consumer_spec.source_id {
        return Ok(ConsumerSourceResolution::ExplicitSourceId(
            source_id.clone(),
        ));
    }

    // Path 2: externalEvents
    if let Some(external_events) = &consumer_spec.external_events {
        return Ok(ConsumerSourceResolution::ExternalEvents(external_events));
    }

    // Path 3: Linked-from resources
    let linked = find_resources_linking_to(
        consumer_resource,
        blueprint_config,
        &[
            CelerityResourceType::CelerityQueue,
            CelerityResourceType::CelerityDatastore,
            CelerityResourceType::CelerityBucket,
        ],
    );

    if linked.is_empty() {
        return Ok(ConsumerSourceResolution::NoSource);
    }

    // Group by resource type.
    let mut queues: Vec<&ResourceWithName> = Vec::new();
    let mut datastores: Vec<&ResourceWithName> = Vec::new();
    let mut buckets: Vec<&ResourceWithName> = Vec::new();
    for resource in &linked {
        match resource.resource.resource_type {
            CelerityResourceType::CelerityQueue => queues.push(resource),
            CelerityResourceType::CelerityDatastore => datastores.push(resource),
            CelerityResourceType::CelerityBucket => buckets.push(resource),
            _ => {}
        }
    }

    // Validate: at most one resource type group should be non-empty.
    let non_empty_count =
        !queues.is_empty() as u8 + !datastores.is_empty() as u8 + !buckets.is_empty() as u8;
    if non_empty_count > 1 {
        return Err(ConfigError::Consumer(format!(
            "consumer '{consumer_name}' is linked from multiple source types \
            (queue, datastore, bucket); a consumer can only have one source type"
        )));
    }

    if !queues.is_empty() {
        let name = disambiguate_linked_resource(
            consumer_name,
            consumer_resource,
            &queues,
            "queue",
            CELERITY_CONSUMER_QUEUE_ANNOTATION_NAME,
        )?;
        return Ok(ConsumerSourceResolution::LinkedQueue {
            queue_resource_name: name,
        });
    }

    if !datastores.is_empty() {
        let name = disambiguate_linked_resource(
            consumer_name,
            consumer_resource,
            &datastores,
            "datastore",
            CELERITY_CONSUMER_DATASTORE_ANNOTATION_NAME,
        )?;
        return Ok(ConsumerSourceResolution::LinkedDatastore {
            datastore_resource_name: name,
        });
    }

    if !buckets.is_empty() {
        let name = disambiguate_linked_resource(
            consumer_name,
            consumer_resource,
            &buckets,
            "bucket",
            CELERITY_CONSUMER_BUCKET_ANNOTATION_NAME,
        )?;
        return Ok(ConsumerSourceResolution::LinkedBucket {
            bucket_resource_name: name,
        });
    }

    Ok(ConsumerSourceResolution::NoSource)
}

/// When multiple resources of the same type link to a consumer,
/// use a disambiguation annotation on the consumer to select the correct one.
fn disambiguate_linked_resource(
    consumer_name: &str,
    consumer_resource: &RuntimeBlueprintResource,
    candidates: &[&ResourceWithName],
    resource_kind: &str,
    annotation_name: &str,
) -> Result<String, ConfigError> {
    if candidates.len() == 1 {
        return Ok(candidates[0].name.clone());
    }

    // Multiple candidates — need disambiguation annotation.
    let selected = consumer_resource
        .metadata
        .annotations
        .as_ref()
        .and_then(|a| a.get(annotation_name));

    match selected {
        Some(name) => {
            if candidates.iter().any(|c| c.name == *name) {
                Ok(name.clone())
            } else {
                Err(ConfigError::Consumer(format!(
                    "consumer '{consumer_name}' specifies {resource_kind} '{name}' \
                    in annotation '{annotation_name}', but no matching {resource_kind} \
                    resource was found among the linked resources"
                )))
            }
        }
        None => Err(ConfigError::Consumer(format!(
            "consumer '{consumer_name}' is linked from multiple {resource_kind} resources; \
            set the '{annotation_name}' annotation on the consumer to disambiguate"
        ))),
    }
}

/// Collects consumer handler definitions from linked handlers of a consumer resource.
fn collect_consumer_handler_definitions(
    consumer_resource: &RuntimeBlueprintResource,
    blueprint_config: &BlueprintConfig,
    collected_handler_names: &mut Vec<String>,
    annotation_name: &str,
) -> Result<Vec<EventHandlerDefinition>, ConfigError> {
    let target_handlers = select_resources(
        &consumer_resource.link_selector,
        blueprint_config,
        CelerityResourceType::CelerityHandler,
    );

    let mut handlers = Vec::new();
    for handler in &target_handlers {
        let annotations = match &handler.resource.metadata.annotations {
            Some(a) => a,
            None => continue,
        };

        let enabled = annotations
            .get(annotation_name)
            .map(|v| v.eq_ignore_ascii_case("true"))
            .unwrap_or(false);

        if !enabled {
            continue;
        }

        check_handler_already_collected(&handler.name, collected_handler_names)?;

        if let CelerityResourceSpec::Handler(handler_spec) = &handler.resource.spec {
            let handler_configs = select_resources(
                &handler.resource.link_selector,
                blueprint_config,
                CelerityResourceType::CelerityHandlerConfig,
            );
            let route = annotations
                .get(CELERITY_CONSUMER_HANDLER_ROUTE_ANNOTATION_NAME)
                .cloned();
            let definition = build_event_handler_definition(
                handler.name.clone(),
                handler_spec,
                handler_configs,
                blueprint_config.metadata.as_ref(),
                route,
            )?;
            handlers.push(definition);
            collected_handler_names.push(handler.name.clone());
        } else {
            return Err(ConfigError::Consumer(format!(
                "handler {} is missing spec or resource is not a handler",
                handler.name
            )));
        }
    }
    Ok(handlers)
}

/// Builds an EventHandlerDefinition from a handler spec, resolving
/// location, timeout, and tracing from the config cascade.
fn build_event_handler_definition(
    handler_name: String,
    handler_spec: &CelerityHandlerSpec,
    handler_configs: Vec<ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
    route: Option<String>,
) -> Result<EventHandlerDefinition, ConfigError> {
    Ok(EventHandlerDefinition {
        name: handler_name.clone(),
        handler: handler_spec.handler.clone(),
        location: resolve_handler_location(
            handler_name,
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        )?,
        timeout: resolve_handler_timeout(handler_spec, handler_configs.first(), blueprint_metadata),
        tracing_enabled: resolve_tracing_enabled(
            handler_spec,
            handler_configs.first(),
            blueprint_metadata,
        ),
        route,
    })
}

/// Reads a string annotation value from a consumer resource.
fn get_consumer_annotation<'a>(
    consumer_resource: &'a RuntimeBlueprintResource,
    annotation_name: &str,
) -> Option<&'a String> {
    consumer_resource
        .metadata
        .annotations
        .as_ref()
        .and_then(|a| a.get(annotation_name))
}

/// Reads an annotation value from a resource and parses it as an i64.
fn get_annotation_i64(resource: &RuntimeBlueprintResource, annotation_name: &str) -> Option<i64> {
    resource
        .metadata
        .annotations
        .as_ref()?
        .get(annotation_name)?
        .parse()
        .ok()
}

/// Reads an annotation value from a resource and parses it as a bool.
fn get_annotation_bool(resource: &RuntimeBlueprintResource, annotation_name: &str) -> Option<bool> {
    resource
        .metadata
        .annotations
        .as_ref()?
        .get(annotation_name)?
        .parse()
        .ok()
}

/// Resolves the dead-letter queue configuration for a consumer.
///
/// For queue sources: checks if the source queue links to another queue (the DLQ)
/// and reads `celerity.queue.deadLetterMaxAttempts` from the DLQ queue resource.
///
/// For topic sources: checks the `celerity.consumer.deadLetterQueue` annotation
/// on the consumer (default `true`) and reads `celerity.consumer.deadLetterQueueMaxAttempts`.
///
/// Event and schedule consumers do not have DLQs.
fn resolve_consumer_dlq(
    consumer_resource: &RuntimeBlueprintResource,
    source_type: &ConsumerSourceType,
    source_id: &str,
    resolution: &ConsumerSourceResolution,
    blueprint_config: &BlueprintConfig,
) -> (Option<String>, Option<i64>) {
    match resolution {
        ConsumerSourceResolution::LinkedQueue {
            queue_resource_name,
        } => {
            let queue_resource = match blueprint_config.resources.get(queue_resource_name) {
                Some(r) => r,
                None => return (None, None),
            };
            let linked_queues = select_resources(
                &queue_resource.link_selector,
                blueprint_config,
                CelerityResourceType::CelerityQueue,
            );
            // The DLQ is a linked queue that isn't the source queue itself.
            let dlq = linked_queues
                .iter()
                .find(|q| &q.name != queue_resource_name);
            match dlq {
                Some(dlq_resource) => {
                    let max_attempts = get_annotation_i64(
                        dlq_resource.resource,
                        CELERITY_QUEUE_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME,
                    );
                    (Some(dlq_resource.name.clone()), max_attempts)
                }
                None => (None, None),
            }
        }
        ConsumerSourceResolution::ExplicitSourceId(_)
            if *source_type == ConsumerSourceType::Topic =>
        {
            let dlq_enabled =
                get_annotation_bool(consumer_resource, CELERITY_CONSUMER_DLQ_ANNOTATION_NAME)
                    .unwrap_or(true);
            if dlq_enabled {
                let max_attempts = get_annotation_i64(
                    consumer_resource,
                    CELERITY_CONSUMER_DLQ_MAX_ATTEMPTS_ANNOTATION_NAME,
                );
                (Some(source_id.to_string()), max_attempts)
            } else {
                (None, None)
            }
        }
        _ => (None, None),
    }
}

/// Collects consumer configuration from blueprint consumer resources.
/// Only produces ConsumerConfig for consumers with an explicit sourceId
/// or a linked-from queue resource.
pub(crate) fn collect_consumer_config(
    blueprint_config: &BlueprintConfig,
    runtime_config: &RuntimeConfig,
    collected_handler_names: &mut Vec<String>,
) -> Result<Option<ConsumersConfig>, ConfigError> {
    let mut consumers = Vec::new();

    for (name, resource) in &blueprint_config.resources {
        if resource.resource_type != CelerityResourceType::CelerityConsumer {
            continue;
        }
        if !matches_app_filter(name, resource, &runtime_config.consumer_app) {
            continue;
        }

        let consumer_spec = match &resource.spec {
            CelerityResourceSpec::Consumer(spec) => spec,
            _ => continue,
        };

        let resolution = resolve_consumer_source(name, resource, consumer_spec, blueprint_config)?;

        let (source_id, source_type) = match &resolution {
            ConsumerSourceResolution::ExplicitSourceId(id) => {
                if let Some(topic_name) = id.strip_prefix("celerity::topic::") {
                    // Source is a Celerity topic — strip the prefix to get the topic name.
                    (topic_name.to_string(), ConsumerSourceType::Topic)
                } else {
                    (id.clone(), ConsumerSourceType::Queue)
                }
            }
            ConsumerSourceResolution::LinkedQueue {
                queue_resource_name,
            } => (queue_resource_name.clone(), ConsumerSourceType::Queue),
            // Other source types are handled by collect_events_config.
            _ => continue,
        };

        let handlers = collect_consumer_handler_definitions(
            resource,
            blueprint_config,
            collected_handler_names,
            CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME,
        )?;

        // Determine the routing key from the first handler that has a route annotation.
        let routing_key = consumer_spec.routing_key.clone();

        let (dlq_source_id, max_retries) = resolve_consumer_dlq(
            resource,
            &source_type,
            &source_id,
            &resolution,
            blueprint_config,
        );

        consumers.push(ConsumerConfig {
            consumer_name: name.clone(),
            source_id,
            source_type,
            batch_size: consumer_spec.batch_size,
            visibility_timeout: consumer_spec.visibility_timeout,
            wait_time_seconds: consumer_spec.wait_time_seconds,
            partial_failures: consumer_spec.partial_failures,
            routing_key,
            dlq_source_id,
            max_retries,
            handlers,
        });
    }

    if consumers.is_empty() {
        Ok(None)
    } else {
        Ok(Some(ConsumersConfig { consumers }))
    }
}

/// Collects events configuration from blueprint consumer resources.
/// Produces EventConfig for consumers with externalEvents or
/// linked-from datastore/bucket resources.
pub(crate) fn collect_events_config(
    blueprint_config: &BlueprintConfig,
    runtime_config: &RuntimeConfig,
    collected_handler_names: &mut Vec<String>,
) -> Result<Option<EventsConfig>, ConfigError> {
    let mut events = Vec::new();

    for (name, resource) in &blueprint_config.resources {
        if resource.resource_type != CelerityResourceType::CelerityConsumer {
            continue;
        }
        if !matches_app_filter(name, resource, &runtime_config.consumer_app) {
            continue;
        }

        let consumer_spec = match &resource.spec {
            CelerityResourceSpec::Consumer(spec) => spec,
            _ => continue,
        };

        let resolution = resolve_consumer_source(name, resource, consumer_spec, blueprint_config)?;

        match resolution {
            ConsumerSourceResolution::ExternalEvents(external_events) => {
                let handlers = collect_consumer_handler_definitions(
                    resource,
                    blueprint_config,
                    collected_handler_names,
                    CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME,
                )?;
                for ext_event in external_events.values() {
                    let event_config =
                        build_event_config_from_external(ext_event, consumer_spec, &handlers);
                    events.push(event_config);
                }
            }
            ConsumerSourceResolution::LinkedDatastore {
                datastore_resource_name,
            } => {
                let handlers = collect_consumer_handler_definitions(
                    resource,
                    blueprint_config,
                    collected_handler_names,
                    CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME,
                )?;
                let start_from_beginning = get_consumer_annotation(
                    resource,
                    CELERITY_CONSUMER_DATASTORE_START_ANNOTATION_NAME,
                )
                .and_then(|v| v.parse::<bool>().ok());
                events.push(EventConfig::Stream(StreamConfig {
                    source_type: StreamSourceType::Datastore,
                    stream_id: datastore_resource_name,
                    batch_size: consumer_spec.batch_size,
                    partial_failures: consumer_spec.partial_failures,
                    start_from_beginning,
                    handlers,
                }));
            }
            ConsumerSourceResolution::LinkedBucket {
                bucket_resource_name,
            } => {
                let handlers = collect_consumer_handler_definitions(
                    resource,
                    blueprint_config,
                    collected_handler_names,
                    CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME,
                )?;
                let event_type = get_consumer_annotation(
                    resource,
                    CELERITY_CONSUMER_BUCKET_EVENTS_ANNOTATION_NAME,
                )
                .cloned()
                .unwrap_or_default();
                events.push(EventConfig::EventTrigger(EventTriggerConfig {
                    event_type,
                    queue_id: bucket_resource_name,
                    batch_size: consumer_spec.batch_size,
                    visibility_timeout: consumer_spec.visibility_timeout,
                    wait_time_seconds: consumer_spec.wait_time_seconds,
                    partial_failures: consumer_spec.partial_failures,
                    handlers,
                }));
            }
            ConsumerSourceResolution::NoSource => {
                warn!(
                    "consumer '{name}' has no sourceId, no externalEvents, \
                    and no linked resources; skipping"
                );
            }
            // ExplicitSourceId and LinkedQueue are handled by collect_consumer_config.
            _ => {}
        }
    }

    if events.is_empty() {
        Ok(None)
    } else {
        Ok(Some(EventsConfig { events }))
    }
}

/// Builds an EventConfig from an ExternalEventConfiguration entry.
fn build_event_config_from_external(
    ext_event: &ExternalEventConfiguration,
    consumer_spec: &CelerityConsumerSpec,
    handlers: &[EventHandlerDefinition],
) -> EventConfig {
    match &ext_event.source_configuration {
        EventSourceConfiguration::DataStream(config) => EventConfig::Stream(StreamConfig {
            source_type: StreamSourceType::DataStream,
            stream_id: config.data_stream_id.clone(),
            batch_size: config.batch_size,
            partial_failures: config.partial_failures,
            start_from_beginning: config.start_from_beginning,
            handlers: handlers.to_vec(),
        }),
        EventSourceConfiguration::DatabaseStream(config) => EventConfig::Stream(StreamConfig {
            source_type: StreamSourceType::Datastore,
            stream_id: config.db_stream_id.clone(),
            batch_size: config.batch_size,
            partial_failures: config.partial_failures,
            start_from_beginning: config.start_from_beginning,
            handlers: handlers.to_vec(),
        }),
        EventSourceConfiguration::ObjectStorage(config) => {
            let event_type = config
                .events
                .iter()
                .map(|e| format!("{:?}", e))
                .collect::<Vec<_>>()
                .join(",");
            EventConfig::EventTrigger(EventTriggerConfig {
                event_type,
                queue_id: config.bucket.clone(),
                batch_size: consumer_spec.batch_size,
                visibility_timeout: consumer_spec.visibility_timeout,
                wait_time_seconds: consumer_spec.wait_time_seconds,
                partial_failures: consumer_spec.partial_failures,
                handlers: handlers.to_vec(),
            })
        }
    }
}

/// Collects schedule configuration from blueprint schedule resources.
pub(crate) fn collect_schedule_config(
    blueprint_config: &BlueprintConfig,
    runtime_config: &RuntimeConfig,
    collected_handler_names: &mut Vec<String>,
) -> Result<Option<SchedulesConfig>, ConfigError> {
    let mut schedules = Vec::new();

    for (name, resource) in &blueprint_config.resources {
        if resource.resource_type != CelerityResourceType::CeleritySchedule {
            continue;
        }
        if !matches_app_filter(name, resource, &runtime_config.schedule_app) {
            continue;
        }

        let schedule_spec = match &resource.spec {
            CelerityResourceSpec::Schedule(spec) => spec,
            _ => continue,
        };

        let handlers = collect_consumer_handler_definitions(
            resource,
            blueprint_config,
            collected_handler_names,
            CELERITY_SCHEDULE_HANDLER_ANNOTATION_NAME,
        )?;

        schedules.push(ScheduleConfig {
            schedule_id: name.clone(),
            schedule_value: schedule_spec.schedule.clone(),
            // Queue ID is resolved in Phase 2 based on platform.
            queue_id: String::new(),
            // Polling config uses platform defaults in Phase 2.
            batch_size: None,
            visibility_timeout: None,
            wait_time_seconds: None,
            partial_failures: None,
            handlers,
            input: schedule_spec.input.clone(),
        });
    }

    if schedules.is_empty() {
        Ok(None)
    } else {
        Ok(Some(SchedulesConfig { schedules }))
    }
}

/// Collects custom handler definitions — handlers that are not
/// claimed by any protocol (HTTP, WebSocket, consumer, schedule).
pub(crate) fn collect_custom_handler_definitions(
    blueprint_config: &BlueprintConfig,
    collected_handler_names: &[String],
) -> Result<Option<CustomHandlersConfig>, ConfigError> {
    let protocol_annotations = [
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME,
        CELERITY_WS_HANDLER_ANNOTATION_NAME,
        CELERITY_CONSUMER_HANDLER_ANNOTATION_NAME,
        CELERITY_SCHEDULE_HANDLER_ANNOTATION_NAME,
    ];

    let mut handlers = Vec::new();

    for (name, resource) in &blueprint_config.resources {
        if resource.resource_type != CelerityResourceType::CelerityHandler {
            continue;
        }
        if collected_handler_names.contains(name) {
            continue;
        }

        // Skip handlers that have any protocol annotation set to "true".
        let has_protocol_annotation = resource
            .metadata
            .annotations
            .as_ref()
            .map(|annotations| {
                protocol_annotations.iter().any(|ann| {
                    annotations
                        .get(*ann)
                        .map(|v| v.eq_ignore_ascii_case("true"))
                        .unwrap_or(false)
                })
            })
            .unwrap_or(false);

        if has_protocol_annotation {
            continue;
        }

        if let CelerityResourceSpec::Handler(handler_spec) = &resource.spec {
            let handler_configs = select_resources(
                &resource.link_selector,
                blueprint_config,
                CelerityResourceType::CelerityHandlerConfig,
            );
            let location = resolve_handler_location(
                name.clone(),
                handler_spec,
                handler_configs.first(),
                blueprint_config.metadata.as_ref(),
            )?;
            handlers.push(CustomHandlerDefinition {
                name: name.clone(),
                handler: handler_spec.handler.clone(),
                location,
                timeout: resolve_handler_timeout(
                    handler_spec,
                    handler_configs.first(),
                    blueprint_config.metadata.as_ref(),
                ),
                tracing_enabled: resolve_tracing_enabled(
                    handler_spec,
                    handler_configs.first(),
                    blueprint_config.metadata.as_ref(),
                ),
            });
        }
    }

    if handlers.is_empty() {
        Ok(None)
    } else {
        Ok(Some(CustomHandlersConfig { handlers }))
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use celerity_blueprint_config_parser::blueprint::{
        BlueprintConfig, BlueprintLinkSelector, BlueprintResourceMetadata, CelerityBucketSpec,
        CelerityConsumerSpec, CelerityDatastoreSpec, CelerityHandlerSpec, CelerityQueueSpec,
        CelerityResourceSpec, CelerityResourceType, CelerityScheduleSpec,
        DataStreamSourceConfiguration, DatabaseStreamSourceConfiguration, EventSourceConfiguration,
        ExternalEventConfiguration, RuntimeBlueprintResource,
    };

    use super::*;
    use crate::config::{EventConfig, RuntimeConfig};
    use celerity_helpers::runtime_types::{RuntimeCallMode, RuntimePlatform};
    use tracing::Level;

    // -- Helpers --

    fn make_resource(
        resource_type: CelerityResourceType,
        labels: Option<HashMap<String, String>>,
        annotations: Option<HashMap<String, String>>,
        link_selector: Option<BlueprintLinkSelector>,
        spec: CelerityResourceSpec,
    ) -> RuntimeBlueprintResource {
        RuntimeBlueprintResource {
            resource_type,
            metadata: BlueprintResourceMetadata {
                display_name: "".to_string(),
                annotations,
                labels,
            },
            link_selector,
            description: None,
            spec,
        }
    }

    fn make_blueprint(resources: Vec<(String, RuntimeBlueprintResource)>) -> BlueprintConfig {
        BlueprintConfig {
            version: "2023-04-20".to_string(),
            transform: None,
            variables: None,
            metadata: None,
            resources: resources.into_iter().collect(),
        }
    }

    fn make_handler_resource(
        labels: Option<HashMap<String, String>>,
        annotations: Option<HashMap<String, String>>,
        handler_name: &str,
        code_location: &str,
    ) -> RuntimeBlueprintResource {
        make_resource(
            CelerityResourceType::CelerityHandler,
            labels,
            annotations,
            None,
            CelerityResourceSpec::Handler(CelerityHandlerSpec {
                handler_name: None,
                code_location: Some(code_location.to_string()),
                handler: handler_name.to_string(),
                runtime: None,
                memory: None,
                timeout: Some(30),
                tracing_enabled: Some(true),
                environment_variables: None,
            }),
        )
    }

    fn make_consumer_resource(
        labels: Option<HashMap<String, String>>,
        annotations: Option<HashMap<String, String>>,
        link_selector: Option<BlueprintLinkSelector>,
        spec: CelerityConsumerSpec,
    ) -> RuntimeBlueprintResource {
        make_resource(
            CelerityResourceType::CelerityConsumer,
            labels,
            annotations,
            link_selector,
            CelerityResourceSpec::Consumer(spec),
        )
    }

    fn make_schedule_resource(
        labels: Option<HashMap<String, String>>,
        annotations: Option<HashMap<String, String>>,
        link_selector: Option<BlueprintLinkSelector>,
        schedule: &str,
    ) -> RuntimeBlueprintResource {
        make_resource(
            CelerityResourceType::CeleritySchedule,
            labels,
            annotations,
            link_selector,
            CelerityResourceSpec::Schedule(CelerityScheduleSpec {
                schedule: schedule.to_string(),
                input: None,
            }),
        )
    }

    fn test_runtime_config() -> RuntimeConfig {
        RuntimeConfig {
            blueprint_config_path: "test.yaml".to_string(),
            runtime_call_mode: RuntimeCallMode::Ffi,
            service_name: "test".to_string(),
            server_port: 8080,
            server_loopback_only: None,
            local_api_port: 8592,
            use_custom_health_check: None,
            trace_otlp_collector_endpoint: "".to_string(),
            runtime_max_diagnostics_level: Level::INFO,
            platform: RuntimePlatform::Local,
            test_mode: true,
            api_resource: None,
            consumer_app: None,
            schedule_app: None,
            resource_store_verify_tls: false,
            resource_store_cache_entry_ttl: 600,
            resource_store_cleanup_interval: 3600,
            client_ip_source: axum_client_ip::ClientIpSource::ConnectInfo,
            log_format: None,
            metrics_enabled: false,
            trace_sample_ratio: 1.0,
        }
    }

    // -- Path conversion tests --

    #[test]
    fn test_to_axum_path() {
        assert_eq!(to_axum_path("/path".to_string()), "/path");
        assert_eq!(to_axum_path("/path/{param}".to_string()), "/path/{param}");
        assert_eq!(to_axum_path("/path/{proxy+}".to_string()), "/path/{*proxy}");
        assert_eq!(
            to_axum_path("/path/{param}/{param2}".to_string()),
            "/path/{param}/{param2}"
        );
    }

    // -- matches_app_filter tests --

    #[test]
    fn test_matches_app_filter_no_filter() {
        let resource = make_resource(
            CelerityResourceType::CelerityConsumer,
            None,
            None,
            None,
            CelerityResourceSpec::NoSpec,
        );
        assert!(matches_app_filter("any-name", &resource, &None));
    }

    #[test]
    fn test_matches_app_filter_by_annotation() {
        let resource = make_resource(
            CelerityResourceType::CelerityConsumer,
            None,
            Some(HashMap::from([(
                "celerity.app".to_string(),
                "payments".to_string(),
            )])),
            None,
            CelerityResourceSpec::NoSpec,
        );
        assert!(matches_app_filter(
            "SomeConsumer",
            &resource,
            &Some("payments".to_string())
        ));
        assert!(!matches_app_filter(
            "SomeConsumer",
            &resource,
            &Some("orders".to_string())
        ));
    }

    #[test]
    fn test_matches_app_filter_by_resource_name() {
        let resource = make_resource(
            CelerityResourceType::CelerityConsumer,
            None,
            None,
            None,
            CelerityResourceSpec::NoSpec,
        );
        assert!(matches_app_filter(
            "MyConsumer",
            &resource,
            &Some("MyConsumer".to_string())
        ));
        assert!(!matches_app_filter(
            "MyConsumer",
            &resource,
            &Some("OtherConsumer".to_string())
        ));
    }

    // -- resolve_consumer_source tests --

    /// Helper to get a consumer resource and spec from a blueprint by name.
    fn get_consumer_from_blueprint<'a>(
        bp: &'a BlueprintConfig,
        name: &str,
    ) -> (&'a RuntimeBlueprintResource, &'a CelerityConsumerSpec) {
        let resource = bp.resources.get(name).expect("consumer not found");
        let spec = match &resource.spec {
            CelerityResourceSpec::Consumer(s) => s,
            _ => panic!("expected consumer spec"),
        };
        (resource, spec)
    }

    #[test]
    fn test_resolve_consumer_source_explicit() {
        let bp = make_blueprint(vec![(
            "Consumer1".to_string(),
            make_consumer_resource(
                None,
                None,
                None,
                CelerityConsumerSpec {
                    source_id: Some("arn:aws:sqs:us-east-1:123456:queue1".to_string()),
                    ..Default::default()
                },
            ),
        )]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "Consumer1");
        let result = resolve_consumer_source("Consumer1", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::ExplicitSourceId(ref id)
                if id == "arn:aws:sqs:us-east-1:123456:queue1"
        ));
    }

    #[test]
    fn test_resolve_consumer_source_external_events() {
        let mut external_events = HashMap::new();
        external_events.insert(
            "dataStream".to_string(),
            ExternalEventConfiguration {
                source_type:
                    celerity_blueprint_config_parser::blueprint::EventSourceType::DataStream,
                source_configuration: EventSourceConfiguration::DataStream(
                    DataStreamSourceConfiguration {
                        data_stream_id: "arn:aws:kinesis:us-east-1:123456:stream/orders"
                            .to_string(),
                        batch_size: Some(100),
                        partial_failures: None,
                        start_from_beginning: Some(true),
                    },
                ),
            },
        );
        let bp = make_blueprint(vec![(
            "Consumer1".to_string(),
            make_consumer_resource(
                None,
                None,
                None,
                CelerityConsumerSpec {
                    external_events: Some(external_events),
                    ..Default::default()
                },
            ),
        )]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "Consumer1");
        let result = resolve_consumer_source("Consumer1", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::ExternalEvents(_)
        ));
    }

    #[test]
    fn test_resolve_consumer_source_linked_queue() {
        let bp = make_blueprint(vec![
            (
                "OrdersQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_consumer_resource(
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    None,
                    CelerityConsumerSpec::default(),
                ),
            ),
        ]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "OrdersConsumer");
        let result = resolve_consumer_source("OrdersConsumer", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::LinkedQueue { ref queue_resource_name }
                if queue_resource_name == "OrdersQueue"
        ));
    }

    #[test]
    fn test_resolve_consumer_source_linked_datastore() {
        let bp = make_blueprint(vec![
            (
                "OrdersDatastore".to_string(),
                make_resource(
                    CelerityResourceType::CelerityDatastore,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Datastore(CelerityDatastoreSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_consumer_resource(
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    None,
                    CelerityConsumerSpec::default(),
                ),
            ),
        ]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "OrdersConsumer");
        let result = resolve_consumer_source("OrdersConsumer", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::LinkedDatastore { ref datastore_resource_name }
                if datastore_resource_name == "OrdersDatastore"
        ));
    }

    #[test]
    fn test_resolve_consumer_source_linked_bucket() {
        let bp = make_blueprint(vec![
            (
                "UploadsBucket".to_string(),
                make_resource(
                    CelerityResourceType::CelerityBucket,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "uploads".to_string())]),
                    }),
                    CelerityResourceSpec::Bucket(CelerityBucketSpec::default()),
                ),
            ),
            (
                "UploadsConsumer".to_string(),
                make_consumer_resource(
                    Some(HashMap::from([("app".to_string(), "uploads".to_string())])),
                    None,
                    None,
                    CelerityConsumerSpec::default(),
                ),
            ),
        ]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "UploadsConsumer");
        let result = resolve_consumer_source("UploadsConsumer", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::LinkedBucket { ref bucket_resource_name }
                if bucket_resource_name == "UploadsBucket"
        ));
    }

    #[test]
    fn test_resolve_consumer_source_disambiguation() {
        let bp = make_blueprint(vec![
            (
                "StandardQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "PriorityQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_consumer_resource(
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    Some(HashMap::from([(
                        "celerity.consumer.queue".to_string(),
                        "PriorityQueue".to_string(),
                    )])),
                    None,
                    CelerityConsumerSpec::default(),
                ),
            ),
        ]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "OrdersConsumer");
        let result = resolve_consumer_source("OrdersConsumer", resource, spec, &bp).unwrap();
        assert!(matches!(
            result,
            ConsumerSourceResolution::LinkedQueue { ref queue_resource_name }
                if queue_resource_name == "PriorityQueue"
        ));
    }

    #[test]
    fn test_resolve_consumer_source_multiple_types_error() {
        let bp = make_blueprint(vec![
            (
                "OrdersQueue".to_string(),
                make_resource(
                    CelerityResourceType::CelerityQueue,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
                ),
            ),
            (
                "OrdersDatastore".to_string(),
                make_resource(
                    CelerityResourceType::CelerityDatastore,
                    None,
                    None,
                    Some(BlueprintLinkSelector {
                        by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
                    }),
                    CelerityResourceSpec::Datastore(CelerityDatastoreSpec::default()),
                ),
            ),
            (
                "OrdersConsumer".to_string(),
                make_consumer_resource(
                    Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                    None,
                    None,
                    CelerityConsumerSpec::default(),
                ),
            ),
        ]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "OrdersConsumer");
        let result = resolve_consumer_source("OrdersConsumer", resource, spec, &bp);
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), ConfigError::Consumer(_)));
    }

    #[test]
    fn test_resolve_consumer_source_no_source() {
        let bp = make_blueprint(vec![(
            "OrdersConsumer".to_string(),
            make_consumer_resource(
                Some(HashMap::from([("app".to_string(), "orders".to_string())])),
                None,
                None,
                CelerityConsumerSpec::default(),
            ),
        )]);
        let (resource, spec) = get_consumer_from_blueprint(&bp, "OrdersConsumer");
        let result = resolve_consumer_source("OrdersConsumer", resource, spec, &bp).unwrap();
        assert!(matches!(result, ConsumerSourceResolution::NoSource));
    }

    // -- collect_consumer_config tests --

    #[test]
    fn test_collect_consumer_config_single_consumer() {
        let consumer = make_consumer_resource(
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                source_id: Some("arn:aws:sqs:us-east-1:123456:queue1".to_string()),
                batch_size: Some(10),
                partial_failures: Some(true),
                ..Default::default()
            },
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("OrdersConsumer".to_string(), consumer),
            ("ProcessOrder".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_consumer_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let consumers = result.unwrap();
        assert_eq!(consumers.consumers.len(), 1);
        assert_eq!(
            consumers.consumers[0].source_id,
            "arn:aws:sqs:us-east-1:123456:queue1"
        );
        assert_eq!(consumers.consumers[0].batch_size, Some(10));
        assert_eq!(consumers.consumers[0].handlers.len(), 1);
        assert_eq!(consumers.consumers[0].handlers[0].name, "ProcessOrder");
        assert_eq!(consumers.consumers[0].handlers[0].handler, "process_order");
        assert!(collected.contains(&"ProcessOrder".to_string()));
    }

    #[test]
    fn test_collect_consumer_config_linked_from_queue() {
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                batch_size: Some(5),
                ..Default::default()
            },
        );
        let queue = make_resource(
            CelerityResourceType::CelerityQueue,
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("OrdersQueue".to_string(), queue),
            ("OrdersConsumer".to_string(), consumer),
            ("ProcessOrder".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_consumer_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let consumers = result.unwrap();
        assert_eq!(consumers.consumers.len(), 1);
        assert_eq!(consumers.consumers[0].source_id, "OrdersQueue");
        assert_eq!(consumers.consumers[0].batch_size, Some(5));
    }

    #[test]
    fn test_collect_consumer_config_filters_by_consumer_app() {
        let consumer1 = make_consumer_resource(
            None,
            Some(HashMap::from([(
                "celerity.app".to_string(),
                "payments".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "pay".to_string())]),
            }),
            CelerityConsumerSpec {
                source_id: Some("queue-1".to_string()),
                ..Default::default()
            },
        );
        let handler1 = make_handler_resource(
            Some(HashMap::from([("handler".to_string(), "pay".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_payment",
            "./handlers/payments",
        );
        let consumer2 = make_consumer_resource(
            None,
            Some(HashMap::from([(
                "celerity.app".to_string(),
                "orders".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "ord".to_string())]),
            }),
            CelerityConsumerSpec {
                source_id: Some("queue-2".to_string()),
                ..Default::default()
            },
        );
        let handler2 = make_handler_resource(
            Some(HashMap::from([("handler".to_string(), "ord".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("PaymentsConsumer".to_string(), consumer1),
            ("PayHandler".to_string(), handler1),
            ("OrdersConsumer".to_string(), consumer2),
            ("OrderHandler".to_string(), handler2),
        ]);
        let mut runtime_config = test_runtime_config();
        runtime_config.consumer_app = Some("payments".to_string());
        let mut collected = Vec::new();

        let result = collect_consumer_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let consumers = result.unwrap();
        assert_eq!(consumers.consumers.len(), 1);
        assert_eq!(consumers.consumers[0].source_id, "queue-1");
    }

    #[test]
    fn test_collect_consumer_config_no_consumers() {
        let bp = make_blueprint(vec![]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_consumer_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_none());
    }

    // -- collect_events_config tests --

    #[test]
    fn test_collect_events_config_data_stream() {
        let mut external_events = HashMap::new();
        external_events.insert(
            "dataStream".to_string(),
            ExternalEventConfiguration {
                source_type:
                    celerity_blueprint_config_parser::blueprint::EventSourceType::DataStream,
                source_configuration: EventSourceConfiguration::DataStream(
                    DataStreamSourceConfiguration {
                        data_stream_id: "arn:aws:kinesis:us-east-1:123456:stream/orders"
                            .to_string(),
                        batch_size: Some(100),
                        partial_failures: None,
                        start_from_beginning: Some(true),
                    },
                ),
            },
        );
        let consumer = make_consumer_resource(
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                external_events: Some(external_events),
                ..Default::default()
            },
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("StreamConsumer".to_string(), consumer),
            ("ProcessOrder".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let events = result.unwrap();
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::Stream(stream) => {
                assert_eq!(
                    stream.stream_id,
                    "arn:aws:kinesis:us-east-1:123456:stream/orders"
                );
                assert_eq!(stream.batch_size, Some(100));
                assert_eq!(stream.start_from_beginning, Some(true));
                assert_eq!(stream.handlers.len(), 1);
            }
            _ => panic!("expected Stream event config"),
        }
    }

    #[test]
    fn test_collect_events_config_db_stream() {
        let mut external_events = HashMap::new();
        external_events.insert(
            "dbStream".to_string(),
            ExternalEventConfiguration {
                source_type:
                    celerity_blueprint_config_parser::blueprint::EventSourceType::DatabaseStream,
                source_configuration: EventSourceConfiguration::DatabaseStream(
                    DatabaseStreamSourceConfiguration {
                        db_stream_id: "arn:aws:dynamodb:us-east-1:123456:table/orders/stream"
                            .to_string(),
                        batch_size: Some(50),
                        partial_failures: Some(true),
                        start_from_beginning: Some(false),
                    },
                ),
            },
        );
        let consumer = make_consumer_resource(
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                external_events: Some(external_events),
                ..Default::default()
            },
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_stream",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("DbStreamConsumer".to_string(), consumer),
            ("StreamHandler".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let events = result.unwrap();
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::Stream(stream) => {
                assert_eq!(
                    stream.stream_id,
                    "arn:aws:dynamodb:us-east-1:123456:table/orders/stream"
                );
                assert_eq!(stream.start_from_beginning, Some(false));
            }
            _ => panic!("expected Stream event config"),
        }
    }

    #[test]
    fn test_collect_events_config_linked_from_datastore() {
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            Some(HashMap::from([(
                "celerity.consumer.datastore.startFromBeginning".to_string(),
                "true".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                batch_size: Some(25),
                ..Default::default()
            },
        );
        let datastore = make_resource(
            CelerityResourceType::CelerityDatastore,
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityResourceSpec::Datastore(CelerityDatastoreSpec::default()),
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_change",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("OrdersDatastore".to_string(), datastore),
            ("OrdersConsumer".to_string(), consumer),
            ("ChangeHandler".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let events = result.unwrap();
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::Stream(stream) => {
                assert_eq!(stream.stream_id, "OrdersDatastore");
                assert_eq!(stream.batch_size, Some(25));
                assert_eq!(stream.start_from_beginning, Some(true));
                assert_eq!(stream.handlers.len(), 1);
            }
            _ => panic!("expected Stream event config"),
        }
    }

    #[test]
    fn test_collect_events_config_linked_from_bucket() {
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "uploads".to_string())])),
            Some(HashMap::from([(
                "celerity.consumer.bucket.events".to_string(),
                "created,deleted".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "uploads".to_string())]),
            }),
            CelerityConsumerSpec::default(),
        );
        let bucket = make_resource(
            CelerityResourceType::CelerityBucket,
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "uploads".to_string())]),
            }),
            CelerityResourceSpec::Bucket(CelerityBucketSpec::default()),
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "uploads".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_upload",
            "./handlers/uploads",
        );
        let bp = make_blueprint(vec![
            ("UploadsBucket".to_string(), bucket),
            ("UploadsConsumer".to_string(), consumer),
            ("UploadHandler".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let events = result.unwrap();
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::EventTrigger(trigger) => {
                assert_eq!(trigger.event_type, "created,deleted");
                assert_eq!(trigger.queue_id, "UploadsBucket");
                assert_eq!(trigger.handlers.len(), 1);
            }
            _ => panic!("expected EventTrigger config"),
        }
    }

    // -- stream source type prefix derivation tests --

    #[test]
    fn test_stream_source_type_datastore_produces_correct_prefix() {
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                batch_size: Some(10),
                ..Default::default()
            },
        );
        let datastore = make_resource(
            CelerityResourceType::CelerityDatastore,
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityResourceSpec::Datastore(CelerityDatastoreSpec::default()),
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_change",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("OrdersDatastore".to_string(), datastore),
            ("OrdersConsumer".to_string(), consumer),
            ("ChangeHandler".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        let events = result.expect("expected events config");
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::Stream(stream) => {
                assert_eq!(stream.source_type, StreamSourceType::Datastore);
                // Verify the stream name that create_consumers_for_celerity_local would derive.
                let stream_name = format!("celerity:datastore:{}", stream.stream_id);
                assert_eq!(stream_name, "celerity:datastore:OrdersDatastore");
            }
            _ => panic!("expected Stream event config"),
        }
    }

    #[test]
    fn test_stream_source_type_data_stream_produces_correct_prefix() {
        let mut external_events = HashMap::new();
        external_events.insert(
            "dataStream".to_string(),
            ExternalEventConfiguration {
                source_type:
                    celerity_blueprint_config_parser::blueprint::EventSourceType::DataStream,
                source_configuration: EventSourceConfiguration::DataStream(
                    DataStreamSourceConfiguration {
                        data_stream_id: "orders-stream".to_string(),
                        batch_size: Some(100),
                        partial_failures: None,
                        start_from_beginning: Some(true),
                    },
                ),
            },
        );
        let consumer = make_consumer_resource(
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec {
                external_events: Some(external_events),
                ..Default::default()
            },
        );
        let handler = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "orders".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers/orders",
        );
        let bp = make_blueprint(vec![
            ("StreamConsumer".to_string(), consumer),
            ("ProcessOrder".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_events_config(&bp, &runtime_config, &mut collected).unwrap();
        let events = result.expect("expected events config");
        assert_eq!(events.events.len(), 1);
        match &events.events[0] {
            EventConfig::Stream(stream) => {
                assert_eq!(stream.source_type, StreamSourceType::DataStream);
                // Verify the stream name that create_consumers_for_celerity_local would derive.
                let stream_name = format!("celerity:stream:{}", stream.stream_id);
                assert_eq!(stream_name, "celerity:stream:orders-stream");
            }
            _ => panic!("expected Stream event config"),
        }
    }

    // -- collect_schedule_config tests --

    #[test]
    fn test_collect_schedule_config_single_schedule() {
        let schedule = make_schedule_resource(
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "sync".to_string())]),
            }),
            "rate(1h)",
        );
        let handler = make_handler_resource(
            Some(HashMap::from([("handler".to_string(), "sync".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.schedule".to_string(),
                "true".to_string(),
            )])),
            "sync_orders",
            "./handlers/sync",
        );
        let bp = make_blueprint(vec![
            ("HourlySync".to_string(), schedule),
            ("SyncHandler".to_string(), handler),
        ]);
        let runtime_config = test_runtime_config();
        let mut collected = Vec::new();

        let result = collect_schedule_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let schedules = result.unwrap();
        assert_eq!(schedules.schedules.len(), 1);
        assert_eq!(schedules.schedules[0].schedule_id, "HourlySync");
        assert_eq!(schedules.schedules[0].schedule_value, "rate(1h)");
        assert_eq!(schedules.schedules[0].queue_id, "");
        assert!(schedules.schedules[0].batch_size.is_none());
        assert_eq!(schedules.schedules[0].handlers.len(), 1);
        assert_eq!(schedules.schedules[0].handlers[0].name, "SyncHandler");
        assert!(collected.contains(&"SyncHandler".to_string()));
    }

    #[test]
    fn test_collect_schedule_config_filters_by_schedule_app() {
        let schedule1 = make_schedule_resource(
            None,
            Some(HashMap::from([(
                "celerity.app".to_string(),
                "reports".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "report".to_string())]),
            }),
            "rate(24h)",
        );
        let handler1 = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "report".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.schedule".to_string(),
                "true".to_string(),
            )])),
            "gen_report",
            "./handlers/reports",
        );
        let schedule2 = make_schedule_resource(
            None,
            Some(HashMap::from([(
                "celerity.app".to_string(),
                "cleanup".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("handler".to_string(), "clean".to_string())]),
            }),
            "rate(1h)",
        );
        let handler2 = make_handler_resource(
            Some(HashMap::from([(
                "handler".to_string(),
                "clean".to_string(),
            )])),
            Some(HashMap::from([(
                "celerity.handler.schedule".to_string(),
                "true".to_string(),
            )])),
            "cleanup",
            "./handlers/cleanup",
        );
        let bp = make_blueprint(vec![
            ("DailyReport".to_string(), schedule1),
            ("ReportHandler".to_string(), handler1),
            ("HourlyCleanup".to_string(), schedule2),
            ("CleanupHandler".to_string(), handler2),
        ]);
        let mut runtime_config = test_runtime_config();
        runtime_config.schedule_app = Some("reports".to_string());
        let mut collected = Vec::new();

        let result = collect_schedule_config(&bp, &runtime_config, &mut collected).unwrap();
        assert!(result.is_some());
        let schedules = result.unwrap();
        assert_eq!(schedules.schedules.len(), 1);
        assert_eq!(schedules.schedules[0].schedule_id, "DailyReport");
    }

    // -- collect_custom_handler_definitions tests --

    #[test]
    fn test_collect_custom_handlers() {
        let handler = make_handler_resource(
            None,
            None, // No protocol annotations
            "custom_handler",
            "./handlers/custom",
        );
        let bp = make_blueprint(vec![("CustomHandler".to_string(), handler)]);
        let collected: Vec<String> = Vec::new();

        let result = collect_custom_handler_definitions(&bp, &collected).unwrap();
        assert!(result.is_some());
        let custom = result.unwrap();
        assert_eq!(custom.handlers.len(), 1);
        assert_eq!(custom.handlers[0].name, "CustomHandler");
        assert_eq!(custom.handlers[0].handler, "custom_handler");
        assert_eq!(custom.handlers[0].location, "./handlers/custom");
    }

    #[test]
    fn test_custom_skips_protocol_handlers() {
        let http_handler = make_handler_resource(
            None,
            Some(HashMap::from([(
                "celerity.handler.http".to_string(),
                "true".to_string(),
            )])),
            "get_order",
            "./handlers/orders",
        );
        let custom_handler =
            make_handler_resource(None, None, "custom_handler", "./handlers/custom");
        let bp = make_blueprint(vec![
            ("HttpHandler".to_string(), http_handler),
            ("CustomHandler".to_string(), custom_handler),
        ]);
        let collected: Vec<String> = Vec::new();

        let result = collect_custom_handler_definitions(&bp, &collected).unwrap();
        assert!(result.is_some());
        let custom = result.unwrap();
        assert_eq!(custom.handlers.len(), 1);
        assert_eq!(custom.handlers[0].name, "CustomHandler");
    }

    #[test]
    fn test_collected_handler_names_prevents_duplicates() {
        let handler = make_handler_resource(None, None, "my_handler", "./handlers");
        let bp = make_blueprint(vec![("MyHandler".to_string(), handler)]);
        // Handler already claimed by consumer.
        let collected = vec!["MyHandler".to_string()];

        let result = collect_custom_handler_definitions(&bp, &collected).unwrap();
        assert!(result.is_none());
    }

    // -- DLQ resolution tests --

    #[test]
    fn test_collect_consumer_config_queue_with_dlq() {
        // Source queue links to consumer (via app=orders) and also links to a
        // DLQ queue (via role=orders-dlq). The DLQ queue has max attempts annotation.
        let handler = make_handler_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers",
        );
        let consumer = make_consumer_resource(
            // Consumer has both labels so it matches the source queue's linkSelector.
            Some(HashMap::from([
                ("app".to_string(), "orders".to_string()),
                ("role".to_string(), "orders-dlq".to_string()),
            ])),
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec::default(),
        );
        let dlq_queue = make_resource(
            CelerityResourceType::CelerityQueue,
            // DLQ queue has labels that match the source queue's linkSelector.
            Some(HashMap::from([
                ("app".to_string(), "orders".to_string()),
                ("role".to_string(), "orders-dlq".to_string()),
            ])),
            Some(HashMap::from([(
                "celerity.queue.deadLetterMaxAttempts".to_string(),
                "5".to_string(),
            )])),
            None,
            CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
        );
        // Source queue links to resources with {app=orders, role=orders-dlq} —
        // matches both the consumer and the DLQ queue.
        let source_queue = make_resource(
            CelerityResourceType::CelerityQueue,
            None,
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([
                    ("app".to_string(), "orders".to_string()),
                    ("role".to_string(), "orders-dlq".to_string()),
                ]),
            }),
            CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
        );
        let bp = make_blueprint(vec![
            ("OrdersHandler".to_string(), handler),
            ("OrdersConsumer".to_string(), consumer),
            ("OrdersQueue".to_string(), source_queue),
            ("OrdersDLQ".to_string(), dlq_queue),
        ]);

        let mut collected = Vec::new();
        let result = collect_consumer_config(&bp, &test_runtime_config(), &mut collected).unwrap();
        let consumers_config = result.expect("should have consumers");
        assert_eq!(consumers_config.consumers.len(), 1);
        let config = &consumers_config.consumers[0];
        assert_eq!(config.dlq_source_id.as_deref(), Some("OrdersDLQ"));
        assert_eq!(config.max_retries, Some(5));
    }

    #[test]
    fn test_collect_consumer_config_queue_without_dlq() {
        // Source queue does not link to any DLQ queue.
        let handler = make_handler_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_order",
            "./handlers",
        );
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "orders".to_string())])),
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityConsumerSpec::default(),
        );
        let source_queue = make_resource(
            CelerityResourceType::CelerityQueue,
            None,
            None,
            // Links to consumer but not to any DLQ queue.
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "orders".to_string())]),
            }),
            CelerityResourceSpec::Queue(CelerityQueueSpec::default()),
        );
        let bp = make_blueprint(vec![
            ("OrdersHandler".to_string(), handler),
            ("OrdersConsumer".to_string(), consumer),
            ("OrdersQueue".to_string(), source_queue),
        ]);

        let mut collected = Vec::new();
        let result = collect_consumer_config(&bp, &test_runtime_config(), &mut collected).unwrap();
        let consumers_config = result.expect("should have consumers");
        assert_eq!(consumers_config.consumers.len(), 1);
        let config = &consumers_config.consumers[0];
        assert!(config.dlq_source_id.is_none());
        assert!(config.max_retries.is_none());
    }

    #[test]
    fn test_collect_consumer_config_topic_dlq_default_enabled() {
        // Topic consumer with no DLQ annotation — defaults to true.
        let handler = make_handler_resource(
            Some(HashMap::from([("app".to_string(), "events".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_event",
            "./handlers",
        );
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "events".to_string())])),
            None,
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "events".to_string())]),
            }),
            CelerityConsumerSpec {
                source_id: Some("celerity::topic::my-topic".to_string()),
                ..Default::default()
            },
        );
        let bp = make_blueprint(vec![
            ("EventsHandler".to_string(), handler),
            ("EventsConsumer".to_string(), consumer),
        ]);

        let mut collected = Vec::new();
        let result = collect_consumer_config(&bp, &test_runtime_config(), &mut collected).unwrap();
        let consumers_config = result.expect("should have consumers");
        assert_eq!(consumers_config.consumers.len(), 1);
        let config = &consumers_config.consumers[0];
        // DLQ source ID is the topic name (prefix stripped).
        assert_eq!(config.dlq_source_id.as_deref(), Some("my-topic"));
        assert!(config.max_retries.is_none());
    }

    #[test]
    fn test_collect_consumer_config_topic_dlq_disabled() {
        // Topic consumer with DLQ explicitly disabled.
        let handler = make_handler_resource(
            Some(HashMap::from([("app".to_string(), "events".to_string())])),
            Some(HashMap::from([(
                "celerity.handler.consumer".to_string(),
                "true".to_string(),
            )])),
            "process_event",
            "./handlers",
        );
        let consumer = make_consumer_resource(
            Some(HashMap::from([("app".to_string(), "events".to_string())])),
            Some(HashMap::from([(
                "celerity.consumer.deadLetterQueue".to_string(),
                "false".to_string(),
            )])),
            Some(BlueprintLinkSelector {
                by_label: HashMap::from([("app".to_string(), "events".to_string())]),
            }),
            CelerityConsumerSpec {
                source_id: Some("celerity::topic::my-topic".to_string()),
                ..Default::default()
            },
        );
        let bp = make_blueprint(vec![
            ("EventsHandler".to_string(), handler),
            ("EventsConsumer".to_string(), consumer),
        ]);

        let mut collected = Vec::new();
        let result = collect_consumer_config(&bp, &test_runtime_config(), &mut collected).unwrap();
        let consumers_config = result.expect("should have consumers");
        assert_eq!(consumers_config.consumers.len(), 1);
        let config = &consumers_config.consumers[0];
        assert!(config.dlq_source_id.is_none());
        assert!(config.max_retries.is_none());
    }
}
