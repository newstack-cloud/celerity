use std::sync::Arc;

use axum::{body::Body, http::Request};
use biscuit::{
    jwa::SignatureAlgorithm,
    jwk::{JWKSet, RSAKeyParameters},
    jws::{RegisteredHeader, Secret},
    ClaimsSet, Empty, RegisteredClaims, SingleOrMultiple, JWT,
};
use chrono::{Duration, Utc};
use http_body_util::BodyExt;
use ring::{
    error::KeyRejected,
    rsa::{KeyPairComponents, PublicKeyComponents},
    signature::RsaKeyPair,
};

use celerity_runtime_core::{application::Application, config::RuntimeConfig};
use httptest::{
    matchers::request,
    responders::{json_encoded, status_code},
    Expectation, Server,
};
use serde_json::json;
use tempfile::NamedTempFile;

mod common;

fn setup_test_oidc_server() -> Server {
    let server = Server::run();
    let jwks_uri = server.url("/.well-known/jwks.json").to_string();
    let metadata = json!({
        "jwks_uri": jwks_uri,
    });
    server.expect(
        Expectation::matching(request::method_path(
            "GET",
            "/.well-known/openid-configuration",
        ))
        .times(0..)
        .respond_with(json_encoded(metadata)),
    );
    let jwks_str = include_str!("data/fixtures/public-jwks.json");
    server.expect(
        Expectation::matching(request::method_path("GET", "/.well-known/jwks.json"))
            .times(0..)
            .respond_with(
                status_code(200)
                    .append_header("Content-Type", "application/json")
                    .body(jwks_str),
            ),
    );
    server
}

fn create_jwt(
    subject: String,
    audience: String,
    issuer: String,
    expiry: chrono::DateTime<Utc>,
) -> Result<String, ()> {
    let private_jwks = serde_json::from_str::<JWKSet<RSAKeyParameters>>(include_str!(
        "data/fixtures/private-jwks.json"
    ))
    .unwrap();
    let private_jwk = private_jwks.keys[0].clone();
    let claims = ClaimsSet::<Empty> {
        registered: RegisteredClaims {
            issuer: Some(issuer),
            subject: Some(subject),
            audience: Some(SingleOrMultiple::Single(audience)),
            expiry: Some(expiry.into()),
            ..Default::default()
        },
        private: Default::default(),
    };
    let jwt = JWT::new_decoded(
        From::from(RegisteredHeader {
            algorithm: SignatureAlgorithm::RS256,
            key_id: private_jwk.common.key_id,
            ..Default::default()
        }),
        claims,
    );
    let rsa_key_pair = jwk_to_rsa_key_pair(&private_jwk.additional).unwrap();
    let token = jwt
        .into_encoded(&Secret::RsaKeyPair(Arc::new(rsa_key_pair)))
        .unwrap();
    Ok(token.unwrap_encoded().to_string())
}

fn jwk_to_rsa_key_pair(jwk: &RSAKeyParameters) -> Result<RsaKeyPair, KeyRejected> {
    RsaKeyPair::from_components(&KeyPairComponents {
        public_key: PublicKeyComponents {
            n: jwk.n.to_bytes_be(),
            e: jwk.e.to_bytes_be(),
        },
        d: jwk.d.as_ref().expect("d is required").to_bytes_be(),
        p: jwk.p.as_ref().expect("p is required").to_bytes_be(),
        q: jwk.q.as_ref().expect("q is required").to_bytes_be(),
        dP: jwk.dp.as_ref().expect("dP is required").to_bytes_be(),
        dQ: jwk.dq.as_ref().expect("dQ is required").to_bytes_be(),
        qInv: jwk.qi.as_ref().expect("qi is required").to_bytes_be(),
    })
}

/// Writes a blueprint with JWT auth to a temporary file.
/// The token source uses a query parameter (`$.query.token`) to avoid
/// the "Bearer " prefix stripping complexity.
fn write_auth_blueprint(issuer: &str) -> NamedTempFile {
    let blueprint = format!(
        r#"version: 2025-11-02
transform: celerity-2026-02-28
variables: {{}}
resources:
  testApi:
    type: "celerity/api"
    metadata:
      displayName: Test Auth API
    linkSelector:
      byLabel:
        application: "test-auth"
    spec:
      protocols: ["http"]
      cors:
        allowCredentials: false
        allowOrigins:
          - "https://example.com"
        allowMethods:
          - "GET"
          - "POST"
        allowHeaders:
          - "Content-Type"
          - "Authorization"
        exposeHeaders:
          - "Content-Length"
        maxAge: 3600
      tracingEnabled: false
      auth:
        defaultGuard: "jwt"
        guards:
          jwt:
            type: jwt
            issuer: "{issuer}"
            tokenSource: "$.query.token"
            audience:
              - "test-audience"

  protectedHandler:
    type: "celerity/handler"
    metadata:
      displayName: Protected Handler
      labels:
        application: "test-auth"
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/protected"
    spec:
      handlerName: Test-ProtectedHandler-v1
      codeLocation: "./test"
      handler: "handlers.protected"
      runtime: "python3.12.x"
      timeout: 60
      tracingEnabled: false
"#
    );
    let mut tmp = NamedTempFile::new().expect("failed to create temp file");
    std::io::Write::write_all(&mut tmp, blueprint.as_bytes())
        .expect("failed to write blueprint to temp file");
    tmp
}

