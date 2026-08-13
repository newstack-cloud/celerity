//! The handler-facing side of the IPC protocol which is one long-lived bidirectional
//! stream per handler process.
//!
//! Everything travels on that one stream. The runtime pushes events down it and
//! the handler pushes results back, and configuration, credit and shutdown
//! share the same channel rather than needing side connections.
//!
//! The frame loop is written against plain streams rather than against tonic's
//! types, so the protocol's behaviour can be exercised without a transport.
//! The service implementation is a thin adapter over it.

use std::{
    collections::HashSet,
    pin::Pin,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
};

use futures::{Stream, StreamExt};
use tokio::sync::{mpsc, oneshot};
use tonic::Status;
use tracing::{debug, info, warn};

use crate::{
    config::AppConfig,
    dispatcher::{DispatcherCommand, StreamFrame, StreamId, StreamRegistration},
    event_queue::InFlightTable,
    ipc_frames::{dispatch_from_event, event_result_from_frame},
    ipc_proto as proto,
    types::{CancelReason, EventOutcome},
};

/// How many frames may be queued towards a handler before the runtime waits.
const OUTBOUND_BUFFER: usize = 256;

/// Everything a stream needs in order to serve one handler process.
pub struct StreamContext {
    /// Sent to a handler before it declares itself, so an SDK can check what it
    /// registered in code against what the blueprint declares.
    pub runtime_config: proto::RuntimeConfig,
    /// The handler tags the blueprint declares, used to answer the handshake.
    pub blueprint_tags: HashSet<String>,
    pub commands: mpsc::Sender<DispatcherCommand>,
    pub in_flight: Arc<InFlightTable>,
}

/// The outcome of checking a handler's declared tags against the blueprint.
#[derive(Debug, PartialEq)]
pub struct TagCheck {
    /// Registered by the handler but absent from the blueprint.
    pub unknown: Vec<String>,
    /// Declared by the blueprint but not registered by the handler.
    pub unhandled: Vec<String>,
}

impl TagCheck {
    /// A handler is accepted when it serves exactly the blueprint's tags.
    ///
    /// Both directions are refused rather than only one. A tag the handler does
    /// not serve means requests for it would be dispatched nowhere, and a tag
    /// the blueprint does not declare means the handler has registered
    /// something that can never be addressed. Today either surfaces as a 404 in
    /// production; here it stops the handler starting.
    pub fn accepted(&self) -> bool {
        self.unknown.is_empty() && self.unhandled.is_empty()
    }
}

/// Compares what a handler says it serves against what the blueprint declares.
pub fn check_tags(declared: &[String], blueprint: &HashSet<String>) -> TagCheck {
    let declared_set: HashSet<&String> = declared.iter().collect();

    let mut unknown: Vec<String> = declared
        .iter()
        .filter(|tag| !blueprint.contains(*tag))
        .cloned()
        .collect();
    let mut unhandled: Vec<String> = blueprint
        .iter()
        .filter(|tag| !declared_set.contains(tag))
        .cloned()
        .collect();

    // Sorted so that a mismatch reads the same way every time it is reported.
    unknown.sort();
    unhandled.sort();

    TagCheck { unknown, unhandled }
}

