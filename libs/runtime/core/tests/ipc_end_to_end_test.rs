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
        self as proto, handler_message, handler_runtime_client::HandlerRuntimeClient,
        runtime_message,
    },
};
use futures::SinkExt;
use http_body_util::BodyExt;
use serde_json::json;
use tokio::{net::UnixStream, sync::mpsc};
use tokio_stream::StreamExt;
use tonic::transport::{Endpoint, Uri};

mod common;

fn ipc_env(service_name: &str, fixture: &str, socket: &str) -> common::MockEnvVars<'static> {
    common::MockEnvVars::new(Some(
        vec![
            ("CELERITY_BLUEPRINT", fixture.to_string()),
            ("CELERITY_SERVICE_NAME", service_name.to_string()),
            ("CELERITY_RUNTIME_PLATFORM", "local".to_string()),
            ("CELERITY_RUNTIME_CALL_MODE", "ipc".to_string()),
            ("CELERITY_SERVER_PORT", "0".to_string()),
            ("CELERITY_LOCAL_API_PORT", "0".to_string()),
            ("CELERITY_RUNTIME_SOCKET", socket.to_string()),
            ("CELERITY_SERVER_LOOPBACK_ONLY", "true".to_string()),
            ("CELERITY_TEST_MODE", "true".to_string()),
            ("CELERITY_VARIABLE_logLevel", "DEBUG".to_string()),
            ("CELERITY_CLIENT_IP_SOURCE", "ConnectInfo".to_string()),
        ]
        .into_iter()
        .collect(),
    ))
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
    let socket = socket_path(name);
    let env_vars = ipc_env(name, fixture, &socket);
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

        let mut client = HandlerRuntimeClient::new(channel);
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
        tokio::spawn(async move {
            while let Some(Ok(message)) = frames.next().await {
                let Some(runtime_message::Frame::Dispatch(dispatch)) = message.frame else {
                    continue;
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

        HandlerStub { dispatches }
    }

    async fn next_dispatch(&mut self) -> Option<proto::Dispatch> {
        tokio::time::timeout(Duration::from_secs(10), self.dispatches.recv())
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

    // The binary framing is [route_len][route][msg_id_len][msg_id][body].
    let route = b"sendMessage";
    let body: Vec<u8> = vec![0xff, 0xfe, 0x00, 0x80, 0x01, 0x02];
    let mut payload = vec![route.len() as u8];
    payload.extend_from_slice(route);
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
