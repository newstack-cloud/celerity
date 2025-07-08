use std::{
    collections::{HashMap, VecDeque},
    sync::{Arc, Mutex},
};

use axum::{
    extract::State,
    response::Json,
    routing::{get, post},
    Router,
};
use celerity_helpers::runtime_types::ResponseMessage;
use serde::{Deserialize, Serialize};
use tracing::debug;

use crate::{
    config::AppConfig,
    errors::{ApplicationStartError, EventResultError, WebSocketsMessageError},
    types::{EventData, EventResult, EventTuple, WebSocketMessages},
    wsconn_registry::WebSocketRegistrySend,
};

// Creates a router for the local runtime API
// that allows for interaction between the runtime
// and the handlers executable over HTTP.
pub fn create_runtime_local_api(
    app_config: &AppConfig,
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
    ws_conn_registry_send: Option<Arc<dyn WebSocketRegistrySend>>,
) -> Result<Router, ApplicationStartError> {
    let local_runtime_api_config = create_local_runtime_api_config(app_config);
    let shared_state = Arc::new(LocalRuntimeAppState {
        event_queue: event_queue.clone(),
        processing_events_map: processing_events_map.clone(),
        ws_conn_registry_send,
        runtime_api_config: local_runtime_api_config,
    });
    Ok(Router::new()
        .route("/events/next", post(next_event_handler))
        .route("/events/result", post(event_result_handler))
        .route("/websockets/messages", post(websockets_messages_handler))
        .route("/runtime/config", get(runtime_config_handler))
        .with_state(shared_state))
}