/// Builds the configuration a handler receives when it connects.
pub fn runtime_config_from_app_config(
    app_config: &AppConfig,
    tracing_enabled: bool,
    metrics_enabled: bool,
) -> proto::RuntimeConfig {
    use crate::config::EventConfig;
    use crate::event_queue::{
        custom_handler_tag, http_handler_tag, source_handler_tag, timeout_from_seconds,
        websocket_handler_tag,
    };

    let mut handlers = Vec::new();
    let mut push = |name: &str, tag: String, timeout: i64, tracing: bool| {
        handlers.push(proto::HandlerConfig {
            handler_name: name.to_string(),
            handler_tag: tag,
            timeout_ms: timeout_from_seconds(timeout).as_millis() as i64,
            tracing_enabled: tracing,
        });
    };

    if let Some(api) = &app_config.api {
        if let Some(http) = &api.http {
            for handler in &http.handlers {
                push(
                    &handler.name,
                    http_handler_tag(&handler.method, &handler.path),
                    handler.timeout,
                    handler.tracing_enabled,
                );
            }
        }
        if let Some(websocket) = &api.websocket {
            for handler in &websocket.handlers {
                push(
                    &handler.name,
                    websocket_handler_tag(&handler.route_key, &handler.route),
                    handler.timeout,
                    handler.tracing_enabled,
                );
            }
        }
    }
    if let Some(consumers) = &app_config.consumers {
        for consumer in &consumers.consumers {
            for handler in &consumer.handlers {
                push(
                    &handler.name,
                    source_handler_tag(&consumer.source_id, &handler.name),
                    handler.timeout,
                    handler.tracing_enabled,
                );
            }
        }
    }
    if let Some(schedules) = &app_config.schedules {
        for schedule in &schedules.schedules {
            for handler in &schedule.handlers {
                push(
                    &handler.name,
                    source_handler_tag(&schedule.schedule_id, &handler.name),
                    handler.timeout,
                    handler.tracing_enabled,
                );
            }
        }
    }
    if let Some(events) = &app_config.events {
        for event in &events.events {
            let (source_id, event_handlers) = match event {
                EventConfig::Stream(config) => (&config.stream_id, &config.handlers),
                EventConfig::EventTrigger(config) => (&config.queue_id, &config.handlers),
            };
            for handler in event_handlers {
                push(
                    &handler.name,
                    source_handler_tag(source_id, &handler.name),
                    handler.timeout,
                    handler.tracing_enabled,
                );
            }
        }
    }
    if let Some(custom) = &app_config.custom_handlers {
        for handler in &custom.handlers {
            push(
                &handler.name,
                custom_handler_tag(&handler.name),
                handler.timeout,
                handler.tracing_enabled,
            );
        }
    }

    proto::RuntimeConfig {
        tracing_enabled,
        metrics_enabled,
        handlers,
    }
}

/// The tags a runtime config declares, which is what a handshake is checked
/// against.
pub fn tags_from_runtime_config(config: &proto::RuntimeConfig) -> HashSet<String> {
    config
        .handlers
        .iter()
        .map(|handler| handler.handler_tag.clone())
        .collect()
}

/// Hands out an identifier per attached stream.
#[derive(Debug, Default)]
pub struct StreamIds(AtomicU64);

impl StreamIds {
    pub fn next(&self) -> StreamId {
        self.0.fetch_add(1, Ordering::Relaxed)
    }
}

/// Runs one handler stream from its first frame to its last.
///
/// Configuration is sent before anything is asked of the handler, then the
/// handshake decides whether it may serve traffic at all. Only after the
/// dispatcher confirms the stream is attached does any event flow, so a handler
/// is never told it is serving traffic that is still going elsewhere.
pub async fn run_stream<I>(
    stream_id: StreamId,
    context: Arc<StreamContext>,
    mut inbound: I,
    outbound: mpsc::Sender<Result<proto::RuntimeMessage, Status>>,
) where
    I: Stream<Item = Result<proto::HandlerMessage, Status>> + Unpin,
{
    if send(&outbound, frame_config(context.runtime_config.clone()))
        .await
        .is_err()
    {
        return;
    }

    let Some(ready) = await_ready(&mut inbound).await else {
        debug!(stream_id, "handler stream closed before declaring itself");
        return;
    };

    let check = check_tags(&ready.handler_tags, &context.blueprint_tags);
    let accepted = check.accepted();
    if !accepted {
        warn!(
            stream_id,
            unknown = ?check.unknown,
            unhandled = ?check.unhandled,
            "refusing a handler whose tags do not match the blueprint"
        );
    }

    let _ = send(
        &outbound,
        frame_ready_ack(proto::ReadyAck {
            accepted,
            unknown_tags: check.unknown,
            unhandled_tags: check.unhandled,
        }),
    )
    .await;

    if !accepted {
        return;
    }

    let (dispatch_tx, mut dispatch_rx) = mpsc::channel(OUTBOUND_BUFFER);
    let (registered_tx, registered_rx) = oneshot::channel();
    if context
        .commands
        .send(DispatcherCommand::Attach {
            stream_id,
            registration: Box::new(StreamRegistration {
                handler_tags: ready.handler_tags,
                initial_credit: ready.initial_credit,
                limits: ready
                    .limits
                    .into_iter()
                    .map(|limit| (limit.handler_tag, limit.max_concurrent))
                    .collect(),
                dispatch_tx,
            }),
            registered: registered_tx,
        })
        .await
        .is_err()
        || registered_rx.await.is_err()
    {
        warn!(stream_id, "dispatcher went away during the handshake");
        return;
    }

    info!(
        stream_id,
        sdk_version = %ready.sdk_version,
        credit = ready.initial_credit,
        "handler stream ready"
    );

    loop {
        tokio::select! {
            frame = dispatch_rx.recv() => match frame {
                Some(frame) => {
                    if send(&outbound, runtime_frame(frame)).await.is_err() {
                        break;
                    }
                }
                None => break,
            },
            message = inbound.next() => match message {
                Some(Ok(message)) => {
                    if !handle_inbound(stream_id, &context, message).await {
                        break;
                    }
                }
                Some(Err(status)) => {
                    warn!(stream_id, "handler stream failed: {status}");
                    break;
                }
                None => break,
            },
        }
    }

    let _ = context
        .commands
        .send(DispatcherCommand::Detach { stream_id })
        .await;
    debug!(stream_id, "handler stream closed");
}

