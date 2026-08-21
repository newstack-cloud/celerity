use std::{
    collections::HashMap,
    fmt::{Debug, Display},
    net::IpAddr,
    ops::ControlFlow,
    sync::Arc,
    time::{Duration, Instant},
};

use crate::websocket_dedupe::MessageIdStore;
use async_trait::async_trait;
use axum::{
    extract::{
        ws::{close_code, CloseFrame, Message, WebSocket},
        FromRequestParts, Request, State, WebSocketUpgrade,
    },
    http::{HeaderMap, StatusCode},
    response::IntoResponse,
    Extension,
};
use axum_client_ip::ClientIp;
use axum_extra::{extract::CookieJar, headers, TypedHeader};
use celerity_blueprint_config_parser::blueprint::{
    CelerityApiAuth, CelerityApiAuthGuard, CelerityApiAuthGuardType, CelerityApiCors,
    WebSocketAuthStrategy,
};
use celerity_helpers::{
    http::ResourceStore,
    request::{headers_to_hashmap, query_from_uri},
    websockets::{encode_reserved_message, ReservedRoute},
};
use celerity_ws_registry::registry::{WebSocketConnRegistry, WebSocketConnSender};
use futures::{SinkExt, StreamExt};
use nanoid::nanoid;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::{
    sync::{mpsc, mpsc::error::SendTimeoutError, Mutex},
    task::{JoinHandle, JoinSet},
};
use tracing::{debug, error, field, info, info_span, warn, Instrument};

use crate::{
    auth_custom::{
        validate_custom_auth_on_connect, AuthGuardHandler, AuthGuardValidateError,
        CustomAuthRequestContext,
    },
    auth_jwt::{validate_jwt_on_ws_connect, ValidateJwtError},
    consts::{
        CELERITY_WS_ACK_SIGNAL, CELERITY_WS_CONNECT_HANDLER_ROUTE,
        CELERITY_WS_DEFAULT_MESSAGE_HANDLER_ROUTE, CELERITY_WS_DISCONNECT_HANDLER_ROUTE,
        CELERITY_WS_FORBIDDEN_ERROR_CODE, CELERITY_WS_UNAUTHORISED_ERROR_CODE,
        WS_CONNECTION_DRAIN_GRACE_MS, WS_CONNECTION_SATURATED_RETRY_AFTER_MS,
        WS_CONNECTION_WORK_BUFFER, WS_CONNECTION_WORK_SHED_GRACE_MS,
    },
    errors::WebSocketsMessageError,
    request::{HttpProtocolVersion, RequestId},
    telemetry_utils::extract_trace_context,
    utils::get_epoch_seconds,
};

/// Returns a lazily-initialised UpDownCounter for tracking active WebSocket connections.
/// Uses the global meter — returns no-op when metrics are disabled.
fn ws_connections_counter() -> opentelemetry::metrics::UpDownCounter<i64> {
    opentelemetry::global::meter("celerity_runtime")
        .i64_up_down_counter("ws.server.active_connections")
        .with_description("Number of active WebSocket connections")
        .init()
}

#[derive(Clone, Debug)]
pub(crate) struct WebSocketAppState {
    pub connections: Arc<WebSocketConnRegistry>,
    pub routes: Arc<Mutex<HashMap<String, Arc<dyn WebSocketMessageHandler + Send + Sync>>>>,
    pub route_key: String,
    pub api_auth: Option<CelerityApiAuth>,
    pub auth_strategy: Option<WebSocketAuthStrategy>,
    pub connection_auth_guard_names: Option<Vec<String>>,
    pub connection_auth_guards:
        Arc<Mutex<HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>>>>,
    // How many of this connection's messages may be handled at the same time.
    pub handler_concurrency: usize,
    pub cors: Option<CelerityApiCors>,
    pub resource_store: Arc<ResourceStore>,
    // What the node has already seen from its clients, so a message resent
    // because an acknowledgement went missing is not acted on twice.
    pub seen_messages: Arc<dyn MessageIdStore>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct MessageRequestContext {
    #[serde(rename = "requestId")]
    pub request_id: String,
    #[serde(rename = "requestTime")]
    pub request_time: u64,
    #[serde(rename = "path")]
    pub path: String,
    #[serde(rename = "protocolVersion")]
    pub protocol_version: HttpProtocolVersion,
    #[serde(rename = "headers")]
    pub headers: HashMap<String, Vec<String>>,
    #[serde(rename = "userAgent")]
    pub user_agent: Option<String>,
    #[serde(rename = "clientIp")]
    pub client_ip: String,
    #[serde(rename = "query")]
    pub query: HashMap<String, Vec<String>>,
    pub cookies: HashMap<String, String>,
    pub auth: Option<serde_json::Value>,
    #[serde(rename = "traceContext")]
    pub trace_context: Option<HashMap<String, String>>,
}

/// A JSON message received from a WebSocket client with additional information.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct JsonMessageInfo {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "eventType")]
    pub event_type: WebSocketEventType,
    #[serde(rename = "messageId")]
    pub message_id: String,
    #[serde(rename = "context")]
    pub request_ctx: Option<MessageRequestContext>,
    pub body: serde_json::Value,
    #[serde(rename = "traceContext")]
    pub trace_context: Option<HashMap<String, String>>,
}

/// A binary message received from a WebSocket client with additional information.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct BinaryMessageInfo<'a> {
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "eventType")]
    pub event_type: WebSocketEventType,
    #[serde(rename = "messageId")]
    pub message_id: String,
    #[serde(rename = "context")]
    pub request_ctx: Option<MessageRequestContext>,
    /// The body after stripping routing information from the beginning of the message.
    pub body: &'a [u8],
    #[serde(rename = "traceContext")]
    pub trace_context: Option<HashMap<String, String>>,
}

/// The type of event that occurred on the WebSocket connection.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub enum WebSocketEventType {
    #[serde(rename = "connect")]
    Connect,
    #[serde(rename = "message")]
    Message,
    #[serde(rename = "disconnect")]
    Disconnect,
}

#[async_trait]
pub trait WebSocketMessageHandler {
    async fn handle_json_message(
        &self,
        message: JsonMessageInfo,
    ) -> Result<(), WebSocketsMessageError>;
    async fn handle_binary_message<'a>(
        &self,
        message: BinaryMessageInfo<'a>,
    ) -> Result<(), WebSocketsMessageError>;
}

impl Debug for dyn WebSocketMessageHandler + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "WebSocketMessageHandler")
    }
}

#[derive(Clone)]
pub struct WebSocketRequestContext {
    pub request_id: RequestId,
    pub request_time: u64,
    pub path: String,
    pub protocol_version: HttpProtocolVersion,
    pub headers: HeaderMap,
    pub user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    pub client_ip: IpAddr,
    pub query: HashMap<String, Vec<String>>,
    pub cookies: CookieJar,
    pub trace_context: Option<HashMap<String, String>>,
    /// Authentication data from the Connect or AuthMessage strategy.
    /// Populated after successful authentication and propagated to all
    /// subsequent message handler invocations.
    pub auth_data: Option<serde_json::Value>,
}

/// Bundles the per-request Axum extractors for the WebSocket upgrade handler,
/// keeping `WebSocketUpgrade`, `State`, and `Request` as separate parameters.
#[derive(FromRequestParts)]
#[from_request(state(WebSocketAppState))]
pub(crate) struct WsHandlerParts {
    user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    headers: HeaderMap,
    #[from_request(via(Extension))]
    request_id: RequestId,
    client_ip: ClientIp,
    cookies: CookieJar,
}

pub(crate) async fn handler(
    ws: WebSocketUpgrade,
    State(state): State<WebSocketAppState>,
    WsHandlerParts {
        user_agent_header,
        headers,
        request_id,
        client_ip,
        cookies,
    }: WsHandlerParts,
    request: Request,
) -> impl IntoResponse {
    let _ = match user_agent_header.clone() {
        Some(header) => header.to_string(),
        None => "Unknown User Agent".to_string(),
    };
    let query = match query_from_uri(request.uri()) {
        Ok(query) => query,
        Err(e) => {
            warn!("failed to parse query from uri: {e}");
            return StatusCode::BAD_REQUEST.into_response();
        }
    };

    ws.on_upgrade(move |socket| {
        let request_ctx = WebSocketRequestContext {
            request_id: request_id.clone(),
            request_time: get_epoch_seconds(),
            path: request.uri().path().to_string(),
            protocol_version: HttpProtocolVersion::Http1_1,
            headers,
            user_agent_header,
            client_ip: client_ip.0,
            query,
            cookies,
            trace_context: extract_trace_context(),
            auth_data: None,
        };
        handle_socket(socket, request_id.0.clone(), request_ctx, state)
    })
}

