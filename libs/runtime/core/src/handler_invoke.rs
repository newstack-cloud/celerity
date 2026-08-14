use std::{collections::HashMap, sync::Arc};

use async_trait::async_trait;
use axum::{extract::State, http::StatusCode, response::IntoResponse, Json};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_helpers::runtime_types::ResponseMessage;
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex as AsyncMutex;
use tracing::{error, instrument};

use crate::{
    event_queue::{admission_wait, EventQueue, HandlerTimeouts},
    types::{
        CustomInvokeEventData, EventData, EventDataPayload, EventOutcome, EventResultData,
        EventType,
    },
};

/// Trait implemented by each handler type to allow invocation by name.
///
/// SDKs register a `HandlerInvoker` for every handler during registration,
/// enabling handler-to-handler invocation and external testing via the invoke API.
#[async_trait]
pub trait HandlerInvoker: Send + Sync {
    async fn invoke(
        &self,
        payload: serde_json::Value,
    ) -> Result<serde_json::Value, HandlerInvokeError>;
}

/// Registry mapping handler names to their invokers.
pub type HandlerInvokeRegistry = Arc<AsyncMutex<HashMap<String, Arc<dyn HandlerInvoker>>>>;

pub fn new_handler_invoke_registry() -> HandlerInvokeRegistry {
    Arc::new(AsyncMutex::new(HashMap::new()))
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvokeHandlerRequest {
    #[serde(rename = "handlerName")]
    pub handler_name: String,
    #[serde(rename = "invocationType")]
    pub invocation_type: InvocationType,
    pub payload: Option<serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum InvocationType {
    #[serde(rename = "requestResponse")]
    RequestResponse,
    #[serde(rename = "async")]
    Async,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InvokeHandlerResponse {
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<String>,
    /// How `data` is encoded, when it is not the handler's output as it stands.
    /// Absent means the output was text and is in `data` unchanged.
    ///
    /// Decided by the runtime from the bytes a handler returned.
    #[serde(rename = "dataEncoding")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data_encoding: Option<String>,
}

#[derive(Debug)]
pub enum HandlerInvokeError {
    NotFound(String),
    BadRequest(String),
    InvocationFailed(String),
    /// The runtime could not have the handler run at all, as opposed to running
    /// it and having it fail.
    Unavailable(String),
    /// The handler ran but did not answer within its configured timeout.
    Timeout(String),
}

impl std::fmt::Display for HandlerInvokeError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            HandlerInvokeError::NotFound(msg) => write!(f, "handler not found: {msg}"),
            HandlerInvokeError::BadRequest(msg) => write!(f, "bad request: {msg}"),
            HandlerInvokeError::InvocationFailed(msg) => write!(f, "invocation failed: {msg}"),
            HandlerInvokeError::Unavailable(msg) => write!(f, "handler unavailable: {msg}"),
            HandlerInvokeError::Timeout(msg) => write!(f, "handler timed out: {msg}"),
        }
    }
}