/// Waits for the handler to declare itself.
///
/// Anything else arriving first is a protocol error: the runtime has nothing to
/// dispatch to a stream that has not said what it serves.
async fn await_ready<I>(inbound: &mut I) -> Option<proto::Ready>
where
    I: Stream<Item = Result<proto::HandlerMessage, Status>> + Unpin,
{
    match inbound.next().await {
        Some(Ok(message)) => match message.frame {
            Some(proto::handler_message::Frame::Ready(ready)) => Some(ready),
            other => {
                warn!("expected a ready frame to open the stream, got {other:?}");
                None
            }
        },
        Some(Err(status)) => {
            warn!("handler stream failed before the handshake: {status}");
            None
        }
        None => None,
    }
}

/// Applies one frame from the handler. Returns whether the stream continues.
async fn handle_inbound(
    stream_id: StreamId,
    context: &StreamContext,
    message: proto::HandlerMessage,
) -> bool {
    match message.frame {
        Some(proto::handler_message::Frame::Result(result)) => {
            complete(stream_id, context, result).await;
            true
        }
        Some(proto::handler_message::Frame::Credit(grant)) => {
            let _ = context
                .commands
                .send(DispatcherCommand::Grant {
                    stream_id,
                    additional: grant.additional,
                })
                .await;
            true
        }
        Some(proto::handler_message::Frame::Draining(draining)) => {
            info!(
                stream_id,
                deadline_unix_ms = draining.deadline_unix_ms,
                "handler is draining"
            );
            false
        }
        Some(proto::handler_message::Frame::Ready(_)) => {
            warn!(stream_id, "handler declared itself twice, ignoring");
            true
        }
        other => {
            debug!(stream_id, "ignoring an unsupported frame: {other:?}");
            true
        }
    }
}

/// Hands a result back to whoever was waiting for it, and returns the slot and
/// credit to the dispatcher.
async fn complete(stream_id: StreamId, context: &StreamContext, result: proto::Result) {
    let event_id = result.id.clone();
    let credit_grant = result.credit_grant;

    match context.in_flight.remove(&event_id) {
        Some(entry) => {
            let event_result = event_result_from_frame(result);
            if entry
                .result_tx
                .send(EventOutcome::Completed(Box::new(entry.event), event_result))
                .is_err()
            {
                debug!(stream_id, %event_id, "the caller stopped waiting for this result");
            }
        }
        // Expected rather than exceptional, the deadline may have passed, or
        // the caller has gone away before the handler answered.
        None => debug!(
            stream_id,
            %event_id,
            "discarding a result for an event that is no longer in flight"
        ),
    }

    // Sent whether or not anyone was still waiting. The event consumed credit
    // when it was dispatched, so a result the caller can no longer use must
    // still return it.
    let _ = context
        .commands
        .send(DispatcherCommand::Completed {
            stream_id,
            event_id,
            credit_grant,
        })
        .await;
}

async fn send(
    outbound: &mpsc::Sender<Result<proto::RuntimeMessage, Status>>,
    frame: proto::RuntimeMessage,
) -> Result<(), ()> {
    outbound.send(Ok(frame)).await.map_err(|_| ())
}

fn frame_config(config: proto::RuntimeConfig) -> proto::RuntimeMessage {
    proto::RuntimeMessage {
        frame: Some(proto::runtime_message::Frame::Config(config)),
    }
}

fn frame_ready_ack(ack: proto::ReadyAck) -> proto::RuntimeMessage {
    proto::RuntimeMessage {
        frame: Some(proto::runtime_message::Frame::ReadyAck(ack)),
    }
}