fn create_env_vars(blueprint_path: &str) -> common::MockEnvVars<'static> {
    common::MockEnvVars::new(Some(
        vec![
            ("CELERITY_BLUEPRINT", blueprint_path.to_string()),
            ("CELERITY_SERVICE_NAME", "http-auth-test".to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ffi".to_string()),
            ("CELERITY_SERVER_PORT", "0".to_string()),
            ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
            ("CELERITY_TEST_MODE", "true".to_string()),
            ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
        ]
        .into_iter()
        .collect(),
    ))
}

async fn setup_auth_app(
    oidc_server: &Server,
) -> (Application, std::net::SocketAddr, NamedTempFile) {
    let issuer = oidc_server.url("").to_string();
    let tmp_blueprint = write_auth_blueprint(&issuer);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();
    app.register_http_handler("/protected", "GET", protected_handler);
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();
    (app, addr, tmp_blueprint)
}

async fn protected_handler() -> &'static str {
    "Protected content"
}

fn http_client(
) -> hyper_util::client::legacy::Client<hyper_util::client::legacy::connect::HttpConnector, Body> {
    hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new()).build_http()
}

#[test_log::test(tokio::test)]
async fn test_http_request_with_valid_jwt_returns_200() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let issuer = oidc_server.url("").to_string();
    let token = create_jwt(
        "test-subject".to_string(),
        "test-audience".to_string(),
        issuer,
        Utc::now() + Duration::hours(1),
    )
    .unwrap();

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token={token}"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(&body[..], b"Protected content");
}

#[test_log::test(tokio::test)]
async fn test_http_request_with_invalid_jwt_returns_401() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token=invalid-token"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 401);
}

#[test_log::test(tokio::test)]
async fn test_http_request_without_token_returns_401() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 401);
}

#[test_log::test(tokio::test)]
async fn test_http_options_request_skips_auth() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .method("OPTIONS")
                .uri(format!("http://{addr}/protected"))
                .header("Host", "localhost")
                .header("Origin", "https://example.com")
                .header("Access-Control-Request-Method", "GET")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    // OPTIONS should pass through auth (CORS preflight).
    // The status may be 200 (CORS layer handles it) or other non-401.
    assert_ne!(response.status(), 401);
}

#[test_log::test(tokio::test)]
async fn test_health_check_endpoint_skips_auth() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/runtime/health/check"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
}

#[test_log::test(tokio::test)]
async fn test_auth_claims_passed_to_handler() {
    let oidc_server = setup_test_oidc_server();
    let issuer_url = oidc_server.url("").to_string();
    let tmp_blueprint = write_auth_blueprint(&issuer_url);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    // Register a handler that inspects auth claims from the request extensions.
    app.register_http_handler(
        "/protected",
        "GET",
        |req: axum::extract::Request| async move {
            use celerity_runtime_core::auth_http::AuthContext;
            let claims = req.extensions().get::<AuthContext>().cloned();
            match claims {
                Some(AuthContext(Some(value))) => {
                    // Claims are namespaced by guard name: { "jwt": { "claims": { ... } } }
                    let sub = value
                        .get("jwt")
                        .and_then(|g| g.get("claims"))
                        .and_then(|c| c.get("sub"))
                        .and_then(|s| s.as_str())
                        .unwrap_or("none");
                    format!("sub={sub}")
                }
                _ => "no-claims".to_string(),
            }
        },
    );

    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    let token = create_jwt(
        "test-user-42".to_string(),
        "test-audience".to_string(),
        issuer_url,
        Utc::now() + Duration::hours(1),
    )
    .unwrap();

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token={token}"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(&body[..], b"sub=test-user-42");
}

