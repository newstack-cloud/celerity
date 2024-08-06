use std::cmp::max;

use celerity_blueprint_config_parser::blueprint::{
    BlueprintConfig, BlueprintLinkSelector, BlueprintScalarValue, CelerityResourceSpec,
    CelerityResourceType, RuntimeBlueprintResource,
};

use crate::{
    config::{ApiConfig, HttpConfig, HttpHandlerDefinition},
    consts::{
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME, CELERITY_HTTP_METHOD_ANNOTATION_NAME,
        CELERITY_HTTP_PATH_ANNOTATION_NAME, MAX_HANDLER_TIMEOUT,
    },
    errors::ConfigError,
};

pub(crate) fn collect_api_config(
    blueprint_config: BlueprintConfig,
) -> Result<ApiConfig, ConfigError> {
    let mut api_config = ApiConfig {
        http: None,
        websocket: None,
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
                            if let Some(location) = &handler_spec.code_location {
                                candidate_http_handlers.push(HttpHandlerDefinition {
                                    path: to_axum_path(path.clone()),
                                    method,
                                    location: location.clone(),
                                    handler: handler.name,
                                    timeout: handler_spec
                                        .timeout
                                        .map(|timeout| max(timeout, MAX_HANDLER_TIMEOUT))
                                        .unwrap_or(MAX_HANDLER_TIMEOUT),
                                });
                            } else {
                                return Err(ConfigError::Api(format!(
                                    "handler {} is missing location",
                                    handler.name
                                )));
                            }
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
        });
    }

    Ok(api_config)
}

// Converts a Celerity path to an Axum path.
// Celerity paths are in the form of `/path/{param1}/{param2}`.
// Axum paths are in the form of `/path/:param1/:param2`.
// Celerity wildcards are of the form `/{param+}` like with
// Amazon API Gateway. Axum wildcards are of the form `/*param`.
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
        assert_eq!(to_axum_path("/path/{param+}".to_string()), "/path/*param");
        assert_eq!(
            to_axum_path("/path/{param}/{param2}".to_string()),
            "/path/:param/:param2"
        );
    }
}