/// Renders what the dispatcher sends down a stream as a protocol frame.
fn runtime_frame(frame: StreamFrame) -> proto::RuntimeMessage {
    let frame = match frame {
        StreamFrame::Dispatch(dispatched) => proto::runtime_message::Frame::Dispatch(
            dispatch_from_event(dispatched.event, dispatched.deadline_unix_ms),
        ),
        StreamFrame::Cancel { event_id, reason } => {
            proto::runtime_message::Frame::Cancel(proto::Cancel {
                id: event_id,
                reason: cancel_reason(reason) as i32,
            })
        }
        StreamFrame::Drain { deadline_unix_ms } => {
            proto::runtime_message::Frame::Drain(proto::Drain { deadline_unix_ms })
        }
    };
    proto::RuntimeMessage { frame: Some(frame) }
}

fn cancel_reason(reason: CancelReason) -> proto::cancel::Reason {
    match reason {
        CancelReason::DeadlineExceeded => proto::cancel::Reason::DeadlineExceeded,
        CancelReason::CallerGone => proto::cancel::Reason::CallerGone,
        CancelReason::Shutdown => proto::cancel::Reason::Shutdown,
    }
}

/// Serves the handler protocol over gRPC.
pub struct HandlerRuntimeService {
    context: Arc<StreamContext>,
    stream_ids: Arc<StreamIds>,
}

impl HandlerRuntimeService {
    pub fn new(context: Arc<StreamContext>) -> Self {
        HandlerRuntimeService {
            context,
            stream_ids: Arc::new(StreamIds::default()),
        }
    }
}

#[tonic::async_trait]
impl proto::handler_runtime_server::HandlerRuntime for HandlerRuntimeService {
    type EventStreamStream =
        Pin<Box<dyn Stream<Item = Result<proto::RuntimeMessage, Status>> + Send + 'static>>;

    async fn event_stream(
        &self,
        request: tonic::Request<tonic::Streaming<proto::HandlerMessage>>,
    ) -> Result<tonic::Response<Self::EventStreamStream>, Status> {
        let stream_id = self.stream_ids.next();
        let context = self.context.clone();
        let inbound = request.into_inner();
        let (outbound_tx, outbound_rx) = mpsc::channel(OUTBOUND_BUFFER);

        tokio::spawn(async move {
            run_stream(stream_id, context, inbound, outbound_tx).await;
        });

        Ok(tonic::Response::new(Box::pin(
            tokio_stream::wrappers::ReceiverStream::new(outbound_rx),
        )))
    }
}

#[cfg(test)]
mod tests {
    use std::{collections::HashMap, time::Duration};

    use serde_json::json;

    use super::*;
    use crate::{
        event_queue::{EventQueueParts, HandlerTimeouts},
        types::{EventData, EventDataPayload, EventType, ScheduleEventData},
    };

    fn blueprint_tags(tags: &[&str]) -> HashSet<String> {
        tags.iter().map(|tag| tag.to_string()).collect()
    }

    struct Harness {
        inbound_tx: mpsc::Sender<Result<proto::HandlerMessage, Status>>,
        outbound_rx: mpsc::Receiver<Result<proto::RuntimeMessage, Status>>,
        commands_rx: mpsc::Receiver<DispatcherCommand>,
        in_flight: Arc<InFlightTable>,
        _cleanup: oneshot::Sender<()>,
    }

    fn start(tags: &[&str]) -> Harness {
        let (handles, cleanup) = EventQueueParts::new(8).into_parts();
        let cleanup_shutdown = cleanup.spawn();
        let (commands_tx, commands_rx) = mpsc::channel(16);
        let (inbound_tx, inbound_rx) = mpsc::channel(16);
        let (outbound_tx, outbound_rx) = mpsc::channel(16);

        let context = Arc::new(StreamContext {
            runtime_config: proto::RuntimeConfig {
                tracing_enabled: false,
                metrics_enabled: false,
                handlers: vec![],
            },
            blueprint_tags: blueprint_tags(tags),
            commands: commands_tx,
            in_flight: handles.in_flight.clone(),
        });

        let in_flight = handles.in_flight.clone();
        tokio::spawn(run_stream(
            1,
            context,
            tokio_stream::wrappers::ReceiverStream::new(inbound_rx),
            outbound_tx,
        ));

        Harness {
            inbound_tx,
            outbound_rx,
            commands_rx,
            in_flight,
            _cleanup: cleanup_shutdown,
        }
    }

