//! The shared harness for the IPC end to end tests.
//!
//! Each of those files compiles as its own crate, so this is included by each
//! rather than linked once, which is why unused items are allowed here. A file
//! covering one application type will not touch every helper.
#![allow(dead_code)]

use std::{collections::HashMap, time::Duration};

use axum::body::Body;
use celerity_runtime_core::{
    application::Application,
    config::RuntimeConfig,
    ipc_proto::{
        self as proto, handler_message,
        handler_runtime_service_client::HandlerRuntimeServiceClient, runtime_message,
    },
    ipc_stream::runtime_protocol_version,
};
use tokio::{net::UnixStream, sync::mpsc};

use tokio_stream::StreamExt;
use tonic::transport::{Endpoint, Uri};

/// The credit the stub handler declares, which is how many events the runtime
/// may have in flight to it at once. Named because a test that fills the window
/// has to know what filling it means.
pub const HANDLER_INITIAL_CREDIT: u32 = 8;

pub fn ipc_env(
    service_name: &str,
    fixture: &str,
    socket: &str,
    overrides: &[(&'static str, &str)],
) -> super::MockEnvVars<'static> {
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
    super::MockEnvVars::new(Some(vars))
}

/// A socket path unique to this test, so tests can run alongside each other.
pub fn socket_path(name: &str) -> String {
    std::env::temp_dir()
        .join(format!("celerity-ipc-{}-{name}.sock", std::process::id()))
        .to_string_lossy()
        .into_owned()
}

/// Starts a runtime serving the given blueprint, returning its public address
/// and the socket its handler stream is on.
pub async fn start_runtime(
    name: &str,
    fixture: &str,
) -> (Application, std::net::SocketAddr, String) {
    start_runtime_with(name, fixture, &[]).await
}

/// Starts a runtime with environment overrides applied on top of the defaults.
pub async fn start_runtime_with(
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
pub struct HandlerStub {
    /// The events the handler was asked to process.
    dispatches: mpsc::Receiver<proto::Dispatch>,
    /// The events the handler was told to stop working on.
    pub cancels: mpsc::Receiver<proto::Cancel>,
    /// Frames the handler sends of its own accord, rather than in answer to a
    /// dispatch.
    outbound: mpsc::Sender<proto::HandlerMessage>,
    /// What the runtime made of each batch of websocket sends.
    ws_acks: mpsc::Receiver<proto::WsSendAck>,
}

impl HandlerStub {
    /// Connects, completes the handshake by declaring every tag the runtime
    /// asked for, then answers dispatches with whatever `respond` returns.
    ///
    /// Returning `None` withholds a result, which is how a handler that has
    /// stopped answering is simulated.
    pub async fn attach(
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
                    protocol_version: Some(runtime_protocol_version()),
                    handler_tags: config
                        .handlers
                        .iter()
                        .map(|handler| handler.handler_tag.clone())
                        .collect(),
                    initial_credit: HANDLER_INITIAL_CREDIT,
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
        let (ws_ack_tx, ws_acks) = mpsc::channel(16);
        let outbound = handler_tx.clone();
        tokio::spawn(async move {
            while let Some(Ok(message)) = frames.next().await {
                let dispatch = match message.frame {
                    Some(runtime_message::Frame::Dispatch(dispatch)) => dispatch,
                    Some(runtime_message::Frame::Cancel(cancel)) => {
                        cancel_tx.send(cancel).await.ok();
                        continue;
                    }
                    Some(runtime_message::Frame::WsAck(ack)) => {
                        ws_ack_tx.send(ack).await.ok();
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
            outbound,
            ws_acks,
        }
    }

    /// Sends a batch of websocket messages the way a handler does, of its own
    /// accord rather than as the result of an event.
    pub async fn send_ws(&self, correlation_id: &str, messages: Vec<proto::WsOutbound>) {
        self.outbound
            .send(proto::HandlerMessage {
                frame: Some(handler_message::Frame::WsSend(proto::WsSend {
                    correlation_id: correlation_id.to_string(),
                    messages,
                })),
            })
            .await
            .expect("the runtime should still be taking frames");
    }

    pub async fn next_ws_ack(&mut self) -> Option<proto::WsSendAck> {
        tokio::time::timeout(Duration::from_secs(10), self.ws_acks.recv())
            .await
            .ok()
            .flatten()
    }

    pub async fn next_dispatch(&mut self) -> Option<proto::Dispatch> {
        tokio::time::timeout(Duration::from_secs(10), self.dispatches.recv())
            .await
            .ok()
            .flatten()
    }

    pub async fn next_cancel(&mut self) -> Option<proto::Cancel> {
        tokio::time::timeout(Duration::from_secs(10), self.cancels.recv())
            .await
            .ok()
            .flatten()
    }
}

pub fn json_response(body: &'static str) -> proto::result::Outcome {
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

pub fn websocket_ack() -> proto::result::Outcome {
    proto::result::Outcome::Websocket(proto::Ack {
        success: true,
        error_message: String::new(),
    })
}

pub fn http_client(
) -> hyper_util::client::legacy::Client<hyper_util::client::legacy::connect::HttpConnector, Body> {
    hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new()).build_http()
}
