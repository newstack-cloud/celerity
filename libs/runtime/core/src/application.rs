use std::{
    collections::{HashMap, VecDeque},
    fmt::Display,
    net::SocketAddr,
    sync::{Arc, Mutex},
    time::Duration,
};

use async_trait::async_trait;
use axum::{
    extract::{MatchedPath, Request},
    handler::Handler,
    middleware,
    routing::{get, post},
    Json, Router,
};
use axum_client_ip::ClientIpSource;
use celerity_blueprint_config_parser::{
    blueprint::{BlueprintConfig, CelerityApiBasePath, CelerityApiProtocol},
    parse::BlueprintParseError,
};
use celerity_helpers::{
    env::EnvVars,
    http::ResourceStore,
    runtime_types::{HealthCheckResponse, RuntimeCallMode},
};
use celerity_ws_registry::{
    errors::WebSocketConnError,
    registry::{
        SendContext, WebSocketConnRegistry, WebSocketConnRegistryConfig, WebSocketRegistrySend,
    },
    types::{AckWorkerConfig, MessageType},
};
use reqwest::Client;
use tokio::{net::TcpListener, sync::Mutex as AsyncMutex, task::JoinHandle};
use tower_http::trace::TraceLayer;
use tracing::{debug, info, info_span, warn};

use crate::{
    auth_custom::AuthGuardHandler,
    config::{ApiConfig, AppConfig, RuntimeConfig, WebSocketConfig},
    consts::DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
    errors::{ApplicationStartError, ConfigError},
    request::request_id,
    runtime_local_api::create_runtime_local_api,
    telemetry::{self, enrich_span, log_request},
    transform_config::collect_api_config,
    types::{ApiAppState, EventTuple},
    utils::get_epoch_seconds,
    websocket::{self, WebSocketMessageHandler},
};

/// Provides an application that can run a HTTP server, WebSocket server,
/// queue/message broker consumer or a hybrid app that combines any of the
/// above.
pub struct Application {
    runtime_config: RuntimeConfig,
    env_vars: Box<dyn EnvVars>,
    app_tracing_enabled: bool,
    http_server_app: Option<Router<ApiAppState>>,
    runtime_local_api: Option<Router>,
    event_queue: Option<Arc<Mutex<VecDeque<EventTuple>>>>,
    processing_events_map: Option<Arc<Mutex<HashMap<String, EventTuple>>>>,
    ws_connections: Option<Arc<dyn WebSocketRegistrySend + 'static>>,
    ws_app_routes: Arc<AsyncMutex<HashMap<String, Arc<dyn WebSocketMessageHandler + Send + Sync>>>>,
    custom_auth_guards: Arc<AsyncMutex<HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>>>>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    local_api_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    resource_store: Option<Arc<ResourceStore>>,
    resource_store_cleanup_task_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
}

impl Application {
    pub fn new(runtime_config: RuntimeConfig, env_vars: Box<dyn EnvVars>) -> Self {
        Application {
            runtime_config,
            env_vars,
            app_tracing_enabled: false,
            http_server_app: None,
            server_shutdown_signal: None,
            runtime_local_api: None,
            local_api_shutdown_signal: None,
            event_queue: None,
            processing_events_map: None,
            ws_connections: None,
            ws_app_routes: Arc::new(AsyncMutex::new(HashMap::new())),
            custom_auth_guards: Arc::new(AsyncMutex::new(HashMap::new())),
            resource_store: None,
            resource_store_cleanup_task_shutdown_signal: None,
        }
    }

    pub fn setup(&mut self) -> Result<AppConfig, ApplicationStartError> {
        let blueprint_config = self.load_and_parse_blueprint()?;
        let mut app_config = AppConfig {
            api: None,
            consumers: None,
            schedules: None,
        };
        match collect_api_config(blueprint_config, &self.runtime_config) {
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

        #[cfg(feature = "aws_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;

            if self.runtime_config.platform == RuntimePlatform::AWS {
                self.setup_consumers_for_aws(&mut app_config)?;
            }
        }

        #[cfg(feature = "azure_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;

            if self.runtime_config.platform == RuntimePlatform::Azure {
                self.setup_consumers_for_azure(&mut app_config)?;
            }
        }

        #[cfg(feature = "gcloud_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;

