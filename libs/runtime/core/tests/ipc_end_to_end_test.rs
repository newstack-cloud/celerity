//! Covers the IPC path end to end: requests and WebSocket messages reach a
//! handler over a real gRPC stream on a real Unix socket, and their results
//! come back.
//!
//! Every piece has unit coverage of its own. What these cover is the assembly,
//! and they do it the way a handlers executable would rather than by reaching
//! into the runtime's internals, so nothing here can pass while the transport
//! is broken.

use std::{collections::HashMap, time::Duration};

use axum::{body::Body, http::Request};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_runtime_core::{
    application::Application,
    config::RuntimeConfig,
    ipc_proto::{
        self as proto, handler_message,
        handler_runtime_service_client::HandlerRuntimeServiceClient, runtime_message,
    },
};
use futures::SinkExt;
use http_body_util::BodyExt;
use serde_json::json;
use tokio::{net::UnixStream, sync::mpsc};
use tokio_stream::StreamExt;
use tonic::transport::{Endpoint, Uri};

mod common;

fn ipc_env(
    service_name: &str,
    fixture: &str,
    socket: &str,
    overrides: &[(&'static str, &str)],
) -> common::MockEnvVars<'static> {
    let mut vars: std::collections::HashMap<&'static str, String> = vec![
        ("CELERITY_BLUEPRINT", fixture.to_string()),
        ("CELERITY_SERVICE_NAME", service_name.to_string()),
        ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
        ("CELERITY_RUNTIME_CALL_MODE", "ipc".to_string()),
        ("CELERITY_SERVER_PORT", "0".to_string()),
        ("CELERITY_RUNTIME_SOCKET_FALLBACK_PORT", "0".to_string()),
        ("CELERITY_RUNTIME_SOCKET", socket.to_string()),
        ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
        ("CELERITY_TEST_MODE", "true".to_string()),
        ("CELERITY_ENABLE_LOCAL_INVOKE", "true".to_string()),
        ("CELERITY_VARIABLE_logLevel", "DEBUG".to_string()),
        ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
    ]
    .into_iter()
    .collect();
    for (key, value) in overrides {
        vars.insert(key, value.to_string());
    }
    common::MockEnvVars::new(Some(vars))
}

/// A socket path unique to this test, so tests can run alongside each other.
fn socket_path(name: &str) -> String {
    std::env::temp_dir()
        .join(format!("celerity-ipc-{}-{name}.sock", std::process::id()))
        .to_string_lossy()
        .into_owned()
}

/// Starts a runtime serving the given blueprint, returning its public address
/// and the socket its handler stream is on.
async fn start_runtime(name: &str, fixture: &str) -> (Application, std::net::SocketAddr, String) {
    start_runtime_with(name, fixture, &[]).await
}

