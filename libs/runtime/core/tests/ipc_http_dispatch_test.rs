//! End-to-end coverage for HTTP handling in the IPC runtime call mode.
//!
//! No test covered this path before, because the path did not exist: nothing
//! registered Axum routes for the blueprint's HTTP handlers outside the FFI
//! call mode, so an HTTP API could not work in this mode at all.
//!
//! These tests stand in for the handlers executable by taking events off the
//! queue and returning results through the in-flight table, which is what the
//! polling local runtime API does today and what the gRPC stream will do once
//! it replaces it.

use std::{collections::HashMap, time::Duration};

use axum::{body::Body, http::Request};
use celerity_runtime_core::{
    application::Application,
    config::RuntimeConfig,
    event_queue::EventQueueHandles,
    types::{EventData, EventResult, EventResultData, HttpResponseData},
};
use http_body_util::BodyExt;

mod common;

fn ipc_env(service_name: &str) -> common::MockEnvVars<'_> {
    common::MockEnvVars::new(Some(
        vec![
            (
                "CELERITY_BLUEPRINT",
                "tests/data/fixtures/ipc-http-api.blueprint.yaml".to_string(),
            ),
            ("CELERITY_SERVICE_NAME", service_name.to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ipc".to_string()),
            ("CELERITY_SERVER_PORT", "0".to_string()),
            ("CELERITY_LOCAL_API_PORT", "0".to_string()),
            ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
            ("CELERITY_TEST_MODE", "true".to_string()),
            ("CELERITY_VARIABLE_logLevel", "DEBUG".to_string()),
            ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
        ]
        .into_iter()
        .collect(),
    ))
}

/// Stands in for the handlers executable: takes the next event off the queue
/// and answers it with the response the caller supplies.
fn spawn_stub_handler(
    event_queue: EventQueueHandles,
    respond: impl FnOnce(&EventData) -> HttpResponseData + Send + 'static,
) -> tokio::task::JoinHandle<EventData> {
    tokio::spawn(async move {
        let (result_tx, event) = event_queue
            .receiver
            .lock()
            .await
            .recv()
            .await
            .expect("an event should be dispatched to the queue");

        let response = respond(&event);
        result_tx
            .send((
                event.clone(),
                EventResult {
                    event_id: event.id.clone(),
                    data: EventResultData::HttpResponse(response),
                    context: None,
                },
            ))
            .expect("the runtime should still be awaiting the result");
        event
    })
}

#[test_log::test(tokio::test)]
async fn dispatches_an_http_request_to_the_event_queue_and_returns_the_handler_response() {
    let env_vars = ipc_env("ipc-http-dispatch-test");
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();

    let event_queue = app
        .event_queue()
        .expect("the IPC call mode should create an event queue");
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    let handler = spawn_stub_handler(event_queue, |_event| HttpResponseData {
        status: 200,
        headers: HashMap::from([("content-type".to_string(), "application/json".to_string())]),
        body: r#"{"id":"order-1"}"#.to_string(),
    });

    let client = hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
        .build_http();
    let response = client
        .request(
            Request::builder()
                .uri(format!(
                    "http://{addr}/orders/order-1?expand=items&expand=totals"
                ))
                .header("Host", "localhost")
                .header("X-Trace", "abc")
                .body(Body::empty())
                .unwrap(),
        )
        .await
        .unwrap();

    let status = response.status();
    let content_type = response
        .headers()
        .get("content-type")
        .map(|value| value.to_str().unwrap().to_string());
    let body = response.into_body().collect().await.unwrap().to_bytes();

    assert_eq!(status, 200);
    assert_eq!(content_type, Some("application/json".to_string()));
    assert_eq!(&body[..], br#"{"id":"order-1"}"#);

    // The event the handler received should describe the request faithfully.
    let event = handler.await.unwrap();
    assert_eq!(event.handler_tag, "GET::/orders/{orderId}");

    let celerity_runtime_core::types::EventDataPayload::HttpRequestEventData(request) = event.data
    else {
        panic!("expected an HTTP request event");
    };
    assert_eq!(request.method, "get");
    assert_eq!(request.path, "/orders/order-1");
    assert_eq!(request.route, "/orders/{orderId}");
    assert_eq!(
        request.path_params.get("orderId"),
        Some(&"order-1".to_string())
    );
    // Repeated query params are preserved in the multi-valued map and the first
    // wins in the single-valued one.
    assert_eq!(
        request.multi_query_params.get("expand"),
        Some(&vec!["items".to_string(), "totals".to_string()])
    );
    assert_eq!(
        request.query_params.get("expand"),
        Some(&"items".to_string())
    );
    // Header names reach the handler lowercased regardless of how they were sent.
    assert_eq!(request.headers.get("x-trace"), Some(&"abc".to_string()));
}

#[test_log::test(tokio::test)]
async fn returns_504_when_no_handler_returns_a_result_within_the_timeout() {
    let env_vars = ipc_env("ipc-http-timeout-test");
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();

    let event_queue = app
        .event_queue()
        .expect("the IPC call mode should create an event queue");
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    // Take the event but never answer it, as a dead handlers executable would.
    let swallowed = tokio::spawn(async move {
        let taken = event_queue.receiver.lock().await.recv().await;
        // Hold the result sender so the request waits on its deadline rather
        // than being woken by a dropped channel.
        tokio::time::sleep(Duration::from_secs(10)).await;
        drop(taken);
    });

    let client = hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
        .build_http();
    let response = tokio::time::timeout(
        Duration::from_secs(15),
        client.request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/orders"))
                .header("Host", "localhost")
                .body(Body::from(r#"{"sku":"abc"}"#))
                .unwrap(),
        ),
    )
    .await
    .expect("the runtime should time the request out rather than hang")
    .unwrap();

    assert_eq!(response.status(), 504);
    swallowed.abort();
}
