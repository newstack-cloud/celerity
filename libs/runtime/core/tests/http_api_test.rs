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
        ]
        .into_iter()
        .collect(),
    ));
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config);
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