async fn next_event_handler(
    State(state): State<Arc<LocalRuntimeAppState>>,
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
    State(state): State<Arc<LocalRuntimeAppState>>,
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

async fn websockets_messages_handler(
    State(state): State<Arc<LocalRuntimeAppState>>,
    Json(messages): Json<WebSocketMessages>,
) -> Result<Json<ResponseMessage>, WebSocketsMessageError> {
    if let Some(ref ws_conn_registry) = state.ws_conn_registry_send {
        for message in messages.messages {
            ws_conn_registry
                .send_message(message.connection_id, message.message)
                .await
                .map_err(|_| WebSocketsMessageError::UnexpectedError)?;
        }
        return Ok(Json(ResponseMessage {
            message: "The messages have been sent".to_string(),
        }));
    }
    Err(WebSocketsMessageError::NotEnabled)
}

async fn runtime_config_handler(
    State(state): State<Arc<LocalRuntimeAppState>>,
) -> Json<LocalRuntimeConfig> {
    Json(state.runtime_api_config.clone())
}

fn create_local_runtime_api_config(app_config: &AppConfig) -> LocalRuntimeConfig {
    let tracing_enabled = app_config
        .api
        .as_ref()
        .map(|api| api.tracing_enabled)
        .unwrap_or(false);

    let http = create_local_runtime_http_config(app_config);
    let websocket = create_local_runtime_websocket_config(app_config);
    let consumer = create_local_runtime_consumer_config(app_config);
    let schedule = create_local_runtime_schedule_config(app_config);

    LocalRuntimeConfig {
        app_config: LocalRuntimeAppConfig {
            tracing_enabled,
            http,
            websocket,
            consumer,
            schedule,
        },
    }
}

fn create_local_runtime_http_config(app_config: &AppConfig) -> LocalRuntimeHttpConfig {
    let mut config = LocalRuntimeHttpConfig { handlers: vec![] };
    if let Some(api) = &app_config.api {
        if let Some(http) = &api.http {
            for handler in &http.handlers {
                config.handlers.push(LocalRuntimeHttpHandlerConfig {
                    handler_name: handler.name.clone(),
                    handler_tag: format!("{}::{}", handler.method, handler.path),
                    path: handler.path.clone(),
                    method: handler.method.clone(),
                    timeout: handler.timeout,
                    tracing_enabled: handler.tracing_enabled,
                });
            }
        }
    }
    config
}

fn create_local_runtime_websocket_config(app_config: &AppConfig) -> LocalRuntimeWebSocketConfig {
    let mut config = LocalRuntimeWebSocketConfig { handlers: vec![] };
    if let Some(api) = &app_config.api {
        if let Some(websocket) = &api.websocket {
            for handler in &websocket.handlers {
                config.handlers.push(LocalRuntimeWebSocketHandlerConfig {
                    handler_name: handler.name.clone(),
                    handler_tag: format!("{}::{}", handler.route_key, handler.route),
                    route_key: handler.route_key.clone(),
                    route: handler.route.clone(),
                    timeout: handler.timeout,
                    tracing_enabled: handler.tracing_enabled,
                });
            }
        }
    }
    config
}

fn create_local_runtime_consumer_config(app_config: &AppConfig) -> LocalRuntimeConsumerConfig {
    let mut config = LocalRuntimeConsumerConfig { handlers: vec![] };
    if let Some(consumers) = &app_config.consumers {
        for consumer in &consumers.consumers {
            for handler in &consumer.handlers {
                config.handlers.push(LocalRuntimeConsumerHandlerConfig {
                    handler_name: handler.name.clone(),
                    handler_tag: format!(
                        "source::{}::{}",
                        consumer.source_id,
                        handler.name.clone()
                    ),
                    source_id: consumer.source_id.clone(),
                    timeout: handler.timeout,
                    tracing_enabled: handler.tracing_enabled,
                });
            }
        }
    }
    config
}

fn create_local_runtime_schedule_config(app_config: &AppConfig) -> LocalRuntimeScheduleConfig {
    let mut config = LocalRuntimeScheduleConfig { handlers: vec![] };
    if let Some(schedules) = &app_config.schedules {
        for schedule in &schedules.schedules {
            for handler in &schedule.handlers {
                config.handlers.push(LocalRuntimeScheduleHandlerConfig {
                    handler_name: handler.name.clone(),
                    handler_tag: format!(
                        "source::{}::{}",
                        schedule.schedule_id,
                        handler.name.clone()
                    ),
                    schedule: schedule.schedule_value.clone(),
                    timeout: handler.timeout,
                    tracing_enabled: handler.tracing_enabled,
                });
            }
        }
    }
    config
}

#[derive(Debug)]
struct LocalRuntimeAppState {
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
    ws_conn_registry_send: Option<Arc<dyn WebSocketRegistrySend>>,
    runtime_api_config: LocalRuntimeConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeConfig {
    #[serde(rename = "appConfig")]
    app_config: LocalRuntimeAppConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeAppConfig {
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
    http: LocalRuntimeHttpConfig,
    websocket: LocalRuntimeWebSocketConfig,
    consumer: LocalRuntimeConsumerConfig,
    schedule: LocalRuntimeScheduleConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeHttpConfig {
    handlers: Vec<LocalRuntimeHttpHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeHttpHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    path: String,
    method: String,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeWebSocketConfig {
    handlers: Vec<LocalRuntimeWebSocketHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeWebSocketHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    #[serde(rename = "routeKey")]
    route_key: String,
    route: String,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeConsumerConfig {
    handlers: Vec<LocalRuntimeConsumerHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeConsumerHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    #[serde(rename = "sourceId")]
    source_id: String,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeScheduleConfig {
    handlers: Vec<LocalRuntimeScheduleHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeScheduleHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    schedule: String,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeEventsConfig {
    handlers: Vec<LocalRuntimeEventHandlerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LocalRuntimeEventHandlerConfig {
    #[serde(rename = "handlerName")]
    handler_name: String,
    #[serde(rename = "handlerTag")]
    handler_tag: String,
    event: Option<String>,
    timeout: i64,
    #[serde(rename = "tracingEnabled")]
    tracing_enabled: bool,
}

#[cfg(test)]
mod tests {
    use std::{
        fmt::{Debug, Display},
        net::{Ipv4Addr, SocketAddr},
    };

    use async_trait::async_trait;
    use axum::{body::Body, http::Request};
    use celerity_blueprint_config_parser::blueprint::{
        CelerityApiAuth, CelerityApiAuthGuard, CelerityApiAuthGuardType,
        CelerityApiAuthGuardValueSource, CelerityApiCors, CelerityApiCorsConfiguration,
    };
    use http_body_util::BodyExt;
    use pretty_assertions::assert_eq;
    use serde_json::json;
    use tokio::sync::{mpsc, oneshot};

    use crate::{
        config::{
            ApiConfig, ConsumerConfig, ConsumersConfig, EventHandlerDefinition, HttpConfig,
            HttpHandlerDefinition, ScheduleConfig, SchedulesConfig, WebSocketConfig,
            WebSocketHandlerDefinition,
        },
        errors::WebSocketConnError,
        types::{
            EventDataPayload, EventResultData, EventType, HttpRequestEventData, HttpResponseData,
            WebSocketMessage,
        },
    };

    use super::*;

    #[test_log::test(tokio::test)]
    async fn test_retrieve_next_event_in_runtime_queue() {
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let expected_event = create_test_http_event();
        event_queue
            .lock()
            .unwrap()
            .push_back((oneshot::channel().0, expected_event.clone()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            None,
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
                    .uri(format!("http://{addr}/events/next"))
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
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            None,
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
                    .uri(format!("http://{addr}/events/next"))
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
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let event = create_test_http_event();
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

        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            None,
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
            data: EventResultData::HttpResponse(HttpResponseData {
                status: 200,
                headers: HashMap::new(),
                body: json!({ "id": "123" }).to_string(),
            }),
            context: None,
        };
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/result"))
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
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));

        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            None,
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
            data: EventResultData::HttpResponse(HttpResponseData {
                status: 200,
                headers: HashMap::new(),
                body: json!({ "id": "8049" }).to_string(),
            }),
            context: None,
        };
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/events/result"))
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
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));

        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            None,
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
                    .uri(format!("http://{addr}/runtime/config"))
                    .header("Host", "localhost")
                    .header("Content-Type", "application/json")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let runtime_config: LocalRuntimeConfig = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert_eq!(runtime_config, create_expected_runtime_config());
    }

    #[test_log::test(tokio::test)]
    async fn test_sends_websocket_messages() {
        let app_config = create_test_app_config();
        let event_queue = Arc::new(Mutex::new(VecDeque::new()));
        let processing_events_map = Arc::new(Mutex::new(HashMap::new()));
        let (tx, mut rx) = mpsc::channel(10);
        let ws_conn_registry = Arc::new(TestWebSocketConnRegistry::new(tx));
        let api = create_runtime_local_api(
            &app_config,
            event_queue.clone(),
            processing_events_map.clone(),
            Some(ws_conn_registry),
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

        let messages = WebSocketMessages {
            messages: vec![
                WebSocketMessage {
                    connection_id: "test-conn-1".to_string(),
                    message: "Hello, World!".to_string(),
                },
                WebSocketMessage {
                    connection_id: "test-conn-2".to_string(),
                    message: "Hello, Solar System!".to_string(),
                },
                WebSocketMessage {
                    connection_id: "test-conn-3".to_string(),
                    message: "Hello, Galaxy!".to_string(),
                },
            ],
        };
        let response = client
            .request(
                Request::builder()
                    .method("POST")
                    .uri(format!("http://{addr}/websockets/messages"))
                    .header("Host", "localhost")
                    .header("Content-Type", "application/json")
                    .body(Body::from(serde_json::to_string(&messages).unwrap()))
                    .unwrap(),
            )
            .await
            .unwrap();

        let status = response.status();
        let body = response.into_body().collect().await.unwrap().to_bytes();
        let response_data: ResponseMessage = serde_json::from_slice(&body).unwrap();
        assert_eq!(status, 200);
        assert_eq!(response_data.message, "The messages have been sent");

        let mut received_messages = vec![];
        for _ in 0..3 {
            let message = rx.recv().await.unwrap();
            received_messages.push(message);
        }
        assert_eq!(received_messages.len(), 3);
        assert_eq!(
            received_messages[0],
            ("test-conn-1".to_string(), "Hello, World!".to_string())
        );
        assert_eq!(
            received_messages[1],
            (
                "test-conn-2".to_string(),
                "Hello, Solar System!".to_string()
            )
        );
        assert_eq!(
            received_messages[2],
            ("test-conn-3".to_string(), "Hello, Galaxy!".to_string())
        );
    }

    #[test_log::test(tokio::test)]
    async fn test_returns_websockets_not_enabled_error_for_app_missing_websocket_support() {}

    fn create_test_app_config() -> AppConfig {
        AppConfig {
            api: Some(ApiConfig {
                http: Some(HttpConfig {
                    handlers: vec![HttpHandlerDefinition {
                        name: "Orders-GetOrder-v1".to_string(),
                        path: "/orders/{id}".to_string(),
                        method: "get".to_string(),
                        location: "./handlers/orders".to_string(),
                        handler: "get_order".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                    base_paths: vec!["/".to_string()],
                }),
                websocket: Some(WebSocketConfig {
                    handlers: vec![WebSocketHandlerDefinition {
                        name: "Orders-StreamOrders-v1".to_string(),
                        route_key: "event".to_string(),
                        route: "order".to_string(),
                        location: "./handlers/order-stream".to_string(),
                        handler: "stream_orders".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                    base_paths: vec!["/ws".to_string()],
                    route_key: "event".to_string(),
                }),
                auth: Some(CelerityApiAuth {
                    default_guard: Some("jwt".to_string()),
                    guards: HashMap::from([(
                        "jwt".to_string(),
                        CelerityApiAuthGuard {
                            guard_type: CelerityApiAuthGuardType::Jwt,
                            issuer: Some("https://example.com".to_string()),
                            audience: Some(vec!["https://example.com".to_string()]),
                            token_source: Some(CelerityApiAuthGuardValueSource::Str(
                                "$.headers.Authorization".to_string(),
                            )),
                        },
                    )]),
                }),
                cors: Some(CelerityApiCors::CorsConfiguration(
                    CelerityApiCorsConfiguration {
                        allow_origins: Some(vec!["*".to_string()]),
                        allow_methods: Some(vec!["GET".to_string(), "POST".to_string()]),
                        allow_headers: Some(vec!["*".to_string()]),
                        expose_headers: Some(vec!["*".to_string()]),
                        allow_credentials: Some(true),
                        max_age: Some(3600),
                    },
                )),
                tracing_enabled: false,
            }),
            consumers: Some(ConsumersConfig {
                consumers: vec![ConsumerConfig {
                    source_id: "arn:aws:sqs:us-east-2:444455556666:queue1".to_string(),
                    batch_size: Some(10),
                    visibility_timeout: None,
                    wait_time_seconds: None,
                    partial_failures: Some(true),
                    handlers: vec![EventHandlerDefinition {
                        name: "Orders-ProcessOrder-v1".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                        location: "./handlers/orders".to_string(),
                        handler: "process_order".to_string(),
                    }],
                }],
            }),
            schedules: Some(SchedulesConfig {
                schedules: vec![ScheduleConfig {
                    schedule_id: "test-schedule-1".to_string(),
                    schedule_value: "rate(1h)".to_string(),
                    queue_id: "arn:aws:sqs:us-east-2:444455556666:queue1".to_string(),
                    batch_size: Some(10),
                    visibility_timeout: None,
                    wait_time_seconds: None,
                    partial_failures: Some(true),
                    handlers: vec![EventHandlerDefinition {
                        name: "Orders-SyncOrders-v1".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                        location: "./handlers/orders".to_string(),
                        handler: "sync_orders".to_string(),
                    }],
                }],
            }),
        }
    }

    fn create_test_http_event() -> EventData {
        EventData {
            id: "test_event_1".to_string(),
            event_type: EventType::HttpRequest,
            handler_tag: "get::/orders/{id}".to_string(),
            timestamp: 1723458289,
            data: EventDataPayload::HttpRequestEventData(Box::new(HttpRequestEventData {
                method: "get".to_string(),
                path: "/orders/123".to_string(),
                route: "/orders/{id}".to_string(),
                path_params: HashMap::from([("id".to_string(), "123".to_string())]),
                query_params: HashMap::new(),
                multi_query_params: HashMap::new(),
                headers: HashMap::from([("Host".to_string(), "localhost".to_string())]),
                multi_headers: HashMap::new(),
                body: None,
                source_ip: "192.168.0.1".to_string(),
                request_id: "test_request_1".to_string(),
            })),
        }
    }

    fn create_expected_runtime_config() -> LocalRuntimeConfig {
        LocalRuntimeConfig {
            app_config: LocalRuntimeAppConfig {
                tracing_enabled: false,
                http: LocalRuntimeHttpConfig {
                    handlers: vec![LocalRuntimeHttpHandlerConfig {
                        handler_name: "Orders-GetOrder-v1".to_string(),
                        handler_tag: "get::/orders/{id}".to_string(),
                        path: "/orders/{id}".to_string(),
                        method: "get".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                },
                websocket: LocalRuntimeWebSocketConfig {
                    handlers: vec![LocalRuntimeWebSocketHandlerConfig {
                        handler_name: "Orders-StreamOrders-v1".to_string(),
                        handler_tag: "event::order".to_string(),
                        route_key: "event".to_string(),
                        route: "order".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                },
                consumer: LocalRuntimeConsumerConfig {
                    handlers: vec![LocalRuntimeConsumerHandlerConfig {
                        handler_name: "Orders-ProcessOrder-v1".to_string(),
                        handler_tag: "source::arn:aws:sqs:us-east-2:444455556666:queue1::Orders-ProcessOrder-v1".to_string(),
                        source_id: "arn:aws:sqs:us-east-2:444455556666:queue1".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                },
                schedule: LocalRuntimeScheduleConfig {
                    handlers: vec![LocalRuntimeScheduleHandlerConfig {
                        handler_name: "Orders-SyncOrders-v1".to_string(),
                        handler_tag: "source::test-schedule-1::Orders-SyncOrders-v1".to_string(),
                        schedule: "rate(1h)".to_string(),
                        timeout: 30,
                        tracing_enabled: true,
                    }],
                },
            },
        }
    }

    struct TestWebSocketConnRegistry {
        tx: mpsc::Sender<(String, String)>,
    }

    impl TestWebSocketConnRegistry {
        fn new(tx: mpsc::Sender<(String, String)>) -> Self {
            Self { tx }
        }
    }

    #[async_trait]
    impl WebSocketRegistrySend for TestWebSocketConnRegistry {
        async fn send_message(
            &self,
            connection_id: String,
            message: String,
        ) -> Result<(), WebSocketConnError> {
            self.tx.send((connection_id, message)).await?;
            Ok(())
        }
    }

    impl Debug for TestWebSocketConnRegistry {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            write!(f, "TestWebSocketConnRegistry")
        }
    }

    impl Display for TestWebSocketConnRegistry {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            write!(f, "TestWebSocketConnRegistry")
        }
    }
}
