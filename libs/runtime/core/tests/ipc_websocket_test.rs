//! The IPC path for a WebSocket API: messages reach a handler over a real gRPC
//! stream on a real Unix socket, acknowledgements and duplicates are dealt with
//! on the way, and connections come and go.
//!
//! Every piece has unit coverage of its own. What these cover is the assembly,
//! and they do it the way a handlers executable and a real client would rather
//! than by reaching into the runtime's internals.

mod common;

use std::time::Duration;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_runtime_core::{
    application::Application,
    config::RuntimeConfig,
    consts::CELERITY_WS_CAPABILITIES_SIGNAL,
    ipc_proto::{self as proto},
};
use common::ipc::{ipc_env, socket_path, start_runtime, websocket_ack, HandlerStub};
use futures::SinkExt;
use serde_json::json;
use tokio_stream::StreamExt;

/// Setting an application up must not need a runtime to be running.
///
/// Deliberately not a tokio test, which is the whole point. An SDK calls setup
/// from wherever the host language happens to be, with no runtime on that
/// thread, so anything here that spawns takes the process down rather than
/// returning an error. That is what a websocket API did until the worker that
/// waits on clients moved to `run`, and a tokio test cannot show it because a
/// tokio test always has the runtime that is missing.
#[test]
fn sets_up_a_websocket_api_without_a_runtime_to_spawn_into() {
    let socket = socket_path("ipc-ws-setup-sync");
    let env_vars = ipc_env(
        "ipc-ws-setup-sync",
        "tests/data/fixtures/ipc-websocket-default-route.blueprint.yaml",
        &socket,
        &[],
    );
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));

    app.setup()
        .expect("setting up a websocket api should not need a runtime");
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

/// A handler sending binary is held to the format its client can read, and is
/// told which of its messages was refused.
///
/// Failures are per message rather than per batch, so a batch carrying one bad
/// message still delivers the rest and names the one that did not go.
#[test_log::test(tokio::test)]
async fn refuses_a_binary_websocket_message_that_is_not_framed() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-binary-framing",
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
    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    let connection_id = message.connection_id;

    // [routeLength][route][requireAck][messageIdLength][messageId][payload]
    let mut framed = vec![7u8];
    framed.extend_from_slice(b"updates");
    framed.push(0x0);
    framed.push(0x0);
    framed.extend_from_slice(&[0xde, 0xad, 0xbe, 0xef]);

    handler
        .send_ws(
            "batch-1",
            vec![
                // Raw bytes, which is what an application reaches for when it
                // does not know the format is required. A protobuf payload
                // here, field one as a length delimited string.
                proto::WsOutbound {
                    connection_id: connection_id.clone(),
                    message: vec![0x0a, 0x05, b'p', b'r', b'i', b'c', b'e'],
                    is_binary: true,
                    ..Default::default()
                },
                proto::WsOutbound {
                    connection_id: connection_id.clone(),
                    message: framed.clone(),
                    is_binary: true,
                    ..Default::default()
                },
            ],
        )
        .await;

    let ack = handler
        .next_ws_ack()
        .await
        .expect("the runtime should report what became of the batch");
    assert_eq!(ack.correlation_id, "batch-1");
    assert!(
        !ack.success,
        "a batch with a refused message is not a success"
    );
    assert_eq!(
        ack.failures.len(),
        1,
        "only the unframed message should fail"
    );
    assert_eq!(
        ack.failures[0].index, 0,
        "the failure should name the message that was refused"
    );
    assert_eq!(ack.failures[0].connection_id, connection_id);

    // The framed one still arrives, exactly as the handler composed it, and it
    // is the only binary frame after the capabilities signal.
    let delivered = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Binary(bytes) = message {
                if bytes[..] == CELERITY_WS_CAPABILITIES_SIGNAL[..] {
                    continue;
                }
                return Some(bytes.to_vec());
            }
        }
        None
    })
    .await
    .expect("the framed message should reach the client");

    assert_eq!(delivered, Some(framed));

    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}