/// Starts a runtime with environment overrides applied on top of the defaults.
async fn start_runtime_with(
    name: &str,
    fixture: &str,
    overrides: &[(&'static str, &str)],
) -> (Application, std::net::SocketAddr, String) {
    let socket = socket_path(name);
    let env_vars = ipc_env(name, fixture, &socket, overrides);
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();
    (app, addr, socket)
}

/// Stands in for a handlers executable.
struct HandlerStub {
    /// The events the handler was asked to process.
    dispatches: mpsc::Receiver<proto::Dispatch>,
    /// The events the handler was told to stop working on.
    cancels: mpsc::Receiver<proto::Cancel>,
}

impl HandlerStub {
    /// Connects, completes the handshake by declaring every tag the runtime
    /// asked for, then answers dispatches with whatever `respond` returns.
    ///
    /// Returning `None` withholds a result, which is how a handler that has
    /// stopped answering is simulated.
    async fn attach(
        socket: &str,
        respond: impl Fn(&proto::Dispatch) -> Option<proto::result::Outcome> + Send + 'static,
    ) -> Self {
        let channel = Endpoint::try_from("http://[::]:50051")
            .unwrap()
            .connect_with_connector(tower_tonic::service_fn({
                let socket = socket.to_string();
                move |_: Uri| {
                    let socket = socket.clone();
                    async move {
                        Ok::<_, std::io::Error>(hyper_util::rt::TokioIo::new(
                            UnixStream::connect(socket).await?,
                        ))
                    }
                }
            }))
            .await
            .expect("the runtime should be serving the handler stream");

        let mut client = HandlerRuntimeServiceClient::new(channel);
        let (handler_tx, handler_rx) = mpsc::channel::<proto::HandlerMessage>(16);
        let mut frames = client
            .event_stream(tokio_stream::wrappers::ReceiverStream::new(handler_rx))
            .await
            .expect("the stream should be accepted")
            .into_inner();

        let Some(Ok(proto::RuntimeMessage {
            frame: Some(runtime_message::Frame::Config(config)),
        })) = frames.next().await
        else {
            panic!("expected configuration to arrive before anything else");
        };

        handler_tx
            .send(proto::HandlerMessage {
                frame: Some(handler_message::Frame::Ready(proto::Ready {
                    handler_tags: config
                        .handlers
                        .iter()
                        .map(|handler| handler.handler_tag.clone())
                        .collect(),
                    initial_credit: 8,
                    sdk_version: "test/0.1".to_string(),
                    limits: vec![],
                })),
            })
            .await
            .unwrap();

        let Some(Ok(proto::RuntimeMessage {
            frame: Some(runtime_message::Frame::ReadyAck(ack)),
        })) = frames.next().await
        else {
            panic!("expected a ready acknowledgement");
        };
        assert!(
            ack.accepted,
            "the handshake should be accepted, unknown={:?} unhandled={:?}",
            ack.unknown_tags, ack.unhandled_tags
        );

        let (dispatch_tx, dispatches) = mpsc::channel(16);
        let (cancel_tx, cancels) = mpsc::channel(16);
        tokio::spawn(async move {
            while let Some(Ok(message)) = frames.next().await {
                let dispatch = match message.frame {
                    Some(runtime_message::Frame::Dispatch(dispatch)) => dispatch,
                    Some(runtime_message::Frame::Cancel(cancel)) => {
                        cancel_tx.send(cancel).await.ok();
                        continue;
                    }
                    _ => continue,
                };
                let outcome = respond(&dispatch);
                let id = dispatch.id.clone();
                dispatch_tx.send(dispatch).await.ok();

                if let Some(outcome) = outcome {
                    handler_tx
                        .send(proto::HandlerMessage {
                            frame: Some(handler_message::Frame::Result(proto::Result {
                                id,
                                credit_grant: 1,
                                outcome: Some(outcome),
                            })),
                        })
                        .await
                        .ok();
                }
            }
        });

        HandlerStub {
            dispatches,
            cancels,
        }
    }

    async fn next_dispatch(&mut self) -> Option<proto::Dispatch> {
        tokio::time::timeout(Duration::from_secs(10), self.dispatches.recv())
            .await
            .ok()
            .flatten()
    }

    async fn next_cancel(&mut self) -> Option<proto::Cancel> {
        tokio::time::timeout(Duration::from_secs(10), self.cancels.recv())
            .await
            .ok()
            .flatten()
    }
}

fn json_response(body: &'static str) -> proto::result::Outcome {
    proto::result::Outcome::Http(proto::HttpResponse {
        status: 200,
        headers: HashMap::from([(
            "content-type".to_string(),
            proto::Values {
                values: vec!["application/json".to_string()],
            },
        )]),
        body: body.as_bytes().to_vec(),
    })
}

fn websocket_ack() -> proto::result::Outcome {
    proto::result::Outcome::Websocket(proto::Ack {
        success: true,
        error_message: String::new(),
    })
}

fn http_client(
) -> hyper_util::client::legacy::Client<hyper_util::client::legacy::connect::HttpConnector, Body> {
    hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new()).build_http()
}

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
async fn restricts_the_handler_socket_to_the_runtime_user() {
    use std::os::unix::fs::PermissionsExt;

    let (_app, _addr, socket) = start_runtime(
        "ipc-socket-mode",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;

    // Anything able to connect can register as a handler and be given events,
    // so the permissions on this socket are the whole of the access control.
    let mode = tokio::fs::metadata(&socket)
        .await
        .expect("the socket should exist")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o600, "socket mode was {mode:o}");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn restricts_a_socket_directory_it_creates_itself() {
    use std::os::unix::fs::PermissionsExt;

    let dir = std::env::temp_dir().join(format!("celerity-ipc-dir-{}", std::process::id()));
    let _ = tokio::fs::remove_dir_all(&dir).await;
    let socket = dir.join("runtime.sock").to_string_lossy().into_owned();

    let (_app, _addr, _) = start_runtime_with(
        "ipc-socket-dir",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
        &[("CELERITY_RUNTIME_SOCKET", &socket)],
    )
    .await;

    // The socket's own mode is not the whole story. Enforcing it on connect is
    // a Linux behaviour rather than a portable one, and a permissive umask
    // would leave a directory another user could replace the socket in.
    let mode = tokio::fs::metadata(&dir)
        .await
        .expect("the runtime should have created the directory")
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o700, "directory mode was {mode:o}");

    let _ = tokio::fs::remove_dir_all(&dir).await;
}

#[test_log::test(tokio::test)]
async fn refuses_to_take_over_a_socket_another_runtime_is_listening_on() {
    let (_app, _addr, socket) = start_runtime(
        "ipc-socket-contended",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
    )
    .await;

    // A second runtime pointed at the same socket must not unlink it. Doing so
    // would leave the first serving a socket nothing can reach any more.
    let env_vars = ipc_env(
        "ipc-socket-contended-second",
        "tests/data/fixtures/ipc-http-api.blueprint.yaml",
        &socket,
        &[("CELERITY_SERVER_PORT", "0")],
    );
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut second = Application::new(runtime_config, Box::new(env_vars));
    second.setup().unwrap();

    let started = second.run(false).await;
    assert!(
        started.is_err(),
        "the second runtime should refuse to start rather than take the socket"
    );

    // The first is still reachable, which is the point of refusing.
    let _handler = HandlerStub::attach(&socket, |_| None).await;

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
async fn routes_a_websocket_message_to_a_handler_over_the_stream() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "event": "sendMessage", "data": { "text": "hello" } }).to_string(),
        ))
        .await
        .unwrap();

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the message should reach the handler");
    assert_eq!(dispatch.handler_tag, "event::sendMessage");

    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    assert_eq!(message.route, "sendMessage");
    assert!(!message.is_binary);
    assert!(!message.connection_id.is_empty());
    let body: serde_json::Value = serde_json::from_slice(&message.message).unwrap();
    assert_eq!(body["data"]["text"], "hello");

    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn answers_heartbeats_while_a_handler_is_still_running() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-heartbeat",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    // Withhold every result, standing in for a handler that takes a while.
    let mut handler = HandlerStub::attach(&socket, |_| None).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "event": "sendMessage", "data": { "text": "hello" } }).to_string(),
        ))
        .await
        .unwrap();

    // Once this arrives the handler is running and has not answered.
    handler
        .next_dispatch()
        .await
        .expect("the message should reach the handler");

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "ping": true }).to_string(),
        ))
        .await
        .unwrap();

    // A client whose heartbeat goes unanswered concludes the connection is
    // dead and reconnects, tearing down work that is still in progress. The
    // handler's own timeout is far longer than any heartbeat interval, so the
    // pong cannot wait on it.
    let pong = tokio::time::timeout(Duration::from_secs(3), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value.get("pong") == Some(&serde_json::Value::Bool(true)) {
                        return true;
                    }
                }
            }
        }
        false
    })
    .await;

    assert_eq!(
        pong,
        Ok(true),
        "the heartbeat should be answered while the handler is still running"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn sheds_a_connection_that_outruns_its_handlers_with_a_retry_hint() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-saturate",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    // Answer nothing, so the worker is stuck on the first message and every
    // message after it piles up behind it.
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // Comfortably more than the buffer holds, so the read loop reaches the
    // point where it would otherwise wait in silence.
    for index in 0..256 {
        if socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                json!({ "event": "sendMessage", "data": { "index": index } }).to_string(),
            ))
            .await
            .is_err()
        {
            // The runtime shed the connection part way through, which is the
            // behaviour under test.
            break;
        }
    }

    // Stalling in silence is what makes a client give up on a connection it
    // could have kept. Closing says the same thing in a way the client can act
    // on, and the hint stops it returning straight into the same saturation.
    let close = tokio::time::timeout(Duration::from_secs(10), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Close(frame) = message {
                return frame;
            }
        }
        None
    })
    .await
    .expect("the runtime should close the connection rather than stall in silence");

    let frame = close.expect("the close should carry a frame rather than be bare");
    assert_eq!(
        u16::from(frame.code),
        1013,
        "expected a try again later code"
    );
    let reason: serde_json::Value =
        serde_json::from_str(&frame.reason).expect("the reason should carry the retry hint");
    assert!(
        reason["retryAfter"].as_u64().is_some_and(|ms| ms > 0),
        "expected a retryAfter hint, got {reason}"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn reads_faster_than_one_message_every_ten_milliseconds() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-throughput",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // The read loop used to sleep 10ms per message to yield a lock it held
    // across every read, capping one connection at about 100 messages a
    // second whatever the handlers did. Reading no longer takes that lock, so
    // 50 messages must not take the 500ms that cap would have cost.
    let started = tokio::time::Instant::now();
    for index in 0..50 {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                json!({ "event": "sendMessage", "data": { "index": index } }).to_string(),
            ))
            .await
            .unwrap();
    }
    for _ in 0..50 {
        handler
            .next_dispatch()
            .await
            .expect("every message should reach the handler");
    }
    let elapsed = started.elapsed();

    // Below the 500ms the old cap would have cost, with enough room above the
    // handful of milliseconds this actually takes that a loaded machine does
    // not fail it for being busy.
    assert!(
        elapsed < Duration::from_millis(450),
        "50 messages took {elapsed:?}, which suggests the read loop is still rate limited"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn tells_an_unauthenticated_client_to_authenticate_first() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-auth-message",
        "tests/data/fixtures/ipc-websocket-auth-message.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the authMessage strategy should let the connection upgrade");

    // Anything that is not an authenticate message, sent before the connection
    // has authenticated. The runtime answers on the same socket it just read
    // from, so this is the path where taking the lock twice would wedge the
    // connection rather than refuse the message.
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "event": "sendMessage", "data": { "text": "hello" } }).to_string(),
        ))
        .await
        .unwrap();

    let rejected = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "error" {
                        return true;
                    }
                }
            }
        }
        false
    })
    .await;

    assert_eq!(
        rejected,
        Ok(true),
        "an unauthenticated message should be refused rather than leave the connection hanging"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

/// An acknowledgement is a message like any other until the client has proved
/// who it is.
///
/// Acknowledgements are taken out of the way before routing, and that used to
/// happen ahead of the authentication gate, so an unauthenticated client could
/// still settle a message and call off the resend and loss event that follow
/// it. The refusal below is the gate doing its job.
#[test_log::test(tokio::test)]
async fn refuses_an_acknowledgement_from_a_client_that_has_not_authenticated() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-auth-message-ack",
        "tests/data/fixtures/ipc-websocket-auth-message.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the authMessage strategy should let the connection upgrade");

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({
                "event": "ack",
                "data": { "messageId": "m-1", "timestamp": "2026-01-01T00:00:00.000Z" }
            })
            .to_string(),
        ))
        .await
        .unwrap();

    let rejected = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "error" {
                        return true;
                    }
                }
            }
        }
        false
    })
    .await;

    assert_eq!(
        rejected,
        Ok(true),
        "an acknowledgement before authentication should be refused rather than settle a message"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn closes_a_connection_out_without_waiting_for_all_the_work_it_queued() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-drain",
        "tests/data/fixtures/ipc-websocket-disconnect.blueprint.yaml",
    )
    .await;
    // Answer nothing, so every queued message runs its timeout out in turn.
    let mut handler = HandlerStub::attach(&socket, |_| None).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // Eight messages against a one second handler timeout, so draining every
    // one of them would take longer than the connection is given to close.
    for index in 0..8 {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                json!({ "event": "sendMessage", "data": { "index": index } }).to_string(),
            ))
            .await
            .unwrap();
    }
    handler
        .next_dispatch()
        .await
        .expect("the first message should reach the handler");

    let closed_at = tokio::time::Instant::now();
    socket_conn
        .close(None)
        .await
        .expect("the client should be able to close");

    // The disconnect handler running is what says teardown finished, and until
    // it does the connection is still in the registry and still counted. It
    // must not wait for messages the client is no longer there to hear about.
    //
    // That it runs exactly once is covered separately, by a client that sends
    // a close frame outright. Closing the way this one does drives the whole
    // handshake and never surfaces a frame to the application, so the second
    // path that could fire a disconnect is not reached from here.
    let disconnected = tokio::time::timeout(Duration::from_secs(7), async {
        while let Some(dispatch) = handler.next_dispatch().await {
            if let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source {
                if message.route == "$disconnect" {
                    return true;
                }
            }
        }
        false
    })
    .await;

    assert_eq!(
        disconnected,
        Ok(true),
        "the disconnect handler should run once the drain window passes"
    );
    assert!(
        closed_at.elapsed() < Duration::from_secs(7),
        "teardown took {:?}, which suggests it waited for the whole queue",
        closed_at.elapsed()
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn does_not_reorder_messages_when_handling_them_off_the_read_loop() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-order",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    for index in 0..5 {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                json!({ "event": "sendMessage", "data": { "index": index } }).to_string(),
            ))
            .await
            .unwrap();
    }

    // Moving handling off the read loop must not reorder what was already in
    // order, which is what pins the worker to one message at a time.
    //
    // This is the behaviour of this runtime rather than a promise the platform
    // makes. The same API deployed to a serverless target invokes a function
    // per message with no ordering between them, and a client resending after
    // a lost acknowledgement reorders its own messages anyway. An application
    // that needs ordering has to carry its own sequence. So this test exists to
    // keep the runtime's behaviour deliberate, not to stop it ever changing.
    let mut seen = Vec::new();
    for _ in 0..5 {
        let dispatch = handler
            .next_dispatch()
            .await
            .expect("every message should reach the handler");
        let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
            panic!("expected a WebSocket source");
        };
        let body: serde_json::Value = serde_json::from_slice(&message.message).unwrap();
        seen.push(body["data"]["index"].as_i64().unwrap());
    }
    assert_eq!(seen, vec![0, 1, 2, 3, 4]);

    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn acknowledges_a_binary_message_that_asks_to_be_acknowledged() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-ack-binary",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // [routeLength][route][requireAck][messageIdLength][messageId][message],
    // built the way the spec defines.
    // The message id is what the acknowledgement has to name.
    let route = b"sendMessage";
    let message_id = b"msg-1";
    let body: Vec<u8> = vec![0xde, 0xad, 0xbe, 0xef];
    let mut payload = vec![route.len() as u8];
    payload.extend_from_slice(route);
    payload.push(0x1);
    payload.push(message_id.len() as u8);
    payload.extend_from_slice(message_id);
    payload.extend_from_slice(&body);

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Binary(payload))
        .await
        .unwrap();

    let ack = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Binary(bytes) = message {
                if bytes.starts_with(&[0x1, 0x4, 0x0, 0x0]) {
                    return Some(bytes[4..].to_vec());
                }
            }
        }
        None
    })
    .await
    .expect("the acknowledgement should not take five seconds");

    let ack = ack.expect("a message that asks to be acknowledged should be acknowledged");
    let ack: serde_json::Value = serde_json::from_slice(&ack).unwrap();
    assert_eq!(ack["messageId"], "msg-1");
    assert!(
        ack["timestamp"].is_string(),
        "the acknowledgement should carry a timestamp, got {ack}"
    );

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the message should still reach the handler");
    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    assert_eq!(&message.message[..], &body[..]);
    // The handler is told the id the client used, so instrumentation on its
    // side can be tied to what the client saw acknowledged.
    assert_eq!(message.message_id, "msg-1");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn acknowledges_a_json_message_that_asks_to_be_acknowledged() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-ack-json",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({
                "event": "sendMessage",
                "messageId": "msg-json-1",
                "ack": true,
                "data": { "text": "hello" },
            })
            .to_string(),
        ))
        .await
        .unwrap();

    // Answered in the encoding it arrived in, so a client that only has text
    // because its environment gave it no choice can still read the reply.
    let ack = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "ack" {
                        return Some(value);
                    }
                }
            }
        }
        None
    })
    .await
    .expect("the acknowledgement should not take five seconds");

    let ack = ack.expect("a message that asks to be acknowledged should be acknowledged");
    assert_eq!(ack["data"]["messageId"], "msg-json-1");
    assert!(ack["data"]["timestamp"].is_string());

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn acknowledges_a_message_queued_behind_a_handler_that_is_still_running() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-ack-queued",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    // Answers nothing, so the first message is still being handled, for the
    // whole of the ten second timeout this fixture sets, while the second
    // arrives.
    let _handler = HandlerStub::attach(&socket, |_| None).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "event": "sendMessage", "data": { "first": true } }).to_string(),
        ))
        .await
        .unwrap();
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({
                "event": "sendMessage",
                "messageId": "queued-1",
                "ack": true,
                "data": { "second": true },
            })
            .to_string(),
        ))
        .await
        .unwrap();

    // Acknowledgement follows the message being taken in, not what happens to
    // it after. Tying it to handling would leave this one waiting on the
    // handler ahead of it, and a client that hears nothing sends it again for a
    // message the runtime was holding the entire time.
    let ack = tokio::time::timeout(Duration::from_secs(3), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "ack" {
                        return Some(value);
                    }
                }
            }
        }
        None
    })
    .await
    .expect("the acknowledgement should not wait for the handler ahead of it");

    let ack = ack.expect("the queued message should be acknowledged");
    assert_eq!(ack["data"]["messageId"], "queued-1");

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn does_not_route_a_client_acknowledgement_to_a_handler() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-inbound-ack",
        "tests/data/fixtures/ipc-websocket-default-route.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // What a client sends to acknowledge something the runtime sent it. A
    // reserved message names its route with `event`, and this API routes on
    // `action`, so left alone it carries no route the runtime knows and lands
    // on the default handler as though the application had been sent it.
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({
                "event": "ack",
                "data": { "messageId": "m-1", "timestamp": "1" },
            })
            .to_string(),
        ))
        .await
        .unwrap();
    let mut binary = vec![0x1, 0x4, 0x0, 0x0];
    binary.extend_from_slice(br#"{"messageId":"m-2","timestamp":"1"}"#);
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Binary(binary))
        .await
        .unwrap();

    // Sent last and expected first, since anything the runtime failed to take
    // out of the way would have reached the handler ahead of it.
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Text(
            json!({ "action": "sendMessage", "data": { "text": "hello" } }).to_string(),
        ))
        .await
        .unwrap();

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the ordinary message should reach the handler");
    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    let body: serde_json::Value = serde_json::from_slice(&message.message).unwrap();
    assert_eq!(
        body["data"]["text"], "hello",
        "an acknowledgement reached the handler ahead of the message that followed it"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn does_not_acknowledge_a_message_that_did_not_ask() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-ack-absent",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let _handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // Carries an id but does not opt in, and opting in without an id has
    // nothing to name. Neither should be answered.
    for message in [
        json!({ "event": "sendMessage", "messageId": "msg-2", "data": {} }),
        json!({ "event": "sendMessage", "ack": true, "data": {} }),
    ] {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                message.to_string(),
            ))
            .await
            .unwrap();
    }

    let unexpected = tokio::time::timeout(Duration::from_secs(2), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "ack" {
                        return true;
                    }
                }
            }
        }
        false
    })
    .await;

    assert_ne!(
        unexpected,
        Ok(true),
        "acknowledgement is opt in, and an opt in without a message id is ignored"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn carries_a_binary_websocket_frame_without_corrupting_it() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-binary",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // [routeLength][route][requireAck][messageIdLength][messageId][message],
    // here with no acknowledgement asked for and no message id.
    let route = b"sendMessage";
    let body: Vec<u8> = vec![0xff, 0xfe, 0x00, 0x80, 0x01, 0x02];
    let mut payload = vec![route.len() as u8];
    payload.extend_from_slice(route);
    payload.push(0);
    payload.push(0);
    payload.extend_from_slice(&body);

    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Binary(payload))
        .await
        .unwrap();

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the message should reach the handler");
    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };

    assert!(message.is_binary);
    // The bytes arrive exactly, with no base64 and no replacement characters.
    assert_eq!(message.message, body);
    // Guard against the encoding leaking onto the wire.
    assert_ne!(message.message, BASE64.encode(&body).into_bytes());

    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}

#[test_log::test(tokio::test)]
async fn fires_the_disconnect_handler_once_for_a_client_that_sends_a_close_frame() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-close-once",
        "tests/data/fixtures/ipc-websocket-disconnect.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // Sent as an ordinary frame rather than through close(), which drives the
    // whole handshake and never surfaces the frame to the application. Sent
    // this way it reaches message processing, which is the path that could
    // once fire a disconnect of its own alongside the one teardown fires.
    socket_conn
        .send(tokio_tungstenite::tungstenite::Message::Close(None))
        .await
        .unwrap();

    let mut disconnects = 0;
    let _ = tokio::time::timeout(Duration::from_secs(4), async {
        while let Some(dispatch) = handler.next_dispatch().await {
            if let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source {
                if message.route == "$disconnect" {
                    disconnects += 1;
                }
            }
        }
    })
    .await;

    assert_eq!(
        disconnects, 1,
        "the disconnect handler should run once, not once per path that can end a connection"
    );

    let _ = tokio::fs::remove_file(&socket).await;
}