async fn handle_socket(
    socket: WebSocket,
    connection_id: String,
    mut request_ctx: WebSocketRequestContext,
    state: WebSocketAppState,
) {
    // Split so that reading and writing are owned separately. Only the sending
    // half is shared, which is all anything other than this task ever needs,
    // and the receiving half stays here. Reading therefore never holds a lock,
    // so a task sending to this connection cannot delay it and it cannot delay
    // them.
    let (socket_tx, mut socket_rx) = socket.split();
    let socket_ref = Arc::new(Mutex::new(socket_tx));
    async {
        info!("websocket connection received: {}", connection_id);

        // Origin check for WebSocket upgrade requests (RFC 6455 §4.1).
        // Browsers MUST send the Origin header; non-browser clients MAY omit it.
        // When CORS is configured, we validate the Origin against allowed origins
        // for browser clients. Connections without an Origin header are assumed
        // to be server-side clients and are allowed through, since the purpose
        // of this check is to prevent third-party web origins — not to block
        // CLI tools, SDKs, or service-to-service connections.
        // When CORS is not configured at all, all connections are allowed.
        if let Some(cors) = &state.cors {
            if let Err(err) = check_cors_origin(cors, &request_ctx) {
                debug!("origin check failed, closing connection: {err}");
                close_connection(socket_ref.clone()).await;
                return;
            }
        }

        let mut auth_result_data = serde_json::Value::Null;
        let requires_auth_message =
            matches!(&state.auth_strategy, Some(WebSocketAuthStrategy::AuthMessage));
        let mut is_authenticated = !requires_auth_message;

        match &state.auth_strategy {
            Some(WebSocketAuthStrategy::Connect) => {
                // Connect strategy: auth happens during the upgrade.
                // $connect only fires if auth succeeds (below).
                let step_after_auth =
                    authenticate_connection(socket_ref.clone(), &state, &request_ctx).await;
                match step_after_auth {
                    ControlFlow::Continue(data) => {
                        auth_result_data = data.clone();
                        request_ctx.auth_data = Some(data);
                    }
                    ControlFlow::Break(_) => {
                        return;
                    }
                }
            }
            Some(WebSocketAuthStrategy::AuthMessage) => {
                // AuthMessage strategy: connection upgrades without auth.
                // $connect fires immediately (below) with null auth data.
                // Auth happens later when client sends an "authenticate" message.
            }
            _ => {} // No auth configured
        }

        // Register the connection only after CORS and auth checks pass.
        // Registering earlier would keep an Arc<WebSocket> in the registry
        // on early return, preventing the socket from being dropped and
        // causing the TCP connection to linger.
        state
            .connections
            .register_connection(connection_id.clone(), socket_ref.clone())
            .await;
        ws_connections_counter().add(1, &[]);

        // Send the capabilities signal, this is a binary frame that indicates the server
        // supports full protocol capabilities (binary messages, custom close codes,
        // binary control frames). In environments where binary frames are not
        // supported (e.g., managed WebSocket gateways that proxy via text-only APIs),
        // this frame will not reach the client, causing it to fall back to constrained
        // capabilities (text-only, JSON-format control frames).
        {
            let mut sock = socket_ref.lock().await;
            let _ = sock
                .send(Message::Binary(
                    encode_reserved_message(ReservedRoute::Capabilities, &[]).into(),
                ))
                .await;
        }

        // Authentication happened during the upgrade for the connect strategy, so
        // the client doesn't know about the auth status at this point.
        if matches!(&state.auth_strategy, Some(WebSocketAuthStrategy::Connect)) {
            let response = create_auth_response(
                true,
                Some(&auth_result_data).filter(|data| !data.is_null()),
                Some("Authenticated successfully"),
            );
            let mut sock = socket_ref.lock().await;
            if let Err(err) = sock.send(Message::Text(response.into())).await {
                error!("failed to tell client {connection_id} it was authenticated: {err}");
            }
        }

        // For Connect strategy, only reached after successful auth (with auth data).
        // For AuthMessage / no auth, fires immediately with null auth data.
        if let ControlFlow::Break(_) = on_connect(
            socket_ref.clone(),
            connection_id.clone(),
            &state,
            &request_ctx,
            auth_result_data.clone(),
        )
        .await
        {
            // Registered above so the connect handler could send to it, so the
            // registration has to be undone here. Left in place it would hold a
            // sender for a connection that was refused and count towards the
            // active connection gauge for the life of the process.
            //
            // No disconnect handler runs. The connect handler turned this
            // connection down, so from the application's side it never
            // connected.
            state
                .connections
                .deregister_connection(connection_id.clone())
                .await;
            ws_connections_counter().add(-1, &[]);
            return;
        }

        // Messages are processed off this loop by a worker for this
        // connection, several at a time unless the deployment asks for one. The
        // protocol promises nothing about ordering and a serverless target runs
        // a function per message with nothing between them, so handling them in
        // order here would be a guarantee only this target could keep.
        //
        // Bounded, so a client that outruns its handlers is pushed back on
        // rather than growing a queue without limit. Once the buffer is full
        // this loop does wait, which is the intended answer to a flood, and a
        // different situation from one slow handler.
        let (work_tx, work_rx) = mpsc::channel::<(Message, Option<Value>, WebSocketRequestContext)>(
            WS_CONNECTION_WORK_BUFFER,
        );
        let mut worker = spawn_message_worker(
            work_rx,
            socket_ref.clone(),
            connection_id.clone(),
            state.clone(),
            state.handler_concurrency,
        );

        let mut connection_alive = true;
        while connection_alive {
            if let Some(Ok(msg)) = socket_rx.next().await {
                // Parsed once here and carried from here on. A JSON string
                // may escape any character, so only a parse can tell what a
                // message says. Held as the parse result rather than the value
                // so that a failure is still reported where it was, past the
                // authentication gate.
                let parsed = match &msg {
                    Message::Text(text) => Some(serde_json::from_str::<Value>(text)),
                    _ => None,
                };

                // Handle Celerity application-level heartbeat pings before
                // route resolution. These are distinct from WebSocket protocol-level
                // Ping frames (handled by tungstenite automatically).
                if let Some(pong) =
                    detect_heartbeat_ping(&msg, parsed.as_ref().and_then(|res| res.as_ref().ok()))
                {
                    let _ = socket_ref.lock().await.send(pong).await;
                    continue;
                }

                if !is_authenticated {
                    // Connection is in unauthenticated state (authMessage strategy).
                    // Only accept an "authenticate" message; reject everything else.
                    // Scoped so the guard is dropped before the arms run. A
                    // guard taken in the arm lives until the end of the
                    // match, and an arm below takes the same lock, which does
                    // not nest.
                    let auth_result = {
                        let mut socket = socket_ref.lock().await;
                        handle_auth_message(&msg, &connection_id, &state, &request_ctx, &mut socket)
                            .await
                    };
                    match auth_result {
                        AuthMessageResult::Authenticated(data) => {
                            is_authenticated = true;
                            request_ctx.auth_data = Some(data);
                        }
                        AuthMessageResult::Failed => {
                            connection_alive = false;
                        }
                        AuthMessageResult::NotAuthMessage => {
                            let reject = serde_json::json!({
                                "event": "error",
                                "data": {
                                    "message": "Authentication required. Send an authenticate message first."
                                }
                            })
                            .to_string();
                            let _ = socket_ref
                                .lock()
                                .await
                                .send(Message::Text(reject.into()))
                                .await;
                        }
                    }
                } else {
                    let parsed = match parsed {
                        Some(Ok(value)) => Some(value),
                        Some(Err(err)) => {
                            error!(
                                "failed to parse JSON message from client \
                                 {connection_id}: {err}"
                            );
                            None
                        }
                        None => None,
                    };

                    // A client acknowledging something the runtime sent it,
                    // taken before routing so it is not mistaken for a routed
                    // message and handed to the default handler.
                    //
                    // Only from a connection that has authenticated, since an
                    // acknowledgement calls off a resend and the loss event
                    // that follows it, and that is not something an unproven
                    // client should be able to do to a message.
                    if let Some(acknowledged) = detect_client_ack(&msg, parsed.as_ref()) {
                        debug!("client {connection_id} acknowledged message {acknowledged}");
                        state
                            .connections
                            .record_client_ack(connection_id.clone(), acknowledged)
                            .await;
                        continue;
                    }

                    // Handed to the worker rather than run here. A handler that
                    // takes a while must not stop this loop reading, or the
                    // heartbeat above goes unanswered and a close frame goes
                    // unnoticed, and the client concludes the connection is
                    // dead while its work is still in progress.
                    //
                    // The wait is bounded. A brief burst is absorbed by the
                    // buffer and by this grace, but a client that stays ahead
                    // of its handlers for longer than this is not going to be
                    // served by waiting for it in silence, since waiting is
                    // what stops the heartbeat being answered. It is shed
                    // instead, with a hint for when to come back.
                    // Read before the message is handed on, since handing it on
                    // gives it away.
                    let ack = ack_request(&msg, parsed.as_ref());

                    match work_tx
                        .send_timeout(
                            (msg, parsed, request_ctx.clone()),
                            Duration::from_millis(WS_CONNECTION_WORK_SHED_GRACE_MS),
                        )
                        .await
                    {
                        Ok(()) => {
                            // Answered as soon as the message is taken in, since
                            // that is what an acknowledgement says. Anything
                            // that happens to it afterwards, however long it
                            // takes, is not what the client is waiting to hear.
                            //
                            // Sent once it is safely queued and not before, so
                            // nothing is acknowledged that then gets shed to
                            // process incoming messages.
                            if let Some((message_id, format)) = ack {
                                acknowledge(&socket_ref, &connection_id, &message_id, format).await;
                            }
                        }
                        Err(SendTimeoutError::Timeout(_)) => {
                            warn!(
                                "client {connection_id} is sending faster than its handlers can \
                                 keep up, closing the connection with a retry hint"
                            );
                            close_with_retry_after(
                                socket_ref.clone(),
                                WS_CONNECTION_SATURATED_RETRY_AFTER_MS,
                            )
                            .await;
                            connection_alive = false;
                        }
                        Err(SendTimeoutError::Closed(_)) => {
                            connection_alive = false;
                        }
                    }
                }
            } else {
                // recv() returned None — the underlying connection was dropped
                // without a WebSocket close handshake (e.g. TCP reset, client
                // killed).
                info!("connection lost, client {connection_id} disconnected without close frame");
                connection_alive = false;
            }
        }

        // Closing the channel ends the worker once it has finished whatever it
        // was given, so the disconnect handler runs after the messages that
        // preceded it rather than alongside them.
        //
        // The wait is bounded but the work is not cut short. Each message the
        // worker still holds waits on its own handler timeout, and waiting for
        // all of them in turn would leave a connection that has already gone
        // sitting in the registry, counted by the gauge, with its disconnect
        // handler unfired. So this stops waiting and lets the worker carry on
        // in its own time. A message the client already sent is closer to a
        // queue message than to a request, and a handler part way through
        // persisting one should finish whether or not anyone is still there to
        // hear about it.
        drop(work_tx);
        if tokio::time::timeout(
            Duration::from_millis(WS_CONNECTION_DRAIN_GRACE_MS),
            &mut worker,
        )
        .await
        .is_err()
        {
            warn!(
                "client {connection_id} left work still running after the drain window, \
                 closing the connection out while it finishes"
            );
        }

        let _ = on_disconnect(connection_id.clone(), &state, &request_ctx).await;
        state
            .connections
            .deregister_connection(connection_id.clone())
            .await;
        ws_connections_counter().add(-1, &[]);
    }
    .instrument(info_span!("websocket_connection", connection_id = %connection_id))
    .await
}

