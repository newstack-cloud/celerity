use std::cmp::max;

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintMetadata, BlueprintScalarValue, CelerityApiProtocol,
    CelerityHandlerSpec, CelerityResourceSpec, CelerityResourceType,
};
use celerity_helpers::blueprint::{select_resources, ResourceWithName};

use crate::{
    config::{
        ApiConfig, HttpConfig, HttpHandlerDefinition, WebSocketConfig, WebSocketHandlerDefinition,
    },
    consts::{
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME, CELERITY_HTTP_METHOD_ANNOTATION_NAME,
        CELERITY_HTTP_PATH_ANNOTATION_NAME, CELERITY_WS_HANDLER_ANNOTATION_NAME,
        CELERITY_WS_ROUTE_ANNOTATION_NAME, DEFAULT_HANDLER_TIMEOUT, DEFAULT_TRACING_ENABLED,
        DEFAULT_WEBSOCKET_API_ROUTE_KEY, MAX_HANDLER_TIMEOUT,
    },
    errors::ConfigError,
};

pub(crate) fn collect_api_config(
    blueprint_config: BlueprintConfig,
) -> Result<ApiConfig, ConfigError> {
    let mut api_config = ApiConfig {
        http: None,
        websocket: None,
        auth: None,
        cors: None,
        tracing_enabled: false,
    };
    // Find the first API resource in the blueprint.
    let (_, api) = blueprint_config
        .resources
        .iter()
        .find(|(_, resource)| resource.resource_type == CelerityResourceType::CelerityApi)
        .ok_or_else(|| ConfigError::ApiMissing)?;

    let target_handlers = select_resources(
        &api.link_selector,
        &blueprint_config,
        CelerityResourceType::CelerityHandler,
    );

    let mut collected_handler_names: Vec<String> = Vec::new();

    let http_handlers = collect_http_handler_definitions(
        &target_handlers,
        &blueprint_config,
        &mut collected_handler_names,
    )?;

    if http_handlers.len() > 0 {
        api_config.http = Some(HttpConfig {
            handlers: http_handlers,
            base_paths: vec![],
        });
    }

    let ws_handlers = collect_ws_handler_definitions(
        &target_handlers,
        &blueprint_config,
        &api.spec,
        &mut collected_handler_names,
    )?;

    if ws_handlers.len() > 0 {
        api_config.websocket = Some(WebSocketConfig {
            handlers: ws_handlers,
            base_paths: vec![],
            route_key: DEFAULT_WEBSOCKET_API_ROUTE_KEY.to_string(),
        });
    }

    api_config.tracing_enabled = resolve_api_tracing_enabled(&api.spec);

    Ok(api_config)
}

fn collect_http_handler_definitions(
    target_handlers: &Vec<ResourceWithName>,
    blueprint_config: &BlueprintConfig,
    collected_handler_names: &mut Vec<String>,
) -> Result<Vec<HttpHandlerDefinition>, ConfigError> {
    let mut http_handlers = Vec::new();

    for handler in target_handlers {
        if let Some(annotations) = &handler.resource.metadata.annotations {
            if let Some(scalar_value) = annotations.get(CELERITY_HTTP_HANDLER_ANNOTATION_NAME) {
                if let BlueprintScalarValue::Bool(http_enabled) = scalar_value {
                    if *http_enabled {
                        check_handler_already_collected(&handler.name, collected_handler_names)?;

                        // Get http-specific annotations and push to http handlers list.
                        let method = annotations
                            .get(CELERITY_HTTP_METHOD_ANNOTATION_NAME)
                            .map(|method| method.to_string())
                            .unwrap_or_else(|| "GET".to_string());
                        let path = annotations
                            .get(CELERITY_HTTP_PATH_ANNOTATION_NAME)
                            .map(|path| path.to_string())
                            .unwrap_or_else(|| "/".to_string());

                        collect_http_handler_definition(
                            handler,
                            method,
                            path,
                            blueprint_config,
                            &mut http_handlers,
                            collected_handler_names,
                        )?;
                    }
                }
            }
        }
    }

    Ok(http_handlers)
}