impl IntoResponse for HandlerInvokeError {
    fn into_response(self) -> axum::response::Response {
        let (status, message) = match self {
            HandlerInvokeError::NotFound(msg) => (StatusCode::NOT_FOUND, msg),
            HandlerInvokeError::BadRequest(msg) => (StatusCode::BAD_REQUEST, msg),
            HandlerInvokeError::InvocationFailed(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
            // Nothing ran, so this is a capacity or availability signal rather
            // than a fault in the handler.
            HandlerInvokeError::Unavailable(msg) => (StatusCode::SERVICE_UNAVAILABLE, msg),
            HandlerInvokeError::Timeout(msg) => (StatusCode::GATEWAY_TIMEOUT, msg),
        };
        (status, Json(ResponseMessage { message })).into_response()
    }
}

#[derive(Clone)]
pub struct InvokeHandlerState {
    pub registry: HandlerInvokeRegistry,
}

/// Axum handler for `POST /runtime/handlers/invoke` (public, local/test only)
/// and `POST /handlers/invoke` (internal, runtime local API, all environments).
#[instrument(
    name = "invoke_handler",
    skip(state, request),
    fields(
        handler_name = %request.handler_name,
        invocation_type = ?request.invocation_type,
    )
)]
pub async fn invoke_handler(
    State(state): State<InvokeHandlerState>,
    Json(request): Json<InvokeHandlerRequest>,
) -> Result<Json<InvokeHandlerResponse>, HandlerInvokeError> {
    let registry = state.registry.lock().await;
    let invoker = registry
        .get(&request.handler_name)
        .cloned()
        .ok_or_else(|| {
            HandlerInvokeError::NotFound(format!("handler '{}' not found", request.handler_name))
        })?;
    drop(registry);

    let payload = request.payload.unwrap_or(serde_json::Value::Null);

    match request.invocation_type {
        InvocationType::RequestResponse => {
            let result = invoker
                .invoke(payload)
                .await
                .map_err(|e| HandlerInvokeError::InvocationFailed(e.to_string()))?;
            Ok(Json(InvokeHandlerResponse {
                message: "Handler invoked successfully".to_string(),
                data: Some(result.to_string()),
                // Always text, since an in-process invoker answers with JSON.
                data_encoding: None,
            }))
        }
        InvocationType::Async => {
            let handler_name = request.handler_name.clone();
            tokio::spawn(async move {
                if let Err(e) = invoker.invoke(payload).await {
                    error!(
                        handler_name = %handler_name,
                        "async handler invocation failed: {e}",
                    );
                }
            });
            Ok(Json(InvokeHandlerResponse {
                message: "Handler invocation started".to_string(),
                data: None,
                data_encoding: None,
            }))
        }
    }
}

/// What the local invoke endpoint needs in order to reach handlers that run in
/// a separate executable.
#[derive(Clone)]
pub struct IpcInvokeState {
    pub event_queue: EventQueue,
    pub timeouts: HandlerTimeouts,
    /// The handler tag for each name a handler answers to, covering every kind
    /// of handler rather than only custom ones, since invoking by name is a
    /// shortcut past whatever normally triggers a handler.
    ///
    /// A handler answers to the name the blueprint publishes it under and to
    /// the resource it is declared as, so either can be typed.
    ///
    /// Dispatch routes by tag, so a name that is not here could never be
    /// served and is refused straight away rather than after the wait for a
    /// handler stream to claim it.
    pub handler_tags: Arc<HashMap<String, String>>,
}

/// Axum handler for `POST /runtime/handlers/invoke` in the IPC call mode.
///
/// Any handler can be reached this way, it is a shortcut past whatever
/// normally triggers a handler, so an HTTP handler can be exercised without shaping a request.
///
/// The FFI version of this calls an in-process invoker registered by the SDK.
/// There is no such thing here, so the invocation becomes an ordinary event
/// addressed to the handler's own tag and travels the same path as a request or
/// a queue message, which means it is subject to the same timeout, credit and
/// cancellation handling rather than being a second way in. The payload reaches
/// the handler untouched, so it is the caller's job to send whatever shape that
/// handler expects.
#[instrument(
    name = "invoke_handler_ipc",
    skip(state, request),
    fields(
        handler_name = %request.handler_name,
        invocation_type = ?request.invocation_type,
    )
)]
pub async fn invoke_handler_ipc(
    State(state): State<IpcInvokeState>,
    Json(request): Json<InvokeHandlerRequest>,
) -> Result<Json<InvokeHandlerResponse>, HandlerInvokeError> {
    let Some(handler_tag) = state.handler_tags.get(&request.handler_name).cloned() else {
        return Err(HandlerInvokeError::NotFound(format!(
            "no handler named '{}', which should be either the blueprint's \
             spec.handlerName or the resource the handler is declared as",
            request.handler_name
        )));
    };

    let timeout = state.timeouts.for_tag(&handler_tag);
    let event = EventData {
        id: nanoid::nanoid!(),
        event_type: EventType::CustomInvoke,
        handler_tag,
        timestamp: std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs(),
        data: EventDataPayload::CustomInvokeEventData(CustomInvokeEventData {
            handler_name: request.handler_name.clone(),
            input: request.payload,
        }),
    };

    let deadline = tokio::time::Instant::now() + timeout;
    let result_rx = state
        .event_queue
        .enqueue(event, admission_wait(timeout))
        .await
        .map_err(|err| HandlerInvokeError::Unavailable(err.to_string()))?;

    if request.invocation_type == InvocationType::Async {
        // The caller is not waiting, so the result is left to be discarded when
        // it arrives. Dropping the receiver is deliberately not treated as the
        // caller going away, the handler should still run to completion.
        return Ok(Json(InvokeHandlerResponse {
            message: "Handler invocation started".to_string(),
            data: None,
            data_encoding: None,
        }));
    }

    match tokio::time::timeout_at(deadline, result_rx).await {
        Ok(Ok(EventOutcome::Completed(_event, result))) => invoke_response(result.data),
        Ok(Ok(EventOutcome::Unservable(reason))) => {
            Err(HandlerInvokeError::Unavailable(reason.to_string()))
        }
        Ok(Err(_)) => Err(HandlerInvokeError::InvocationFailed(
            "the handler did not return a result".to_string(),
        )),
        Err(_) => Err(HandlerInvokeError::Timeout(format!(
            "the handler did not respond within {timeout:?}"
        ))),
    }
}