/// A message the client sends twice is acted on once, and answered both times.
///
/// The two halves have to go together. A client resends because it did not receive
/// an acknowledgement, and what went missing may have been the acknowledgement
/// rather than the message, so a duplicate that is met with silence is one the
/// client keeps sending. Acknowledging without deduplicating runs the handler
/// twice; deduplicating without acknowledging never ends.
#[test_log::test(tokio::test)]
async fn acts_once_on_a_message_the_client_sent_twice_but_answers_both() {
    let (_app, addr, socket) = start_runtime(
        "ipc-ws-dedupe",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
    )
    .await;
    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    let message = json!({
        "event": "sendMessage",
        "messageId": "m-1",
        "ack": true,
        "data": { "text": "hello" }
    })
    .to_string();

    for _ in 0..2 {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                message.clone(),
            ))
            .await
            .unwrap();
    }

    let dispatch = handler
        .next_dispatch()
        .await
        .expect("the first copy should reach the handler");
    let Some(proto::dispatch::Source::Websocket(delivered)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    assert_eq!(delivered.message_id, "m-1");

    // Both copies are answered, so the client stops resending.
    let mut acknowledgements = 0;
    let _ = tokio::time::timeout(Duration::from_millis(600), async {
        while let Some(Ok(frame)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = frame {
                let value: serde_json::Value = serde_json::from_str(&text).unwrap();
                if value["event"] == "ack" && value["data"]["messageId"] == "m-1" {
                    acknowledgements += 1;
                    if acknowledgements == 2 {
                        return;
                    }
                }
            }
        }
    })
    .await;
    assert_eq!(
        acknowledgements, 2,
        "both copies should be acknowledged, or the client keeps resending"
    );

    // Only the first copy was acted on. Asked for after the acknowledgements,
    // so the second copy has had every chance to arrive at the handler.
    let second = tokio::time::timeout(Duration::from_millis(400), handler.next_dispatch()).await;
    assert!(
        second.is_err() || second.unwrap().is_none(),
        "the same message should not reach the handler twice"
    );

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

    // The first carries an id but does not opt in, and the second opts in with
    // no id for an acknowledgement to name. Neither should be answered.
    //
    // The third asks properly and is sent last, so its answer is what says the
    // runtime was answering at all. Without it this test passes just as well
    // against a runtime that acknowledges nothing, which is what it used to do.
    for message in [
        json!({ "event": "sendMessage", "messageId": "msg-no-opt-in", "data": {} }),
        json!({ "event": "sendMessage", "ack": true, "data": {} }),
        json!({ "event": "sendMessage", "messageId": "msg-asks", "ack": true, "data": {} }),
    ] {
        socket_conn
            .send(tokio_tungstenite::tungstenite::Message::Text(
                message.to_string(),
            ))
            .await
            .unwrap();
    }

    // Messages are handled in the order they arrive, so by the time the third
    // is answered any answer to the first two has had its chance.
    let acknowledged = tokio::time::timeout(Duration::from_secs(5), async {
        let mut acknowledged = Vec::new();
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Text(text) = message {
                if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                    if value["event"] == "ack" {
                        let message_id = value["data"]["messageId"]
                            .as_str()
                            .unwrap_or("")
                            .to_string();
                        let answered_the_one_that_asked = message_id == "msg-asks";
                        acknowledged.push(message_id);
                        // Everything sent before it has been handled by now, so
                        // any answer they were going to get has arrived.
                        if answered_the_one_that_asked {
                            break;
                        }
                    }
                }
            }
        }
        acknowledged
    })
    .await
    .expect("the message that asked to be acknowledged should have been");

    assert_eq!(
        acknowledged,
        vec!["msg-asks".to_string()],
        "only the message that asked should be answered, and it should be"
    );

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

/// A message the client never acknowledges is declared lost, on the timings the
/// deployment configured rather than the ones compiled in.
///
/// The acknowledgement timeout and the attempt limit are what the protocol asks
/// to be configurable, and until now they were fixed at whatever the worker
/// defaulted to. This drives the whole path with both set small, so what it
/// proves is not only that a message can be declared lost but that the
/// configured values are the ones being used. On the defaults the loss would
/// come after about half a minute, three resends ten seconds apart, so the five
/// second bound below is what says the configured values reached the worker.
#[test_log::test(tokio::test)]
async fn declares_a_message_lost_on_the_timings_it_was_configured_with() {
    let socket = socket_path("ipc-ws-ack-timings");
    let env_vars = ipc_env(
        "ipc-ws-ack-timings",
        "tests/data/fixtures/ipc-websocket-api.blueprint.yaml",
        &socket,
        &[
            ("CELERITY_WS_ACK_TIMEOUT_MS", "200"),
            ("CELERITY_WS_ACK_MAX_ATTEMPTS", "1"),
        ],
    );
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();
    let addr = app.run(false).await.unwrap().http_server_address.unwrap();

    let mut handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");
    // Sent only so the handler learns the connection id, which is what it
    // addresses the message below to.
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
    let Some(proto::dispatch::Source::Websocket(message)) = dispatch.source else {
        panic!("expected a WebSocket source");
    };
    let connection_id = message.connection_id;

    // [routeLength][route][requireAck][messageIdLength][messageId][payload],
    // this one asking to be acknowledged under the id the loss event will name.
    let mut framed = vec![7u8];
    framed.extend_from_slice(b"updates");
    framed.push(0x1);
    framed.push(5u8);
    framed.extend_from_slice(b"m-ack");
    framed.extend_from_slice(&[0xde, 0xad]);

    handler
        .send_ws(
            "batch-1",
            vec![proto::WsOutbound {
                connection_id: connection_id.clone(),
                message: framed,
                is_binary: true,
                message_id: "m-ack".to_string(),
                // Without this nobody is told, since a loss event goes to the
                // clients the sender named rather than to the one that missed
                // the message.
                inform_clients_on_loss: vec![connection_id.clone()],
                ..Default::default()
            }],
        )
        .await;

    // The client reads the message and deliberately does not acknowledge it,
    // which is the case a resend and then a loss event exist for.
    let lost = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            if let tokio_tungstenite::tungstenite::Message::Binary(bytes) = message {
                // A reserved frame, route 0x3, with no ack asked for and no id.
                if bytes.len() > 4 && bytes[..4] == [0x1, 0x3, 0x0, 0x0] {
                    return Some(bytes[4..].to_vec());
                }
            }
        }
        None
    })
    .await
    .expect(
        "a message nobody acknowledged should be declared lost, and within the configured \
         timings rather than the compiled in ones",
    );

    let lost = lost.expect("the connection closed before the message was declared lost");
    let lost: serde_json::Value = serde_json::from_slice(&lost).unwrap();
    assert_eq!(
        lost["messageId"], "m-ack",
        "the loss event should name the message that went unacknowledged"
    );
    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}

