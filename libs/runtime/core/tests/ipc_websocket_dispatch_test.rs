//! End-to-end coverage for WebSocket message routing in the IPC runtime call
//! mode.
//!
//! As with HTTP, nothing populated the WebSocket route map outside the FFI call
//! mode, so messages had nowhere to route to and this path could not work.

use std::time::Duration;

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_runtime_core::{
    application::Application,
    config::RuntimeConfig,
    types::{EventDataPayload, EventResult, EventResultData, EventType, SimpleResponseData},
};
use futures::SinkExt;
use serde_json::json;
use tokio_tungstenite::tungstenite::Message;

mod common;

fn ipc_ws_env(service_name: &str) -> common::MockEnvVars<'_> {
    common::MockEnvVars::new(Some(
        vec![
            (
                "CELERITY_BLUEPRINT",
                "tests/data/fixtures/ipc-websocket-api.blueprint.yaml".to_string(),
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

#[test_log::test(tokio::test)]
async fn routes_a_websocket_message_to_the_event_queue() {
    let env_vars = ipc_ws_env("ipc-ws-dispatch-test");
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();

    let event_queue = app
        .event_queue()
        .expect("the IPC call mode should create an event queue");
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    // Stand in for the handlers executable: take the event and acknowledge it.
    let handler = tokio::spawn(async move {
        let (result_tx, event) = event_queue
            .receiver
            .lock()
            .await
            .recv()
            .await
            .expect("a message should be dispatched to the queue");

        result_tx
            .send((
                event.clone(),
                EventResult {
                    event_id: event.id.clone(),
                    data: EventResultData::WebSocketResponse(SimpleResponseData {
                        success: true,
                        error_message: None,
                    }),
                    context: None,
                },
            ))
            .expect("the runtime should still be awaiting the result");
        event
    });

    let (mut socket, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    socket
        .send(Message::Text(
            json!({ "event": "sendMessage", "data": { "text": "hello" } }).to_string(),
        ))
        .await
        .unwrap();

    let event = tokio::time::timeout(Duration::from_secs(10), handler)
        .await
        .expect("the message should reach the event queue")
        .unwrap();

    assert_eq!(event.event_type, EventType::WsMessage);
    assert_eq!(event.handler_tag, "event::sendMessage");

    let EventDataPayload::WsMessageEventData(message) = event.data else {
        panic!("expected a WebSocket message event");
    };
    assert_eq!(message.route, "sendMessage");
    assert!(!message.connection_id.is_empty());
    // The body reaches the handler as the JSON the client sent.
    let body: serde_json::Value = serde_json::from_str(&message.message).unwrap();
    assert_eq!(body["data"]["text"], "hello");

    assert!(!message.is_binary);

    let _ = socket.close(None).await;
}

#[test_log::test(tokio::test)]
async fn carries_a_binary_websocket_frame_without_corrupting_it() {
    let env_vars = ipc_ws_env("ipc-ws-binary-test");
    let runtime_config = RuntimeConfig::from_env(&env_vars);
    let mut app = Application::new(runtime_config, Box::new(env_vars));
    app.setup().unwrap();

    let event_queue = app
        .event_queue()
        .expect("the IPC call mode should create an event queue");
    let app_info = app.run(false).await.unwrap();
    let addr = app_info.http_server_address.unwrap();

    let handler = tokio::spawn(async move {
        let (result_tx, event) = event_queue
            .receiver
            .lock()
            .await
            .recv()
            .await
            .expect("a message should be dispatched to the queue");
        result_tx
            .send((
                event.clone(),
                EventResult {
                    event_id: event.id.clone(),
                    data: EventResultData::WebSocketResponse(SimpleResponseData {
                        success: true,
                        error_message: None,
                    }),
                    context: None,
                },
            ))
            .expect("the runtime should still be awaiting the result");
        event
    });

    let (mut socket, _) = tokio_tungstenite::connect_async(format!("ws://{addr}/"))
        .await
        .expect("the WebSocket server should accept the connection");

    // The runtime's binary framing is [route_len][route][msg_id_len][msg_id][body].
    let route = b"sendMessage";
    // Bytes that are not valid UTF-8, which a lossy conversion would replace
    // with the replacement character and corrupt beyond recovery.
    let body: Vec<u8> = vec![0xff, 0xfe, 0x00, 0x80, 0x01, 0x02];

    let mut payload = vec![route.len() as u8];
    payload.extend_from_slice(route);
    payload.push(0); // no message id
    payload.extend_from_slice(&body);

    socket.send(Message::Binary(payload)).await.unwrap();

    let event = tokio::time::timeout(Duration::from_secs(10), handler)
        .await
        .expect("the message should reach the event queue")
        .unwrap();

    let EventDataPayload::WsMessageEventData(message) = event.data else {
        panic!("expected a WebSocket message event");
    };
    assert!(message.is_binary);

    // The handler can recover the frame's bytes exactly, replacement
    // characters would mean the payload had been corrupted in transit.
    let decoded = BASE64
        .decode(&message.message)
        .expect("the body should be base64 encoded");
    assert_eq!(decoded, body);

    let _ = socket.close(None).await;
}