async fn authenticate_connection(
    socket_ref: Arc<Mutex<WebSocketConnSender>>,
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
) -> ControlFlow<(), serde_json::Value> {
    let guard_names = match &state.connection_auth_guard_names {
        Some(names) if !names.is_empty() => names,
        _ => return ControlFlow::Continue(serde_json::Value::Null),
    };

    let mut accumulated_claims = serde_json::Map::new();

    for guard_name in guard_names {
        let auth_guard_config = match find_auth_guard_config(guard_name, &state.api_auth) {
            Some(config) => config,
            None => {
                warn!("auth guard config not found for guard: {guard_name}");
                return handle_validate_auth_on_connect_error(
                    socket_ref,
                    ValidateAuthError::Custom(AuthGuardValidateError::UnexpectedError(format!(
                        "guard config not found for \"{guard_name}\""
                    ))),
                    guard_name,
                )
                .await;
            }
        };

        match auth_guard_config.guard_type {
            CelerityApiAuthGuardType::Jwt => {
                match validate_jwt_on_ws_connect(
                    auth_guard_config,
                    &request_ctx.headers,
                    &request_ctx.query,
                    &request_ctx.cookies,
                    state.resource_store.clone(),
                )
                .await
                {
                    Ok(data) => {
                        accumulated_claims.insert(guard_name.clone(), data);
                    }
                    Err(e) => {
                        return handle_validate_auth_on_connect_error(
                            socket_ref,
                            ValidateAuthError::Jwt(e),
                            "JWT",
                        )
                        .await;
                    }
                }
            }
            CelerityApiAuthGuardType::Custom => {
                let guard_handler = {
                    let guards = state.connection_auth_guards.lock().await;
                    guards.get(guard_name).cloned()
                };
                match validate_custom_auth_on_connect(
                    auth_guard_config,
                    CustomAuthRequestContext {
                        method: "GET",
                        path: &request_ctx.path,
                        headers: &request_ctx.headers,
                        query: &request_ctx.query,
                        cookies: &request_ctx.cookies,
                        request_id: &request_ctx.request_id,
                        client_ip: &request_ctx.client_ip,
                    },
                    guard_handler,
                    &accumulated_claims,
                )
                .await
                {
                    Ok(data) => {
                        accumulated_claims.insert(guard_name.clone(), data);
                    }
                    Err(err) => {
                        return handle_validate_auth_on_connect_error(
                            socket_ref,
                            ValidateAuthError::Custom(err),
                            "custom auth guard",
                        )
                        .await;
                    }
                }
            }
            CelerityApiAuthGuardType::NoGuardType => {
                debug!("no auth guard type configured for guard \"{guard_name}\", skipping");
            }
        }
    }

    ControlFlow::Continue(serde_json::Value::Object(accumulated_claims))
}

async fn on_connect(
    socket_ref: Arc<Mutex<WebSocketConnSender>>,
    connection_id: String,
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
    auth_result_data: serde_json::Value,
) -> ControlFlow<(), ()> {
    if let Some(connect_handler) = state
        .routes
        .lock()
        .await
        .get(CELERITY_WS_CONNECT_HANDLER_ROUTE)
    {
        async {
            if let Err(err) = connect_handler
                .handle_json_message(create_connect_message(
                    connection_id,
                    request_ctx,
                    auth_result_data,
                ))
                .await
            {
                tracing::Span::current().record("otel.status_code", "ERROR");
                error!("connect handler failed, closing connection: {err}");
                close_connection(socket_ref.clone()).await;
                ControlFlow::Break(())
            } else {
                ControlFlow::Continue(())
            }
        }
        .instrument(info_span!("on_connect", route = %CELERITY_WS_CONNECT_HANDLER_ROUTE, otel.status_code = field::Empty))
        .await
    } else {
        ControlFlow::Continue(())
    }
}

fn create_connect_message(
    connection_id: String,
    request_ctx: &WebSocketRequestContext,
    auth_result_data: serde_json::Value,
) -> JsonMessageInfo {
    JsonMessageInfo {
        connection_id,
        event_type: WebSocketEventType::Connect,
        message_id: "".to_string(),
        request_ctx: Some(create_message_request_context(
            request_ctx,
            Some(auth_result_data),
        )),
        body: serde_json::Value::Null,
        trace_context: extract_trace_context(),
    }
}

