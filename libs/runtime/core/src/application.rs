use std::{
    collections::{HashMap, VecDeque},
    fmt::Display,
    net::SocketAddr,
    sync::{Arc, Mutex},
    time::Duration,
};

use async_trait::async_trait;
use axum::http::{HeaderName, HeaderValue, Method as HttpMethod};
use axum::{
    extract::{MatchedPath, Request},
    handler::Handler,
    middleware,
    routing::{get, post},
    Json, Router,
};
use celerity_blueprint_config_parser::{
    blueprint::{
        BlueprintConfig, CelerityApiBasePath, CelerityApiCors, CelerityApiCorsConfiguration,
        CelerityApiProtocol,
    },
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
use tower_http::{cors::CorsLayer, trace::TraceLayer};
use tracing::{debug, info, info_span, warn};

use crate::{
    auth_custom::AuthGuardHandler,
    auth_http::{http_auth_middleware, HttpAuthState},
    config::{
        ApiConfig, AppConfig, ConsumerConfig, EventConfig, RuntimeConfig, ScheduleConfig,
        WebSocketConfig,
    },
    consts::DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
    consumer_handler::{
        ConsumerEventHandler, EventQueueConsumerEventHandler, SharedConsumerEventHandler,
    },
    errors::{ApplicationStartError, ConfigError},
    handler_invoke::{
        invoke_handler as invoke_handler_fn, new_handler_invoke_registry, HandlerInvokeRegistry,
        HandlerInvoker, InvokeHandlerState,
    },
    request::request_id,
    runtime_local_api::create_runtime_local_api,
    telemetry::{self, enrich_span, log_request},
    transform_config::{
        collect_api_config, collect_consumer_config, collect_custom_handler_definitions,
        collect_events_config, collect_schedule_config,
    },
    types::{ApiAppState, EventTuple},
    utils::get_epoch_seconds,
    websocket::{self, WebSocketMessageHandler},
};

/// Shutdown signal for a consumer — either oneshot (SQS) or broadcast (Redis).
#[allow(dead_code)]
enum ConsumerShutdownSignal {
    Oneshot(tokio::sync::oneshot::Sender<()>),
    Broadcast(tokio::sync::broadcast::Sender<()>),
}

type ConsumerShutdownSignals = HashMap<String, ConsumerShutdownSignal>;

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
    consumer_shutdown_signals: Option<Arc<Mutex<ConsumerShutdownSignals>>>,
    resource_store: Option<Arc<ResourceStore>>,
    resource_store_cleanup_task_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    http_auth_state: Option<HttpAuthState>,
    api_cors: Option<CelerityApiCors>,
    handler_names: HashMap<(String, String), String>,
    /// Consumer configs collected during setup(), used to create consumers in run().
    consumer_configs: Vec<ConsumerConfig>,
    /// Schedule configs collected during setup(), used to create schedule consumers in run().
    schedule_configs: Vec<ScheduleConfig>,
    /// Event configs (datastore streams, bucket events) collected during setup().
    event_configs: Vec<EventConfig>,
    /// The shared consumer event handler — set by SDK (FFI) or event queue (HTTP) before run().
    consumer_event_handler: Arc<SharedConsumerEventHandler>,
    /// JoinHandles for spawned consumer tasks, aborted on shutdown.
    consumer_task_handles: Vec<JoinHandle<()>>,
    /// Registry mapping handler names to invokers for handler-to-handler invocation
    /// and the invoke API.
    handler_invoke_registry: HandlerInvokeRegistry,
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
            consumer_shutdown_signals: None,
            event_queue: None,
            processing_events_map: None,
            ws_connections: None,
            ws_app_routes: Arc::new(AsyncMutex::new(HashMap::new())),
            custom_auth_guards: Arc::new(AsyncMutex::new(HashMap::new())),
            resource_store: None,
            resource_store_cleanup_task_shutdown_signal: None,
            http_auth_state: None,
            api_cors: None,
            handler_names: HashMap::new(),
            consumer_configs: Vec::new(),
            schedule_configs: Vec::new(),
            event_configs: Vec::new(),
            consumer_event_handler: Arc::new(SharedConsumerEventHandler::new()),
            consumer_task_handles: Vec::new(),
            handler_invoke_registry: new_handler_invoke_registry(),
        }
    }

    pub fn setup(&mut self) -> Result<AppConfig, ApplicationStartError> {
        let blueprint_config = self.load_and_parse_blueprint()?;
        let mut app_config = AppConfig {
            api: None,
            consumers: None,
            schedules: None,
            events: None,
            custom_handlers: None,
        };

        let mut collected_handler_names: Vec<String> = Vec::new();

        match collect_api_config(&blueprint_config, &self.runtime_config) {
            Ok((api_config, api_handler_names)) => {
                self.http_server_app = Some(self.setup_http_server_app(&api_config)?);
                self.api_cors = api_config.cors.clone();
                app_config.api = Some(api_config);
                collected_handler_names.extend(api_handler_names);
            }
            Err(ConfigError::ApiMissing) => (),
            Err(err) => return Err(ApplicationStartError::Config(err)),
        }

        app_config.consumers = collect_consumer_config(
            &blueprint_config,
            &self.runtime_config,
            &mut collected_handler_names,
        )?;
        app_config.events = collect_events_config(
            &blueprint_config,
            &self.runtime_config,
            &mut collected_handler_names,
        )?;
        app_config.schedules = collect_schedule_config(
            &blueprint_config,
            &self.runtime_config,
            &mut collected_handler_names,
        )?;
        app_config.custom_handlers =
            collect_custom_handler_definitions(&blueprint_config, &collected_handler_names)?;

        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Http {
            self.runtime_local_api = Some(self.setup_runtime_local_api(&app_config)?);
        }

        // Store consumer/schedule configs for later creation in run() (async context required).
        if let Some(consumers_config) = &app_config.consumers {
            self.consumer_configs = consumers_config.consumers.clone();
        }
        if let Some(schedules_config) = &app_config.schedules {
            self.schedule_configs = schedules_config.schedules.clone();
        }
        if let Some(events_config) = &app_config.events {
            self.event_configs = events_config.events.clone();
        }

        // In HTTP call mode, wire the event queue as the consumer event handler.
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Http {
            if let Some(event_queue) = &self.event_queue {
                let eq_handler = EventQueueConsumerEventHandler::new(event_queue.clone());
                self.consumer_event_handler.set(Arc::new(eq_handler));
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

    /// Registers a consumer event handler for FFI call mode.
    /// The SDK calls this after `setup()` and before `run()` to provide
    /// its handler implementation.
    pub fn register_consumer_handler(&self, handler: Arc<dyn ConsumerEventHandler>) {
        self.consumer_event_handler.set(handler);
    }

    /// Registers a handler invoker so the handler can be invoked by name
    /// through the invoke API or handler-to-handler calls.
    pub fn register_handler_invoker(&self, name: String, invoker: Arc<dyn HandlerInvoker>) {
        self.handler_invoke_registry
            .blocking_lock()
            .insert(name, invoker);
    }

    /// Returns the handler invoke registry for use in route setup or SDK access.
    pub fn handler_invoke_registry(&self) -> HandlerInvokeRegistry {
        self.handler_invoke_registry.clone()
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
                    auth_strategy: api_config
                        .auth
                        .as_ref()
                        .map(|_| websocket_config.auth_strategy.clone()),
                    connection_auth_guard_names: websocket_config.connection_auth_guard.clone(),
                    connection_auth_guards: self
                        .get_custom_auth_guards_blocking(&websocket_config.connection_auth_guard),
                    cors: api_config.cors.clone(),
                    resource_store: resource_store.clone(),
                }),
            );
        }

        {
            use celerity_helpers::runtime_types::RuntimePlatform;
            if self.runtime_config.platform == RuntimePlatform::Local
                || self.runtime_config.test_mode
            {
                http_server_app = http_server_app.route(
                    "/runtime/handlers/invoke",
                    post(invoke_handler_fn).with_state(InvokeHandlerState {
                        registry: self.handler_invoke_registry.clone(),
                    }),
                );
            }
        }

        if let Some(http_config) = &api_config.http {
            for handler in &http_config.handlers {
                self.handler_names.insert(
                    (handler.method.to_uppercase(), handler.path.clone()),
                    handler.name.clone(),
                );
            }
        }
        if let Some(api_auth) = &api_config.auth {
            let mut route_guards = HashMap::new();
            if let Some(http_config) = &api_config.http {
                for handler in &http_config.handlers {
                    if !handler.public {
                        route_guards.insert(
                            (handler.method.to_uppercase(), handler.path.clone()),
                            handler.auth_guard.clone(),
                        );
                    }
                }
            }
            self.http_auth_state = Some(HttpAuthState {
                api_auth: api_auth.clone(),
                resource_store,
                custom_auth_guards: self.custom_auth_guards.clone(),
                route_guards,
                handler_names: self.handler_names.clone(),
            });
        }

        Ok(http_server_app)
    }

    fn get_custom_auth_guards_blocking(
        &self,
        guard_names: &Option<Vec<String>>,
    ) -> HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>> {
        let mut guards = HashMap::new();
        if let Some(names) = guard_names {
            // This is only called in the setup phase, so using a thread-blocking lock is safe
            // as the setup http server method will be the only caller accessing
            // the custom auth guards map at this point.
            let all_guards = self.custom_auth_guards.blocking_lock();
            for name in names {
                if let Some(guard) = all_guards.get(name) {
                    guards.insert(name.clone(), guard.clone());
                }
            }
        }
        guards
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
            self.handler_invoke_registry.clone(),
        )
    }

    pub async fn run(&mut self, block: bool) -> Result<AppInfo, ApplicationStartError> {
        // Tracing setup is in `run` instead of `setup` because
        // we need to be in an async context (tokio runtime) in order to set up tracing.
        telemetry::setup_tracing(&self.runtime_config, self.app_tracing_enabled)?;

        // Set up OTel metrics when enabled. This must happen before RuntimeMetrics::new()
        // so the global MeterProvider is available for creating real instruments.
        if self.runtime_config.metrics_enabled {
            telemetry::setup_metrics(&self.runtime_config)?;
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

        // Create and start consumers in the async context.
        if !self.consumer_configs.is_empty()
            || !self.schedule_configs.is_empty()
            || !self.event_configs.is_empty()
        {
            self.start_consumers().await?;
        }

        if block {
            if let Some(task) = server_task {
                task.await?;
            }
            if let Some(task) = local_api_task {
                task.await?;
            }
            for handle in self.consumer_task_handles.drain(..) {
                let _ = handle.await;
            }
        }

        Ok(AppInfo {
            http_server_address: server_address,
        })
    }

    async fn start_consumers(&mut self) -> Result<(), ApplicationStartError> {
        use crate::consumer_handler::ManagedConsumer;

        let managed: Vec<Box<dyn ManagedConsumer>> = self.create_platform_consumers().await?;

        for consumer in managed {
            let handle = tokio::spawn(async move {
                if let Err(e) = consumer.start().await {
                    tracing::error!("consumer failed: {e}");
                }
            });
            self.consumer_task_handles.push(handle);
        }

        Ok(())
    }

    async fn create_platform_consumers(
        &mut self,
    ) -> Result<Vec<Box<dyn crate::consumer_handler::ManagedConsumer>>, ApplicationStartError> {
        #[cfg(feature = "celerity_local_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;
            if self.runtime_config.platform == RuntimePlatform::Local {
                return self.create_consumers_for_celerity_local().await;
            }
        }
        #[cfg(feature = "aws_consumers")]
        {
            use celerity_helpers::runtime_types::RuntimePlatform;
            if self.runtime_config.platform == RuntimePlatform::AWS {
                return self.create_consumers_for_aws().await;
            }
        }
        if !self.consumer_configs.is_empty()
            || !self.schedule_configs.is_empty()
            || !self.event_configs.is_empty()
        {
            warn!(
                "consumer/schedule/event configs present but no consumer implementation \
                 for platform {:?}",
                self.runtime_config.platform
            );
        }
        Ok(Vec::new())
    }

    #[cfg(feature = "celerity_local_consumers")]
    async fn create_consumers_for_celerity_local(
        &mut self,
    ) -> Result<Vec<Box<dyn crate::consumer_handler::ManagedConsumer>>, ApplicationStartError> {
        use celerity_consumer_redis::types::RedisMessageMetadata;
        use celerity_helpers::{
            consumers::MessageConsumer as _,
            redis::{get_redis_connection, ConnectionConfig},
        };

        use crate::consumer_handler::{
            ManagedConsumer, ManagedRedisConsumer, ScheduleHandlerBridge,
        };

        let redis_url = self
            .env_vars
            .var("CELERITY_LOCAL_QUEUE_ENDPOINT")
            .or_else(|_| self.env_vars.var("CELERITY_LOCAL_REDIS_URL"))
            .unwrap_or_else(|_| "redis://127.0.0.1:6379".to_string())
            .replace("valkey://", "redis://");

        let conn_config = ConnectionConfig {
            nodes: vec![redis_url],
            password: None,
            cluster_mode: false,
        };
        // Verify connectivity with an initial connection.
        let _verify_conn = get_redis_connection(&conn_config, None)
            .await
            .map_err(|e| {
                ApplicationStartError::ConsumerSetup(format!("redis connection failed: {e}"))
            })?;

        let mut managed: Vec<Box<dyn ManagedConsumer>> = Vec::new();
        let mut shutdown_signals = HashMap::new();
        let service_name = self.runtime_config.service_name.clone();
        let provider = self
            .runtime_config
            .resolve_body_transform_provider()
            .unwrap_or_default();

        for consumer_config in &self.consumer_configs {
            let consumer_name = format!("consumer-{}", consumer_config.source_id);
            let (shutdown_tx, _) = tokio::sync::broadcast::channel(1);

            let stream = match consumer_config.source_type {
                crate::config::ConsumerSourceType::Queue => {
                    format!("celerity:queue:{}", consumer_config.source_id)
                }
                crate::config::ConsumerSourceType::Topic => {
                    format!(
                        "celerity:topic:{}:{}",
                        consumer_config.source_id, consumer_config.consumer_name
                    )
                }
            };

            // Each consumer gets its own connection because XREAD BLOCK
            // is a blocking command that does not work correctly when
            // multiple callers share a single MultiplexedConnection.
            let consumer_conn = get_redis_connection(&conn_config, None)
                .await
                .map_err(|e| {
                    ApplicationStartError::ConsumerSetup(format!(
                        "redis connection for consumer {} failed: {e}",
                        consumer_name
                    ))
                })?;

            let mut consumer = create_redis_consumer(
                consumer_conn,
                conn_config.clone(),
                shutdown_tx.clone(),
                RedisConsumerParams {
                    service_name: service_name.clone(),
                    consumer_name: consumer_name.clone(),
                    stream,
                    dlq_stream: consumer_config
                        .dlq_source_id
                        .as_ref()
                        .map(|id| format!("celerity:dlq:consumer-{}", id)),
                    polling_wait_time_ms: consumer_config
                        .wait_time_seconds
                        .map(|w| w as u64 * 1000),
                    batch_size: consumer_config.batch_size.map(|b| b as usize),
                    message_handler_timeout: consumer_config
                        .handlers
                        .first()
                        .map(|h| h.timeout as u64)
                        .unwrap_or(30),
                    lock_duration_ms: consumer_config.visibility_timeout.map(|v| v as u64 * 1000),
                    max_retries: consumer_config.max_retries,
                },
            );

            let handler: Arc<
                dyn celerity_helpers::consumers::MessageHandler<RedisMessageMetadata> + Send + Sync,
            > = build_consumer_message_handler::<RedisMessageMetadata>(
                consumer_config,
                self.consumer_event_handler.clone(),
                provider,
            );
            consumer.register_handler(handler);

            shutdown_signals.insert(
                consumer_name,
                ConsumerShutdownSignal::Broadcast(shutdown_tx),
            );
            managed.push(Box::new(ManagedRedisConsumer(consumer)));
        }

        for schedule_config in &self.schedule_configs {
            let consumer_name = format!("schedule-consumer-{}", schedule_config.schedule_id);
            let (shutdown_tx, _) = tokio::sync::broadcast::channel(1);

            let schedule_conn = get_redis_connection(&conn_config, None)
                .await
                .map_err(|e| {
                    ApplicationStartError::ConsumerSetup(format!(
                        "redis connection for schedule consumer {} failed: {e}",
                        consumer_name
                    ))
                })?;

            let mut consumer = create_redis_consumer(
                schedule_conn,
                conn_config.clone(),
                shutdown_tx.clone(),
                RedisConsumerParams {
                    service_name: service_name.clone(),
                    consumer_name: consumer_name.clone(),
                    stream: format!("celerity:schedules:{}", schedule_config.schedule_id),
                    dlq_stream: None,
                    polling_wait_time_ms: schedule_config
                        .wait_time_seconds
                        .map(|w| w as u64 * 1000),
                    batch_size: Some(1),
                    message_handler_timeout: schedule_config
                        .handlers
                        .first()
                        .map(|h| h.timeout as u64)
                        .unwrap_or(30),
                    lock_duration_ms: schedule_config.visibility_timeout.map(|v| v as u64 * 1000),
                    max_retries: None,
                },
            );

            if let Some(handler_def) = schedule_config.handlers.first() {
                let handler_tag = format!(
                    "source::{}::{}",
                    schedule_config.schedule_id, handler_def.name
                );
                let bridge = ScheduleHandlerBridge::<RedisMessageMetadata>::new(
                    self.consumer_event_handler.clone(),
                    handler_tag,
                    schedule_config.schedule_id.clone(),
                    schedule_config.schedule_value.clone(),
                    schedule_config.input.clone(),
                );
                consumer.register_handler(Arc::new(bridge));
            }

            shutdown_signals.insert(
                consumer_name,
                ConsumerShutdownSignal::Broadcast(shutdown_tx),
            );
            managed.push(Box::new(ManagedRedisConsumer(consumer)));
        }

        for event_config in &self.event_configs {
            let (
                stream_name,
                source_id,
                handlers,
                batch_size,
                lock_duration_ms,
                polling_wait_time_ms,
                // Source label used by parse_source() for body transforms and
                // telemetry span context. Must use "celerity:<type>:<name>" format.
                event_source_label,
            ) = match event_config {
                crate::config::EventConfig::Stream(cfg) => {
                    let prefix = match cfg.source_type {
                        crate::config::StreamSourceType::Datastore => "celerity:datastore",
                        crate::config::StreamSourceType::DataStream => "celerity:stream",
                    };
                    let stream_name = format!("{}:{}", prefix, cfg.stream_id);
                    (
                        stream_name.clone(),
                        &cfg.stream_id,
                        &cfg.handlers,
                        cfg.batch_size,
                        None,
                        None,
                        stream_name,
                    )
                }
                crate::config::EventConfig::EventTrigger(cfg) => (
                    format!("celerity:bucket:{}", cfg.queue_id),
                    &cfg.queue_id,
                    &cfg.handlers,
                    cfg.batch_size,
                    cfg.visibility_timeout.map(|v| v as u64 * 1000),
                    cfg.wait_time_seconds.map(|w| w as u64 * 1000),
                    format!("celerity:bucket:{}", cfg.queue_id),
                ),
            };

            let consumer_name = format!("event-consumer-{}", source_id);
            let (shutdown_tx, _) = tokio::sync::broadcast::channel(1);

            let event_conn = get_redis_connection(&conn_config, None)
                .await
                .map_err(|e| {
                    ApplicationStartError::ConsumerSetup(format!(
                        "redis connection for event consumer {} failed: {e}",
                        consumer_name
                    ))
                })?;

            let mut consumer = create_redis_consumer(
                event_conn,
                conn_config.clone(),
                shutdown_tx.clone(),
                RedisConsumerParams {
                    service_name: service_name.clone(),
                    consumer_name: consumer_name.clone(),
                    stream: stream_name,
                    dlq_stream: None,
                    polling_wait_time_ms,
                    batch_size: batch_size.map(|b| b as usize),
                    message_handler_timeout: handlers
                        .first()
                        .map(|h| h.timeout as u64)
                        .unwrap_or(30),
                    lock_duration_ms,
                    max_retries: None,
                },
            );

            if let Some(handler_def) = handlers.first() {
                let handler_tag = format!("source::{}::{}", source_id, handler_def.name);
                let bridge =
                    crate::consumer_handler::ConsumerHandlerBridge::<RedisMessageMetadata>::new(
                        self.consumer_event_handler.clone(),
                        handler_tag,
                        event_source_label.clone(),
                        provider.to_string(),
                    );
                consumer.register_handler(Arc::new(bridge));
            }

            shutdown_signals.insert(
                consumer_name,
                ConsumerShutdownSignal::Broadcast(shutdown_tx),
            );
            managed.push(Box::new(ManagedRedisConsumer(consumer)));
        }

        self.consumer_shutdown_signals = Some(Arc::new(Mutex::new(shutdown_signals)));
        Ok(managed)
    }

    #[cfg(feature = "aws_consumers")]
    async fn create_consumers_for_aws(
        &mut self,
    ) -> Result<Vec<Box<dyn crate::consumer_handler::ManagedConsumer>>, ApplicationStartError> {
        use celerity_consumer_sqs::{
            message_consumer::{SQSConsumerConfig, SQSMessageConsumer},
            types::SQSMessageMetadata,
            visibility_timeout::{VisibilityTimeoutExtender, VisibilityTimeoutExtenderConfig},
        };
        use celerity_helpers::consumers::MessageConsumer as _;

        use crate::consumer_handler::{ManagedConsumer, ManagedSqsConsumer, ScheduleHandlerBridge};

        let aws_config = aws_config::load_defaults(aws_config::BehaviorVersion::latest()).await;
        let sqs_client = Arc::new(aws_sdk_sqs::Client::new(&aws_config));

        let mut managed: Vec<Box<dyn ManagedConsumer>> = Vec::new();
        let provider = self
            .runtime_config
            .resolve_body_transform_provider()
            .unwrap_or("aws");

        // Queue consumers
        for consumer_config in &self.consumer_configs {
            let queue_url = consumer_config.source_id.clone();
            let vis_extender = Arc::new(VisibilityTimeoutExtender::new(
                sqs_client.clone(),
                VisibilityTimeoutExtenderConfig {
                    queue_url: queue_url.clone(),
                    visibility_timeout: consumer_config.visibility_timeout.map(|v| v as i32),
                    heartbeat_interval: Some(10),
                },
            ));

            let sqs_config = SQSConsumerConfig {
                queue_url,
                polling_wait_time_ms: consumer_config
                    .wait_time_seconds
                    .map(|w| w as u64 * 1000)
                    .unwrap_or(5000),
                batch_size: consumer_config.batch_size.map(|b| b as i32),
                message_handler_timeout: consumer_config
                    .handlers
                    .first()
                    .map(|h| h.timeout as u64)
                    .unwrap_or(30),
                visibility_timeout: consumer_config.visibility_timeout.map(|v| v as i32),
                wait_time_seconds: consumer_config.wait_time_seconds.map(|w| w as i32),
                auth_error_timeout: None,
                terminate_visibility_timeout: true,
                should_delete_messages: true,
                delete_messages_on_handler_failure: None,
                attribute_names: None,
                message_attribute_names: None,
                num_workers: None,
            };

            let mut consumer =
                SQSMessageConsumer::new(sqs_client.clone(), vis_extender, sqs_config);

            let handler: Arc<
                dyn celerity_helpers::consumers::MessageHandler<SQSMessageMetadata> + Send + Sync,
            > = build_consumer_message_handler::<SQSMessageMetadata>(
                consumer_config,
                self.consumer_event_handler.clone(),
                provider,
            );
            consumer.register_handler(handler);

            managed.push(Box::new(ManagedSqsConsumer(consumer)));
        }

        // Schedule consumers
        for schedule_config in &self.schedule_configs {
            if schedule_config.queue_id.is_empty() {
                warn!(
                    "schedule {} has no queue_id; skipping SQS consumer creation",
                    schedule_config.schedule_id
                );
                continue;
            }

            let queue_url = schedule_config.queue_id.clone();
            let vis_extender = Arc::new(VisibilityTimeoutExtender::new(
                sqs_client.clone(),
                VisibilityTimeoutExtenderConfig {
                    queue_url: queue_url.clone(),
                    visibility_timeout: schedule_config.visibility_timeout.map(|v| v as i32),
                    heartbeat_interval: Some(10),
                },
            ));

            let sqs_config = SQSConsumerConfig {
                queue_url,
                polling_wait_time_ms: schedule_config
                    .wait_time_seconds
                    .map(|w| w as u64 * 1000)
                    .unwrap_or(5000),
                batch_size: Some(1),
                message_handler_timeout: schedule_config
                    .handlers
                    .first()
                    .map(|h| h.timeout as u64)
                    .unwrap_or(30),
                visibility_timeout: schedule_config.visibility_timeout.map(|v| v as i32),
                wait_time_seconds: schedule_config.wait_time_seconds.map(|w| w as i32),
                auth_error_timeout: None,
                terminate_visibility_timeout: true,
                should_delete_messages: true,
                delete_messages_on_handler_failure: None,
                attribute_names: None,
                message_attribute_names: None,
                num_workers: None,
            };

            let mut consumer =
                SQSMessageConsumer::new(sqs_client.clone(), vis_extender, sqs_config);

            if let Some(handler_def) = schedule_config.handlers.first() {
                let handler_tag = format!(
                    "source::{}::{}",
                    schedule_config.schedule_id, handler_def.name
                );
                let bridge = ScheduleHandlerBridge::<SQSMessageMetadata>::new(
                    self.consumer_event_handler.clone(),
                    handler_tag,
                    schedule_config.schedule_id.clone(),
                    schedule_config.schedule_value.clone(),
                    schedule_config.input.clone(),
                );
                consumer.register_handler(Arc::new(bridge));
            }

            managed.push(Box::new(ManagedSqsConsumer(consumer)));
        }

        Ok(managed)
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

        let runtime_metrics = if self.runtime_config.metrics_enabled {
            Some(Arc::new(telemetry::RuntimeMetrics::new()))
        } else {
            None
        };
        let api_app_state = ApiAppState {
            platform: self.runtime_config.platform.clone(),
            handler_names: self.handler_names.clone(),
            metrics: runtime_metrics,
        };
        let http_app = http_app.layer(middleware::from_fn_with_state(
            api_app_state.clone(),
            log_request,
        ));
        let http_app = if let Some(http_auth_state) = &self.http_auth_state {
            http_app.layer(middleware::from_fn_with_state(
                http_auth_state.clone(),
                http_auth_middleware,
            ))
        } else {
            http_app
        };
        let http_app = if let Some(cors) = &self.api_cors {
            http_app.layer(build_cors_layer(cors))
        } else {
            http_app
        };
        let final_http_app =
            attach_tracing_layers(http_app, api_app_state.clone(), self.app_tracing_enabled)
                .layer(
                    self.runtime_config
                        .client_ip_source
                        .clone()
                        .into_extension(),
                )
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
                // Ensure we capture `ConnectInfo` to feed into the client IP extractor
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
        if let Some(consumer_shutdown_signals_lock) = self.consumer_shutdown_signals.take() {
            let mut consumer_shutdown_signals = consumer_shutdown_signals_lock
                .lock()
                .expect("consumer shutdown signals lock should not be poisoned");

            for (name, signal) in consumer_shutdown_signals.drain() {
                match signal {
                    ConsumerShutdownSignal::Oneshot(tx) => {
                        if tx.send(()).is_err() {
                            warn!("failed to send shutdown signal to consumer {name}");
                        }
                    }
                    ConsumerShutdownSignal::Broadcast(tx) => {
                        if tx.send(()).is_err() {
                            warn!("failed to send shutdown signal to consumer {name}");
                        }
                    }
                }
            }
        }
        for handle in self.consumer_task_handles.drain(..) {
            handle.abort();
        }
    }
}

