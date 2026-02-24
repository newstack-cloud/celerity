use std::{collections::HashMap, fmt::Display, net::IpAddr, sync::Arc};

use async_trait::async_trait;
use axum::http::HeaderMap;
use axum_extra::extract::CookieJar;
use celerity_blueprint_config_parser::blueprint::CelerityApiAuthGuard;

use crate::{
    request::{RequestId, RequestInfo},
    utils::{get_http_token_source, get_websocket_token_source, strip_auth_scheme},
    value_sources::{extract_value_from_request_elements, ExtractValueError},
};

pub struct AuthGuardValidateInput {
    pub token: String,
    // Provides a representation of the HTTP request that the auth guard is protecting,
    // this may be a standard HTTP request or the initial HTTP request that will be upgraded
    // to a WebSocket connection.
    pub request: RequestInfo,
    // Accumulated auth context from preceding guards in the chain.
    // Keyed by guard name, e.g. `{ "jwt": { "claims": { ... } } }`.
    // Empty for the first guard in the chain.
    pub auth: serde_json::Map<String, serde_json::Value>,
    // The blueprint handler name for the route being protected.
    // None for WebSocket connections and when no name is available.
    pub handler_name: Option<String>,
}

#[derive(Debug)]
pub enum AuthGuardValidateError {
    Unauthorised(String),
    Forbidden(String),
    UnexpectedError(String),
    ExtractTokenFailed(ExtractValueError),
    TokenSourceMissing,
}

impl Display for AuthGuardValidateError {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        match self {
            AuthGuardValidateError::Unauthorised(e) => write!(f, "Unauthorised: {e}"),
            AuthGuardValidateError::Forbidden(e) => write!(f, "Forbidden: {e}"),
            AuthGuardValidateError::UnexpectedError(e) => write!(f, "Unexpected error: {e}"),
            AuthGuardValidateError::ExtractTokenFailed(e) => {
                write!(f, "Extract token failed: {e}")
            }
            AuthGuardValidateError::TokenSourceMissing => write!(f, "Token source missing"),
        }
    }
}

impl From<ExtractValueError> for AuthGuardValidateError {
    fn from(e: ExtractValueError) -> Self {
        AuthGuardValidateError::ExtractTokenFailed(e)
    }
}

#[async_trait]
pub trait AuthGuardHandler: std::fmt::Debug {
    async fn validate(
        &self,
        input: AuthGuardValidateInput,
    ) -> Result<serde_json::Value, AuthGuardValidateError>;
}

/// Borrow-based view of an incoming request, shared by both the WebSocket connect
/// and HTTP custom auth validators. Defined here so the dependency flows from
/// `auth_http` → `auth_custom`, not the reverse.
pub struct CustomAuthRequestContext<'a> {
    pub method: &'a str,
    pub path: &'a str,
    pub headers: &'a HeaderMap,
    pub query: &'a HashMap<String, Vec<String>>,
    pub cookies: &'a CookieJar,
    pub request_id: &'a RequestId,
    pub client_ip: &'a IpAddr,
}

/// Validates a token with a custom auth guard on a WebSocket connection for the `connect` auth strategy.
pub async fn validate_custom_auth_on_connect(
    auth_guard_config: &CelerityApiAuthGuard,
    ctx: CustomAuthRequestContext<'_>,
    auth_guard_opt: Option<Arc<dyn AuthGuardHandler + Send + Sync>>,
    accumulated_auth: &serde_json::Map<String, serde_json::Value>,
) -> Result<serde_json::Value, AuthGuardValidateError> {
    let token_source_opt = get_websocket_token_source(auth_guard_config);

    let token = match token_source_opt {
        Some(token_source) => {
            let extracted = extract_value_from_request_elements(
                token_source,
                serde_json::Value::Null,
                ctx.headers,
                ctx.query,
                ctx.cookies,
            )?;
            match extracted {
                serde_json::Value::String(t) => strip_auth_scheme(&t, auth_guard_config),
                _ => {
                    return Err(AuthGuardValidateError::Unauthorised(
                        "Invalid token value provided, token must be a string".to_string(),
                    ))
                }
            }
        }
        // tokenSource is optional for custom guards; pass an empty token so the
        // handler can still run (e.g. inspecting accumulated auth from prior guards).
        None => String::new(),
    };

    if let Some(auth_guard) = &auth_guard_opt {
        let input = AuthGuardValidateInput {
            token,
            request: RequestInfo {
                method: ctx.method.to_string(),
                path: ctx.path.to_string(),
                headers: ctx.headers.clone(),
                query: ctx.query.clone(),
                cookies: ctx.cookies.clone(),
                body: None,
                request_id: ctx.request_id.clone(),
                client_ip: ctx.client_ip.to_string(),
            },
            auth: accumulated_auth.clone(),
            handler_name: None,
        };
        auth_guard.validate(input).await
    } else {
        Err(AuthGuardValidateError::UnexpectedError(
            "No auth guard handler configured".to_string(),
        ))
    }
}

