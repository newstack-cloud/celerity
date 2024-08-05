use std::net::SocketAddr;

use axum::{
    handler::Handler,
    routing::{get, post},
    Router,
};
use celerity_blueprint_config_parser::{
    blueprint::{
        BlueprintConfig, BlueprintLinkSelector, BlueprintScalarValue, CelerityResourceSpec,
        CelerityResourceType, RuntimeBlueprintResource,
    },
    parse::BlueprintParseError,
};
use tokio::net::TcpListener;

use crate::{
    config::RuntimeConfig,
    consts::{
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME, CELERITY_HTTP_METHOD_ANNOTATION_NAME,
        CELERITY_HTTP_PATH_ANNOTATION_NAME,
    },
    errors::{ApplicationStartError, ConfigError},
};

/// Provides an application that can run a HTTP server, WebSocket server,
/// queue/message broker consumer or a hybrid app that combines any of the
/// above.
pub struct Application {
    runtime_config: RuntimeConfig,
    http_server_app: Option<Router>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
}

impl Application {
    pub fn new(runtime_config: RuntimeConfig) -> Self {
        Application {
            runtime_config,
            http_server_app: None,
            server_shutdown_signal: None,
        }
    }

    pub fn setup(&mut self) -> Result<AppConfig, ApplicationStartError> {
        let blueprint_config = self.load_and_parse_blueprint()?;
        let mut app_config = AppConfig { api: None };
        match collect_api_config(blueprint_config) {
            Ok(api_config) => {
                app_config.api = Some(api_config);
                self.http_server_app = Some(Router::new());
            }
            Err(ConfigError::ApiMissing) => (),
            Err(err) => return Err(ApplicationStartError::Config(err)),
        }
        Ok(app_config)
    }

    pub async fn run(&mut self) -> Result<AppInfo, ApplicationStartError> {
        if let Some(http_app_unwrapped) = self.http_server_app.clone() {
            println!("About to bind listener!");
            let port = self.runtime_config.server_port;
            let host = if self.runtime_config.server_loopback_only.unwrap_or(true) {
                "127.0.0.1"
            } else {
                "0.0.0.0"
            };
            let listener = TcpListener::bind(format!("{host}:{port}")).await.unwrap();
            let listener_addr = listener.local_addr().unwrap();
            println!("About to spawn server!");
            let (tx, rx) = tokio::sync::oneshot::channel::<()>();
            tokio::spawn(async move {
                axum::serve(listener, http_app_unwrapped)
                    .with_graceful_shutdown(async {
                        rx.await.ok();
                    })
                    .await
                    .unwrap();
            });
            println!("Server spawned!");
            self.server_shutdown_signal = Some(tx);

            return Ok(AppInfo {
                http_server_address: Some(listener_addr),
            });
        }

        // 2. Determine what kinds of apps to run based on blueprint and env vars.
        //      Can only run one kind of consumer app at a time.
        //      Can run a single HTTP server app.
        //      Can run a single WebSocket server app.
        //      Can run a hybrid app that serves both HTTP and WebSocket.
        // 3. Set up apps with routes and middleware/plugins?!?
        // 4. Start apps in separate tokio tasks.
        Err(ApplicationStartError::Environment(
            "no HTTP server app provided".to_string(),
        ))
    }

    fn load_and_parse_blueprint(&self) -> Result<BlueprintConfig, BlueprintParseError> {
        if self.runtime_config.blueprint_config_path.ends_with(".json") {
            BlueprintConfig::from_json_file(&self.runtime_config.blueprint_config_path)
        } else {
            BlueprintConfig::from_yaml_file(&self.runtime_config.blueprint_config_path)
        }
    }

    pub fn register_http_handler<T>(
        &mut self,
        path: &str,
        method: &str,
        handler: impl Handler<T, ()>,
    ) where
        T: 'static,
    {
        if let Some(http_app) = &self.http_server_app {
            match method.to_lowercase().as_str() {
                "get" => self.http_server_app = Some(http_app.clone().route(path, get(handler))),
                "head" => {
                    self.http_server_app =
                        Some(http_app.clone().route(path, axum::routing::head(handler)))
                }
                "options" => {
                    self.http_server_app = Some(
                        http_app
                            .clone()
                            .route(path, axum::routing::options(handler)),
                    )
                }
                "trace" => {
                    self.http_server_app =
                        Some(http_app.clone().route(path, axum::routing::trace(handler)))
                }
                "post" => self.http_server_app = Some(http_app.clone().route(path, post(handler))),
                "put" => {
                    self.http_server_app =
                        Some(http_app.clone().route(path, axum::routing::put(handler)))
                }
                "patch" => {
                    self.http_server_app =
                        Some(http_app.clone().route(path, axum::routing::patch(handler)))
                }
                "delete" => {
                    self.http_server_app =
                        Some(http_app.clone().route(path, axum::routing::delete(handler)))
                }
                _ => (),
            }
        }
    }

    pub fn shutdown(&mut self) {
        if let Some(tx) = self.server_shutdown_signal.take() {
            tx.send(()).expect("failed to send shutdown signal");
        }
    }
}

#[derive(Debug)]
pub struct AppInfo {
    pub http_server_address: Option<SocketAddr>,
}

#[derive(Debug)]
pub struct HttpHandlerDefinition {
    pub path: String,
    pub method: String,
    pub location: String,
    pub handler: String,
}

#[derive(Debug)]
pub struct AppConfig {
    pub api: Option<ApiConfig>,
}

#[derive(Debug)]
pub struct ApiConfig {
    pub http: Option<HttpConfig>,
    pub websocket: Option<WebsocketConfig>,
}

#[derive(Debug)]
pub struct HttpConfig {
    pub handlers: Vec<HttpHandlerDefinition>,
}

#[derive(Debug)]
pub struct ResourceWithName<'a> {
    pub name: String,
    pub resource: &'a RuntimeBlueprintResource,
}

#[derive(Debug)]
pub struct WebsocketConfig {}

fn collect_api_config(blueprint_config: BlueprintConfig) -> Result<ApiConfig, ConfigError> {
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
                            candidate_http_handlers.push(HttpHandlerDefinition {
                                path: to_axum_path(path.clone()),
                                method,
                                location: handler_spec.code_location.clone(),
                                handler: handler.name,
                            });
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
// Celerity paths are in the form of /path/{param1}/{param2}
// Axum paths are in the form of /path/:param1/:param2
fn to_axum_path(celerity_path: String) -> String {
    celerity_path
        .split('/')
        .map(|part| {
            if part.starts_with('{') && part.ends_with('}') {
                format!(":{}", &part[1..part.len() - 1])
            } else {
                part.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("/")
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