#[test_log::test(tokio::test)]
async fn test_cors_preflight_returns_cors_headers_on_protected_route() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app(&oidc_server).await;

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .method("OPTIONS")
                .uri(format!("http://{addr}/protected"))
                .header("Host", "localhost")
                .header("Origin", "https://example.com")
                .header("Access-Control-Request-Method", "GET")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    assert_eq!(
        response
            .headers()
            .get("access-control-allow-origin")
            .and_then(|v| v.to_str().ok()),
        Some("https://example.com"),
    );
    assert!(response
        .headers()
        .get("access-control-allow-methods")
        .is_some());
}

/// Writes a blueprint with JWT auth that includes both a protected and a public handler.
fn write_auth_blueprint_with_public_handler(issuer: &str) -> NamedTempFile {
    let blueprint = format!(
        r#"version: 2025-11-02
transform: celerity-2026-02-28
variables: {{}}
resources:
  testApi:
    type: "celerity/api"
    metadata:
      displayName: Test Auth API
    linkSelector:
      byLabel:
        application: "test-auth"
    spec:
      protocols: ["http"]
      cors:
        allowCredentials: false
        allowOrigins:
          - "https://example.com"
        allowMethods:
          - "GET"
          - "POST"
        allowHeaders:
          - "Content-Type"
          - "Authorization"
        exposeHeaders:
          - "Content-Length"
        maxAge: 3600
      tracingEnabled: false
      auth:
        defaultGuard: "jwt"
        guards:
          jwt:
            type: jwt
            issuer: "{issuer}"
            tokenSource: "$.query.token"
            audience:
              - "test-audience"

  protectedHandler:
    type: "celerity/handler"
    metadata:
      displayName: Protected Handler
      labels:
        application: "test-auth"
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/protected"
    spec:
      handlerName: Test-ProtectedHandler-v1
      codeLocation: "./test"
      handler: "handlers.protected"
      runtime: "python3.12.x"
      timeout: 60
      tracingEnabled: false

  publicHandler:
    type: "celerity/handler"
    metadata:
      displayName: Public Handler
      labels:
        application: "test-auth"
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/public"
        celerity.handler.public: true
    spec:
      handlerName: Test-PublicHandler-v1
      codeLocation: "./test"
      handler: "handlers.public"
      runtime: "python3.12.x"
      timeout: 60
      tracingEnabled: false
"#
    );
    let mut tmp = NamedTempFile::new().expect("failed to create temp file");
    std::io::Write::write_all(&mut tmp, blueprint.as_bytes())
        .expect("failed to write blueprint to temp file");
    tmp
}

async fn public_handler() -> &'static str {
    "Public content"
}

/// A test custom auth guard handler that validates tokens.
/// Returns claims on success for any non-empty token.
#[derive(Debug)]
struct TestCustomAuthGuard;

#[async_trait::async_trait]
impl celerity_runtime_core::auth_custom::AuthGuardHandler for TestCustomAuthGuard {
    async fn validate(
        &self,
        input: celerity_runtime_core::auth_custom::AuthGuardValidateInput,
    ) -> Result<serde_json::Value, celerity_runtime_core::auth_custom::AuthGuardValidateError> {
        if input.token.is_empty() {
            return Err(
                celerity_runtime_core::auth_custom::AuthGuardValidateError::Unauthorised(
                    "empty token".to_string(),
                ),
            );
        }
        Ok(json!({
            "role": "admin",
            "permissions": ["read", "write"]
        }))
    }
}

/// A test custom auth guard that always returns Forbidden.
#[derive(Debug)]
struct ForbiddenCustomAuthGuard;

#[async_trait::async_trait]
impl celerity_runtime_core::auth_custom::AuthGuardHandler for ForbiddenCustomAuthGuard {
    async fn validate(
        &self,
        _input: celerity_runtime_core::auth_custom::AuthGuardValidateInput,
    ) -> Result<serde_json::Value, celerity_runtime_core::auth_custom::AuthGuardValidateError> {
        Err(
            celerity_runtime_core::auth_custom::AuthGuardValidateError::Forbidden(
                "access denied by RBAC policy".to_string(),
            ),
        )
    }
}

async fn setup_auth_app_with_public_handler(
    oidc_server: &Server,
) -> (Application, std::net::SocketAddr, NamedTempFile) {
    let issuer = oidc_server.url("").to_string();
    let tmp_blueprint = write_auth_blueprint_with_public_handler(&issuer);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();
    app.register_http_handler("/protected", "GET", protected_handler);
    app.register_http_handler("/public", "GET", public_handler);
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();
    (app, addr, tmp_blueprint)
}

