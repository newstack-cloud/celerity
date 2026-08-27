// The handler stream these tests drive is served on a unix socket.
#![cfg(unix)]

//! The IPC path for an HTTP API: a request reaches a handler over a real gRPC
//! stream on a real Unix socket and its response comes back, including what
//! happens when the handler is slow, absent, or answering with bytes.

mod common;

use std::{collections::HashMap, time::Duration};

use axum::{body::Body, http::Request};
use celerity_runtime_core::ipc_proto::{self as proto};
use common::ipc::{http_client, json_response, start_runtime, HandlerStub};
use http_body_util::BodyExt;

#[test_log::test(tokio::test)]
async fn serves_an_http_request_through_a_handler_over_the_stream() {
    let (_app, addr, socket) = start_runtime(
        "ipc-http",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler =
        HandlerStub::attach(&socket, |_| Some(json_response(r#"{"id":"order-1"}"#))).await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .uri(format!(
                    "http://{addr}/orders/order-1?expand=items&expand=totals"
                ))
                .header("Host", "localhost")
                .header("X-Trace", "abc")
                .body(Body::empty())
                .unwrap(),
        ),
    )
    .await
    .expect("the request should be served rather than time out")
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

    // The frame the handler received should describe the request faithfully.
    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the request");
    assert_eq!(dispatch.handler_tag, "GET::/orders/{orderId}");
    assert!(dispatch.deadline_unix_ms > 0);

    let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
        panic!("expected an HTTP source");
    };
    assert_eq!(request.method, "GET");
    assert_eq!(request.path, "/orders/order-1");
    assert_eq!(request.route, "/orders/{orderId}");
    assert_eq!(
        request.path_params.get("orderId").map(|v| v.values.clone()),
        Some(vec!["order-1".to_string()])
    );
    // Repeated query parameters survive as multiple values.
    assert_eq!(
        request.query_params.get("expand").map(|v| v.values.clone()),
        Some(vec!["items".to_string(), "totals".to_string()])
    );
    // Header names reach the handler lowercased however they were sent.
    assert_eq!(
        request.headers.get("x-trace").map(|v| v.values.clone()),
        Some(vec!["abc".to_string()])
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn splits_a_catch_all_path_into_segments_without_losing_encoded_separators() {
    let (_app, addr, socket) = start_runtime(
        "ipc-catch-all",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(json_response(r#"{"ok":true}"#))).await;

    // The middle segment carries an encoded separator, which must stay inside
    // that segment rather than splitting it in two.
    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .uri(format!("http://{addr}/files/docs/a%2Fb/report%20final.pdf"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        ),
    )
    .await
    .expect("the request should be served rather than time out")
    .unwrap();
    assert_eq!(response.status(), 200);

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the request");
    let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
        panic!("expected an HTTP source");
    };
    assert_eq!(request.route, "/files/{*filePath}");
    assert_eq!(
        request
            .path_params
            .get("filePath")
            .map(|v| v.values.clone()),
        Some(vec![
            "docs".to_string(),
            "a/b".to_string(),
            "report final.pdf".to_string(),
        ])
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn carries_a_request_body_that_is_not_text_without_corrupting_it() {
    let (_app, addr, socket) = start_runtime(
        "ipc-binary-body",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(json_response("{}"))).await;

    // Bytes a lossy UTF-8 conversion would replace with the replacement
    // character, destroying them.
    let payload: Vec<u8> = vec![0xff, 0xfe, 0x00, 0x80, 0x01, 0x02];
    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .method("POST")
                .uri(format!("http://{addr}/orders"))
                .header("Host", "localhost")
                .header("content-type", "application/octet-stream")
                .body(Body::from(payload.clone()))
                .unwrap(),
        ),
    )
    .await
    .expect("the request should be served")
    .unwrap();
    assert_eq!(response.status(), 200);

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the request");
    let Some(proto::dispatch::Source::Http(request)) = dispatch.source else {
        panic!("expected an HTTP source");
    };
    // The handler receives the bytes exactly, not a base64 string and not a
    // string of replacement characters.
    assert_eq!(request.body, payload);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn serves_a_binary_response_body_from_an_http_handler() {
    let raw: Vec<u8> = vec![0x89, 0x50, 0x4e, 0x47, 0x00, 0xff];
    let (_app, addr, socket) = start_runtime(
        "ipc-http-binary-response",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| {
        Some(proto::result::Outcome::Http(proto::HttpResponse {
            status: 200,
            headers: HashMap::from([(
                "content-type".to_string(),
                proto::Values {
                    values: vec!["image/png".to_string()],
                },
            )]),
            body: vec![0x89, 0x50, 0x4e, 0x47, 0x00, 0xff],
        }))
    })
    .await;

    let response = tokio::time::timeout(
        Duration::from_secs(10),
        http_client().request(
            Request::builder()
                .uri(format!("http://{addr}/orders/order-1"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        ),
    )
    .await
    .expect("the request should be served rather than time out")
    .unwrap();

    assert_eq!(response.status(), 200);
    let body = response.into_body().collect().await.unwrap().to_bytes();

    // The production response path carries bytes end to end, so a handler can
    // answer with an image or a protobuf and nothing touches it.
    assert_eq!(&body[..], &raw[..]);

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn returns_504_when_the_handler_never_answers() {
    let (_app, addr, socket) = start_runtime(
        "ipc-timeout",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    // Withhold every result, as a handler that has stopped responding would.
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    // The POST route's timeout is deliberately short in this fixture.
    let response = tokio::time::timeout(
        Duration::from_secs(20),
        http_client().request(
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

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn sheds_with_503_when_no_handler_stream_is_attached() {
    let (_app, addr, socket) = start_runtime(
        "ipc-no-handler",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    // No handler stub connects, which is what an application whose handlers
    // executable has not started, or has crashed, looks like.

    let started = tokio::time::Instant::now();
    let response = tokio::time::timeout(
        Duration::from_secs(20),
        http_client().request(
            Request::builder()
                .uri(format!("http://{addr}/orders/order-1"))
                .header("Host", "localhost")
                .body(Body::empty())
                .unwrap(),
        ),
    )
    .await
    .expect("the runtime should answer rather than hang")
    .unwrap();

    // A capacity signal rather than a fault, and retryable.
    assert_eq!(response.status(), 503);
    assert!(response.headers().get("retry-after").is_some());
    // This route's handler timeout is the sixty second default, so answering
    // in seconds is what distinguishes shedding from waiting it out.
    assert!(
        started.elapsed() < Duration::from_secs(15),
        "the request should have been shed after the grace window, took {:?}",
        started.elapsed()
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn tells_the_handler_to_stop_work_whose_deadline_has_passed() {
    let (_app, addr, socket) = start_runtime(
        "ipc-cancel-deadline",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;
    // Withhold every result, so the event outlives its deadline.
    let mut handler = HandlerStub::attach(&socket, |_| None).await;

    // The POST route's timeout is deliberately short in this fixture.
    let response = tokio::time::timeout(
        Duration::from_secs(20),
        http_client().request(
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

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the handler should have been given the request");
    let cancel = handler
        .next_cancel()
        .await
        .expect("the handler should be told to stop work nobody is waiting for");
    assert_eq!(cancel.id, dispatch.id);
    assert_eq!(cancel.reason(), proto::cancel::Reason::DeadlineExceeded);

    let _ = tokio::fs::remove_file(&socket).await;
}
