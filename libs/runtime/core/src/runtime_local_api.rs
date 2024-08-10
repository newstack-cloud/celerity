use std::{
    collections::{HashMap, VecDeque},
    sync::{Arc, Mutex},
};

use axum::{extract::State, response::Json, routing::post, Router};

use crate::{
    config::RuntimeConfig,
    errors::{ApplicationStartError, EventResultError},
    types::{EventData, EventResult, EventTuple, ResponseMessage, WebSocketMessages},
};

// Creates a router for the local runtime API
// that allows for interaction between the runtime
// and the handlers executable over HTTP.
pub fn create_runtime_local_api(
    runtime_config: &RuntimeConfig,
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
) -> Result<Router, ApplicationStartError> {
    let shared_state = Arc::new(LocalRuntimeAppState {
        event_queue: event_queue.clone(),
        processing_events_map: processing_events_map.clone(),
    });
    Ok(Router::new()
        .route("/events/next", post(next_event_handler))
        .route("/events/result", post(event_result_handler))
        .route("/websockets/messages", post(websockets_messages_handler))
        .with_state(shared_state))
}

async fn next_event_handler(
    State(state): State<Arc<LocalRuntimeAppState>>,
) -> Json<Option<EventData>> {
    let mut event_queue = state.event_queue.lock().unwrap();
    let mut processing_events_map = state.processing_events_map.lock().unwrap();

    if let Some((tx, event)) = event_queue.pop_front() {
        let event_for_response = event.clone();
        processing_events_map.insert(event.id.clone(), (tx, event));
        return Json(Some(event_for_response));
    }
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
) -> Result<Json<ResponseMessage>, EventResultError> {
    Err(EventResultError::EventNotFound)
}

#[derive(Debug)]
struct LocalRuntimeAppState {
    event_queue: Arc<Mutex<VecDeque<EventTuple>>>,
    processing_events_map: Arc<Mutex<HashMap<String, EventTuple>>>,
}
