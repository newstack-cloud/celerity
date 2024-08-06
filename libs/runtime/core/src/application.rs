use std::{cmp::max, net::SocketAddr};

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
    config::{AppConfig, RuntimeConfig},
    consts::{
        CELERITY_HTTP_HANDLER_ANNOTATION_NAME, CELERITY_HTTP_METHOD_ANNOTATION_NAME,
        CELERITY_HTTP_PATH_ANNOTATION_NAME, MAX_HANDLER_TIMEOUT,
    },
    errors::{ApplicationStartError, ConfigError},
    transform_config::collect_api_config,
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
