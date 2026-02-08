use std::{
    collections::HashMap,
    net::{IpAddr, Ipv4Addr},
    sync::Arc,
};

use axum::{
    body::Body,
    extract::{MatchedPath, State},
    http::{Method, StatusCode},
    middleware::Next,
    response::{IntoResponse, Response},
};
use axum_extra::extract::CookieJar;
use celerity_blueprint_config_parser::blueprint::{
    CelerityApiAuth, CelerityApiAuthGuard, CelerityApiAuthGuardType,
    CelerityApiAuthGuardValueSource, CelerityApiProtocol,
};
use celerity_helpers::http::ResourceStore;
use tracing::warn;

use crate::{
    auth_custom::{AuthGuardHandler, AuthGuardValidateError},
    request::{RequestId, ResolvedClientIp},
};

// Maximum body size to buffer for body-based token extraction.
const MAX_AUTH_BODY_BUFFER_SIZE: usize = 1024 * 1024; // 1 MiB

/// Claims extracted from a successful authentication.
/// Inserted into request extensions for downstream handlers to consume.
#[derive(Clone, Debug)]
pub struct AuthClaims(pub Option<serde_json::Value>);

/// State required by the HTTP auth middleware.
#[derive(Clone)]
pub struct HttpAuthState {
    pub api_auth: CelerityApiAuth,
    pub resource_store: Arc<ResourceStore>,
    pub custom_auth_guards:
        Arc<tokio::sync::Mutex<HashMap<String, Arc<dyn AuthGuardHandler + Send + Sync>>>>,
    // Maps Axum matched path patterns (e.g. "/orders/{orderId}") to an optional per-handler guard name.
    // If the value is None, the handler uses the default guard.
    // If a path is absent from this map, no auth is required for that route.
    pub route_guards: HashMap<String, Option<String>>,
}

/// Resolves the auth guard name for the current request.
/// Returns `None` if auth should be skipped (public handler, unmatched route, no guard configured).
fn resolve_guard_name(request: &axum::extract::Request, state: &HttpAuthState) -> Option<String> {
    let matched_path = request
        .extensions()
        .get::<MatchedPath>()
        .map(|mp| mp.as_str());

    let per_handler_guard = matched_path.and_then(|mp| state.route_guards.get(mp))?;

    per_handler_guard
        .as_ref()
        .or(state.api_auth.default_guard.as_ref())
        .cloned()
}

/// Parses query parameters from a URI into a multi-valued map.
fn parse_query_params(uri: &axum::http::Uri) -> HashMap<String, Vec<String>> {
    let Some(query_str) = uri.query() else {
        return HashMap::new();
    };
    let mut map: HashMap<String, Vec<String>> = HashMap::new();
    for pair in query_str.split('&') {
        if let Some((key, value)) = pair.split_once('=') {
            map.entry(key.to_string())
                .or_default()
                .push(value.to_string());
        }
    }
    map
}

/// Returns `true` if the guard's token source references the request body.
fn token_source_needs_body(guard_config: &CelerityApiAuthGuard) -> bool {
    guard_config
        .token_source
        .as_ref()
        .map(|ts| {
            let source_str = match ts {
                CelerityApiAuthGuardValueSource::Str(s) => s.as_str(),
                CelerityApiAuthGuardValueSource::ValueSourceConfiguration(configs) => configs
                    .iter()
                    .find(|c| matches!(c.protocol, CelerityApiProtocol::Http))
                    .map(|c| c.source.as_str())
                    .unwrap_or_default(),
            };
            source_str.starts_with("$.body.")
        })
        .unwrap_or(false)
}

/// Buffers the request body as JSON, reconstructing the request for downstream handlers.
/// Returns `Err` with a 413 response if the body exceeds `MAX_AUTH_BODY_BUFFER_SIZE`.
async fn buffer_request_body(
    request: axum::extract::Request,
) -> Result<(serde_json::Value, axum::extract::Request), Response> {
    let (parts, body) = request.into_parts();
    let bytes = axum::body::to_bytes(body, MAX_AUTH_BODY_BUFFER_SIZE)
        .await
        .map_err(|_| {
            (
                StatusCode::PAYLOAD_TOO_LARGE,
                "Request body too large for auth",
            )
                .into_response()
        })?;
    let json_body: serde_json::Value =
        serde_json::from_slice(&bytes).unwrap_or(serde_json::Value::Null);
    let request = axum::extract::Request::from_parts(parts, Body::from(bytes.to_vec()));
    Ok((json_body, request))
}

