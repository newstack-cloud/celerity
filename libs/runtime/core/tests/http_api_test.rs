use axum::{body::Body, http::Request};
use celerity_runtime_core::{application::Application, config::RuntimeConfig};
use http_body_util::BodyExt;

mod common;

#[test_log::test(tokio::test)]
async fn sets_up_and_runs_http_server_application_in_ffi_mode() {
    let env_vars = common::MockEnvVars::new(Some(
        vec![
            (
                "CELERITY_BLUEPRINT",
                "tests/data/fixtures/http-api.blueprint.yaml".to_string(),
            ),
            ("CELERITY_SERVICE_NAME", "http-api-test".to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ffi".to_string()),
            ("CELERITY_SERVER_PORT", "2345".to_string()),
            ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
            ("CELERITY_TEST_MODE", "true".to_string()),
            (
                "CELERITY_VARIABLE_secretStoreId",
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId",
                "certificate-id".to_string(),
            ),
            ("CELERITY_VARIABLE_logLevel", "DEBUG".to_string()),
            (
                "CELERITY_VARIABLE_paymentApiSecret",
                "payment-api-secret".to_string(),
            ),
            ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
        ]
        .into_iter()
        .collect(),
    ));
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();

    app.register_http_handler("/hello", "GET", hello_handler);
    let app_info = app.run(false).await.unwrap();

    let client = hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
        .build_http();

    println!("About to make request!");
    let response = client
        .request(
            Request::builder()
                .uri(format!(
                    "http://{addr}/hello",
                    addr = app_info.http_server_address.unwrap()
                ))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();
    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200);
    assert_eq!(&body[..], b"Hello, World!");
}

async fn hello_handler() -> &'static str {
    "Hello, World!"
}

fn http_client(
) -> hyper_util::client::legacy::Client<hyper_util::client::legacy::connect::HttpConnector, Body> {
    hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new()).build_http()
}

/// Sets up an app with the existing blueprint (which has CORS configured)
/// using port 0 to avoid conflicts with the fixed-port test above.
async fn setup_cors_app() -> (Application, std::net::SocketAddr) {
    let env_vars = common::MockEnvVars::new(Some(
        vec![
            (
                "CELERITY_BLUEPRINT",
                "tests/data/fixtures/http-api.blueprint.yaml".to_string(),
            ),
            ("CELERITY_SERVICE_NAME", "http-cors-test".to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ffi".to_string()),
            ("CELERITY_SERVER_PORT", "0".to_string()),
            ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
            ("CELERITY_TEST_MODE", "true".to_string()),
            (
                "CELERITY_VARIABLE_secretStoreId",
                "secret-store-id".to_string(),
            ),
            (
                "CELERITY_VARIABLE_certificateId",
                "certificate-id".to_string(),
            ),
            ("CELERITY_VARIABLE_logLevel", "DEBUG".to_string()),
            (
                "CELERITY_VARIABLE_paymentApiSecret",
                "payment-api-secret".to_string(),
            ),
            ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
        ]
        .into_iter()
        .collect(),
    ));
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    let _ = app.setup().unwrap();
    // Register a handler at a path NOT in the blueprint so auth is skipped.
    app.register_http_handler("/cors-test", "GET", hello_handler);
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();
    (app, addr)
}

#[test_log::test(tokio::test)]
async fn test_http_response_includes_cors_headers() {
    let (_app, addr) = setup_cors_app().await;
    let client = http_client();

    // The blueprint allows origin "https://example.com".
    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/cors-test"))
                .header("Host", "localhost")
                .header("Origin", "https://example.com")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    assert_eq!(response.status(), 200);
    let allow_origin = response
        .headers()
        .get("access-control-allow-origin")
        .map(|v| v.to_str().unwrap().to_string());
    assert_eq!(
        allow_origin,
        Some("https://example.com".to_string()),
        "response should include CORS allow-origin header for allowed origin"
    );
}

#[test_log::test(tokio::test)]
async fn test_cors_preflight_returns_correct_headers() {
    let (_app, addr) = setup_cors_app().await;
    let client = http_client();

    let response = client
        .request(
            Request::builder()
                .method("OPTIONS")
                .uri(format!("http://{addr}/cors-test"))
                .header("Host", "localhost")
                .header("Origin", "https://example.com")
                .header("Access-Control-Request-Method", "GET")
                .header("Access-Control-Request-Headers", "Authorization")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    // Preflight should succeed.
    assert_eq!(response.status(), 200);
    assert_eq!(
        response
            .headers()
            .get("access-control-allow-origin")
            .map(|v| v.to_str().unwrap()),
        Some("https://example.com"),
    );
    // Allow methods should be present.
    assert!(
        response
            .headers()
            .get("access-control-allow-methods")
            .is_some(),
        "preflight should include allow-methods header"
    );
    // Max-age should be present (configured as 3600 in blueprint).
    assert!(
        response.headers().get("access-control-max-age").is_some(),
        "preflight should include max-age header"
    );
}

#[test_log::test(tokio::test)]
async fn test_cors_disallowed_origin_gets_no_allow_origin() {
    let (_app, addr) = setup_cors_app().await;
    let client = http_client();

    let response = client
        .request(
            Request::builder()
                .uri(format!("http://{addr}/cors-test"))
                .header("Host", "localhost")
                .header("Origin", "https://evil.example.com")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    // The request still goes through (CORS is enforced by the browser),
    // but the response should NOT include allow-origin for the disallowed origin.
    let allow_origin = response
        .headers()
        .get("access-control-allow-origin")
        .map(|v| v.to_str().unwrap().to_string());
    assert!(
        allow_origin.is_none() || allow_origin.as_deref() != Some("https://evil.example.com"),
        "response should not include allow-origin for disallowed origin"
    );
}
