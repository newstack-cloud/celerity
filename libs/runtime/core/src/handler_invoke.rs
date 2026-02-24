use std::{collections::HashMap, sync::Arc};

use async_trait::async_trait;
use axum::{extract::State, http::StatusCode, response::IntoResponse, Json};
use celerity_helpers::runtime_types::ResponseMessage;
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex as AsyncMutex;
use tracing::{error, instrument};

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
}

#[derive(Debug)]
pub enum HandlerInvokeError {
    NotFound(String),
    BadRequest(String),
    InvocationFailed(String),
}

impl std::fmt::Display for HandlerInvokeError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            HandlerInvokeError::NotFound(msg) => write!(f, "handler not found: {msg}"),
            HandlerInvokeError::BadRequest(msg) => write!(f, "bad request: {msg}"),
            HandlerInvokeError::InvocationFailed(msg) => write!(f, "invocation failed: {msg}"),
        }
    }
}

impl IntoResponse for HandlerInvokeError {
    fn into_response(self) -> axum::response::Response {
        let (status, message) = match self {
            HandlerInvokeError::NotFound(msg) => (StatusCode::NOT_FOUND, msg),
            HandlerInvokeError::BadRequest(msg) => (StatusCode::BAD_REQUEST, msg),
            HandlerInvokeError::InvocationFailed(msg) => (StatusCode::INTERNAL_SERVER_ERROR, msg),
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
            }))
        }
    }
}

#[cfg(test)]
mod tests {
    use std::{
        net::{Ipv4Addr, SocketAddr},
        sync::atomic::{AtomicBool, Ordering},
    };

    use axum::{body::Body, http::Request, routing::post, Router};
    use http_body_util::BodyExt;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    use super::*;

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