/// Runs a connection's messages off its read loop, up to `concurrency` of them
/// at a time.
///
/// A close is settled here rather than alongside the messages it follows, so
/// the connection ends after them however many were in flight.
fn spawn_message_worker(
    mut work_rx: mpsc::Receiver<(Message, Option<Value>, WebSocketRequestContext)>,
    socket_ref: Arc<Mutex<WebSocketConnSender>>,
    connection_id: String,
    state: WebSocketAppState,
    concurrency: usize,
) -> JoinHandle<()> {
    tokio::spawn(async move {
        let mut in_flight = JoinSet::new();

        while let Some((message, parsed, request_ctx)) = work_rx.recv().await {
            // Only an application message is worth running beside another. A
            // close ends the connection and the rest carry no handler, so they
            // wait for what came before them and are settled here. At a
            // concurrency of one nothing is ever in flight and this is the only
            // path, which is what keeps the default exactly as it was.
            let alongside_others =
                concurrency > 1 && matches!(message, Message::Text(_) | Message::Binary(_));

            if !alongside_others {
                while in_flight.join_next().await.is_some() {}
                if process_message(message, parsed, connection_id.clone(), request_ctx, &state)
                    .await
                    .is_break()
                {
                    // The connection is finished, and the read loop is waiting on a
                    // socket that may never produce anything again. Closing it is
                    // what wakes the loop so the connection can be torn down.
                    close_connection(socket_ref.clone()).await;
                    return;
                }
                continue;
            }

            // Waits for room rather than queueing behind it, so the buffer the
            // read loop pushes back on stays the one thing bounding how far a
            // client can run ahead.
            while in_flight.len() >= concurrency {
                in_flight.join_next().await;
            }

            let connection_id = connection_id.clone();
            let state = state.clone();
            in_flight.spawn(async move {
                // Only a close asks for the connection to end, and a close is
                // never spawned, so there is no outcome to read here.
                let _ = process_message(message, parsed, connection_id, request_ctx, &state).await;
            });
        }

        // The read loop has finished with this connection. What is still
        // running is left to finish, since teardown waits on this task and
        // bounds that wait itself.
        while in_flight.join_next().await.is_some() {}
    })
}

async fn on_disconnect(
    connection_id: String,
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
) -> ControlFlow<(), ()> {
    if let Some(disconnect_handler) = state
        .routes
        .lock()
        .await
        .get(CELERITY_WS_DISCONNECT_HANDLER_ROUTE)
    {
        async {
            if let Err(err) = disconnect_handler
                .handle_json_message(create_disconnect_message(connection_id, request_ctx))
                .await
            {
                error!("disconnect handler failed: {err}");
                ControlFlow::Break(())
            } else {
                ControlFlow::Continue(())
            }
        }
        .instrument(info_span!("on_disconnect", route = %CELERITY_WS_DISCONNECT_HANDLER_ROUTE))
        .await
    } else {
        ControlFlow::Continue(())
    }
}

fn create_disconnect_message(
    connection_id: String,
    request_ctx: &WebSocketRequestContext,
) -> JsonMessageInfo {
    JsonMessageInfo {
        connection_id,
        event_type: WebSocketEventType::Disconnect,
        message_id: "".to_string(),
        request_ctx: Some(create_message_request_context(request_ctx, None)),
        body: serde_json::Value::Null,
        trace_context: extract_trace_context(),
    }
}

fn create_message_request_context(
    request_ctx: &WebSocketRequestContext,
    auth_override: Option<serde_json::Value>,
) -> MessageRequestContext {
    let headers = headers_to_hashmap(&request_ctx.headers);

    let cookies = request_ctx
        .cookies
        .iter()
        .map(|cookie| (cookie.name().to_string(), cookie.value().to_string()))
        .collect();

    MessageRequestContext {
        request_id: request_ctx.request_id.0.clone(),
        request_time: request_ctx.request_time,
        path: request_ctx.path.clone(),
        protocol_version: request_ctx.protocol_version.clone(),
        headers,
        user_agent: request_ctx
            .user_agent_header
            .as_ref()
            .map(|h| h.to_string()),
        client_ip: request_ctx.client_ip.to_string(),
        query: request_ctx.query.clone(),
        cookies,
        // Use explicit override if provided (e.g. on_connect passes fresh auth data),
        // otherwise use the auth data stored on the request context.
        auth: auth_override.or_else(|| request_ctx.auth_data.clone()),
        trace_context: extract_trace_context(),
    }
}

#[derive(Debug)]
enum ValidateAuthError {
    Jwt(ValidateJwtError),
    Custom(AuthGuardValidateError),
}

impl Display for ValidateAuthError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            ValidateAuthError::Jwt(e) => write!(f, "JWT: {e}"),
            ValidateAuthError::Custom(e) => write!(f, "Custom: {e}"),
        }
    }
}

async fn handle_validate_auth_on_connect_error(
    socket_ref: Arc<Mutex<WebSocketConnSender>>,
    validate_error: ValidateAuthError,
    token_type: &str,
) -> ControlFlow<(), serde_json::Value> {
    warn!("failed to validate {token_type} on connect: {validate_error}");
    let mut socket = socket_ref.lock().await;
    let message = match validate_error {
        ValidateAuthError::Jwt(_) => unauthorised_error_close_message(),
        ValidateAuthError::Custom(AuthGuardValidateError::Unauthorised(err)) => {
            debug!("unauthorised error: {err}");
            unauthorised_error_close_message()
        }
        ValidateAuthError::Custom(AuthGuardValidateError::Forbidden(err)) => {
            debug!("forbidden error: {err}");
            forbidden_error_close_message()
        }
        ValidateAuthError::Custom(AuthGuardValidateError::UnexpectedError(err)) => {
            error!("custom auth guard validation failed with unexpected error: {err}");
            Message::Close(None)
        }
        ValidateAuthError::Custom(AuthGuardValidateError::ExtractTokenFailed(err)) => {
            error!("custom auth guard validation failed with extract token failed error: {err}");
            Message::Close(None)
        }
        ValidateAuthError::Custom(AuthGuardValidateError::TokenSourceMissing) => {
            error!("custom auth guard validation failed with token source missing error");
            Message::Close(None)
        }
    };
    if let Err(err) = socket.send(message).await {
        error!(
            "failed to send authentication error close frame to client: {}",
            err
        );
        if let Err(err) = socket.send(Message::Close(None)).await {
            error!("failed to close connection to client: {err}");
        }
        return ControlFlow::Break(());
    }
    ControlFlow::Break(())
}

fn unauthorised_error_close_message() -> Message {
    Message::Close(Some(CloseFrame {
        code: CELERITY_WS_UNAUTHORISED_ERROR_CODE,
        reason: "Authentication failed".into(),
    }))
}

fn forbidden_error_close_message() -> Message {
    Message::Close(Some(CloseFrame {
        code: CELERITY_WS_FORBIDDEN_ERROR_CODE,
        reason: "Forbidden".into(),
    }))
}

// ---------- authMessage strategy ----------

enum AuthMessageResult {
    /// Auth succeeded; carries the validated claims/user info.
    Authenticated(serde_json::Value),
    /// Auth failed; the handler already sent failure + close frames.
    Failed,
    /// The message was not an "authenticate" message.
    NotAuthMessage,
}

/// Handles a message received while the connection is in the unauthenticated
/// state (`authMessage` strategy). If the message matches the "authenticate"
/// route, the token is extracted and validated through the guard chain.
async fn handle_auth_message(
    msg: &Message,
    connection_id: &str,
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
    socket: &mut WebSocketConnSender,
) -> AuthMessageResult {
    let text = match msg {
        Message::Text(t) => t.to_string(),
        _ => return AuthMessageResult::NotAuthMessage,
    };

    let data: serde_json::Value = match serde_json::from_str(&text) {
        Ok(v) => v,
        Err(_) => return AuthMessageResult::NotAuthMessage,
    };

    let route = data.get(&state.route_key).and_then(|v| v.as_str());
    if route != Some("authenticate") {
        return AuthMessageResult::NotAuthMessage;
    }

    // Extract token from $.data.token in the message body.
    let token = data
        .pointer("/data/token")
        .and_then(|v| v.as_str())
        .map(|s| s.to_string());

    let token = match token {
        Some(t) => t,
        None => {
            let fail_msg = create_auth_response(false, None, Some("Token not found in message"));
            let _ = socket.send(Message::Text(fail_msg.into())).await;
            let _ = socket.send(Message::Close(None)).await;
            return AuthMessageResult::Failed;
        }
    };

    // Validate through the guard chain by creating a synthetic request context
    // with the token placed in an Authorization header so the existing validation
    // functions can extract it from their configured tokenSource.
    match validate_auth_message_token(&token, state, request_ctx).await {
        Ok(auth_data) => {
            let success_msg = create_auth_response(true, Some(&auth_data), None);
            let _ = socket.send(Message::Text(success_msg.into())).await;
            AuthMessageResult::Authenticated(auth_data)
        }
        Err(e) => {
            warn!(
                connection_id = %connection_id,
                "authMessage validation failed: {e}",
            );
            let fail_msg = create_auth_response(false, None, Some("Authentication failed"));
            let _ = socket.send(Message::Text(fail_msg.into())).await;
            let _ = socket.send(Message::Close(None)).await;
            AuthMessageResult::Failed
        }
    }
}