fn collect_http_handler_definition(
    handler: &ResourceWithName,
    method: String,
    path: String,
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
            method,
            path,
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
            if let Some(scalar_value) = annotations.get(CELERITY_WS_HANDLER_ANNOTATION_NAME) {
                if let BlueprintScalarValue::Bool(ws_enabled) = scalar_value {
                    if *ws_enabled {
                        check_handler_already_collected(&handler.name, collected_handler_names)?;

                        // Get websocket-specific annotations and push to websocket handlers list.
                        let route = annotations
                            .get(CELERITY_WS_ROUTE_ANNOTATION_NAME)
                            .map(|method| method.to_string())
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
                CelerityApiProtocol::WebSocketConfig(config) => match config.route_key.clone() {
                    Some(route_key) => route_key.clone(),
                    None => DEFAULT_WEBSOCKET_API_ROUTE_KEY.to_string(),
                },
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
    collected_handler_names: &Vec<String>,
) -> Result<(), ConfigError> {
    if collected_handler_names.contains(handler_name) {
        return Err(ConfigError::Api(format!(
            "handler {} is configured for multiple kinds of applications, \
            a handler can only be configured for one kind of application \
            (e.g. HTTP, WebSocket, Queue Consumer etc.)",
            handler_name
        )));
    }
    Ok(())
}

fn apply_http_handler_configurations(
    handler_name: String,
    handler_spec: &CelerityHandlerSpec,
    handler_configs: Vec<ResourceWithName>,
    blueprint_metadata: Option<&BlueprintMetadata>,
    method: String,
    path: String,
) -> Result<HttpHandlerDefinition, ConfigError> {
    let mut handler_definition = HttpHandlerDefinition::default();
    handler_definition.handler = handler_name.clone();
    handler_definition.path = to_axum_path(path);
    handler_definition.method = method;

    // Handler spec takes precedence, then handler config
    // resources and then global `sharedHandlerConfig`
    // in the blueprint metadata.
    //
    // Only consider the first handler config resource to avoid complexity
    // involved in merging multiple handler configs and determining precedence.
    let handler_config = handler_configs.get(0);

    handler_definition.location = resolve_handler_location(
        handler_name,
        handler_spec,
        handler_config,
        blueprint_metadata,
    )?;

    handler_definition.timeout =
        resolve_handler_timeout(handler_spec, handler_config, blueprint_metadata);

    handler_definition.tracing_enabled =
        resolve_tracing_enabled(handler_spec, handler_config, blueprint_metadata);

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
    let mut handler_definition = WebSocketHandlerDefinition::default();
    handler_definition.handler = handler_name.clone();
    handler_definition.route = route;
    handler_definition.route_key = route_key;

    // Handler spec takes precedence, then handler config
    // resources and then global `sharedHandlerConfig`
    // in the blueprint metadata.
    //
    // Only consider the first handler config resource to avoid complexity
    // involved in merging multiple handler configs and determining precedence.
    let handler_config = handler_configs.get(0);

    handler_definition.location = resolve_handler_location(
        handler_name,
        handler_spec,
        handler_config,
        blueprint_metadata,
    )?;

    handler_definition.timeout =
        resolve_handler_timeout(handler_spec, handler_config, blueprint_metadata);

    handler_definition.tracing_enabled =
        resolve_tracing_enabled(handler_spec, handler_config, blueprint_metadata);

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
            "handler {} is missing code location, define it in the \
            handler spec or one of the supported handler config locations",
            handler_name
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
        .map(|timeout| max(timeout, MAX_HANDLER_TIMEOUT))
        .or_else(|| {
            handler_config
                .and_then(|config| match &config.resource.spec {
                    CelerityResourceSpec::HandlerConfig(handler_config) => handler_config.timeout,
                    _ => None,
                })
                .map(|timeout| max(timeout, MAX_HANDLER_TIMEOUT))
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

// Converts a Celerity path to an Axum path.
// Celerity paths are in the form of `/path/{param1}/{param2}`.
// Axum paths are in the form of `/path/:param1/:param2`.
// Celerity wildcards are of the form `/{param+}`.
// Axum wildcards are of the form `/*param`.
fn to_axum_path(celerity_path: String) -> String {
    celerity_path
        .split('/')
        .map(|part| {
            if part.starts_with('{') && part.ends_with('}') {
                let inner = &part[1..part.len() - 1];
                if inner.ends_with('+') {
                    format!("*{}", &inner[..inner.len() - 1])
                } else {
                    format!(":{}", inner)
                }
            } else {
                part.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("/")
}

#[cfg(test)]
mod tests {
    use super::*;
    use coverage_helper::test;

    #[test]
    fn test_to_axum_path() {
        assert_eq!(to_axum_path("/path".to_string()), "/path");
        assert_eq!(to_axum_path("/path/{param}".to_string()), "/path/:param");
        assert_eq!(to_axum_path("/path/{proxy+}".to_string()), "/path/*proxy");
        assert_eq!(
            to_axum_path("/path/{param}/{param2}".to_string()),
            "/path/:param/:param2"
        );
    }
}
