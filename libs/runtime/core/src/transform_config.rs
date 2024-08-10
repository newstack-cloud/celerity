use std::cmp::max;

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintLinkSelector, BlueprintMetadata, BlueprintScalarValue,
    CelerityHandlerSpec, CelerityResourceSpec, CelerityResourceType, RuntimeBlueprintResource,
};

use crate::{
    config::{ApiConfig, HttpConfig, HttpHandlerDefinition},
    consts::{
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME, CELERITY_HTTP_METHOD_ANNOTATION_NAME,
        CELERITY_HTTP_PATH_ANNOTATION_NAME, DEFAULT_HANDLER_TIMEOUT, DEFAULT_TRACING_ENABLED,
        MAX_HANDLER_TIMEOUT,
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
    )?;

    let mut candidate_http_handlers = Vec::new();
    for handler in target_handlers {
        if let Some(annotations) = &handler.resource.metadata.annotations {
            if let Some(scalar_value) = annotations.get(CELERITY_HTTP_HANDLER_ANNOTATION_NAME) {
                if let BlueprintScalarValue::Bool(http_enabled) = scalar_value {
                    if *http_enabled {
                        // Get http-specific annotations and push to candidate http handlers list.
                        let method = annotations
                            .get(CELERITY_HTTP_METHOD_ANNOTATION_NAME)
                            .map(|method| method.to_string())
                            .unwrap_or_else(|| "GET".to_string());
                        let path = annotations
                            .get(CELERITY_HTTP_PATH_ANNOTATION_NAME)
                            .map(|path| path.to_string())
                            .unwrap_or_else(|| "/".to_string());

                        if let CelerityResourceSpec::Handler(handler_spec) = &handler.resource.spec
                        {
                            let handler_configs = select_resources(
                                &handler.resource.link_selector,
                                &blueprint_config,
                                CelerityResourceType::CelerityHandlerConfig,
                            )?;
                            let handler_definition = apply_http_handler_configurations(
                                handler.name,
                                handler_spec,
                                handler_configs,
                                blueprint_config.metadata.as_ref(),
                                method,
                                path,
                            )?;
                            candidate_http_handlers.push(handler_definition);
                        } else {
                            return Err(ConfigError::Api(format!(
                                "handler {} is missing spec or resource is not a handler",
                                handler.name
                            )));
                        }
                    }
                }
            }
        }
    }

    if candidate_http_handlers.len() > 0 {
        api_config.http = Some(HttpConfig {
            handlers: candidate_http_handlers,
            base_paths: vec![],
        });
    }

    Ok(api_config)
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

#[derive(Debug)]
pub struct ResourceWithName<'a> {
    pub name: String,
    pub resource: &'a RuntimeBlueprintResource,
}

fn select_resources<'a>(
    link_selector: &'a Option<BlueprintLinkSelector>,
    blueprint_config: &'a BlueprintConfig,
    target_type: CelerityResourceType,
) -> Result<Vec<ResourceWithName<'a>>, ConfigError> {
    let mut handlers = Vec::new();
    if let Some(link_selector) = link_selector {
        for (key, value) in &link_selector.by_label {
            let matching_handlers = blueprint_config
                .resources
                .iter()
                .filter(|(_, resource)| {
                    if let Some(labels) = &resource.metadata.labels {
                        labels
                            .get(key)
                            .map(|search_label_val| search_label_val == value)
                            .is_some()
                            && resource.resource_type == target_type
                    } else {
                        false
                    }
                })
                .map(|(name, resource)| ResourceWithName {
                    name: name.clone(),
                    resource,
                })
                .collect::<Vec<_>>();
            handlers.extend(matching_handlers);
        }
    }
    Ok(handlers)
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
