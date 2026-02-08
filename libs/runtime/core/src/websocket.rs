use std::{
    collections::HashMap,
    fmt::{Debug, Display},
    net::IpAddr,
    ops::ControlFlow,
    sync::Arc,
    time::{Duration, Instant},
};

use async_trait::async_trait;
use axum::{
    extract::{
        ws::{CloseFrame, Message, WebSocket},
        Request, State, WebSocketUpgrade,
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
};
use celerity_ws_registry::registry::WebSocketConnRegistry;
use nanoid::nanoid;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::{sync::Mutex, time::sleep};
use tracing::{debug, error, info, info_span, warn, Instrument};

use crate::{
    auth_custom::{validate_custom_auth_on_connect, AuthGuardHandler, AuthGuardValidateError},
    auth_jwt::{validate_jwt_on_ws_connect, ValidateJwtError},
    consts::{
        CELERITY_WS_CONNECT_HANDLER_ROUTE, CELERITY_WS_DEFAULT_MESSAGE_HANDLER_ROUTE,
        CELERITY_WS_DISCONNECT_HANDLER_ROUTE, CELERITY_WS_FORBIDDEN_ERROR_CODE,
        CELERITY_WS_UNAUTHORISED_ERROR_CODE,
    },
    errors::WebSocketsMessageError,
    request::{HttpProtocolVersion, RequestId},
    telemetry_utils::extract_trace_context,
    utils::get_epoch_seconds,
};

#[derive(Clone, Debug)]
pub(crate) struct WebSocketAppState {
    pub connections: Arc<WebSocketConnRegistry>,
    pub routes: Arc<Mutex<HashMap<String, Arc<dyn WebSocketMessageHandler + Send + Sync>>>>,
    pub route_key: String,
    pub api_auth: Option<CelerityApiAuth>,
    pub auth_strategy: Option<WebSocketAuthStrategy>,
    pub connection_auth_guard_names: Option<Vec<String>>,
    pub connection_auth_guards: HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>>,
    pub cors: Option<CelerityApiCors>,
    pub resource_store: Arc<ResourceStore>,
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
    // Loosely typed JSON value that represents the message body parsed
    // from a text WebSocket message.
    pub body: serde_json::Value,
    #[serde(rename = "traceContext")]
    pub trace_context: Option<HashMap<String, String>>,
}

/// A binary message received from a WebSocket client with additional information.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct BinaryMessageInfo<'a> {
    // The connection ID for the client that sent the message.
    #[serde(rename = "connectionId")]
    pub connection_id: String,
    #[serde(rename = "eventType")]
    pub event_type: WebSocketEventType,
    #[serde(rename = "messageId")]
    pub message_id: String,
    #[serde(rename = "context")]
    pub request_ctx: Option<MessageRequestContext>,
    // The body of the binary message after stripping routing information
    // from the beginning of the message.
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
}

#[allow(clippy::too_many_arguments)]
pub(crate) async fn handler(
    ws: WebSocketUpgrade,
    user_agent_header: Option<TypedHeader<headers::UserAgent>>,
    headers: HeaderMap,
    Extension(request_id): Extension<RequestId>,
    State(state): State<WebSocketAppState>,
    ClientIp(client_ip): ClientIp,
    cookies: CookieJar,
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
            client_ip,
            query,
            cookies,
            trace_context: extract_trace_context(),
        };
        handle_socket(socket, request_id.0.clone(), request_ctx, state)
    })
}

async fn handle_socket(
    socket: WebSocket,
    connection_id: String,
    request_ctx: WebSocketRequestContext,
    state: WebSocketAppState,
) {
    let socket_ref = Arc::new(Mutex::new(socket));
    async {
        info!("websocket connection received: {}", connection_id);
        state
            .connections
            .add_connection(connection_id.clone(), socket_ref.clone());

        // For the purpose of establishing a WebSocket connection,
        // an origin check is carried out on the server-side to determine
        // whether or not a browser client is allowed to connect to the server.
        // Browsers do not send a CORS preflight request for WebSocket connections
        // and do not check and block the connection based on CORS response headers.
        // When CORS is not configured for the API, the connection will be allowed
        // without origin checks.
        // Server-side clients should provide a known allowed origin header when an API
        // is configured with a set of allowed origins (bypassing the origin check).
        // This check is for preventing third-party origins from connecting to the API
        // in web browsers, there is no risk in letting server-side clients bypass
        // the origin check.
        if let Some(cors) = &state.cors {
            if let Err(err) = check_cors_origin(cors, &request_ctx) {
                debug!("origin check failed, closing connection: {err}");
                close_connection(socket_ref.clone()).await;
                return;
            }
        }

        let mut auth_result_data = serde_json::Value::Null;
        if let Some(WebSocketAuthStrategy::Connect) = &state.auth_strategy {
            let step_after_auth =
                authenticate_connection(socket_ref.clone(), &state, &request_ctx).await;
            match step_after_auth {
                ControlFlow::Continue(data) => auth_result_data = data,
                ControlFlow::Break(_) => {
                    return;
                }
            }
        }

        if let ControlFlow::Break(_) = on_connect(
            socket_ref.clone(),
            connection_id.clone(),
            &state,
            &request_ctx,
            auth_result_data,
        )
        .await
        {
            return;
        }

        let mut connection_alive = true;
        while connection_alive {
            // Wait some time before acquiring the lock again to allow other tasks to write
            // to the socket. (i.e. a message received from another node in the cluster)
            sleep(Duration::from_millis(10)).await;
            let mut acquired_socket = socket_ref.lock().await;

            if let Some(Ok(msg)) = acquired_socket.recv().await {
                if process_message(msg, connection_id.clone(), request_ctx.clone(), &state)
                    .await
                    .is_break()
                {
                    state.connections.remove_connection(connection_id.clone());
                    connection_alive = false;
                }
            } else {
                // client disconnected
                state.connections.remove_connection(connection_id.clone());
                connection_alive = false;
            }
        }
    }
    .instrument(info_span!("websocket_connection", connection_id = %connection_id))
    .await
}