            if self.runtime_config.platform == RuntimePlatform::GCP {
                self.setup_consumers_for_gcloud(&mut app_config)?;
            }
        }

        #[cfg(feature = "celerity_one_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;

            if self.runtime_config.platform == RuntimePlatform::Local {
                self.setup_consumers_for_celerity_one(&mut app_config)?;
            }
        }

        Ok(app_config)
    }

    pub fn websocket_registry(&self) -> Arc<dyn WebSocketRegistrySend> {
        if let Some(ws_connections) = &self.ws_connections {
            ws_connections.clone()
        } else {
            Arc::new(NoopWebSocketRegistrySend {})
        }
    }

    #[cfg(feature = "aws_consumers")]
    fn setup_consumers_for_aws(
        &mut self,
        app_config: &mut AppConfig,
    ) -> Result<(), ApplicationStartError> {
        if let Some(consumers_config) = &mut app_config.consumers {
            for _consumer in consumers_config.consumers.iter() {}
        }
        Ok(())
    }

    #[cfg(feature = "azure_consumers")]
    fn setup_consumers_for_azure(
        &mut self,
        _: &mut AppConfig,
    ) -> Result<(), ApplicationStartError> {
        Ok(())
    }

    #[cfg(feature = "gcloud_consumers")]
    fn setup_consumers_for_gcloud(
        &mut self,
        _: &mut AppConfig,
    ) -> Result<(), ApplicationStartError> {
        Ok(())
    }

    #[cfg(feature = "celerity_one_consumers")]
    fn setup_consumers_for_celerity_one(
        &mut self,
        _: &mut AppConfig,
    ) -> Result<(), ApplicationStartError> {
        Ok(())
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

        let resource_store = Arc::new(ResourceStore::new(
            create_http_client(self.runtime_config.resource_store_verify_tls)?,
            self.runtime_config.resource_store_cache_entry_ttl,
        ));
        self.resource_store = Some(resource_store.clone());

        if let Some(websocket_config) = &api_config.websocket {
            let websocket_base_path = resolve_websocket_base_path(api_config, websocket_config)?;
            let conn_registry = Arc::new(WebSocketConnRegistry::new(
                WebSocketConnRegistryConfig {
                    ack_worker_config: Some(AckWorkerConfig {
                        message_action_check_interval_ms: None,
                        message_timeout_ms: None,
                        max_attempts: None,
                    }),
                    server_node_name: "node1".to_string(),
                },
                None,
            ));
            self.ws_connections = Some(conn_registry.clone());
            http_server_app = http_server_app.route(
                websocket_base_path,
                get(websocket::handler).with_state(websocket::WebSocketAppState {
                    connections: conn_registry,
                    routes: self.ws_app_routes.clone(),
                    route_key: websocket_config.route_key.clone(),
                    api_auth: api_config.auth.clone(),
                    auth_strategy: Some(websocket_config.auth_strategy.clone()),
                    connection_auth_guard_name: websocket_config.connection_auth_guard.clone(),
                    connection_auth_guard: self
                        .get_custom_auth_guard_blocking(&websocket_config.connection_auth_guard),
                    cors: api_config.cors.clone(),
                    resource_store,
                }),
            );
        }

        Ok(http_server_app)
    }

    fn get_custom_auth_guard_blocking(
        &self,
        guard_name: &Option<String>,
    ) -> Option<Arc<dyn AuthGuardHandler + Send + Sync>> {
        if let Some(guard_name) = guard_name {
            // This is only called in the setup phase, so using a thread-blocking lock is safe
            // as the setup http server method will be the only caller accessing
            //the custom auth guards map at this point.
            self.custom_auth_guards
                .blocking_lock()
                .get(guard_name)
                .cloned()
        } else {
            None
        }
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
        // Tracing setup is in `run` instead of `setup` because
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

        if self.resource_store.is_some() {
            self.run_resource_store_cleanup_task();
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
                // TODO: make client IP extractor configurable from env vars.
                .layer(ClientIpSource::ConnectInfo.into_extension())
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
        let blueprint_config_path = self.runtime_config.blueprint_config_path.as_str();
        if blueprint_config_path.ends_with(".json") || blueprint_config_path.ends_with(".jsonc") {
            BlueprintConfig::from_jsonc_file(blueprint_config_path, self.env_vars.clone())
        } else {
            BlueprintConfig::from_yaml_file(blueprint_config_path, self.env_vars.clone())
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

    pub fn register_websocket_message_handler(
        &mut self,
        route: &str,
        handler: impl WebSocketMessageHandler + Send + Sync + 'static,
    ) {
        let mut ws_app_routes = self.ws_app_routes.blocking_lock();
        ws_app_routes.insert(route.to_string(), Arc::new(handler));
    }

    pub async fn register_custom_auth_guard(
        &mut self,
        guard_name: &str,
        handler: impl AuthGuardHandler + Send + Sync + 'static,
    ) {
        let mut custom_auth_guards = self.custom_auth_guards.lock().await;
        custom_auth_guards.insert(guard_name.to_string(), Arc::new(handler));
    }

    fn run_resource_store_cleanup_task(&mut self) {
        if let Some(resource_store) = self.resource_store.clone() {
            let (tx, mut rx) = tokio::sync::oneshot::channel::<()>();
            tokio::spawn(async move {
                loop {
                    if rx.try_recv().is_ok() {
                        info!("received shutdown signal, stopping resource store cleanup task");
                        break;
                    }

                    debug!("cleaning expired cache entries in resource store");
                    resource_store.clean_expired_cache_entries().await;
                    tokio::time::sleep(Duration::from_secs(60)).await;
                }
            });
            self.resource_store_cleanup_task_shutdown_signal = Some(tx);
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
        if let Some(tx) = self.resource_store_cleanup_task_shutdown_signal.take() {
            tx.send(())
                .expect("failed to send shutdown signal to resource store cleanup task");
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
    let is_hybrid_api = api_config.http.is_some();
    if websocket_config.base_paths.is_empty() && is_hybrid_api {
        return Err(ApplicationStartError::Config(ConfigError::Api(
            "A WebSocket-specific base path must be defined for a hybrid API \
            that provides a WebSocket and HTTP interface"
                .to_string(),
        )));
    }

    let ws_base_paths = websocket_config
        .base_paths
        .iter()
        .filter(|path| match path {
            // Only consider a base path string that is not protocol specific
            // if the API is only for WebSockets.
            CelerityApiBasePath::Str(_) => !is_hybrid_api,
            CelerityApiBasePath::BasePathConfiguration(base_path_config) => {
                base_path_config.protocol == CelerityApiProtocol::WebSocket
            }
        })
        .collect::<Vec<_>>();

    if ws_base_paths.len() > 1 {
        warn!(
            "Multiple WebSocket base paths are not supported by the runtime, \
         only the first one will be used"
        );
    }

    if ws_base_paths.is_empty() {
        Ok("/")
    } else {
        match &ws_base_paths[0] {
            CelerityApiBasePath::Str(base_path) => Ok(base_path.as_str()),
            CelerityApiBasePath::BasePathConfiguration(base_path_config) => {
                match base_path_config.protocol {
                    CelerityApiProtocol::WebSocket => Ok(base_path_config.base_path.as_str()),
                    _ => Err(ApplicationStartError::Config(ConfigError::Api(
                        "WebSocket base path configuration must be used for WebSocket APIs"
                            .to_string(),
                    ))),
                }
            }
        }
    }
}

#[derive(Debug)]
pub struct AppInfo {
    pub http_server_address: Option<SocketAddr>,
}

fn create_http_client(verify_tls: bool) -> Result<Client, ApplicationStartError> {
    Client::builder()
        .danger_accept_invalid_certs(!verify_tls)
        .build()
        .map_err(ApplicationStartError::HttpClient)
}

#[derive(Debug, Clone)]
struct NoopWebSocketRegistrySend {}

#[async_trait]
impl WebSocketRegistrySend for NoopWebSocketRegistrySend {
    async fn send_message(
        &self,
        _: String,
        _: String,
        _: MessageType,
        _: String,
        _: Option<SendContext>,
    ) -> Result<(), WebSocketConnError> {
        debug!("no-op websocket registry send called, a websocket API has not been configured");
        Ok(())
    }
}

impl Display for NoopWebSocketRegistrySend {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "NoopWebSocketRegistrySend")
    }
}