fn create_auth_response(
    success: bool,
    auth_data: Option<&serde_json::Value>,
    message: Option<&str>,
) -> String {
    let mut data = serde_json::json!({"success": success});
    if let Some(auth) = auth_data {
        data["userInfo"] = auth.clone();
    }
    if let Some(msg) = message {
        data["message"] = serde_json::Value::String(msg.to_string());
    }
    serde_json::json!({"event": "authenticated", "data": data}).to_string()
}

/// Validates the token extracted from an authMessage by running it through
/// the configured guard chain. Creates a synthetic `HeaderMap` with
/// `Authorization: Bearer <token>` so the existing JWT/custom guard
/// validation functions can extract the token from their configured source.
async fn validate_auth_message_token(
    token: &str,
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
) -> Result<serde_json::Value, ValidateAuthError> {
    let guard_names = match &state.connection_auth_guard_names {
        Some(names) if !names.is_empty() => names,
        _ => {
            // No guards configured — fall through to the default guard.
            let default_guards = state
                .api_auth
                .as_ref()
                .and_then(|auth| auth.default_guard.as_ref());
            if let Some(defaults) = default_guards {
                return validate_auth_message_token_with_guards(
                    token,
                    defaults,
                    state,
                    request_ctx,
                )
                .await;
            }
            return Ok(serde_json::Value::Null);
        }
    };

    validate_auth_message_token_with_guards(token, guard_names, state, request_ctx).await
}

async fn validate_auth_message_token_with_guards(
    token: &str,
    guard_names: &[String],
    state: &WebSocketAppState,
    request_ctx: &WebSocketRequestContext,
) -> Result<serde_json::Value, ValidateAuthError> {
    // Build a synthetic HeaderMap with the token as a Bearer token.
    let mut synthetic_headers = HeaderMap::new();
    if let Ok(val) = format!("Bearer {token}").parse() {
        synthetic_headers.insert("authorization", val);
    }

    let empty_query: HashMap<String, Vec<String>> = HashMap::new();
    let empty_cookies = CookieJar::new();

    let mut accumulated_claims = serde_json::Map::new();

    for guard_name in guard_names {
        let auth_guard_config = match find_auth_guard_config(guard_name, &state.api_auth) {
            Some(config) => config,
            None => {
                warn!("auth guard config not found for guard: {guard_name}");
                return Err(ValidateAuthError::Custom(
                    AuthGuardValidateError::UnexpectedError(format!(
                        "guard config not found for \"{guard_name}\""
                    )),
                ));
            }
        };

        match auth_guard_config.guard_type {
            CelerityApiAuthGuardType::Jwt => {
                match validate_jwt_on_ws_connect(
                    auth_guard_config,
                    &synthetic_headers,
                    &empty_query,
                    &empty_cookies,
                    state.resource_store.clone(),
                )
                .await
                {
                    Ok(data) => {
                        accumulated_claims.insert(guard_name.clone(), data);
                    }
                    Err(e) => {
                        return Err(ValidateAuthError::Jwt(e));
                    }
                }
            }
            CelerityApiAuthGuardType::Custom => {
                let guard_handler = {
                    let guards = state.connection_auth_guards.lock().await;
                    guards.get(guard_name).cloned()
                };
                match validate_custom_auth_on_connect(
                    auth_guard_config,
                    CustomAuthRequestContext {
                        method: "GET",
                        path: &request_ctx.path,
                        headers: &synthetic_headers,
                        query: &empty_query,
                        cookies: &empty_cookies,
                        request_id: &request_ctx.request_id,
                        client_ip: &request_ctx.client_ip,
                    },
                    guard_handler,
                    &accumulated_claims,
                )
                .await
                {
                    Ok(data) => {
                        accumulated_claims.insert(guard_name.clone(), data);
                    }
                    Err(err) => {
                        return Err(ValidateAuthError::Custom(err));
                    }
                }
            }
            CelerityApiAuthGuardType::NoGuardType => {
                debug!("no auth guard type configured for guard \"{guard_name}\", skipping");
            }
        }
    }

    Ok(serde_json::Value::Object(accumulated_claims))
}

/// Whether a message has already been acted on, so a resend of it is not acted
/// on again.
///
/// Only asked of a message the client gave an id to. An id the runtime made up
/// for one that carried none is unique by construction, so it would never match
/// anything, and remembering it would fill the store with entries nothing can
/// look up.
///
/// This runs after the acknowledgement has gone out, which is the order it has
/// to be in. A client resends because it did not hear one, and the thing that
/// went missing may have been the acknowledgement rather than the message.
/// Staying silent about a duplicate would leave it resending the message the
/// runtime is refusing to answer for.
async fn already_acted_on(
    message_id: &Option<String>,
    connection_id: &str,
    state: &WebSocketAppState,
) -> bool {
    let Some(message_id) = message_id else {
        return false;
    };

    if state.seen_messages.record_and_check_seen(message_id).await {
        debug!(
            "client {connection_id} sent message {message_id} again, which has already been \
             handled, so it is not handled a second time"
        );
        return true;
    }

    false
}

async fn process_message(
    msg: Message,
    parsed: Option<Value>,
    connection_id: String,
    request_ctx: WebSocketRequestContext,
    state: &WebSocketAppState,
) -> ControlFlow<(), ()> {
    match msg {
        Message::Text(_) => {
            let resolved = resolve_route(parsed, connection_id.clone(), state.route_key.clone())?;
            if let Some((route, message_id, _requires_ack, data)) = resolved {
                if already_acted_on(&message_id, &connection_id, state).await {
                    return ControlFlow::Continue(());
                }
                if let Some(handler) = get_message_route_handler(&route, state).await {
                    handle_json_message(
                        handler.clone(),
                        connection_id.clone(),
                        route.clone(),
                        message_id,
                        data,
                        request_ctx,
                    )
                    .await;
                } else {
                    error!(
                        "no handler found for route `{}` in WebSocket text message from client {}",
                        route, connection_id
                    );
                }
            }
        }
        Message::Binary(bytes) => {
            let resolved = resolve_binary_route(&bytes, connection_id.clone())?;
            if let Some((route, message_id, _requires_ack, bytes_stripped)) = resolved {
                if already_acted_on(&message_id, &connection_id, state).await {
                    return ControlFlow::Continue(());
                }
                if let Some(handler) = get_message_route_handler(&route, state).await {
                    handle_binary_message(
                        handler.clone(),
                        connection_id.clone(),
                        route.clone(),
                        message_id,
                        bytes_stripped,
                        request_ctx,
                    )
                    .await;
                } else {
                    error!(
                        "no handler found for route `{route}` in WebSocket binary message from client {connection_id}",
                    );
                }
            }
        }
        Message::Close(close) => {
            let info_msg = match close {
                Some(close_frame) => {
                    format!(
                        "connection closed, client {connection_id} sent close with code {code} and reason `{reason}`",
                        code = close_frame.code,
                        reason = close_frame.reason,
                    )
                }
                None => {
                    format!(
                        "connection closed, client {connection_id} sent close without close frame",
                    )
                }
            };
            info!(info_msg);
            // The disconnect handler is fired by the connection's teardown
            // rather than here, so that it runs exactly once however the
            // connection ends and whether or not this worker is still going.
            return ControlFlow::Break(());
        }
        Message::Ping(_) | Message::Pong(_) => {
            // WebSocket protocol-level ping/pong — handled automatically by tungstenite.
        }
    }
    ControlFlow::Continue(())
}