/// Validates a token with a custom auth guard on an HTTP request.
pub async fn validate_custom_auth_on_http_request(
    auth_guard_config: &CelerityApiAuthGuard,
    ctx: CustomAuthRequestContext<'_>,
    body: serde_json::Value,
    auth_guard_opt: Option<Arc<dyn AuthGuardHandler + Send + Sync>>,
    accumulated_auth: &serde_json::Map<String, serde_json::Value>,
    handler_name: Option<String>,
) -> Result<serde_json::Value, AuthGuardValidateError> {
    let token_source_opt = get_http_token_source(auth_guard_config);

    let token = match token_source_opt {
        Some(token_source) => {
            let extracted = extract_value_from_request_elements(
                token_source,
                body,
                ctx.headers,
                ctx.query,
                ctx.cookies,
            )?;
            match extracted {
                serde_json::Value::String(t) => strip_auth_scheme(&t, auth_guard_config),
                _ => {
                    return Err(AuthGuardValidateError::Unauthorised(
                        "Invalid token value provided, token must be a string".to_string(),
                    ))
                }
            }
        }
        // tokenSource is optional for custom guards; pass an empty token so the
        // handler can still run (e.g. inspecting accumulated auth from prior guards).
        None => String::new(),
    };

    if let Some(auth_guard) = &auth_guard_opt {
        let input = AuthGuardValidateInput {
            token,
            request: RequestInfo {
                method: ctx.method.to_string(),
                path: ctx.path.to_string(),
                headers: ctx.headers.clone(),
                query: ctx.query.clone(),
                cookies: ctx.cookies.clone(),
                body: None,
                request_id: ctx.request_id.clone(),
                client_ip: ctx.client_ip.to_string(),
            },
            auth: accumulated_auth.clone(),
            handler_name,
        };
        auth_guard.validate(input).await
    } else {
        Err(AuthGuardValidateError::UnexpectedError(
            "No auth guard handler configured".to_string(),
        ))
    }
}

#[cfg(test)]
mod tests {
    use std::{
        collections::HashMap,
        net::{IpAddr, Ipv4Addr},
        sync::Arc,
    };

    use async_trait::async_trait;
    use axum::http::{HeaderMap, HeaderName, HeaderValue};
    use axum_extra::extract::CookieJar;
    use celerity_blueprint_config_parser::blueprint::{
        CelerityApiAuthGuard, CelerityApiAuthGuardScheme, CelerityApiAuthGuardType,
        CelerityApiAuthGuardValueSource,
    };
    use serde_json::json;

    use crate::{
        auth_custom::{
            validate_custom_auth_on_connect, AuthGuardHandler, AuthGuardValidateError,
            AuthGuardValidateInput, CustomAuthRequestContext,
        },
        request::RequestId,
        value_sources::ExtractValueError,
    };

    #[derive(Debug)]
    struct TestAuthGuardHandler {}

    impl TestAuthGuardHandler {
        fn new() -> Self {
            Self {}
        }
    }

    #[async_trait]
    impl AuthGuardHandler for TestAuthGuardHandler {
        async fn validate(
            &self,
            input: AuthGuardValidateInput,
        ) -> Result<serde_json::Value, AuthGuardValidateError> {
            match input.token.as_str() {
                "valid" => Ok(json!({
                    "userId": "104932",
                    "email": "test@test.com",
                    "name": "Test User",
                })),
                "unauthorised" => Err(AuthGuardValidateError::Unauthorised(
                    "Invalid token".to_string(),
                )),
                "forbidden" => Err(AuthGuardValidateError::Forbidden(
                    "Forbidden token".to_string(),
                )),
                _ => Err(AuthGuardValidateError::UnexpectedError(
                    "Unexpected token".to_string(),
                )),
            }
        }
    }

