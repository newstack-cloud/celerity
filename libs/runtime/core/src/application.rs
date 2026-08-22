use std::{
    collections::HashMap,
    fmt::Display,
    net::SocketAddr,
    sync::{Arc, Mutex, OnceLock},
    time::Duration,
};

use async_trait::async_trait;
use axum::http::{HeaderName, HeaderValue, Method as HttpMethod, StatusCode};
use axum::{
    extract::{MatchedPath, RawPathParams, Request},
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
use tokio::{
    net::TcpListener,
    sync::{mpsc, Mutex as AsyncMutex},
    task::JoinHandle,
};
use tokio_stream::wrappers::UnixListenerStream;
use tonic::transport::Server;
use tower_http::{cors::CorsLayer, trace::TraceLayer};
use tracing::{debug, error, info, info_span, warn};

use crate::{
    auth_custom::AuthGuardHandler,
    auth_http::{http_auth_middleware, HttpAuthState},
    config::{
        ApiConfig, AppConfig, ConsumerConfig, EventConfig, RuntimeConfig, ScheduleConfig,
        WebSocketConfig,
    },
    consts::{
        DEFAULT_EVENT_QUEUE_CAPACITY, DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
        DISPATCHER_COMMAND_BUFFER, WS_CONNECTION_DEFAULT_HANDLER_CONCURRENCY,
    },
    consumer_handler::{
        ConsumerEventHandler, EventQueueConsumerEventHandler, SharedConsumerEventHandler,
    },
    dispatcher::{drain_timeout, Dispatcher, DispatcherCommand, HandlerReadiness},
    errors::{ApplicationStartError, ConfigError},
    event_queue::{
        collect_handler_timeouts, http_handler_tag, timeout_from_seconds, websocket_handler_tag,
        EventCleanupTask, EventQueueHandles, EventQueueParts, EventQueueReceivers,
    },
    handler_invoke::{
        invoke_handler as invoke_handler_fn, invoke_handler_ipc, new_handler_invoke_registry,
        HandlerInvokeRegistry, HandlerInvoker, InvokeHandlerState, IpcInvokeState,
        INVOKE_HANDLER_ROUTE,
    },
    ipc_http::{self, IpcHttpRoute},
    ipc_proto::handler_runtime_service_server::HandlerRuntimeServiceServer,
    ipc_stream::{
        handler_tags_by_name, runtime_config_from_app_config, tags_from_runtime_config,
        HandlerStreamService, StreamContext,
    },
    ipc_websocket::IpcWebSocketHandler,
    request::request_id,
    telemetry::{self, enrich_span, log_request},
    transform_config::{
        collect_api_config, collect_consumer_config, collect_custom_handler_definitions,
        collect_events_config, collect_schedule_config,
    },
    types::ApiAppState,
    utils::get_epoch_seconds,
    websocket::{self, WebSocketMessageHandler},
    websocket_dedupe::{SeenMessages, DEFAULT_MESSAGE_ID_TTL_MS},
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
    event_queue: Option<EventQueueHandles>,
    /// Moved into the dispatcher when it starts, since it is the sole consumer.
    event_queue_receivers: Option<EventQueueReceivers>,
    /// Created during setup, started in `run`.
    event_cleanup_task: Option<EventCleanupTask>,
    /// Built during setup, served in `run` once there is a runtime to spawn on.
    ipc_stream_context: Option<Arc<StreamContext>>,
    ipc_dispatcher: Option<Dispatcher>,
    /// Taken from the dispatcher during setup, since starting it consumes it.
    ///
    /// Shared with the health check route, which is registered before the
    /// dispatcher is built, and stays empty in the FFI call mode.
    handler_readiness: Arc<OnceLock<HandlerReadiness>>,
    ipc_commands_rx: Option<mpsc::Receiver<DispatcherCommand>>,
    ipc_dispatcher_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    ipc_server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    event_cleanup_task_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    ws_connections: Option<Arc<dyn WebSocketRegistrySend + 'static>>,
    // The same registry as above, held as itself rather than as what it sends
    // through, so that `run` can start the worker that waits on clients.
    // Taken when it does, since it is only started once.
    ws_conn_registry: Option<Arc<WebSocketConnRegistry>>,
    // Kept for `run` to start its sweep, for the same reason as the registry
    // above. The store itself is built in setup, since building it spawns
    // nothing.
    ws_seen_messages: Option<Arc<SeenMessages>>,
    ws_app_routes: Arc<AsyncMutex<HashMap<String, Arc<dyn WebSocketMessageHandler + Send + Sync>>>>,
    custom_auth_guards: Arc<AsyncMutex<HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>>>>,
    server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    consumer_shutdown_signals: Option<Arc<Mutex<ConsumerShutdownSignals>>>,
    resource_store: Option<Arc<ResourceStore>>,
    resource_store_cleanup_task_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    #[cfg(feature = "ws_clustering")]
    ws_cluster_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
    /// What this node is called among the others serving the same API.
    ///
    /// This is determined once and kept, because a name that had to be generated is a
    /// different one every time it is worked out, and the registry and the
    /// cluster have to agree on what this node is called.
    server_node_name: String,
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

/// Names this node among the others serving the same WebSocket API.
///
/// Taken from the configuration, then the host name, which is the pod name on
/// Kubernetes and the container id under Docker, and finally a generated id so
/// that a node always has a name of its own. Two nodes sharing one would each
/// take the other's acknowledgements.
fn resolve_server_node_name(runtime_config: &RuntimeConfig, env_vars: &dyn EnvVars) -> String {
    runtime_config
        .server_node_name
        .clone()
        .or_else(|| {
            env_vars
                .var("HOSTNAME")
                .ok()
                .map(|hostname| hostname.trim().to_string())
                .filter(|hostname| !hostname.is_empty())
        })
        .unwrap_or_else(|| nanoid::nanoid!())
}

impl Application {
    pub fn new(runtime_config: RuntimeConfig, env_vars: Box<dyn EnvVars>) -> Self {
        let server_node_name = resolve_server_node_name(&runtime_config, env_vars.as_ref());
        Application {
            server_node_name,
            runtime_config,
            env_vars,
            app_tracing_enabled: false,
            http_server_app: None,
            server_shutdown_signal: None,
            consumer_shutdown_signals: None,
            event_queue: None,
            event_queue_receivers: None,
            event_cleanup_task: None,
            event_cleanup_task_shutdown_signal: None,
            ipc_stream_context: None,
            ipc_dispatcher: None,
            handler_readiness: Arc::new(OnceLock::new()),
            ipc_commands_rx: None,
            ipc_dispatcher_shutdown_signal: None,
            ipc_server_shutdown_signal: None,
            ws_connections: None,
            ws_conn_registry: None,
            ws_seen_messages: None,
            ws_app_routes: Arc::new(AsyncMutex::new(HashMap::new())),
            custom_auth_guards: Arc::new(AsyncMutex::new(HashMap::new())),
            resource_store: None,
            resource_store_cleanup_task_shutdown_signal: None,
            #[cfg(feature = "ws_clustering")]
            ws_cluster_shutdown_signal: None,
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

        // The event queue has to exist before the API router is built, because
        // in the IPC call mode every HTTP route holds a producer for it. Only
        // the queue is created here; the cleanup task that goes with it is
        // started in `run`, where there is a runtime to spawn it on.
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc {
            let (handles, receivers, cleanup_task) =
                EventQueueParts::new(DEFAULT_EVENT_QUEUE_CAPACITY).into_parts();
            self.event_queue = Some(handles);
            self.event_queue_receivers = Some(receivers);
            self.event_cleanup_task = Some(cleanup_task);
        }

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

        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc {
            self.setup_ipc_stream(&app_config);
        }

        // Registered here rather than with the rest of the routes because it
        // needs the custom handler definitions, which are only collected once
        // every other kind of handler has been.
        self.register_local_invoke_route(&app_config);

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

        // In IPC call mode, wire the event queue as the consumer event handler.
        if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc {
            if let Some(event_queue) = &self.event_queue {
                let eq_handler = EventQueueConsumerEventHandler::new(
                    event_queue.queue.clone(),
                    collect_handler_timeouts(&app_config),
                );
                self.consumer_event_handler.set(Arc::new(eq_handler));
            }
        }

        Ok(app_config)
    }

    /// The event queue handles for the IPC call mode, or `None` in the FFI
    /// call mode where handlers run in-process and no queue is created.
    ///
    /// This is what a component draining the queue needs the receiver to take
    /// events from, and the in-flight table to return results through.
    pub fn event_queue(&self) -> Option<EventQueueHandles> {
        self.event_queue.clone()
    }

    /// Whether a handlers executable is attached, or `None` in the FFI call
    /// mode where handlers run in-process and cannot be absent.
    ///
    /// A runtime that starts the handlers executable itself uses this to give
    /// up when nothing attaches, which is the only signal an orchestrator gets
    /// for a handler process that is alive but never serves.
    pub fn handler_readiness(&self) -> Option<HandlerReadiness> {
        self.handler_readiness.get().cloned()
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
            let handler_readiness = self.handler_readiness.clone();
            http_server_app = http_server_app.route(
                DEFAULT_RUNTIME_HEALTH_CHECK_ENDPOINT,
                get(move |()| {
                    let handler_readiness = handler_readiness.clone();
                    async move {
                        // In the IPC call mode this includes
                        // whether a handlers executable is attached, since a
                        // runtime without one sheds every event it takes.
                        //
                        // An instance is therefore unhealthy from the moment it
                        // binds its port until its handlers attach. A deployment
                        // has to allow for that before it treats a 503 as a
                        // failure, through a startup probe or a start period
                        // longer than CELERITY_HANDLERS_START_TIMEOUT, or it
                        // will kill every instance during its cold start.
                        let serving = handler_readiness
                            .get()
                            .is_none_or(HandlerReadiness::is_ready);
                        let status = if serving {
                            StatusCode::OK
                        } else {
                            // 503 rather than 500 as nothing has failed, there is
                            // just nothing to hand work to yet.
                            StatusCode::SERVICE_UNAVAILABLE
                        };
                        (
                            status,
                            Json(HealthCheckResponse {
                                timestamp: get_epoch_seconds(),
                            }),
                        )
                    }
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
                        message_timeout_ms: self.runtime_config.ws_ack_timeout_ms,
                        max_attempts: self.runtime_config.ws_ack_max_attempts,
                    }),
                    server_node_name: self.server_node_name.clone(),
                },
                None,
            ));
            self.ws_connections = Some(conn_registry.clone());
            // Kept for `run` to start the worker that tracks the messages
            // asking their client to acknowledge them. Starting one here would
            // mean spawning, and this runs outside the async runtime when in FFI mode.
            self.ws_conn_registry = Some(conn_registry.clone());

            let seen_messages = SeenMessages::new(DEFAULT_MESSAGE_ID_TTL_MS);
            self.ws_seen_messages = Some(seen_messages.clone());

            // As with HTTP routes, the FFI call mode has the SDK register these
            // as it binds each in-process handler. In the IPC call mode the
            // runtime registers one per blueprint handler, so that messages
            // have somewhere to route to.
            if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc {
                if let Some(event_queue) = &self.event_queue {
                    // `try_lock` rather than `blocking_lock`, which panics when
                    // called from within a runtime, and setup runs inside one.
                    // Nothing else holds the route map during setup, so failing
                    // to take it would mean the invariant no longer holds.
                    let mut ws_app_routes = self.ws_app_routes.try_lock().map_err(|_| {
                        ApplicationStartError::Config(ConfigError::Api(
                            "the WebSocket route map was already held during setup".to_string(),
                        ))
                    })?;
                    for handler in &websocket_config.handlers {
                        ws_app_routes.insert(
                            handler.route.clone(),
                            Arc::new(IpcWebSocketHandler::new(
                                event_queue.queue.clone(),
                                websocket_handler_tag(&handler.route_key, &handler.route),
                                handler.route.clone(),
                                timeout_from_seconds(handler.timeout),
                            )),
                        );
                    }
                }
            }
            http_server_app = http_server_app.route(
                websocket_base_path,
                get(websocket::handler).with_state(websocket::WebSocketAppState {
                    connections: conn_registry,
                    seen_messages: seen_messages.clone(),
                    routes: self.ws_app_routes.clone(),
                    route_key: websocket_config.route_key.clone(),
                    api_auth: api_config.auth.clone(),
                    auth_strategy: api_config
                        .auth
                        .as_ref()
                        .map(|_| websocket_config.auth_strategy.clone()),
                    connection_auth_guard_names: websocket_config.connection_auth_guard.clone(),
                    connection_auth_guards: self.custom_auth_guards.clone(),
                    handler_concurrency: self
                        .runtime_config
                        .ws_handler_concurrency
                        .unwrap_or(WS_CONNECTION_DEFAULT_HANDLER_CONCURRENCY),
                    cors: api_config.cors.clone(),
                    resource_store: resource_store.clone(),
                }),
            );
        }

        if let Some(http_config) = &api_config.http {
            for handler in &http_config.handlers {
                self.handler_names.insert(
                    (handler.method.to_uppercase(), handler.path.clone()),
                    handler.name.clone(),
                );
            }

            // In the FFI call mode the SDK registers these routes itself as it
            // binds each in-process handler. In the IPC call mode there is no
            // in-process handler to bind, so the runtime registers a route per
            // blueprint handler that dispatches over the event queue.
            if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc {
                if let Some(event_queue) = &self.event_queue {
                    for handler in &http_config.handlers {
                        http_server_app = register_ipc_http_route(
                            http_server_app,
                            &handler.method,
                            &handler.path,
                            IpcHttpRoute {
                                event_queue: event_queue.queue.clone(),
                                handler_tag: http_handler_tag(&handler.method, &handler.path),
                                route: handler.path.clone(),
                                timeout: timeout_from_seconds(handler.timeout),
                            },
                        );
                    }
                }
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

    /// Adds the endpoint that invokes a handler directly by name, which exists
    /// so that a handler can be exercised while developing or testing.
    ///
    /// Served only when the switch is on and the runtime is on a local platform
    /// or in test mode. Both are required, so the switch can turn this off
    /// anywhere but cannot turn it on somewhere it does not belong. It runs a
    /// handler with a payload the caller supplies, bypassing whatever normally
    /// triggers it, and carries no auth of its own.
    ///
    /// An application with no HTTP API gets a server for this alone. That is
    /// the case the endpoint is most useful in, since a queue, schedule or
    /// custom handler has no other way to be triggered by hand, and skipping it
    /// there would leave exactly those projects without the shortcut.
    fn register_local_invoke_route(&mut self, app_config: &AppConfig) {
        use celerity_helpers::runtime_types::RuntimePlatform;

        if !self.runtime_config.enable_local_invoke {
            return;
        }
        // Checked separately from the switch so that the switch can never turn
        // this on somewhere it should not be. Turning it off is always allowed,
        // turning it on is not enough on its own.
        if self.runtime_config.platform != RuntimePlatform::Local && !self.runtime_config.test_mode
        {
            warn!(
                "the local handler invoke endpoint is enabled but the platform is not local and \
                 test mode is off, so it will not be served"
            );
            return;
        }
        // An application that declares no HTTP API has no router yet, so one is
        // started to carry this endpoint alone.
        let http_server_app = self.http_server_app.take().unwrap_or_default();

        // Deliberately a warning rather than an informational line. It is the
        // only signal in a running system that anything able to reach this
        // server can run any declared handler with a payload of its choosing.
        warn!(
            "serving POST /runtime/handlers/invoke, which runs any handler the blueprint declares \
             by name, bypassing whatever normally triggers it and with no authentication, this \
             must not be reachable outside development"
        );

        // The IPC call mode has no in-process invokers to call, so an
        // invocation becomes an event addressed to the handler's own tag and
        // reaches the handlers executable over the stream, taking the same
        // timeout, credit and cancellation handling as any other event.
        //
        // The name to tag mapping comes from the configuration the handler
        // itself was sent, so the runtime cannot accept a name the handler was
        // never told about.
        let with_route = match (&self.event_queue, &self.ipc_stream_context) {
            (Some(event_queue), Some(stream_context))
                if self.runtime_config.runtime_call_mode == RuntimeCallMode::Ipc =>
            {
                http_server_app.route(
                    INVOKE_HANDLER_ROUTE,
                    post(invoke_handler_ipc).with_state(IpcInvokeState {
                        event_queue: event_queue.queue.clone(),
                        timeouts: collect_handler_timeouts(app_config),
                        handler_tags: Arc::new(handler_tags_by_name(
                            &stream_context.runtime_config,
                        )),
                    }),
                )
            }
            _ => http_server_app.route(
                INVOKE_HANDLER_ROUTE,
                post(invoke_handler_fn).with_state(InvokeHandlerState {
                    registry: self.handler_invoke_registry.clone(),
                }),
            ),
        };
        self.http_server_app = Some(with_route);
    }

    /// Prepares the handler stream including the dispatcher that decides where events go,
    /// and the context each connected handler stream serves from.
    ///
    /// Nothing is spawned or bound here. The dispatcher and the server both need
    /// a runtime, so `run` starts them.
    fn setup_ipc_stream(&mut self, app_config: &AppConfig) {
        // Taken before the queue is borrowed, since resolving it borrows self.
        // The WebSocket API is set up before this runs, so a registry is
        // already in place when the blueprint declares one.
        let ws_registry = self.websocket_registry();

        let Some(event_queue) = &self.event_queue else {
            return;
        };

        let runtime_config = runtime_config_from_app_config(
            app_config,
            self.app_tracing_enabled,
            self.runtime_config.metrics_enabled,
        );
        let blueprint_tags = tags_from_runtime_config(&runtime_config);
        let (commands_tx, commands_rx) = mpsc::channel(DISPATCHER_COMMAND_BUFFER);

        let handler_timeouts = collect_handler_timeouts(app_config);
        let drain_timeout = drain_timeout(self.runtime_config.drain_timeout, &handler_timeouts);
        info!(
            ?drain_timeout,
            "shutdown will wait this long for in-flight events"
        );

        let dispatcher = Dispatcher::new(
            event_queue.in_flight.clone(),
            handler_timeouts,
            drain_timeout,
        );
        // Filled before anything serves, so the health check never answers on a
        // readiness handle that is missing only because setup is still running.
        let _ = self.handler_readiness.set(dispatcher.readiness());
        self.ipc_dispatcher = Some(dispatcher);
        self.ipc_commands_rx = Some(commands_rx);
        self.ipc_stream_context = Some(Arc::new(StreamContext {
            runtime_config,
            blueprint_tags,
            commands: commands_tx,
            in_flight: event_queue.in_flight.clone(),
            ws_registry,
        }));
    }

    /// Starts the dispatcher and serves the handler stream.
    ///
    /// A Unix socket is preferred as it is faster than loopback on Linux, needs no
    /// port allocation, and its access control is filesystem permissions rather
    /// than reachability of localhost. Where one cannot be bound the runtime
    /// falls back to loopback TCP so the platform is still serviceable.
    async fn run_ipc_stream_server(&mut self) -> Result<(), ApplicationStartError> {
        let (Some(dispatcher), Some(receivers), Some(commands_rx), Some(context)) = (
            self.ipc_dispatcher.take(),
            self.event_queue_receivers.take(),
            self.ipc_commands_rx.take(),
            self.ipc_stream_context.clone(),
        ) else {
            return Ok(());
        };

        let (dispatcher_shutdown_tx, dispatcher_shutdown_rx) = tokio::sync::oneshot::channel();
        tokio::spawn(dispatcher.run(receivers, commands_rx, dispatcher_shutdown_rx));
        self.ipc_dispatcher_shutdown_signal = Some(dispatcher_shutdown_tx);

        let service = HandlerRuntimeServiceServer::new(HandlerStreamService::new(context));
        let (server_shutdown_tx, server_shutdown_rx) = tokio::sync::oneshot::channel();
        let shutdown = async {
            server_shutdown_rx.await.ok();
        };

        match bind_runtime_socket(&self.runtime_config.runtime_socket).await {
            Ok(listener) => {
                info!(
                    socket = %self.runtime_config.runtime_socket,
                    "serving the handler stream on a unix socket"
                );
                let incoming = UnixListenerStream::new(listener);
                tokio::spawn(async move {
                    if let Err(err) = Server::builder()
                        .add_service(service)
                        .serve_with_incoming_shutdown(incoming, shutdown)
                        .await
                    {
                        error!("handler stream server stopped: {err}");
                    }
                });
            }
            Err(err) if !self.runtime_config.runtime_socket_fallback_enabled => {
                // Refused rather than fallen back on. Serving the stream over
                // loopback instead would widen who can register as a handler,
                // from the one user the socket's permissions allow to any
                // process that can reach loopback, and would do it silently at
                // the moment something is already wrong.
                error!(
                    socket = %self.runtime_config.runtime_socket,
                    "could not bind the handler stream socket: {err}"
                );
                return Err(ApplicationStartError::Environment(format!(
                    "could not bind the handler stream socket at {}: {err}. Set \
                     CELERITY_RUNTIME_SOCKET_FALLBACK_ENABLED to serve it over loopback TCP \
                     instead, which lets any process that can reach loopback register as a \
                     handler",
                    self.runtime_config.runtime_socket
                )));
            }
            Err(err) => {
                let port = self.runtime_config.runtime_socket_fallback_port;
                warn!(
                    socket = %self.runtime_config.runtime_socket,
                    "could not bind a unix socket ({err}), serving the handler stream over \
                     loopback tcp on {port} as configured, which lets any process that can \
                     reach loopback register as a handler and be given events"
                );
                let addr = SocketAddr::from((std::net::Ipv4Addr::LOCALHOST, port));
                tokio::spawn(async move {
                    if let Err(err) = Server::builder()
                        .add_service(service)
                        .serve_with_shutdown(addr, shutdown)
                        .await
                    {
                        error!("handler stream server stopped: {err}");
                    }
                });
            }
        }

        self.ipc_server_shutdown_signal = Some(server_shutdown_tx);
        Ok(())
    }

    /// Joins this node to the others serving the same WebSocket API.
    ///
    /// Nothing here runs unless a Redis deployment is configured to cluster
    /// over, so an application with the feature compiled in serves a single
    /// node until it is told otherwise.
    #[cfg(feature = "ws_clustering")]
    async fn join_websocket_cluster(&mut self) -> Result<(), ApplicationStartError> {
        let (Some(cluster_config), Some(registry)) = (
            self.runtime_config.ws_cluster.clone(),
            self.ws_conn_registry.take(),
        ) else {
            return Ok(());
        };

        self.ws_cluster_shutdown_signal = Some(
            crate::websocket_cluster::join_cluster(
                registry,
                self.ws_seen_messages.take(),
                cluster_config,
                &self.runtime_config.service_name,
                &self.server_node_name,
            )
            .await?,
        );

        Ok(())
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

        // Started before anything serves requests, so that no event can be
        // dispatched before its deadline is being watched.
        if let Some(cleanup_task) = self.event_cleanup_task.take() {
            self.event_cleanup_task_shutdown_signal = Some(cleanup_task.spawn());
        }

        // Here rather than in `setup` for the same reason tracing and the
        // consumers are, spawning needs a runtime and `setup` is called from
        // outside one. Before anything serves, so that no message can go out
        // before there is anything waiting to hear it was received.
        if let Some(conn_registry) = self.ws_conn_registry.clone() {
            conn_registry.start_client_ack_worker();
        }

        if let Some(seen_messages) = self.ws_seen_messages.clone() {
            seen_messages.start_eviction();
        }

        #[cfg(feature = "ws_clustering")]
        self.join_websocket_cluster().await?;

        self.run_ipc_stream_server().await?;

        let mut server_task = None;
        let mut server_address = None;
        if let Some(http_app_unwrapped) = self.http_server_app.clone() {
            let (task, addr) = self.run_http_server_app(http_app_unwrapped).await;
            server_task = Some(task);
            server_address = Some(addr);
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
        if blueprint_config_path.ends_with(".bp") || blueprint_config_path.ends_with(".blueprint") {
            BlueprintConfig::from_blueprint_lang_file(blueprint_config_path, self.env_vars.clone())
        } else if blueprint_config_path.ends_with(".json")
            || blueprint_config_path.ends_with(".jsonc")
        {
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
        if let Some(tx) = self.ipc_server_shutdown_signal.take() {
            let _ = tx.send(());
        }
        if let Some(tx) = self.ipc_dispatcher_shutdown_signal.take() {
            let _ = tx.send(());
        }
        if let Some(tx) = self.event_cleanup_task_shutdown_signal.take() {
            // The cleanup task also stops when the arm channel closes, so a failed
            // send here just means it has already gone.
            let _ = tx.send(());
        }
        if let Some(tx) = self.resource_store_cleanup_task_shutdown_signal.take() {
            tx.send(())
                .expect("failed to send shutdown signal to resource store cleanup task");
        }
        #[cfg(feature = "ws_clustering")]
        if let Some(tx) = self.ws_cluster_shutdown_signal.take() {
            // The heartbeat leaves the node group and takes this node's
            // connection entries away when it sees this.
            let _ = tx.send(());
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

/// Registers a route that dispatches to a handler in the separate handlers
/// executable over the event queue.
///
/// Mirrors the method dispatch of `Application::register_http_handler`, which
/// serves the same purpose in the FFI call mode.
fn register_ipc_http_route(
    router: Router<ApiAppState>,
    method: &str,
    path: &str,
    route: IpcHttpRoute,
) -> Router<ApiAppState> {
    let handler = move |path_params: RawPathParams, request: Request| {
        let route = route.clone();
        async move { ipc_http::handle_request(route, path_params, request).await }
    };

    match method.to_lowercase().as_str() {
        "get" => router.route(path, get(handler)),
        "head" => router.route(path, axum::routing::head(handler)),
        "options" => router.route(path, axum::routing::options(handler)),
        "trace" => router.route(path, axum::routing::trace(handler)),
        "post" => router.route(path, post(handler)),
        "put" => router.route(path, axum::routing::put(handler)),
        "patch" => router.route(path, axum::routing::patch(handler)),
        "delete" => router.route(path, axum::routing::delete(handler)),
        unsupported => {
            warn!(
                method = %unsupported,
                path = %path,
                "skipping route registration for an unsupported HTTP method"
            );
            router
        }
    }
}

/// Binds the Unix socket the handler stream is served on.
///
/// The parent directory is created because the default path lives under
/// `/var/run`, which a container image may not have.
///
/// A socket file left behind by a previous run would make binding fail, so a
/// stale one is removed. One that another instance is still listening on is
/// not: removing it would leave that instance running on a socket nothing can
/// reach any more, and two runtimes silently fighting over a path is worse than
/// refusing to start.
async fn bind_runtime_socket(path: &str) -> std::io::Result<tokio::net::UnixListener> {
    let path = std::path::Path::new(path);
    if let Some(parent) = path.parent() {
        let existed = tokio::fs::try_exists(parent).await.unwrap_or(false);
        tokio::fs::create_dir_all(parent).await?;
        if !existed {
            restrict_runtime_socket_dir(parent).await?;
        }
    }

    if tokio::fs::try_exists(path).await.unwrap_or(false) {
        // Connecting is the only way to tell a live socket from a leftover
        // file, a refused connection means nothing is listening.
        if tokio::net::UnixStream::connect(path).await.is_ok() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::AddrInUse,
                format!("another runtime is already listening on {}", path.display()),
            ));
        }
        tokio::fs::remove_file(path).await?;
    }

    let listener = tokio::net::UnixListener::bind(path)?;
    restrict_runtime_socket(path).await?;
    Ok(listener)
}

/// Restricts the socket to the user the runtime runs as.
async fn restrict_runtime_socket(path: &std::path::Path) -> std::io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    tokio::fs::set_permissions(path, std::fs::Permissions::from_mode(0o600)).await
}

/// Restricts a directory the runtime created for its socket to the user the
/// runtime runs as.
async fn restrict_runtime_socket_dir(path: &std::path::Path) -> std::io::Result<()> {
    use std::os::unix::fs::PermissionsExt;

    tokio::fs::set_permissions(path, std::fs::Permissions::from_mode(0o700)).await
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use super::*;

    #[derive(Clone)]
    struct MapEnv(HashMap<&'static str, String>);

    impl EnvVars for MapEnv {
        fn var(&self, key: &str) -> Result<String, std::env::VarError> {
            self.0
                .get(key)
                .cloned()
                .ok_or(std::env::VarError::NotPresent)
        }

        fn clone_env_vars(&self) -> Box<dyn EnvVars> {
            Box::new(self.clone())
        }
    }

    /// The smallest environment a runtime config will accept, plus whatever a
    /// test is about.
    fn env(overrides: &[(&'static str, &str)]) -> MapEnv {
        let mut vars: HashMap<&'static str, String> = vec![
            ("CELERITY_BLUEPRINT", "blueprint.yaml".to_string()),
            ("CELERITY_SERVICE_NAME", "test".to_string()),
            ("CELERITY_SERVER_PORT", "8080".to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ipc".to_string()),
        ]
        .into_iter()
        .collect();
        for (key, value) in overrides {
            vars.insert(key, value.to_string());
        }
        MapEnv(vars)
    }

    /// A node with nothing to take a name from is given a new one every time it
    /// is asked, which is why the answer is settled once and kept.
    ///
    /// Working it out a second time would have the registry stamp one name on
    /// the messages it sends while the cluster knows this node by another, and
    /// an acknowledgement addressed to the first would be delivered to nobody.
    #[test]
    fn test_a_generated_node_name_is_a_different_one_every_time() {
        let vars = env(&[]);
        let config = RuntimeConfig::from_env(&vars);

        assert_ne!(
            resolve_server_node_name(&config, &vars),
            resolve_server_node_name(&config, &vars)
        );
    }

    /// Whatever the name is worked out from, the application settles on one and
    /// hands that same one to everything that needs it.
    #[test]
    fn test_an_application_keeps_the_name_it_settled_on() {
        let vars = env(&[]);
        let app = Application::new(RuntimeConfig::from_env(&vars), Box::new(vars.clone()));

        assert!(!app.server_node_name.is_empty());
        assert_ne!(
            app.server_node_name,
            resolve_server_node_name(&app.runtime_config, &vars),
            "a name worked out again is not the one the application is known by"
        );
    }

    /// A configured name is taken as it is, so nodes are named by whoever
    /// deployed them rather than by chance.
    #[test]
    fn test_a_configured_node_name_is_the_one_the_application_takes() {
        let vars = env(&[("CELERITY_SERVER_NODE_NAME", "node-a")]);
        let app = Application::new(RuntimeConfig::from_env(&vars), Box::new(vars.clone()));

        assert_eq!(app.server_node_name, "node-a");
    }

    /// Falling back to the hostname names a node after the machine running it,
    /// which is stable across everything that asks.
    #[test]
    fn test_a_node_falls_back_to_the_hostname_it_is_running_on() {
        let vars = env(&[("HOSTNAME", "container-7")]);
        let app = Application::new(RuntimeConfig::from_env(&vars), Box::new(vars.clone()));

        assert_eq!(app.server_node_name, "container-7");
    }
}