    async fn next_frame(
        rx: &mut mpsc::Receiver<Result<proto::RuntimeMessage, Status>>,
    ) -> Option<proto::runtime_message::Frame> {
        tokio::time::timeout(Duration::from_secs(2), rx.recv())
            .await
            .ok()
            .flatten()
            .and_then(|message| message.ok())
            .and_then(|message| message.frame)
    }

    /// Plays the dispatcher's part in the handshake: takes the attach and
    /// confirms it, which is what releases the stream into its frame loop.
    ///
    /// The registration is returned rather than dropped, because it owns the
    /// sender the stream receives dispatches on. Dropping it closes that
    /// channel, which correctly tells the stream to shut down, and the real
    /// dispatcher holds it for exactly as long as the stream is attached.
    async fn complete_handshake(harness: &mut Harness) -> Box<StreamRegistration> {
        let command = tokio::time::timeout(Duration::from_secs(2), harness.commands_rx.recv())
            .await
            .expect("the stream should attach")
            .expect("the command channel should be open");
        let DispatcherCommand::Attach {
            registered,
            registration,
            ..
        } = command
        else {
            panic!("expected an attach, got {command:?}");
        };
        registered.send(()).expect("the stream should be waiting");
        registration
    }

    fn ready(tags: &[&str]) -> proto::HandlerMessage {
        proto::HandlerMessage {
            frame: Some(proto::handler_message::Frame::Ready(proto::Ready {
                handler_tags: tags.iter().map(|tag| tag.to_string()).collect(),
                initial_credit: 4,
                sdk_version: "test/0.1".to_string(),
                limits: vec![],
            })),
        }
    }

    #[tokio::test]
    async fn sends_configuration_before_asking_the_handler_for_anything() {
        let mut harness = start(&["schedule::a"]);

        let frame = next_frame(&mut harness.outbound_rx).await;
        assert!(
            matches!(frame, Some(proto::runtime_message::Frame::Config(_))),
            "configuration should arrive first, got {frame:?}"
        );
    }