#[test_log::test(tokio::test)]
async fn test_public_handler_skips_auth_with_default_guard() {
    let oidc_server = setup_test_oidc_server();
    let (_app, addr, _tmp) = setup_auth_app_with_public_handler(&oidc_server).await;

    let client = http_client();

    // Public handler should be accessible without a token.
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/public"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(&body[..], b"Public content");

    // Protected handler should still require a token.
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    assert_eq!(response.status(), 401);
}

// ---------------------------------------------------------------------------
// Multi-guard chain tests
// ---------------------------------------------------------------------------

/// Writes a blueprint with a multi-guard defaultGuard chain: ["jwt", "customGuard"].
/// Both guards use the same query-based token source for simplicity.
fn write_multi_guard_blueprint(issuer: &str) -> NamedTempFile {
    let blueprint = format!(
        r#"version: 2025-11-02
transform: celerity-2026-02-28
variables: {{}}
resources:
  testApi:
    type: "celerity/api"
    metadata:
      displayName: Test Multi-Guard API
    linkSelector:
      byLabel:
        application: "test-multi-guard"
    spec:
      protocols: ["http"]
      cors:
        allowCredentials: false
        allowOrigins:
          - "https://example.com"
        allowMethods:
          - "GET"
        allowHeaders:
          - "Content-Type"
          - "Authorization"
        exposeHeaders:
          - "Content-Length"
        maxAge: 3600
      tracingEnabled: false
      auth:
        defaultGuard:
          - "jwt"
          - "customGuard"
        guards:
          jwt:
            type: jwt
            issuer: "{issuer}"
            tokenSource: "$.query.token"
            audience:
              - "test-audience"
          customGuard:
            type: custom
            tokenSource: "$.query.token"

  protectedHandler:
    type: "celerity/handler"
    metadata:
      displayName: Protected Handler
      labels:
        application: "test-multi-guard"
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/protected"
    spec:
      handlerName: Test-MultiGuardProtectedHandler-v1
      codeLocation: "./test"
      handler: "handlers.protected"
      runtime: "python3.12.x"
      timeout: 60
      tracingEnabled: false
"#
    );
    let mut tmp = NamedTempFile::new().expect("failed to create temp file");
    std::io::Write::write_all(&mut tmp, blueprint.as_bytes())
        .expect("failed to write blueprint to temp file");
    tmp
}

/// Writes a blueprint where defaultGuard is a single guard ("jwt") but
/// one handler overrides with a comma-separated `protectedBy` annotation.
fn write_per_handler_override_blueprint(issuer: &str) -> NamedTempFile {
    let blueprint = format!(
        r#"version: 2025-11-02
transform: celerity-2026-02-28
variables: {{}}
resources:
  testApi:
    type: "celerity/api"
    metadata:
      displayName: Test Per-Handler Override API
    linkSelector:
      byLabel:
        application: "test-override"
    spec:
      protocols: ["http"]
      cors:
        allowCredentials: false
        allowOrigins:
          - "https://example.com"
        allowMethods:
          - "GET"
        allowHeaders:
          - "Content-Type"
          - "Authorization"
        exposeHeaders:
          - "Content-Length"
        maxAge: 3600
      tracingEnabled: false
      auth:
        defaultGuard: "jwt"
        guards:
          jwt:
            type: jwt
            issuer: "{issuer}"
            tokenSource: "$.query.token"
            audience:
              - "test-audience"
          customGuard:
            type: custom
            tokenSource: "$.query.token"

  overriddenHandler:
    type: "celerity/handler"
    metadata:
      displayName: Overridden Handler
      labels:
        application: "test-override"
      annotations:
        celerity.handler.http: true
        celerity.handler.http.method: "GET"
        celerity.handler.http.path: "/overridden"
        celerity.handler.guard.protectedBy: "jwt,customGuard"
    spec:
      handlerName: Test-OverriddenHandler-v1
      codeLocation: "./test"
      handler: "handlers.overridden"
      runtime: "python3.12.x"
      timeout: 60
      tracingEnabled: false
"#
    );
    let mut tmp = NamedTempFile::new().expect("failed to create temp file");
    std::io::Write::write_all(&mut tmp, blueprint.as_bytes())
        .expect("failed to write blueprint to temp file");
    tmp
}

