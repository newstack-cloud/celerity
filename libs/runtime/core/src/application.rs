use std::{
    collections::{HashMap, VecDeque},
    net::SocketAddr,
    sync::{Arc, Mutex},
};

use axum::{
    handler::Handler,
    routing::{get, post},
    Json, Router,
};
use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};
use serde::{Deserialize, Serialize};
use tokio::{net::TcpListener, task::JoinHandle};

use crate::{
    config::{AppConfig, RuntimeCallMode, RuntimeConfig},
    consts::DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
    errors::{ApplicationStartError, ConfigError},
    runtime_local_api::create_runtime_local_api,
    transform_config::collect_api_config,
    types::EventTuple,
    utils::get_epoch_seconds,
    wsconn_registry::WebSocketRegistrySend,
};

/// Provides an application that can run a HTTP server, WebSocket server,
/// queue/message broker consumer or a hybrid app that combines any of the
/// above.
pub struct Application {
    runtime_config: RuntimeConfig,
    http_server_app: Option<Router>,
    runtime_local_api: Option<Router>,
    event_queue: Option<Arc<Mutex<VecDeque<EventTuple>>>>,
    processing_events_map: Option<Arc<Mutex<HashMap<String, EventTuple>>>>,
    ws_connections: Option<Arc<dyn WebSocketRegistrySend + 'static>>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    local_api_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
}

impl Application {
    pub fn new(runtime_config: RuntimeConfig) -> Self {
        Application {
            runtime_config,
            http_server_app: None,
            server_shutdown_signal: None,
            runtime_local_api: None,
            local_api_shutdown_signal: None,
            event_queue: None,
            processing_events_map: None,
            ws_connections: None,
        }
    }

    pub fn setup(&mut self) -> Result<AppConfig, ApplicationStartError> {
        let blueprint_config = self.load_and_parse_blueprint()?;
        let mut app_config = AppConfig {
            api: None,
            consumers: None,
            schedules: None,
            events: None,
        };
        match collect_api_config(blueprint_config) {
            Ok(api_config) => {
                app_config.api = Some(api_config);
                self.http_server_app = Some(self.setup_http_server_app());
            }
            Err(ConfigError::ApiMissing) => (),
            Err(err) => return Err(ApplicationStartError::Config(err)),
        }
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Http {
            self.runtime_local_api = Some(self.setup_runtime_local_api(&app_config)?);
        }
        Ok(app_config)
    }

    fn setup_http_server_app(&self) -> Router {
        let mut http_server_app = Router::new();
        let use_custom_health_check = self.runtime_config.use_custom_health_check.unwrap_or(false);
        if !use_custom_health_check {
            http_server_app = http_server_app.route(
                DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
                get(|()| async {
                    Json(HealthCheckResponse {
                        timestamp: get_epoch_seconds(),
                    })
                }),
            );
        }
        http_server_app
    }

    fn setup_runtime_local_api(
        &mut self,
        app_config: &AppConfig,
    ) -> Result<Router, ApplicationStartError> {
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        self.event_queue = Some(event_queue.clone());
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        self.processing_events_map = Some(processing_events_map.clone());
        create_runtime_local_api(
            app_config,
            event_queue,
            processing_events_map,
            self.ws_connections.clone(),
        )
    }

    pub async fn run(&mut self, block: bool) -> Result<AppInfo, ApplicationStartError> {
        let mut server_task = None;
        let mut local_api_task = None;
        let mut server_address = None;
        if let Some(http_app_unwrapped) = self.http_server_app.clone() {
            let (task, addr) = self.run_http_server_app(http_app_unwrapped).await;
            server_task = Some(task);
            server_address = Some(addr);
        }

        if let Some(runtime_local_api_unwrapped) = self.runtime_local_api.clone() {
            let task = self
                .start_runtime_local_api(runtime_local_api_unwrapped)
                .await;
            local_api_task = Some(task);
        }

        if block {
            if let Some(task) = server_task {
                task.await?;
            }
            if let Some(task) = local_api_task {
                task.await?;
            }
        }

        // 2. Determine what kinds of apps to run based on blueprint and env vars.
        //      Can only run one kind of consumer app at a time.
        //      Can run a single HTTP server app.
        //      Can run a single WebSocket server app.
        //      Can run a hybrid app that serves both HTTP and WebSocket.
        // 3. Set up apps with routes and middleware/plugins?!?
        // 4. Start apps in separate tokio tasks.
        Ok(AppInfo {
            http_server_address: server_address,
        })
    }

    async fn start_runtime_local_api(&mut self, runtime_local_api: Router) -> JoinHandle<()> {
        let port = self.runtime_config.local_api_port;
        // Bind on loopback only as this API must not be exposed to the outside world.
        let listener = TcpListener::bind(format!("127.0.0.1:{port}"))
            .await
            .unwrap();
        let (tx, rx) = tokio::sync::oneshot::channel::<()>();
        let task = tokio::spawn(async move {
            axum::serve(listener, runtime_local_api)
                .with_graceful_shutdown(async {
                    rx.await.ok();
                })
                .await
                .unwrap();
        });
        self.local_api_shutdown_signal = Some(tx);
        task
    }

    async fn run_http_server_app(&mut self, http_app: Router) -> (JoinHandle<()>, SocketAddr) {
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
        let task = tokio::spawn(async move {
            axum::serve(listener, http_app)
                .with_graceful_shutdown(async {
                    rx.await.ok();
                })
                .await
                .unwrap();
        });
        println!("Server spawned!");
        self.server_shutdown_signal = Some(tx);
        (task, listener_addr)
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
            tx.send(())
                .expect("failed to send shutdown signal to http server");
        }
        if let Some(tx) = self.local_api_shutdown_signal.take() {
            tx.send(())
                .expect("failed to send shutdown signal to local api server");
        }
    }
}

#[derive(Debug)]
pub struct AppInfo {
    pub http_server_address: Option<SocketAddr>,
}

#[derive(Deserialize, Serialize)]
pub struct HealthCheckResponse {
    pub timestamp: u64,
}