    #[tokio::test]
    async fn accepts_a_handler_whose_tags_match_the_blueprint() {
        let mut harness = start(&["schedule::a"]);
        let _config = next_frame(&mut harness.outbound_rx).await;

        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a"])))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::ReadyAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected a ready acknowledgement");
        };
        assert!(ack.accepted);
        assert!(ack.unknown_tags.is_empty());
        assert!(ack.unhandled_tags.is_empty());

        // Only now is the stream attached, so nothing was dispatched earlier.
        let command = harness.commands_rx.recv().await;
        assert!(matches!(command, Some(DispatcherCommand::Attach { .. })));
    }

    #[tokio::test]
    async fn refuses_a_handler_that_registered_a_tag_the_blueprint_does_not_declare() {
        let mut harness = start(&["schedule::a"]);
        let _config = next_frame(&mut harness.outbound_rx).await;

        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a", "schedule::typo"])))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::ReadyAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected a ready acknowledgement");
        };
        assert!(!ack.accepted);
        assert_eq!(ack.unknown_tags, vec!["schedule::typo".to_string()]);
    }

    #[tokio::test]
    async fn refuses_a_handler_that_does_not_serve_everything_the_blueprint_declares() {
        let mut harness = start(&["schedule::a", "schedule::b"]);
        let _config = next_frame(&mut harness.outbound_rx).await;

        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a"])))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::ReadyAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected a ready acknowledgement");
        };
        assert!(!ack.accepted);
        assert_eq!(ack.unhandled_tags, vec!["schedule::b".to_string()]);
    }

    #[tokio::test]
    async fn returns_a_result_to_the_waiting_caller_and_credits_the_dispatcher() {
        let mut harness = start(&["schedule::a"]);
        let _config = next_frame(&mut harness.outbound_rx).await;
        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a"])))
            .await
            .unwrap();
        let _ack = next_frame(&mut harness.outbound_rx).await;
        let _registration = complete_handshake(&mut harness).await;

        // Stand in for the dispatcher having sent this event.
        let (result_tx, result_rx) = oneshot::channel();
        let event = EventData {
            id: "event-1".to_string(),
            event_type: EventType::ScheduleMessage,
            handler_tag: "schedule::a".to_string(),
            timestamp: 0,
            data: EventDataPayload::ScheduleMessageEventData(ScheduleEventData {
                schedule_id: "schedule-1".to_string(),
                message_id: "message-1".to_string(),
                schedule: "rate(1 minute)".to_string(),
                input: None,
                vendor: json!({}),
            }),
        };
        harness.in_flight.insert(
            crate::event_queue::InFlightEntry { result_tx, event },
            Duration::from_secs(60),
        );

        harness
            .inbound_tx
            .send(Ok(proto::HandlerMessage {
                frame: Some(proto::handler_message::Frame::Result(proto::Result {
                    id: "event-1".to_string(),
                    credit_grant: 1,
                    outcome: Some(proto::result::Outcome::Schedule(proto::Ack {
                        success: true,
                        error_message: String::new(),
                    })),
                })),
            }))
            .await
            .unwrap();

        let outcome = tokio::time::timeout(Duration::from_secs(2), result_rx)
            .await
            .expect("the caller should be woken")
            .expect("the result should arrive");
        let EventOutcome::Completed(_event, result) = outcome else {
            panic!("expected a completed outcome, got {outcome:?}");
        };
        assert_eq!(result.event_id, "event-1");

        let command = harness.commands_rx.recv().await;
        let Some(DispatcherCommand::Completed { credit_grant, .. }) = command else {
            panic!("expected the slot and credit to be returned, got {command:?}");
        };
        assert_eq!(credit_grant, 1);
    }

    #[tokio::test]
    async fn detaches_when_the_handler_stops_sending() {
        let mut harness = start(&["schedule::a"]);
        let _config = next_frame(&mut harness.outbound_rx).await;
        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a"])))
            .await
            .unwrap();
        let _ack = next_frame(&mut harness.outbound_rx).await;
        let _registration = complete_handshake(&mut harness).await;

        drop(harness.inbound_tx);

        let command = tokio::time::timeout(Duration::from_secs(2), harness.commands_rx.recv())
            .await
            .expect("the stream should detach");
        assert!(matches!(command, Some(DispatcherCommand::Detach { .. })));
    }

    #[tokio::test]
    async fn closes_a_stream_that_does_not_declare_itself_first() {
        let mut harness = start(&["schedule::a"]);
        let _config = next_frame(&mut harness.outbound_rx).await;

        // A credit grant before the handshake is a protocol error.
        harness
            .inbound_tx
            .send(Ok(proto::HandlerMessage {
                frame: Some(proto::handler_message::Frame::Credit(proto::CreditGrant {
                    additional: 4,
                })),
            }))
            .await
            .unwrap();

        assert!(next_frame(&mut harness.outbound_rx).await.is_none());
        // Never attached, so there is nothing to detach.
        assert!(harness.commands_rx.try_recv().is_err());
    }

    #[test]
    fn a_tag_check_reports_both_directions() {
        let check = check_tags(
            &["a".to_string(), "typo".to_string()],
            &blueprint_tags(&["a", "b"]),
        );

        assert!(!check.accepted());
        assert_eq!(check.unknown, vec!["typo".to_string()]);
        assert_eq!(check.unhandled, vec!["b".to_string()]);
    }

    #[test]
    fn timeouts_reach_the_handler_in_milliseconds() {
        use crate::config::{ApiConfig, AppConfig, HttpConfig, HttpHandlerDefinition};

        let app_config = AppConfig {
            api: Some(ApiConfig {
                http: Some(HttpConfig {
                    handlers: vec![HttpHandlerDefinition {
                        name: "GetOrder".to_string(),
                        path: "/orders/{id}".to_string(),
                        method: "GET".to_string(),
                        timeout: 30,
                        ..Default::default()
                    }],
                    base_paths: vec![],
                }),
                websocket: None,
                guards: None,
                auth: None,
                cors: None,
                tracing_enabled: false,
            }),
            consumers: None,
            schedules: None,
            events: None,
            custom_handlers: None,
        };

        let config = runtime_config_from_app_config(&app_config, true, false);

        assert!(config.tracing_enabled);
        assert_eq!(config.handlers.len(), 1);
        assert_eq!(config.handlers[0].handler_tag, "GET::/orders/{id}");
        assert_eq!(config.handlers[0].timeout_ms, 30_000);
        assert_eq!(
            tags_from_runtime_config(&config),
            blueprint_tags(&["GET::/orders/{id}"])
        );
    }

    #[test]
    fn handler_timeouts_and_runtime_config_agree_on_tags() {
        // The two are built from the same tag builders, so a handler cannot be
        // told about a tag whose timeout the runtime would not resolve.
        let timeouts = HandlerTimeouts::new(HashMap::new(), Duration::from_secs(60));
        assert_eq!(timeouts.for_tag("anything"), Duration::from_secs(60));
    }
}