async fn authenticate_connection(
    socket_ref: Arc<Mutex<WebSocket>>,
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
                    ValidateAuthError::Custom(AuthGuardValidateError::UnexpectedError(
                        format!("guard config not found for \"{guard_name}\""),
                    )),
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
                let guard_handler = state.connection_auth_guards.get(guard_name).cloned();
                match validate_custom_auth_on_connect(
                    auth_guard_config,
                    &request_ctx.headers,
                    &request_ctx.query,
                    &request_ctx.cookies,
                    &request_ctx.request_id,
                    &request_ctx.client_ip,
                    guard_handler,
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
    socket_ref: Arc<Mutex<WebSocket>>,
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
                error!("connect handler failed, closing connection: {err}");
                close_connection(socket_ref.clone()).await;
                ControlFlow::Break(())
            } else {
                ControlFlow::Continue(())
            }
        }
        .instrument(info_span!("on_connect", route = %CELERITY_WS_CONNECT_HANDLER_ROUTE))
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
    auth_result_data: Option<serde_json::Value>,
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
        auth: auth_result_data,
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
    socket_ref: Arc<Mutex<WebSocket>>,
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

async fn process_message(
    msg: Message,
    connection_id: String,
    request_ctx: WebSocketRequestContext,
    state: &WebSocketAppState,
) -> ControlFlow<(), ()> {
    match msg {
        Message::Text(text) => {
            let resolved = resolve_route(
                text.to_string(),
                connection_id.clone(),
                state.route_key.clone(),
            )?;
            if let Some((route, message_id, data)) = resolved {
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
            if let Some((route, message_id, bytes_stripped)) = resolved {
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
            if let ControlFlow::Break(_) =
                on_disconnect(connection_id.clone(), state, &request_ctx).await
            {
                return ControlFlow::Break(());
            }
        }
        _ => {}
    }
    ControlFlow::Continue(())
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
    ))
    .await;
}

type JsonRouteData = (String, Option<String>, Value);

fn resolve_route(
    msg_text: String,
    connection_id: String,
    route_key: String,
) -> ControlFlow<(), Option<JsonRouteData>> {
    let data: Value = match serde_json::from_str(&msg_text) {
        Ok(data) => data,
        Err(e) => {
            error!(
                "failed to parse JSON message from client {}: {}",
                connection_id, e
            );
            return ControlFlow::Continue(None);
        }
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
    let route_opt = data_obj.get(&route_key);
    if let Some(route_val) = route_opt {
        if let Value::String(route) = route_val {
            ControlFlow::Continue(Some((route.clone(), None, data)))
        } else {
            error!(
                "invalid JSON message from client {}, expected route value to be a string",
                connection_id
            );
            ControlFlow::Continue(None)
        }
    } else {
        error!(
            "invalid JSON message from client {}, missing route key",
            connection_id
        );
        ControlFlow::Continue(None)
    }
}

type BinaryRouteData<'a> = (String, Option<String>, &'a [u8]);

fn resolve_binary_route<'a>(
    msg_bytes: &'a [u8],
    connection_id: String,
) -> ControlFlow<(), Option<BinaryRouteData<'a>>> {
    let route_length = msg_bytes[0];
    if route_length as usize > msg_bytes.len() - 1 {
        error!(
            "invalid binary message from client {}, route length exceeds message length",
            connection_id
        );
        return ControlFlow::Continue(None);
    }

    let route_result = std::str::from_utf8(&msg_bytes[1..=route_length as usize]);
    let route = match route_result {
        Ok(route) => route,
        Err(e) => {
            error!(
                "invalid binary message from client {}, failed to parse route: {}",
                connection_id, e
            );
            return ControlFlow::Continue(None);
        }
    };

    let message_id_length = msg_bytes[route_length as usize + 1];
    if message_id_length as usize > msg_bytes.len() - 1 {
        error!(
            "invalid binary message from client {}, message id length exceeds message length",
            connection_id
        );
        return ControlFlow::Continue(None);
    }

    let message_id = if message_id_length > 0 {
        let start_idx = route_length as usize + 2;
        let end_idx = start_idx + message_id_length as usize;
        let message_id_bytes = &msg_bytes[start_idx..end_idx];
        let message_id_str = std::str::from_utf8(message_id_bytes).unwrap();
        Some(message_id_str.to_string())
    } else {
        None
    };

    let data_start_idx = route_length as usize + 2 + message_id_length as usize;

    ControlFlow::Continue(Some((
        route.to_string(),
        message_id,
        &msg_bytes[data_start_idx..],
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

            Err("cors origin check failed, origin header is missing".to_string())
        }
    }
}

async fn close_connection(socket_ref: Arc<Mutex<WebSocket>>) {
    let mut socket = socket_ref.lock().await;
    if let Err(err) = socket.send(Message::Close(None)).await {
        error!("failed to send close frame to client: {err}");
    }
}