fn invoke_response(
    data: EventResultData,
) -> Result<Json<InvokeHandlerResponse>, HandlerInvokeError> {
    let EventResultData::CustomInvokeResponse(response) = data else {
        return Err(HandlerInvokeError::InvocationFailed(
            "the handler returned a result that is not a custom invocation response".to_string(),
        ));
    };

    if let Some(error_message) = response.error_message {
        return Err(HandlerInvokeError::InvocationFailed(error_message));
    }

    // The response is JSON, and a JSON string cannot carry arbitrary bytes, so
    // output that is not text is encoded and said to be encoded. Reporting it
    // as text would corrupt it.
    //
    // Which of the two applies is read from the bytes rather than declared, so
    // a handler does nothing differently for this endpoint.
    let (data, data_encoding) = match String::from_utf8(response.output.to_vec()) {
        Ok(text) => (text, None),
        Err(err) => (BASE64.encode(err.as_bytes()), Some("base64".to_string())),
    };

    Ok(Json(InvokeHandlerResponse {
        message: "Handler invoked successfully".to_string(),
        data: Some(data),
        data_encoding,
    }))
}

#[cfg(test)]
mod tests {
    #[test]
    fn carries_a_handler_s_output_through_untouched() {
        let response = invoke_response(EventResultData::CustomInvokeResponse(
            CustomInvokeResponseData {
                output: Bytes::from_static(br#"{"reindexed":12}"#),
                error_message: None,
            },
        ))
        .expect("the output should be returned");

        assert_eq!(response.data.as_deref(), Some(r#"{"reindexed":12}"#));
        // Text needs no encoding, so nothing is said about one.
        assert!(response.data_encoding.is_none());
    }

    #[test]
    fn encodes_output_this_endpoint_cannot_carry_as_it_stands() {
        let raw = [0xff, 0xfe, 0x00, 0x80, 0x01];
        let response = invoke_response(EventResultData::CustomInvokeResponse(
            CustomInvokeResponseData {
                output: Bytes::from_static(&[0xff, 0xfe, 0x00, 0x80, 0x01]),
                error_message: None,
            },
        ))
        .expect("binary output should still be returned");

        // Said to be encoded rather than reported as text, which would corrupt
        // it, and rather than refused, which would leave a handler returning an
        // image or a protobuf with no way to be exercised here.
        assert_eq!(response.data_encoding.as_deref(), Some("base64"));
        let decoded = BASE64
            .decode(response.data.as_deref().expect("output should be carried"))
            .expect("the output should be base64");
        assert_eq!(decoded, raw);
    }

    use std::{
        net::{Ipv4Addr, SocketAddr},
        sync::atomic::{AtomicBool, Ordering},
    };

    use axum::{body::Body, http::Request, routing::post, Router};
    use bytes::Bytes;
    use http_body_util::BodyExt;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    use super::*;
    use crate::types::CustomInvokeResponseData;

    struct MockInvoker {
        response: serde_json::Value,
        fail: bool,
    }

    #[async_trait]
    impl HandlerInvoker for MockInvoker {
        async fn invoke(
            &self,
            _payload: serde_json::Value,
        ) -> Result<serde_json::Value, HandlerInvokeError> {
            if self.fail {
                return Err(HandlerInvokeError::InvocationFailed(
                    "handler error".to_string(),
                ));
            }
            Ok(self.response.clone())
        }
    }

    fn create_test_router(registry: HandlerInvokeRegistry) -> Router {
        Router::new().route(
            "/runtime/handlers/invoke",
            post(invoke_handler).with_state(InvokeHandlerState { registry }),
        )
    }

    async fn start_test_server(router: Router) -> SocketAddr {
        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, router).await.unwrap();
        });
        addr
    }