/// Detects Celerity application-level heartbeat pings and returns the
/// corresponding pong message. Returns `None` if the message is not a ping.
///
/// Supported formats:
/// - JSON: `{"ping": true}` → responds with `{"pong": true}`
/// - Binary: `[0x1, 0x1, 0x0, 0x0]` → responds with `[0x1, 0x2, 0x0, 0x0]`
fn detect_heartbeat_ping(msg: &Message, parsed: Option<&Value>) -> Option<Message> {
    match msg {
        Message::Text(_) => {
            if parsed?.get("ping") == Some(&Value::Bool(true)) {
                let pong = serde_json::json!({"pong": true}).to_string();
                return Some(Message::Text(pong.into()));
            }
            None
        }
        Message::Binary(bytes) => {
            if bytes.len() == 4
                && bytes[0] == 0x1
                && bytes[1] == 0x1
                && bytes[2] == 0x0
                && bytes[3] == 0x0
            {
                return Some(Message::Binary(
                    encode_reserved_message(ReservedRoute::Pong, &[]).into(),
                ));
            }
            None
        }
        _ => None,
    }
}

async fn get_message_route_handler(
    route: &str,
    state: &WebSocketAppState,
) -> Option<Arc<dyn WebSocketMessageHandler + Send + Sync>> {
    if let Some(handler) = state.routes.lock().await.get(route) {
        return Some(handler.clone());
    }

    if let Some(default_handler) = state
        .routes
        .lock()
        .await
        .get(CELERITY_WS_DEFAULT_MESSAGE_HANDLER_ROUTE)
    {
        Some(default_handler.clone())
    } else {
        None
    }
}

async fn handle_json_message(
    handler: Arc<dyn WebSocketMessageHandler + Send + Sync>,
    connection_id: String,
    route: String,
    message_id: Option<String>,
    data: Value,
    request_ctx: WebSocketRequestContext,
) {
    let final_message_id = message_id.unwrap_or_else(|| nanoid!());
    async {
        info!("JSON websocket message received");
        let start = Instant::now();
        let result = handler
            .handle_json_message(JsonMessageInfo {
                connection_id: connection_id.clone(),
                event_type: WebSocketEventType::Message,
                message_id: final_message_id.clone(),
                request_ctx: Some(create_message_request_context(&request_ctx, None)),
                body: data,
                trace_context: extract_trace_context(),
            })
            .await;

        let success = result.is_ok();
        if let Err(e) = result {
            tracing::Span::current().record("otel.status_code", "ERROR");
            error!(
                "failed to handle websocket message from client {}: {}",
                connection_id, e
            );
        }
        log_message_processing_finished(start.elapsed(), success);
    }
    .instrument(info_span!(
        "websocket_json_message",
        message_id = %final_message_id,
        route = %route,
        otel.status_code = field::Empty,
    ))
    .await;
}

async fn handle_binary_message(
    handler: Arc<dyn WebSocketMessageHandler + Send + Sync>,
    connection_id: String,
    route: String,
    message_id: Option<String>,
    data: &[u8],
    request_ctx: WebSocketRequestContext,
) {
    let final_message_id = message_id.unwrap_or_else(|| nanoid!());
    async {
        info!("binary websocket message received");
        let start = Instant::now();
        let result = handler
            .handle_binary_message(BinaryMessageInfo {
                connection_id: connection_id.clone(),
                event_type: WebSocketEventType::Message,
                message_id: final_message_id.clone(),
                request_ctx: Some(create_message_request_context(&request_ctx, None)),
                body: data,
                trace_context: extract_trace_context(),
            })
            .await;

        let success = result.is_ok();
        if let Err(e) = result {
            tracing::Span::current().record("otel.status_code", "ERROR");
            error!(
                "failed to handle websocket message from client {}: {}",
                connection_id, e
            );
        }
        log_message_processing_finished(start.elapsed(), success);
    }
    .instrument(info_span!(
        "websocket_binary_message",
        message_id = %final_message_id,
        route = %route,
        otel.status_code = field::Empty,
    ))
    .await;
}

/// Which encoding an acknowledgement goes back in.
///
/// It mirrors whatever the message being acknowledged arrived as. A client that
/// can send binary can read it, and one that only has text, because its
/// environment gave it no choice, gets text back.
#[derive(Clone, Copy, Debug)]
enum AckFormat {
    Json,
    Binary,
}

/// Recognises a client acknowledging something the runtime sent it, returning
/// the id of the message being acknowledged.
///
/// Reads both encodings, the reserved `0x4` binary frame and the JSON form
/// carrying `ack` as the value of `event`. That key is fixed by the protocol
/// rather than being the API's configured route key, which is what makes these
/// messages unroutable and why they have to be recognised here.
fn detect_client_ack(msg: &Message, parsed: Option<&Value>) -> Option<String> {
    let body = match msg {
        Message::Binary(bytes) => {
            if !bytes.starts_with(&CELERITY_WS_ACK_SIGNAL) {
                return None;
            }
            let value: Value = serde_json::from_slice(bytes.get(4..)?).ok()?;
            value
        }
        Message::Text(_) => {
            let value = parsed?;
            if value.get("event").and_then(Value::as_str) != Some("ack") {
                return None;
            }
            value.get("data")?.clone()
        }
        _ => return None,
    };

    body.get("messageId")
        .and_then(Value::as_str)
        .map(str::to_string)
}

/// Recognises a message asking to be acknowledged, returning the id to
/// acknowledge it by and the encoding to answer in.
///
/// Read off the frame rather than out of the routing, so a message can be
/// answered as soon as it is taken in. Asking without a message id returns
/// nothing, since an acknowledgement names the message it is for and there
/// would be nothing to name.
fn ack_request(msg: &Message, parsed: Option<&Value>) -> Option<(String, AckFormat)> {
    match msg {
        Message::Text(_) => {
            let object = parsed?.as_object()?;
            if object.get("ack").and_then(Value::as_bool) != Some(true) {
                return None;
            }
            let message_id = object.get("messageId").and_then(Value::as_str)?;
            Some((message_id.to_string(), AckFormat::Json))
        }
        Message::Binary(bytes) => {
            // `[routeLength][route][requireAck][messageIdLength][messageId]`,
            // read far enough to answer and no further.
            let route_length = *bytes.first()? as usize;
            let requires_ack = *bytes.get(1 + route_length)? == 0x1;
            if !requires_ack {
                return None;
            }
            let message_id_length = *bytes.get(route_length + 2)? as usize;
            let start = route_length + 3;
            let message_id = bytes.get(start..start + message_id_length)?;
            let message_id = std::str::from_utf8(message_id).ok()?;
            if message_id.is_empty() {
                return None;
            }
            Some((message_id.to_string(), AckFormat::Binary))
        }
        _ => None,
    }
}

/// Tells the client its message arrived.
async fn acknowledge(
    socket_ref: &Arc<Mutex<WebSocketConnSender>>,
    connection_id: &str,
    message_id: &str,
    format: AckFormat,
) {
    let timestamp = get_epoch_seconds().to_string();
    let message = match format {
        AckFormat::Json => Message::Text(
            serde_json::json!({
                "event": "ack",
                "data": { "messageId": message_id, "timestamp": timestamp },
            })
            .to_string()
            .into(),
        ),
        AckFormat::Binary => {
            let body =
                serde_json::json!({ "messageId": message_id, "timestamp": timestamp }).to_string();
            Message::Binary(encode_reserved_message(ReservedRoute::Ack, body.as_bytes()).into())
        }
    };

    if let Err(err) = socket_ref.lock().await.send(message).await {
        error!("failed to acknowledge a message from client {connection_id}: {err}");
    }
}

/// A JSON message's route, its message id, whether it asked to be acknowledged,
/// and the body as it arrived.
type JsonRouteData = (String, Option<String>, bool, Value);