/// Builds a `MessageHandler<M>` for a consumer config, using either a routed
/// handler (when routing is configured) or a simple bridge.
///
/// The `provider` string (e.g. `"aws"`, `"gcp"`) selects which body transform
/// implementation to use for event source types like bucket and datastore.
#[cfg(any(feature = "aws_consumers", feature = "celerity_local_consumers"))]
fn build_consumer_message_handler<M>(
    consumer_config: &ConsumerConfig,
    event_handler: Arc<dyn ConsumerEventHandler>,
    provider: &str,
) -> Arc<dyn celerity_helpers::consumers::MessageHandler<M> + Send + Sync>
where
    M: std::fmt::Debug + Clone + Send + Sync + 'static,
    celerity_helpers::consumers::Message<M>: crate::consumer_handler::ToConsumerEventData,
{
    use crate::consumer_handler::{ConsumerHandlerBridge, RoutedConsumerHandlerBridge};
    use celerity_helpers::consumers::MessageHandlerWithRouter;

    let has_routing = consumer_config.routing_key.is_some()
        && consumer_config.handlers.iter().any(|h| h.route.is_some());

    if has_routing {
        let routing_key = consumer_config.routing_key.clone();

        // Use the first handler without a route (or the very first handler) as fallback.
        let fallback_handler_def = consumer_config
            .handlers
            .iter()
            .find(|h| h.route.is_none())
            .unwrap_or(&consumer_config.handlers[0]);
        let fallback_tag = format!(
            "source::{}::{}",
            consumer_config.source_id, fallback_handler_def.name
        );
        let fallback = Arc::new(RoutedConsumerHandlerBridge::<M>::new(
            event_handler.clone(),
            fallback_tag,
            consumer_config.source_id.clone(),
            provider.to_string(),
        ));

        let mut router = MessageHandlerWithRouter::new(routing_key, None, fallback);

        for handler_def in &consumer_config.handlers {
            if let Some(route) = &handler_def.route {
                let handler_tag = format!(
                    "source::{}::{}",
                    consumer_config.source_id, handler_def.name
                );
                let routed_bridge = Arc::new(RoutedConsumerHandlerBridge::<M>::new(
                    event_handler.clone(),
                    handler_tag,
                    consumer_config.source_id.clone(),
                    provider.to_string(),
                ));
                router.register_route(route.clone(), routed_bridge);
            }
        }

        Arc::new(router)
    } else {
        // Single handler, no routing.
        let handler_def = &consumer_config.handlers[0];
        let handler_tag = format!(
            "source::{}::{}",
            consumer_config.source_id, handler_def.name
        );
        Arc::new(ConsumerHandlerBridge::<M>::new(
            event_handler,
            handler_tag,
            consumer_config.source_id.clone(),
            provider.to_string(),
        ))
    }
}