    fn create_http_client(
    ) -> hyper_util::client::legacy::Client<hyper_util::client::legacy::connect::HttpConnector, Body>
    {
        hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
            .build_http()
    }

    #[test_log::test(tokio::test)]
    async fn test_invoke_handler_request_response() {
        let registry = new_handler_invoke_registry();
        registry.lock().await.insert(
            "TestHandler".to_string(),
            Arc::new(MockInvoker {
                response: json!({"result": "ok"}),
                fail: false,
            }),
        );
        let router = create_test_router(registry);
        let addr = start_test_server(router).await;
        let client = create_http_client();

        let body = json!({
            "handlerName": "TestHandler",
            "invocationType": "requestResponse",
            "payload": {"input": "data"}
        });
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/runtime/handlers/invoke"))
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&body).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), 200);
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let resp: InvokeHandlerResponse = serde_json::from_slice(&body).unwrap();
        assert_eq!(resp.message, "Handler invoked successfully");
        assert!(resp.data.is_some());
    }

    #[test_log::test(tokio::test)]
    async fn test_invoke_handler_not_found() {
        let registry = new_handler_invoke_registry();
        let router = create_test_router(registry);
        let addr = start_test_server(router).await;
        let client = create_http_client();

        let body = json!({
            "handlerName": "NonExistent",
            "invocationType": "requestResponse"
        });
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/runtime/handlers/invoke"))
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&body).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), 404);
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let resp: ResponseMessage = serde_json::from_slice(&body).unwrap();
        assert!(resp.message.contains("not found"));
    }

    #[test_log::test(tokio::test)]
    async fn test_invoke_handler_async_returns_immediately() {
        let invoked = Arc::new(AtomicBool::new(false));
        let invoked_clone = invoked.clone();

        struct SlowInvoker {
            invoked: Arc<AtomicBool>,
        }

        #[async_trait]
        impl HandlerInvoker for SlowInvoker {
            async fn invoke(
                &self,
                _payload: serde_json::Value,
            ) -> Result<serde_json::Value, HandlerInvokeError> {
                tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;
                self.invoked.store(true, Ordering::SeqCst);
                Ok(json!({"done": true}))
            }
        }

        let registry = new_handler_invoke_registry();
        registry.lock().await.insert(
            "SlowHandler".to_string(),
            Arc::new(SlowInvoker {
                invoked: invoked_clone,
            }),
        );
        let router = create_test_router(registry);
        let addr = start_test_server(router).await;
        let client = create_http_client();

        let body = json!({
            "handlerName": "SlowHandler",
            "invocationType": "async"
        });
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/runtime/handlers/invoke"))
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&body).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();

        // Should return immediately
        assert_eq!(response.status(), 200);
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let resp: InvokeHandlerResponse = serde_json::from_slice(&body).unwrap();
        assert_eq!(resp.message, "Handler invocation started");
        assert!(resp.data.is_none());

        // Wait for the async handler to complete
        tokio::time::sleep(tokio::time::Duration::from_millis(200)).await;
        assert!(invoked.load(Ordering::SeqCst));
    }

    #[test_log::test(tokio::test)]
    async fn test_invoke_handler_invocation_failed() {
        let registry = new_handler_invoke_registry();
        registry.lock().await.insert(
            "FailHandler".to_string(),
            Arc::new(MockInvoker {
                response: json!(null),
                fail: true,
            }),
        );
        let router = create_test_router(registry);
        let addr = start_test_server(router).await;
        let client = create_http_client();

        let body = json!({
            "handlerName": "FailHandler",
            "invocationType": "requestResponse"
        });
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/runtime/handlers/invoke"))
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&body).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), 500);
    }
}
