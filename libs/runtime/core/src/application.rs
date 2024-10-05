use std::{
    collections::{HashMap, VecDeque},
    net::SocketAddr,
    sync::{Arc, Mutex},
};

use axum::{
    extract::{MatchedPath, Request},
    handler::Handler,
    middleware,
    routing::{get, post},
    Json, Router,
};
use axum_client_ip::SecureClientIpSource;
use celerity_blueprint_config_parser::{blueprint::BlueprintConfig, parse::BlueprintParseError};
use celerity_helpers::runtime_types::{HealthCheckResponse, RuntimeCallMode};
use tokio::{net::TcpListener, task::JoinHandle};
use tower_http::trace::TraceLayer;
use tracing::{debug, info_span, warn};

use crate::{
    config::{ApiConfig, AppConfig, RuntimeConfig, WebSocketConfig},
    consts::DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
    errors::{ApplicationStartError, ConfigError},
    request::request_id,
    runtime_local_api::create_runtime_local_api,
    telemetry::{self, enrich_span, log_request},
    transform_config::collect_api_config,
    types::{ApiAppState, EventTuple},
    utils::get_epoch_seconds,
    websocket,
    wsconn_registry::{WebSocketConnRegistry, WebSocketRegistrySend},
};

/// Provides an application that can run a HTTP server, WebSocket server,
/// queue/message broker consumer or a hybrid app that combines any of the
/// above.
pub struct Application {
    runtime_config: RuntimeConfig,
    app_tracing_enabled: bool,
    http_server_app: Option<Router<ApiAppState>>,
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
            app_tracing_enabled: false,
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
                self.http_server_app = Some(self.setup_http_server_app(&api_config)?);
                app_config.api = Some(api_config);
            }
            Err(ConfigError::ApiMissing) => (),
            Err(err) => return Err(ApplicationStartError::Config(err)),
        }
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Http {
            self.runtime_local_api = Some(self.setup_runtime_local_api(&app_config)?);
        }
        Ok(app_config)
    }

    fn setup_http_server_app(
        &mut self,
        api_config: &ApiConfig,
    ) -> Result<Router<ApiAppState>, ApplicationStartError> {
        self.app_tracing_enabled = api_config.tracing_enabled;

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

        if let Some(websocket_config) = &api_config.websocket {
            let websocket_base_path = resolve_websocket_base_path(api_config, websocket_config)?;
            let conn_registry = Arc::new(WebSocketConnRegistry::new(None));
            self.ws_connections = Some(conn_registry.clone());
            http_server_app = http_server_app.route(
                websocket_base_path,
                get(websocket::handler).with_state(websocket::WebSocketAppState {
                    connections: conn_registry,
                    routes: Arc::new(HashMap::new()),
                    route_key: websocket_config.route_key.clone(),
                }),
            );
        }

        Ok(http_server_app)
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
        // Tracing setup is the in `run` instead of `setup` because
        // we need to be in an async context (tokio runtime) in order to set up tracing.
        if self.app_tracing_enabled {
            telemetry::setup_tracing(&self.runtime_config)?;
        }

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

    async fn run_http_server_app(
        &mut self,
        http_app: Router<ApiAppState>,
    ) -> (JoinHandle<()>, SocketAddr) {
        // Attach layers at the run phase instead of the set up phase as we need to attach the tracing
        // layer after the tokio runtime has been started and tracing has been initialised.
        // We also need to make sure the tracing layers are attached first so that layers such as the client IP
        // extractor run first and extracted data can be added to the current span.

        let api_app_state = ApiAppState {
            platform: self.runtime_config.platform.clone(),
        };
        let http_app = http_app.layer(middleware::from_fn(log_request));
        let final_http_app =
            attach_tracing_layers(http_app, api_app_state.clone(), self.app_tracing_enabled)
                .layer(SecureClientIpSource::ConnectInfo.into_extension())
                .layer(middleware::from_fn(request_id))
                .with_state(api_app_state);

        let port = self.runtime_config.server_port;
        let host = if self.runtime_config.server_loopback_only.unwrap_or(true) {
            "127.0.0.1"
        } else {
            "0.0.0.0"
        };

        debug!("binding listener");
        let listener = TcpListener::bind(format!("{host}:{port}")).await.unwrap();
        let listener_addr = listener.local_addr().unwrap();
        debug!("spawning server");
        let (tx, rx) = tokio::sync::oneshot::channel::<()>();
        let task = tokio::spawn(async move {
            axum::serve(
                listener,
                // Ensure to capture `ConnectInfo` to feed into the client IP extractor
                // when not behind a proxy.
                final_http_app.into_make_service_with_connect_info::<SocketAddr>(),
            )
            .with_graceful_shutdown(async {
                rx.await.ok();
            })
            .await
            .unwrap();
        });
        debug!("server spawned");
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
        handler: impl Handler<T, ApiAppState>,
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

fn attach_tracing_layers(
    http_app: Router<ApiAppState>,
    api_app_state: ApiAppState,
    tracing_enabled: bool,
) -> Router<ApiAppState> {
    if !tracing_enabled {
        return http_app;
    }

    http_app
        .layer(middleware::from_fn_with_state(api_app_state, enrich_span))
        .layer(
            TraceLayer::new_for_http().make_span_with(|request: &Request<_>| {
                let matched_path = request
                    .extensions()
                    .get::<MatchedPath>()
                    .map(MatchedPath::as_str);

                info_span!(
                    "http_request",
                    method = ?request.method(),
                    matched_path,
                    original_uri = ?request.uri(),
                    trace_id = tracing::field::Empty,
                    client_ip = tracing::field::Empty,
                    connection_id = tracing::field::Empty,
                    request_id = tracing::field::Empty,
                    // AWS X-Ray trace ID is only recorded for the AWS platform,
                    // but needs to be defined in span creation so it can be
                    // recorded later.
                    xray_trace_id = tracing::field::Empty,
                    user_agent = tracing::field::Empty,
                )
            }),
        )
}

fn resolve_websocket_base_path<'a>(
    api_config: &'a ApiConfig,
    websocket_config: &'a WebSocketConfig,
) -> Result<&'a str, ApplicationStartError> {
    if websocket_config.base_paths.is_empty() && api_config.http.is_some() {
        return Err(ApplicationStartError::Config(ConfigError::Api(
            "A WebSocket-specific base path must be defined for a hybrid API \
            that provides a WebSocket and HTTP interface"
                .to_string(),
        )));
    }

    if websocket_config.base_paths.len() > 1 {
        warn!(
            "Multiple WebSocket base paths are not supported by the runtime, \
         only the first one will be used"
        );
    }

    if websocket_config.base_paths.is_empty() {
        Ok("/")
    } else {
        Ok(websocket_config.base_paths[0].as_str())
    }
}

#[derive(Debug)]
pub struct AppInfo {
    pub http_server_address: Option<SocketAddr>,
}