/// A guard that lets any non-empty token through, since what is under test is
/// what the runtime says once a guard has passed rather than how it decides.
#[derive(Debug)]
struct AcceptAnyTokenGuard;

#[async_trait::async_trait]
impl celerity_runtime_core::auth_custom::AuthGuardHandler for AcceptAnyTokenGuard {
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
        Ok(json!({ "id": "user-1" }))
    }
}

/// A client authenticated during the upgrade is told so afterwards.
///
/// The connect strategy authenticates before the connection is upgraded, so
/// nothing the client receives says whether it worked. The protocol has the
/// server say so once the connection is up, and a client waits for it: the
/// official one moves into an authenticating state after the capabilities
/// signal and leaves it only when this arrives. Without it, connecting never
/// finishes and nothing the application queued is ever sent.
///
/// The order matters as much as the message. A client reads this as an answer
/// about authentication only once it knows what the transport can carry, so
/// sending it before the capabilities signal would have it taken for an
/// ordinary message on a route named `authenticated`.
#[test_log::test(tokio::test)]
async fn tells_a_client_authenticated_during_the_upgrade_that_it_was() {
    let socket = socket_path("ipc-ws-auth-connect");
    let env_vars = ipc_env(
        "ipc-ws-auth-connect",
        "tests/data/fixtures/ipc-websocket-auth-connect.blueprint.yaml",
        &socket,
        &[],
    );
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();
    // Registered after setup, which is the only order an SDK can use, since
    // setup is what returns the configuration telling it which guards the
    // blueprint asks for. A guard the connection path cannot see is a
    // connection refused, so this ordering is part of what the test asserts.
    app.register_custom_auth_guard("customGuard", AcceptAnyTokenGuard)
        .await;
    let addr = app.run(false).await.unwrap().http_server_address.unwrap();

    let _handler = HandlerStub::attach(&socket, |_| Some(websocket_ack())).await;

    let mut request =
        tokio_tungstenite::tungstenite::client::IntoClientRequest::into_client_request(format!(
            "ws://{addr}/"
        ))
        .unwrap();
    request
        .headers_mut()
        .insert("Authorization", "Bearer a-token".parse().unwrap());

    let (mut socket_conn, _) = tokio_tungstenite::connect_async(request)
        .await
        .expect("a connection carrying a token the guard accepts should be upgraded");

    let mut saw_capabilities = false;
    let authenticated = tokio::time::timeout(Duration::from_secs(5), async {
        while let Some(Ok(message)) = socket_conn.next().await {
            match message {
                tokio_tungstenite::tungstenite::Message::Binary(bytes)
                    if bytes[..] == CELERITY_WS_CAPABILITIES_SIGNAL[..] =>
                {
                    saw_capabilities = true;
                }
                tokio_tungstenite::tungstenite::Message::Text(text) => {
                    if let Ok(value) = serde_json::from_str::<serde_json::Value>(&text) {
                        if value["event"] == "authenticated" {
                            return Some(value);
                        }
                    }
                }
                _ => {}
            }
        }
        None
    })
    .await
    .expect("a client authenticated during the upgrade should be told so");

    let authenticated = authenticated.expect("the connection closed before saying anything");
    assert!(
        saw_capabilities,
        "the capabilities signal should come first, or a client takes this for an ordinary message"
    );
    assert_eq!(authenticated["data"]["success"], true);
    // Claims are collected under the name of the guard that produced them, so
    // an API with more than one can tell them apart.
    assert_eq!(
        authenticated["data"]["userInfo"]["customGuard"]["id"], "user-1",
        "what the guard returned should reach the client"
    );

    let _ = socket_conn.close(None).await;
    let _ = tokio::fs::remove_file(&socket).await;
}
