use std::{
    collections::{HashMap, VecDeque},
    sync::{Arc, Mutex},
};

use axum::{
    extract::State,
    routing::{get, post},
    Json, Router,
};
use celerity_helpers::runtime_types::ResponseMessage;
use serde::{Deserialize, Serialize};
use tracing::debug;

use crate::{
    config::WorkflowAppConfig,
    errors::{EventResultError, WorkflowApplicationStartError},
    types::{EventData, EventResult, EventTuple},
};

// Creates a router for the local runtime API
// that allows for interaction between the workflow runtime
// and the handlers executable over HTTP.
pub fn create_workflow_runtime_local_api(
    app_config: &WorkflowAppConfig,
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
) -> Result<Router, WorkflowApplicationStartError> {
    let local_runtime_api_config = create_local_runtime_api_config(app_config);
    let shared_state = Arc::new(WorkflowLocalRuntimeAppState {
        event_queue: event_queue.clone(),
        processing_events_map: processing_events_map.clone(),
        runtime_api_config: local_runtime_api_config,
    });
    Ok(Router::new()
        .route("/events/next", post(next_event_handler))
        .route("/events/result", post(event_result_handler))
        .route("/runtime/config", get(runtime_config_handler))
        .with_state(shared_state))
}

async fn next_event_handler(
    State(state): State<Arc<WorkflowLocalRuntimeAppState>>,
) -> Json<Option<EventData>> {
    debug!("retrieving next event in the runtime queue");
    let mut event_queue = state.event_queue.lock().unwrap();
    let mut processing_events_map = state.processing_events_map.lock().unwrap();

    if let Some((tx, event)) = event_queue.pop_front() {
        let event_for_response = event.clone();
        processing_events_map.insert(event.id.clone(), (tx, event));
        return Json(Some(event_for_response));
    }
    debug!("no events in the queue, returning null");
    Json(None)
}

async fn event_result_handler(
    State(state): State<Arc<WorkflowLocalRuntimeAppState>>,
    Json(event_result): Json<EventResult>,
) -> Result<Json<ResponseMessage>, EventResultError> {
    let mut processing_events_map = state.processing_events_map.lock().unwrap();
    if let Some((tx, event)) = processing_events_map.remove(&event_result.event_id) {
        tx.send((event, event_result))
            .map_err(|_| EventResultError::UnexpectedError)?;
        return Ok(Json(ResponseMessage {
            message: "The result has been successfully processed".to_string(),
        }));
    }
    Err(EventResultError::EventNotFound)
}

async fn runtime_config_handler(
    State(state): State<Arc<WorkflowLocalRuntimeAppState>>,
) -> Json<LocalWorkflowRuntimeConfig> {
    Json(state.runtime_api_config.clone())
}

fn create_local_runtime_api_config(app_config: &WorkflowAppConfig) -> LocalWorkflowRuntimeConfig {
    let state_handlers = create_local_runtime_state_handlers(app_config);

    LocalWorkflowRuntimeConfig {
        app_config: LocalWorkflowRuntimeAppConfig {
            tracing_enabled: true,
            state_handlers,
        },
    }
}

fn create_local_runtime_state_handlers(
    app_config: &WorkflowAppConfig,
) -> Vec<LocalWorkflowRuntimeStateHandlerConfig> {
    if let Some(state_handlers) = &app_config.state_handlers {
        return state_handlers
            .iter()
            .map(|handler| LocalWorkflowRuntimeStateHandlerConfig {
                handler_name: handler.name.clone(),
                handler_tag: format!("state::{}", handler.state),
                state: handler.state.clone(),
                timeout: handler.timeout,
                tracing_enabled: handler.tracing_enabled,
            })
            .collect();
    }

    vec![]
}