/// Parameters for creating a single Redis-backed consumer, capturing only the
/// fields that vary between queue, schedule, and event consumer types.
#[cfg(feature = "celerity_local_consumers")]
struct RedisConsumerParams {
    service_name: String,
    consumer_name: String,
    stream: String,
    dlq_stream: Option<String>,
    polling_wait_time_ms: Option<u64>,
    batch_size: Option<usize>,
    message_handler_timeout: u64,
    lock_duration_ms: Option<u64>,
    max_retries: Option<i64>,
}

#[cfg(feature = "celerity_local_consumers")]
fn create_redis_consumer(
    redis_conn: celerity_helpers::redis::ConnectionWrapper,
    conn_config: celerity_helpers::redis::ConnectionConfig,
    shutdown_tx: tokio::sync::broadcast::Sender<()>,
    params: RedisConsumerParams,
) -> celerity_consumer_redis::message_consumer::RedisMessageConsumer {
    use celerity_consumer_redis::{
        lock_durations::{LockDurationExtender, LockDurationExtenderConfig},
        locks::MessageLocks,
        message_consumer::{RedisConsumerConfig, RedisMessageConsumer},
    };
    use celerity_helpers::time::DefaultClock;

    let message_locks = Arc::new(tokio::sync::Mutex::new(MessageLocks::new(
        params.service_name.clone(),
        params.consumer_name.clone(),
        redis_conn.clone(),
    )));
    let lock_extender = Arc::new(LockDurationExtender::new(
        message_locks,
        LockDurationExtenderConfig {
            lock_duration_ms: params.lock_duration_ms.unwrap_or(30_000),
            heartbeat_interval: 10,
        },
    ));
    let clock: Arc<dyn celerity_helpers::time::Clock + Send + Sync> = Arc::new(DefaultClock::new());

    let redis_config = RedisConsumerConfig {
        service_name: params.service_name,
        name: params.consumer_name,
        stream: params.stream,
        dlq_stream: params.dlq_stream,
        last_message_id_key: None,
        block_time_ms: None,
        polling_wait_time_ms: params.polling_wait_time_ms,
        batch_size: params.batch_size,
        message_handler_timeout: params.message_handler_timeout,
        lock_duration_ms: params.lock_duration_ms,
        max_retries: params.max_retries,
        retry_base_delay_ms: None,
        retry_max_delay: None,
        backoff_rate: None,
        trim_stream_interval: None,
        max_stream_length: None,
        trim_lock_timeout_ms: None,
        num_workers: None,
    };

    RedisMessageConsumer::new(
        lock_extender,
        clock,
        redis_conn,
        conn_config,
        shutdown_tx,
        redis_config,
    )
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
                    handler_name = tracing::field::Empty,
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

fn build_cors_layer(cors: &CelerityApiCors) -> CorsLayer {
    match cors {
        CelerityApiCors::Str(s) if s == "*" => CorsLayer::permissive(),
        CelerityApiCors::Str(s) => {
            warn!("unrecognised CORS shorthand \"{s}\", only \"*\" is supported; defaulting to restrictive CORS policy");
            CorsLayer::new()
        }
        CelerityApiCors::CorsConfiguration(config) => build_cors_layer_from_config(config),
    }
}

fn build_cors_layer_from_config(config: &CelerityApiCorsConfiguration) -> CorsLayer {
    let mut layer = CorsLayer::new();

    // Allow origins.
    if let Some(origins) = &config.allow_origins {
        let origins: Vec<HeaderValue> = origins
            .iter()
            .filter_map(|o| HeaderValue::from_str(o).ok())
            .collect();
        layer = layer.allow_origin(origins);
    }

    // Allow methods.
    if let Some(methods) = &config.allow_methods {
        let methods: Vec<HttpMethod> = methods
            .iter()
            .filter_map(|m| m.parse::<HttpMethod>().ok())
            .collect();
        layer = layer.allow_methods(methods);
    }

    // Allow headers.
    if let Some(headers) = &config.allow_headers {
        let headers: Vec<HeaderName> = headers
            .iter()
            .filter_map(|h| h.parse::<HeaderName>().ok())
            .collect();
        layer = layer.allow_headers(headers);
    }

    // Expose headers.
    if let Some(headers) = &config.expose_headers {
        let headers: Vec<HeaderName> = headers
            .iter()
            .filter_map(|h| h.parse::<HeaderName>().ok())
            .collect();
        layer = layer.expose_headers(headers);
    }

    // Allow credentials.
    if let Some(true) = config.allow_credentials {
        layer = layer.allow_credentials(true);
    }

    // Max age.
    if let Some(max_age) = config.max_age {
        layer = layer.max_age(Duration::from_secs(max_age as u64));
    }

    layer
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
