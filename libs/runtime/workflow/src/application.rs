// use std::{
//     collections::{HashMap, VecDeque},
//     sync::{Arc, Mutex},
// };

// /// Provides an application that can run an executable workflow
// /// with a HTTP API to trigger, monitor and retrieve historical
// /// executions for the workflow.
// pub struct Application {
//     runtime_config: RuntimeConfig,
//     app_tracing_enabled: bool,
//     workflow_api: Option<Router<ApiAppState>>,
//     runtime_local_api: Option<Router>,
//     event_queue: Option<Arc<Mutex<VecDeque<EventTuple>>>>,
//     processing_events_map: Option<Arc<Mutex<HashMap<String, EventTuple>>>>,
//     server_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
//     local_api_shutdown_signal: Option<tokio::sync::oneshot::Sender<()>>,
// }