fn resolve_route(
    parsed: Option<Value>,
    connection_id: String,
    route_key: String,
) -> ControlFlow<(), Option<JsonRouteData>> {
    // A failed parse is reported by the read loop.
    let Some(data) = parsed else {
        return ControlFlow::Continue(None);
    };
    let data_obj = match &data {
        Value::Object(obj) => obj,
        _ => {
            error!(
                "invalid JSON message from client {}, expected object",
                connection_id
            );
            return ControlFlow::Continue(None);
        }
    };
    // The id the application chose, which is what a lost message notification
    // has to name for the application to act on it. The runtime falls back to
    // one of its own only when a message carries none.
    let message_id = data_obj
        .get("messageId")
        .and_then(Value::as_str)
        .map(str::to_string);
    // Opting in without an id is ignored, since an acknowledgement has nothing
    // to name.
    let requires_ack =
        message_id.is_some() && data_obj.get("ack").and_then(Value::as_bool) == Some(true);

    let route_opt = data_obj.get(&route_key);
    if let Some(route_val) = route_opt {
        if let Value::String(route) = route_val {
            ControlFlow::Continue(Some((route.clone(), message_id, requires_ack, data)))
        } else {
            error!(
                "invalid JSON message from client {}, expected route value to be a string",
                connection_id
            );
            ControlFlow::Continue(None)
        }
    } else {
        // No route key found — fall through to the $default handler.
        // If no $default handler is registered, the caller will log an error.
        debug!(
            "message from client {} has no route key \"{}\", falling back to $default",
            connection_id, route_key,
        );
        ControlFlow::Continue(Some((
            CELERITY_WS_DEFAULT_MESSAGE_HANDLER_ROUTE.to_string(),
            message_id,
            requires_ack,
            data,
        )))
    }
}

/// A binary message's route, its message id, whether it asked to be
/// acknowledged, and the payload left once the framing is stripped.
type BinaryRouteData<'a> = (String, Option<String>, bool, &'a [u8]);

/// Splits a binary message into its framing and its payload.
///
/// The format is `<routeLength><route><requireAck><messageIdLength><messageId><message>`.
/// Every length is a single byte, so nothing here can read past a message that
/// carries the bytes it says it does, and a message that does not is refused
/// rather than trusted.
fn resolve_binary_route<'a>(
    msg_bytes: &'a [u8],
    connection_id: String,
) -> ControlFlow<(), Option<BinaryRouteData<'a>>> {
    let Some(&route_length) = msg_bytes.first() else {
        error!("invalid binary message from client {connection_id}, message is empty");
        return ControlFlow::Continue(None);
    };
    let route_end = 1 + route_length as usize;

    // The route is followed by the acknowledgement flag and the message id
    // length, so both have to be there for the message to be complete.
    if msg_bytes.len() < route_end + 2 {
        error!(
            "invalid binary message from client {}, message is shorter than its own framing",
            connection_id
        );
        return ControlFlow::Continue(None);
    }

    let route = match std::str::from_utf8(&msg_bytes[1..route_end]) {
        Ok(route) => route,
        Err(e) => {
            error!(
                "invalid binary message from client {}, failed to parse route: {}",
                connection_id, e
            );
            return ControlFlow::Continue(None);
        }
    };

    let requires_ack = msg_bytes[route_end] == 0x1;

    let message_id_length = msg_bytes[route_end + 1] as usize;
    let message_id_start = route_end + 2;
    let message_id_end = message_id_start + message_id_length;
    if msg_bytes.len() < message_id_end {
        error!(
            "invalid binary message from client {}, message id length exceeds message length",
            connection_id
        );
        return ControlFlow::Continue(None);
    }

    let message_id = if message_id_length > 0 {
        match std::str::from_utf8(&msg_bytes[message_id_start..message_id_end]) {
            Ok(message_id) => Some(message_id.to_string()),
            Err(e) => {
                error!(
                    "invalid binary message from client {}, failed to parse message id: {}",
                    connection_id, e
                );
                return ControlFlow::Continue(None);
            }
        }
    } else {
        None
    };

    ControlFlow::Continue(Some((
        route.to_string(),
        message_id,
        requires_ack,
        &msg_bytes[message_id_end..],
    )))
}

fn log_message_processing_finished(elapsed: Duration, success: bool) {
    let millis_precise = elapsed.as_micros() as f64 / 1000.0;

    if success {
        info!(
            "websocket message processing finished in {} milliseconds",
            millis_precise
        );
    } else {
        error!(
            "websocket message processing failed after {} milliseconds",
            millis_precise
        );
    }
}

fn find_auth_guard_config<'a>(
    auth_guard: &'a str,
    api_auth_opt: &'a Option<CelerityApiAuth>,
) -> Option<&'a CelerityApiAuthGuard> {
    if let Some(api_auth) = api_auth_opt {
        api_auth
            .guards
            .iter()
            .find(|guard| guard.0 == auth_guard)
            .map(|(_, guard_config)| guard_config)
    } else {
        None
    }
}

fn check_cors_origin(
    cors: &CelerityApiCors,
    request_ctx: &WebSocketRequestContext,
) -> Result<(), String> {
    match cors {
        CelerityApiCors::Str(cors_string) => {
            if cors_string == "*" {
                return Ok(());
            }
            Err(format!(
                "cors origin check failed, only `*` is allowed for CORS configuration \
                represented as a string, \"{cors_string}\" was provided",
            ))
        }
        CelerityApiCors::CorsConfiguration(cors_config) => {
            if let Some(origin) = request_ctx.headers.get("origin") {
                match origin.to_str() {
                    Ok(origin_str) => {
                        if let Some(allowed_origins) = &cors_config.allow_origins {
                            if allowed_origins.contains(&origin_str.to_string()) {
                                return Ok(());
                            }
                        }

                        return Err(format!(
                            "cors origin check failed, origin \"{origin_str}\" is not allowed",
                        ));
                    }
                    Err(e) => {
                        return Err(format!(
                            "cors origin check failed, failed to parse origin header: {e}",
                        ));
                    }
                }
            }

            // Per RFC 6455 §4.1, browser clients MUST send the Origin header
            // on WebSocket upgrade requests; non-browser clients MAY omit it.
            // A missing Origin header therefore indicates a server-side client
            // (CLI tool, SDK, service-to-service) so allow the connection.
            Ok(())
        }
    }
}

async fn close_connection(socket_ref: Arc<Mutex<WebSocketConnSender>>) {
    let mut socket = socket_ref.lock().await;
    if let Err(err) = socket.send(Message::Close(None)).await {
        error!("failed to send close frame to client: {err}");
    }
}