#[derive(Debug)]
struct WorkflowLocalRuntimeAppState {
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
    runtime_api_config: LocalWorkflowRuntimeConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalWorkflowRuntimeConfig {
    #[serde(rename = "appConfig")]
    app_config: LocalWorkflowRuntimeAppConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalWorkflowRuntimeAppConfig {
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
    state_handlers: Vec<LocalWorkflowRuntimeStateHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalWorkflowRuntimeStateHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    state: String,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[cfg(test)]
mod tests {
    use std::net::{Ipv4Addr, SocketAddr};

    use axum::{body::Body, http::Request};
    use celerity_blueprint_config_parser::blueprint::CelerityWorkflowSpec;
    use http_body_util::BodyExt;
    use pretty_assertions::assert_eq;
    use serde_json::json;
    use tokio::sync::{mpsc, oneshot};

    use crate::{
        config::{StateHandlerDefinition, WorkflowAppConfig},
        types::EventType,
    };

    use super::*;

    #[test_log::test(tokio::test)]
    async fn test_retrieve_next_event_in_runtime_queue() {
        let app_config = create_test_workflow_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let expected_event = create_test_execute_step_event();
        event_queue
            .lock()
            .unwrap()
            .push_back((oneshot::channel().0, expected_event.clone()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let api = create_workflow_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
        )
        .unwrap();

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, api).await.unwrap();
        });

        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();

        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/next", addr = addr))
                    .header("Host", "localhost")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let response_event: EventData = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert_eq!(response_event, expected_event);

        // Make sure that the event has been removed from the queue
        // and added to the processing events map, so that when the result
        // is sent back, the event can be accessed and processed for producing
        // the final response to the caller.
        // The event needs to be cleared from the queue so that the next event
        // can be retrieved immediately, allowing the handlers process to handle
        // multiple events concurrently.
        assert_eq!(event_queue.lock().unwrap().len(), 0);
        assert_eq!(processing_events_map.lock().unwrap().len(), 1);
    }

    #[test_log::test(tokio::test)]
    async fn test_returns_null_when_no_events_in_queue() {
        let app_config = create_test_workflow_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let api = create_workflow_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
        )
        .unwrap();

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, api).await.unwrap();
        });

        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();

        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/next", addr = addr))
                    .header("Host", "localhost")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let response_event: Option<EventData> = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert!(response_event.is_none());
    }

    #[test_log::test(tokio::test)]
    async fn test_proceses_event_result() {
        let app_config = create_test_workflow_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let event = create_test_execute_step_event();
        let (tx, rx) = oneshot::channel();
        // Create a channel to verify that the result has been processed with a buffer
        // of 1 to avoid blocking when the result is sent to the channel.
        let (verify_tx, mut verify_rx) = mpsc::channel(1);
        tokio::spawn(async move {
            let res = rx.await;
            verify_tx.send(res).await.unwrap();
        });

        processing_events_map
            .lock()
            .unwrap()
            .insert(event.id.clone(), (tx, event.clone()));

        let api = create_workflow_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
        )
        .unwrap();

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, api).await.unwrap();
        });

        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();

        let result = EventResult {
            event_id: event.id.clone(),
            data: json!({
                "processedLocation": "/tmp/processed-document-1.pdf",
            }),
            context: None,
        };
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/result", addr = addr))
                    .header("Host", "localhost")
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&result).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let response_data: ResponseMessage = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert_eq!(
            response_data.message,
            "The result has been successfully processed"
        );

        tokio::select! {
            _ = tokio::time::sleep(tokio::time::Duration::from_secs(10)) => {
                panic!("Timed out waiting for the result on verification channel");
            }
            received_wrapped = verify_rx.recv() => {
                let received = received_wrapped.unwrap().unwrap();
                assert_eq!(received, (event, result));
            }
        }
    }

    #[test_log::test(tokio::test)]
    async fn test_returns_error_when_event_for_provided_result_is_not_found() {
        let app_config = create_test_workflow_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));

        let api = create_workflow_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
        )
        .unwrap();

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, api).await.unwrap();
        });

        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();

        let result = EventResult {
            event_id: "test-event-1".to_string(),
            data: json!({
                "processedLocation": "/tmp/processed-document-1000.pdf",
            }),
            context: None,
        };
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/result", addr = addr))
                    .header("Host", "localhost")
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&result).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let response_data: ResponseMessage = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 404);
        assert_eq!(
            response_data.message,
            "Event with provided ID was not found"
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_retrieves_runtime_config() {
        let app_config = create_test_workflow_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));

        let api = create_workflow_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
        )
        .unwrap();

        let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::UNSPECIFIED, 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            axum::serve(listener, api).await.unwrap();
        });

        let client =
            hyper_util::client::legacy::Client::builder(hyper_util::rt::TokioExecutor::new())
                .build_http();

        let response = client
            .request(
                Request::builder()
                    .method("GET")
                    .uri(format!("http://{addr}/runtime/config", addr = addr))
                    .header("Host", "localhost")
                    .header("Content-Type", "application/json")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let runtime_config: LocalWorkflowRuntimeConfig = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert_eq!(runtime_config, create_expected_workflow_runtime_config());
    }

    fn create_expected_workflow_runtime_config() -> LocalWorkflowRuntimeConfig {
        LocalWorkflowRuntimeConfig {
            app_config: LocalWorkflowRuntimeAppConfig {
                tracing_enabled: true,
                state_handlers: vec![LocalWorkflowRuntimeStateHandlerConfig {
                    handler_name: "processDocumentHandler".to_string(),
                    handler_tag: "state::processDocument".to_string(),
                    state: "processDocument".to_string(),
                    timeout: 10,
                    tracing_enabled: true,
                }],
            },
        }
    }

    fn create_test_workflow_app_config() -> WorkflowAppConfig {
        WorkflowAppConfig {
            state_handlers: Some(vec![StateHandlerDefinition {
                name: "processDocumentHandler".to_string(),
                location: "processing".to_string(),
                handler: "process_document".to_string(),
                state: "processDocument".to_string(),
                timeout: 10,
                tracing_enabled: true,
            }]),
            workflow: CelerityWorkflowSpec {
                states: HashMap::new(),
            },
        }
    }

    fn create_test_execute_step_event() -> EventData {
        EventData {
            id: "test_event_1".to_string(),
            event_type: EventType::ExecuteStep,
            handler_tag: "state::processDocument".to_string(),
            timestamp: 1723458289,
            data: json!({
                "downloaded": {
                    "filePath": "/tmp/test-event-1--document.pdf",
                }
            }),
        }
    }
}