/// Axum middleware that enforces HTTP auth based on blueprint configuration.
pub async fn http_auth_middleware(
    State(state): State<HttpAuthState>,
    request: axum::extract::Request,
    next: Next,
) -> Response {
    // Skip auth for OPTIONS (CORS preflight).
    if request.method() == Method::OPTIONS {
        let mut request = request;
        request.extensions_mut().insert(AuthClaims(None));
        return next.run(request).await;
    }

    let guard_name = match resolve_guard_name(&request, &state) {
        Some(name) => name,
        None => {
            let mut request = request;
            request.extensions_mut().insert(AuthClaims(None));
            return next.run(request).await;
        }
    };

    let guard_config = match state.api_auth.guards.get(&guard_name) {
        Some(config) => config.clone(),
        None => {
            warn!(guard = %guard_name, "auth guard referenced but not defined in API auth configuration");
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                "Auth guard misconfigured",
            )
                .into_response();
        }
    };

    // Extract request elements needed for token extraction.
    let request_id = request
        .extensions()
        .get::<RequestId>()
        .cloned()
        .unwrap_or(RequestId("unknown".to_string()));

    let client_ip = request
        .extensions()
        .get::<ResolvedClientIp>()
        .map(|rci| rci.0)
        .unwrap_or(IpAddr::V4(Ipv4Addr::UNSPECIFIED));

    let headers = request.headers().clone();
    let query = parse_query_params(request.uri());
    let cookies = CookieJar::from_headers(&headers);

    let (body_json, request) = if token_source_needs_body(&guard_config) {
        match buffer_request_body(request).await {
            Ok(result) => result,
            Err(response) => return response,
        }
    } else {
        (serde_json::Value::Null, request)
    };

    // Perform validation based on guard type.
    let result = match guard_config.guard_type {
        CelerityApiAuthGuardType::Jwt => crate::auth_jwt::validate_jwt_on_http_request(
            &guard_config,
            &headers,
            &query,
            &cookies,
            body_json,
            state.resource_store.clone(),
        )
        .await
        .map_err(|e| {
            warn!(guard = %guard_name, request_id = %request_id.0, client_ip = %client_ip, "JWT auth failed: {e}");
            (StatusCode::UNAUTHORIZED, "Unauthorized")
        }),
        CelerityApiAuthGuardType::Custom => {
            let guard_handler = {
                let guards = state.custom_auth_guards.lock().await;
                guards.get(&guard_name).cloned()
            };
            crate::auth_custom::validate_custom_auth_on_http_request(
                &guard_config,
                &headers,
                &query,
                &cookies,
                body_json,
                &request_id,
                &client_ip,
                guard_handler,
            )
            .await
            .map_err(|e| match e {
                AuthGuardValidateError::Forbidden(_) => {
                    warn!(guard = %guard_name, request_id = %request_id.0, client_ip = %client_ip, "custom auth forbidden: {e}");
                    (StatusCode::FORBIDDEN, "Forbidden")
                }
                _ => {
                    warn!(guard = %guard_name, request_id = %request_id.0, client_ip = %client_ip, "custom auth failed: {e}");
                    (StatusCode::UNAUTHORIZED, "Unauthorized")
                }
            })
        }
        CelerityApiAuthGuardType::NoGuardType => {
            let mut request = request;
            request.extensions_mut().insert(AuthClaims(None));
            return next.run(request).await;
        }
    };

    match result {
        Ok(claims) => {
            let mut request = request;
            request.extensions_mut().insert(AuthClaims(Some(claims)));
            next.run(request).await
        }
        Err(resp) => resp.into_response(),
    }
}