/// Closes a connection with the protocol's server-initiated backoff hint, so
/// the client waits before reconnecting rather than returning immediately into
/// whatever made the runtime shed it.
async fn close_with_retry_after(socket_ref: Arc<Mutex<WebSocketConnSender>>, retry_after_ms: u64) {
    let reason = serde_json::json!({ "retryAfter": retry_after_ms }).to_string();
    let frame = CloseFrame {
        code: close_code::AGAIN,
        reason: reason.into(),
    };
    let mut socket = socket_ref.lock().await;
    if let Err(err) = socket.send(Message::Close(Some(frame))).await {
        error!("failed to send close frame to client: {err}");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    // Only needed to hold the constant against the encoder, since nothing
    // outside the tests matches on it.
    use crate::consts::CELERITY_WS_CAPABILITIES_SIGNAL;

    /// Pairs a text message with its parsed form, as the read loop does.
    fn text_message(text: &str) -> (Message, Option<Value>) {
        (
            Message::Text(text.into()),
            serde_json::from_str::<Value>(text).ok(),
        )
    }

    /// The constants are for matching an inbound frame, where a fixed size
    /// array is what is wanted, and the encoder is for building an outbound
    /// one. Two spellings of the same four bytes is how the header came to be
    /// written short somewhere else, so they are held against each other here.
    #[test]
    fn test_the_reserved_signals_agree_with_what_the_encoder_builds() {
        assert_eq!(
            CELERITY_WS_ACK_SIGNAL.to_vec(),
            encode_reserved_message(ReservedRoute::Ack, &[])
        );
        assert_eq!(
            CELERITY_WS_CAPABILITIES_SIGNAL.to_vec(),
            encode_reserved_message(ReservedRoute::Capabilities, &[])
        );
    }

    #[test]
    fn test_detect_client_ack_reads_both_encodings() {
        let mut frame = CELERITY_WS_ACK_SIGNAL.to_vec();
        frame.extend_from_slice(br#"{"messageId":"m-1","timestamp":"1"}"#);
        assert_eq!(
            detect_client_ack(&Message::Binary(frame.into()), None),
            Some("m-1".to_string())
        );

        let (msg, parsed) =
            text_message(r#"{"event":"ack","data":{"messageId":"m-2","timestamp":"1"}}"#);
        assert_eq!(
            detect_client_ack(&msg, parsed.as_ref()),
            Some("m-2".to_string())
        );
    }

    #[test]
    fn test_detect_client_ack_leaves_application_messages_alone() {
        // Carries the word but is an ordinary message, and the runtime must not
        // swallow it on the way to its handler.
        let (msg, parsed) = text_message(r#"{"event":"sendMessage","data":{"kind":"ack"}}"#);
        assert!(detect_client_ack(&msg, parsed.as_ref()).is_none());

        // An acknowledgement request, which travels the other way.
        let (msg, parsed) = text_message(r#"{"event":"sendMessage","ack":true,"messageId":"m-3"}"#);
        assert!(detect_client_ack(&msg, parsed.as_ref()).is_none());

        // A binary application message whose route happens to be one byte.
        let frame = vec![0x1, 0x9, 0x0, 0x0, 0xff];
        assert!(detect_client_ack(&Message::Binary(frame.into()), None).is_none());
    }

    #[test]
    fn test_detect_client_ack_reads_an_escaped_acknowledgement() {
        let text = r#"{"event":"\u0061ck","data":{"messageId":"m-4","timestamp":"1"}}"#;
        // Holds the fixture to the case, since spelling it plainly would
        // pass without the parse being what decides.
        assert!(!text.contains(r#""ack""#));

        let (msg, parsed) = text_message(text);
        assert_eq!(
            detect_client_ack(&msg, parsed.as_ref()),
            Some("m-4".to_string()),
            "an acknowledgement missed is a message resent and then declared lost \
             to a client that already confirmed it"
        );
    }

    #[test]
    fn test_ack_request_reads_an_escaped_opt_in() {
        let text = r#"{"\u0061ck":true,"messageId":"m-5"}"#;
        assert!(!text.contains(r#""ack""#));

        let (msg, parsed) = text_message(text);
        let request = ack_request(&msg, parsed.as_ref());
        let Some((message_id, AckFormat::Json)) = request else {
            panic!("expected a JSON acknowledgement request, got {request:?}");
        };
        assert_eq!(message_id, "m-5");
    }

    /// The protocol reserves `ack` as an event value, so a message carrying it
    /// there is an ordinary one rather than an opt in.
    #[test]
    fn test_ack_request_tells_the_key_from_the_value() {
        let (msg, parsed) = text_message(r#"{"event":"ack","data":{}}"#);
        assert!(ack_request(&msg, parsed.as_ref()).is_none());

        let (msg, parsed) = text_message(r#"{"event":"send","data":{"type":"ack"}}"#);
        assert!(ack_request(&msg, parsed.as_ref()).is_none());

        // Present as a value elsewhere as well as a key, which is an opt in.
        let (msg, parsed) = text_message(r#"{"event":"ack","ack":true,"messageId":"m-6"}"#);
        assert!(ack_request(&msg, parsed.as_ref()).is_some());
    }

    /// Opting in without an id is ignored, since an acknowledgement names the
    /// message it settles and there would be nothing to name.
    #[test]
    fn test_ack_request_ignores_an_opt_in_with_no_id() {
        let (msg, parsed) = text_message(r#"{"event":"send","ack":true}"#);
        assert!(ack_request(&msg, parsed.as_ref()).is_none());
    }

    #[test]
    fn test_neither_check_reads_a_message_that_is_not_json() {
        let msg = Message::Text("not json at all".into());
        assert!(detect_client_ack(&msg, None).is_none());
        assert!(ack_request(&msg, None).is_none());
    }

    #[test]
    fn test_ack_request_reads_a_binary_frame() {
        // [routeLength][route][requireAck][messageIdLength][messageId][payload]
        let mut frame = vec![2u8];
        frame.extend_from_slice(b"ab");
        frame.push(0x1);
        frame.push(3);
        frame.extend_from_slice(b"m-1");
        frame.extend_from_slice(&[0xff, 0x00]);

        let request = ack_request(&Message::Binary(frame.into()), None);
        let Some((message_id, AckFormat::Binary)) = request else {
            panic!("expected a binary acknowledgement request, got {request:?}");
        };
        assert_eq!(message_id, "m-1");
    }

    #[test]
    fn test_ack_request_ignores_a_binary_frame_that_did_not_ask() {
        let mut frame = vec![2u8];
        frame.extend_from_slice(b"ab");
        frame.push(0x0);
        frame.push(3);
        frame.extend_from_slice(b"m-1");

        assert!(ack_request(&Message::Binary(frame.into()), None).is_none());
    }

    #[test]
    fn test_ack_request_refuses_a_frame_that_is_shorter_than_it_claims() {
        // Claims a route longer than the bytes that follow it, so reading the
        // acknowledgement flag would run off the end.
        let frame = vec![9u8, b'a', b'b'];
        assert!(ack_request(&Message::Binary(frame.into()), None).is_none());
    }

    #[test]
    fn test_detect_json_ping() {
        let (msg, parsed) = text_message(r#"{"ping":true}"#);
        let result = detect_heartbeat_ping(&msg, parsed.as_ref());
        assert!(result.is_some());
        match result.unwrap() {
            Message::Text(text) => {
                let val: serde_json::Value = serde_json::from_str(&text).unwrap();
                assert_eq!(val.get("pong"), Some(&serde_json::Value::Bool(true)));
            }
            _ => panic!("expected text message"),
        }
    }

    #[test]
    fn test_detect_binary_ping() {
        let msg = Message::Binary(vec![0x1, 0x1, 0x0, 0x0].into());
        let result = detect_heartbeat_ping(&msg, None);
        assert!(result.is_some());
        match result.unwrap() {
            Message::Binary(bytes) => {
                assert_eq!(bytes.as_ref(), &[0x1, 0x2, 0x0, 0x0]);
            }
            _ => panic!("expected binary message"),
        }
    }

    #[test]
    fn test_detect_non_ping_text() {
        let (msg, parsed) = text_message(r#"{"event":"myAction","data":"hello"}"#);
        assert!(detect_heartbeat_ping(&msg, parsed.as_ref()).is_none());
    }

    #[test]
    fn test_detect_non_ping_binary() {
        let msg = Message::Binary(vec![0x5, b'h', b'e', b'l', b'l', b'o'].into());
        assert!(detect_heartbeat_ping(&msg, None).is_none());
    }

    #[test]
    fn test_detect_non_ping_close() {
        let msg = Message::Close(None);
        assert!(detect_heartbeat_ping(&msg, None).is_none());
    }

    #[test]
    fn test_detect_json_ping_written_with_escapes() {
        let text = r#"{"\u0070ing":true}"#;
        assert!(!text.contains(r#""ping""#));

        let (msg, parsed) = text_message(text);
        assert!(detect_heartbeat_ping(&msg, parsed.as_ref()).is_some());
    }

    #[test]
    fn test_detect_ping_false_is_not_heartbeat() {
        let (msg, parsed) = text_message(r#"{"ping":false}"#);
        assert!(detect_heartbeat_ping(&msg, parsed.as_ref()).is_none());
    }

    #[test]
    fn test_create_auth_response_success() {
        let auth_data = serde_json::json!({"claims": {"sub": "user123"}});
        let response = create_auth_response(true, Some(&auth_data), None);
        let val: serde_json::Value = serde_json::from_str(&response).unwrap();
        assert_eq!(val["event"], "authenticated");
        assert_eq!(val["data"]["success"], true);
        assert_eq!(val["data"]["userInfo"]["claims"]["sub"], "user123");
    }

    #[test]
    fn test_create_auth_response_failure() {
        let response = create_auth_response(false, None, Some("Authentication failed"));
        let val: serde_json::Value = serde_json::from_str(&response).unwrap();
        assert_eq!(val["event"], "authenticated");
        assert_eq!(val["data"]["success"], false);
        assert_eq!(val["data"]["message"], "Authentication failed");
    }

    #[test]
    fn test_create_auth_response_failure_without_message() {
        let response = create_auth_response(false, None, None);
        let val: serde_json::Value = serde_json::from_str(&response).unwrap();
        assert_eq!(val["event"], "authenticated");
        assert_eq!(val["data"]["success"], false);
        assert!(val["data"]["message"].is_null());
    }
}