    fn create_test_auth_guard_config(token_source: Option<String>) -> CelerityApiAuthGuard {
        CelerityApiAuthGuard {
            guard_type: CelerityApiAuthGuardType::Custom,
            issuer: None,
            token_source: Some(CelerityApiAuthGuardValueSource::Str(
                token_source.unwrap_or("$.headers.Authorization".to_string()),
            )),
            audience: None,
            auth_scheme: Some(CelerityApiAuthGuardScheme::Bearer),
            discovery_mode: None,
        }
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_for_valid_token() {
        let auth_guard_config = create_test_auth_guard_config(None);

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer valid").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-1".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_ok());
        assert_eq!(
            result.unwrap(),
            json!({
                "userId": "104932",
                "email": "test@test.com",
                "name": "Test User",
            })
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_fails_with_unauthorised_error() {
        let auth_guard_config = create_test_auth_guard_config(None);

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer unauthorised").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-2".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::Unauthorised(_)
        ));
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_fails_with_forbidden_error() {
        let auth_guard_config = create_test_auth_guard_config(None);

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer forbidden").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-3".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::Forbidden(_)
        ));
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_fails_with_unexpected_error() {
        let auth_guard_config = create_test_auth_guard_config(None);

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer unexpected-token-value").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-4".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::UnexpectedError(_)
        ));
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_fails_due_to_missing_token_source_value() {
        let auth_guard_config =
            create_test_auth_guard_config(Some("$.headers.missing-token".to_string()));

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer valid").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-5".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::ExtractTokenFailed(ExtractValueError::ValueSourceNotFound(_))
        ));
    }

    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_fails_due_to_missing_auth_guard_handler() {
        let auth_guard_config =
            create_test_auth_guard_config(Some("$.headers.Authorization".to_string()));

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer valid").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-6".to_string());

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            None,
            &serde_json::Map::new(),
        )
        .await;

        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::UnexpectedError(_)
        ));
    }

    // When token_source is absent, an empty token is passed to allow guards that
    // operate on accumulated auth context from prior guards rather than a raw token.
    // The guard handler is still called and its result determines the outcome.
    #[test_log::test(tokio::test)]
    async fn test_validate_custom_auth_on_connect_with_no_token_source_calls_guard_with_empty_token(
    ) {
        let auth_guard_config = CelerityApiAuthGuard {
            guard_type: CelerityApiAuthGuardType::Custom,
            issuer: None,
            token_source: None,
            audience: None,
            auth_scheme: Some(CelerityApiAuthGuardScheme::Bearer),
            discovery_mode: None,
        };

        let headers = HeaderMap::from_iter([(
            HeaderName::from_static("authorization"),
            HeaderValue::from_str("Bearer valid").unwrap(),
        )]);
        let query = HashMap::new();
        let cookies = CookieJar::new();
        let request_id = RequestId("test-request-6".to_string());

        let auth_guard: Arc<dyn AuthGuardHandler + Send + Sync> =
            Arc::new(TestAuthGuardHandler::new());
        let auth_guard_opt = Some(auth_guard);

        let client_ip = IpAddr::V4(Ipv4Addr::new(127, 0, 0, 1));
        let result = validate_custom_auth_on_connect(
            &auth_guard_config,
            CustomAuthRequestContext {
                method: "GET",
                path: "/test",
                headers: &headers,
                query: &query,
                cookies: &cookies,
                request_id: &request_id,
                client_ip: &client_ip,
            },
            auth_guard_opt,
            &serde_json::Map::new(),
        )
        .await;

        // The guard is invoked with an empty token; the test handler returns
        // UnexpectedError for any token it doesn't recognise.
        assert!(result.is_err());
        assert!(matches!(
            result.unwrap_err(),
            AuthGuardValidateError::UnexpectedError(_)
        ));
    }
}