#[test_log::test(tokio::test)]
async fn test_multi_guard_chain_both_pass_returns_200_with_namespaced_claims() {
    let oidc_server = setup_test_oidc_server();
    let issuer_url = oidc_server.url("").to_string();
    let tmp_blueprint = write_multi_guard_blueprint(&issuer_url);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    // Register the custom auth guard that accepts any non-empty token.
    app.register_custom_auth_guard("customGuard", TestCustomAuthGuard)
        .await;

    // Register a handler that returns the full claims JSON.
    app.register_http_handler(
        "/protected",
        "GET",
        |req: axum::extract::Request| async move {
            use celerity_runtime_core::auth_http::AuthContext;
            let claims = req.extensions().get::<AuthContext>().cloned();
            match claims {
                Some(AuthContext(Some(value))) => {
                    // Verify both guard namespaces are present.
                    let has_jwt = value.get("jwt").is_some();
                    let has_custom = value.get("customGuard").is_some();
                    let sub = value
                        .get("jwt")
                        .and_then(|g| g.get("claims"))
                        .and_then(|c| c.get("sub"))
                        .and_then(|s| s.as_str())
                        .unwrap_or("none");
                    let role = value
                        .get("customGuard")
                        .and_then(|g| g.get("role"))
                        .and_then(|r| r.as_str())
                        .unwrap_or("none");
                    format!("jwt={has_jwt},custom={has_custom},sub={sub},role={role}")
                }
                _ => "no-claims".to_string(),
            }
        },
    );

    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    let token = create_jwt(
        "multi-guard-user".to_string(),
        "test-audience".to_string(),
        issuer_url,
        Utc::now() + Duration::hours(1),
    )
    .unwrap();

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token={token}"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();
    let body_str = std::str::from_utf8(&body).unwrap();
    assert_eq!(
        body_str,
        "jwt=true,custom=true,sub=multi-guard-user,role=admin"
    );
}

#[test_log::test(tokio::test)]
async fn test_multi_guard_chain_second_guard_forbidden_returns_403() {
    let oidc_server = setup_test_oidc_server();
    let issuer_url = oidc_server.url("").to_string();
    let tmp_blueprint = write_multi_guard_blueprint(&issuer_url);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    // Register a custom guard that always returns Forbidden.
    app.register_custom_auth_guard("customGuard", ForbiddenCustomAuthGuard)
        .await;

    app.register_http_handler("/protected", "GET", protected_handler);

    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    // JWT is valid but the second guard (customGuard) rejects with Forbidden.
    let token = create_jwt(
        "test-subject".to_string(),
        "test-audience".to_string(),
        issuer_url,
        Utc::now() + Duration::hours(1),
    )
    .unwrap();

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token={token}"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 403);
}

#[test_log::test(tokio::test)]
async fn test_multi_guard_chain_first_guard_fails_short_circuits() {
    let oidc_server = setup_test_oidc_server();
    let issuer_url = oidc_server.url("").to_string();
    let tmp_blueprint = write_multi_guard_blueprint(&issuer_url);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    // Register the custom auth guard (won't be reached due to JWT failure).
    app.register_custom_auth_guard("customGuard", TestCustomAuthGuard)
        .await;

    app.register_http_handler("/protected", "GET", protected_handler);

    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    // Send an invalid JWT — the first guard (jwt) should fail with 401.
    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/protected?token=invalid-jwt"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 401);
}

#[test_log::test(tokio::test)]
async fn test_per_handler_protectedby_override_with_multi_guard() {
    let oidc_server = setup_test_oidc_server();
    let issuer_url = oidc_server.url("").to_string();
    let tmp_blueprint = write_per_handler_override_blueprint(&issuer_url);
    let blueprint_path = tmp_blueprint.path().to_str().unwrap().to_string();
    let env_vars = create_env_vars(&blueprint_path);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    // Register the custom auth guard.
    app.register_custom_auth_guard("customGuard", TestCustomAuthGuard)
        .await;

    // Register a handler that returns the claims shape.
    app.register_http_handler(
        "/overridden",
        "GET",
        |req: axum::extract::Request| async move {
            use celerity_runtime_core::auth_http::AuthContext;
            let claims = req.extensions().get::<AuthContext>().cloned();
            match claims {
                Some(AuthContext(Some(value))) => {
                    let has_jwt = value.get("jwt").is_some();
                    let has_custom = value.get("customGuard").is_some();
                    format!("jwt={has_jwt},custom={has_custom}")
                }
                _ => "no-claims".to_string(),
            }
        },
    );

    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    let token = create_jwt(
        "override-user".to_string(),
        "test-audience".to_string(),
        issuer_url,
        Utc::now() + Duration::hours(1),
    )
    .unwrap();

    let client = http_client();
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/overridden?token={token}"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();
    let body_str = std::str::from_utf8(&body).unwrap();
    assert_eq!(body_str, "jwt=true,custom=true");
}
