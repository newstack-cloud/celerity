// The handler stream these tests drive is served on a unix socket.
#![cfg(unix)]

//! The IPC path for handlers invoked by name, rather than by a request or a
//! message arriving. Covers the invoke endpoint, what it will and will not
//! address, and what comes back.

mod common;

use std::time::Duration;

use axum::{body::Body, http::Request};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_runtime_core::ipc_proto::{self as proto};
use common::ipc::{http_client, start_runtime, start_runtime_with, HandlerStub};
use http_body_util::BodyExt;

#[test_log::test(tokio::test)]
async fn invokes_a_custom_handler_by_name_over_the_stream() {
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Custom(proto::CustomInvokeResult {
            output: br#"{"reindexed":12}"#.to_vec(),
            error_message: String::new(),
        }))
    })
    .await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"reindexHandler","invocationType":"requestResponse","payload":{"full":true}}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be served rather than time out")
    .unwrap();

    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200, "body: {}", String::from_utf8_lossy(&body));
    let body: serde_json::Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(body["data"], r#"{"reindexed":12}"#);

    // The invocation reached the handlers executable as an ordinary event, on
    // the custom tag, carrying the payload the caller sent.
    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the invocation");
    assert_eq!(dispatch.handler_tag, "custom::reindexHandler");
    let Some(proto::dispatch::Source::Custom(invoke)) = dispatch.source else {
        panic!("expected a custom invocation source");
    };
    assert_eq!(invoke.handler_name, "reindexHandler");
    assert_eq!(invoke.input, br#"{"full":true}"#);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn invokes_an_http_handler_by_name_without_shaping_a_request() {
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-http",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Custom(proto::CustomInvokeResult {
            output: br#"{"id":"order-1"}"#.to_vec(),
            error_message: String::new(),
        }))
    })
    .await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"getOrderHandler","invocationType":"requestResponse","payload":{"pathParams":{"orderId":"order-1"}}}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be served rather than time out")
    .unwrap();

    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200, "body: {}", String::from_utf8_lossy(&body));

    // Routed on the HTTP handler's own tag, since that is the tag the handler
    // stream serves, and carrying the payload straight through rather than
    // being dressed up as a request.
    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the invocation");
    assert_eq!(dispatch.handler_tag, "GET::/orders/{orderId}");
    let Some(proto::dispatch::Source::Custom(invoke)) = dispatch.source else {
        panic!("expected a direct invocation source");
    };
    assert_eq!(invoke.handler_name, "getOrderHandler");
    assert_eq!(invoke.input, br#"{"pathParams":{"orderId":"order-1"}}"#);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn invokes_a_handler_by_the_name_the_blueprint_publishes_it_under() {
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-published",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Custom(proto::CustomInvokeResult {
            output: br#"{"ok":true}"#.to_vec(),
            error_message: String::new(),
        }))
    })
    .await;

    // `spec.handlerName`, which is what a deployment addresses this handler by
    // and what the invoke API documents, rather than the blueprint resource it
    // is declared as.
    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"Orders-GetOrderHandler-v1","invocationType":"requestResponse"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be served rather than time out")
    .unwrap();

    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200, "body: {}", String::from_utf8_lossy(&body));

    // It reaches the same handler the resource name would.
    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the invocation");
    assert_eq!(dispatch.handler_tag, "GET::/orders/{orderId}");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn refuses_to_invoke_a_handler_the_blueprint_does_not_declare() {
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-unknown",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    let started = tokio::time::Instant::now();
    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"noSuchHandler","invocationType":"requestResponse"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be refused rather than time out")
    .unwrap();

    assert_eq!(response.status(), 404);
    // Refused on the name, so the caller is not left waiting for a stream to
    // claim a tag that is not in the blueprint at all.
    assert!(started.elapsed() < Duration::from_secs(2));

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn serves_the_invoke_endpoint_for_an_application_with_no_http_api() {
    // The case the endpoint matters most in: nothing here can be triggered from
    // outside, so without this there is no way to run these handlers by hand.
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-no-api",
        "tests/data/fixtures/ipc-handlers-only.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Custom(proto::CustomInvokeResult {
            output: br#"{"reconciled":3}"#.to_vec(),
            error_message: String::new(),
        }))
    })
    .await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"reconcileHandler","invocationType":"requestResponse"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be served rather than time out")
    .unwrap();

    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200, "body: {}", String::from_utf8_lossy(&body));
    let body: serde_json::Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(body["data"], r#"{"reconciled":3}"#);

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the invocation");
    assert_eq!(dispatch.handler_tag, "custom::reconcileHandler");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn does_not_serve_the_invoke_endpoint_unless_it_is_enabled() {
    // Test mode and a local platform are both still on, as they are for every
    // other test here, so this pins the switch as a condition in its own right.
    let (_app, addr, socket) = start_runtime_with(
        "ipc-invoke-disabled",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
        &[("CELERITY_ENABLE_LOCAL_INVOKE", "false")],
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"reindexHandler","invocationType":"requestResponse"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the request should be answered rather than hang")
    .unwrap();

    // Not routed at all, as opposed to routed and refused.
    assert_eq!(response.status(), 404);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn returns_binary_output_from_an_invoked_handler_without_corrupting_it() {
    let raw: Vec<u8> = vec![0xff, 0xfe, 0x00, 0x80, 0x01];
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-binary",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Custom(proto::CustomInvokeResult {
            output: vec![0xff, 0xfe, 0x00, 0x80, 0x01],
            error_message: String::new(),
        }))
    })
    .await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"reindexHandler","invocationType":"requestResponse"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("the invocation should be served rather than time out")
    .unwrap();

    let status = response.status();
    let body = response.into_body().collect().await.unwrap().to_bytes();
    assert_eq!(status, 200, "body: {}", String::from_utf8_lossy(&body));

    // A handler returning an image or a protobuf can still be exercised here.
    // The bytes come back encoded, and the response says they are encoded, so
    // they cannot be mistaken for text.
    let body: serde_json::Value = serde_json::from_slice(&body).unwrap();
    assert_eq!(body["dataEncoding"], "base64");
    let decoded = BASE64
        .decode(body["data"].as_str().expect("output should be carried"))
        .expect("the output should be base64");
    assert_eq!(decoded, raw);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn lets_an_async_invocation_run_on_after_the_caller_has_been_answered() {
    let (_app, addr, socket) = start_runtime(
        "ipc-invoke-async",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    // Withhold the result, so the handler is still working when the caller has
    // already been answered and its receiver dropped.
    let mut handler = HandlerStub::attach(&socket, |_| None).await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/runtime/handlers/invoke"))
                .header("Host", "localhost")
                .header("content-type", "application/json")
                .body(Body::from(
                    r#"{"handlerName":"reindexHandler","invocationType":"async"}"#,
                ))
                .unwrap(),
        ),
    )
    .await
    .expect("an async invocation should be answered without waiting for the handler")
    .unwrap();
    assert_eq!(response.status(), 200);

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the invocation");
    assert_eq!(dispatch.handler_tag, "custom::reindexHandler");

    // The distinguishing property of an async invocation. Nobody is waiting for
    // the result, but the work was asked for and should run to completion, so
    // dropping the caller's receiver must not read as the caller going away.
    let cancel = tokio::time::timeout(Duration::from_millis(500), handler.cancels.recv()).await;
    assert!(
        cancel.is_err(),
        "an async invocation should not be cancelled once its caller has been answered"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}
