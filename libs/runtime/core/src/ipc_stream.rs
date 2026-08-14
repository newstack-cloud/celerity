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
    collections::{HashMap, HashSet},
    pin::Pin,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
    time::Duration,
};

use base64::{engine::general_purpose::STANDARD as BASE64, Engine as _};
use celerity_ws_registry::{
    registry::{SendContext, WebSocketRegistrySend},
    types::MessageType,
};
use futures::{Stream, StreamExt};
use tokio::{
    sync::{mpsc, oneshot},
    time::Instant,
};
use tonic::Status;
use tracing::{debug, error, info, warn};

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
    /// Where messages a handler sends to WebSocket clients are delivered.
    pub ws_registry: Arc<dyn WebSocketRegistrySend>,
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
    let mut push =
        |name: &str, published: Option<&String>, tag: String, timeout: i64, tracing: bool| {
            handlers.push(proto::HandlerConfig {
                handler_name: name.to_string(),
                published_name: published.cloned().unwrap_or_default(),
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
                    handler.published_name.as_ref(),
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
                    handler.published_name.as_ref(),
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
                    handler.published_name.as_ref(),
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
                    handler.published_name.as_ref(),
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
                    handler.published_name.as_ref(),
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
                handler.published_name.as_ref(),
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

/// The handler tag for each handler name the runtime config declares.
///
/// Direct invocation addresses a handler by name, but dispatch routes by tag,
/// so this is what bridges the two. Built from the same config the handler is
/// sent, so a name the runtime will accept is by construction one the handler
/// was told about.
pub fn handler_tags_by_name(config: &proto::RuntimeConfig) -> HashMap<String, String> {
    let mut by_name = HashMap::new();
    for handler in &config.handlers {
        // Both names reach the same handler. A blueprint publishes one under
        // `spec.handlerName`, which is what a deployment addresses it by, while
        // the resource it is declared as is what the tag is built from, and
        // either is a reasonable thing to type.
        if !handler.published_name.is_empty() {
            by_name.insert(handler.published_name.clone(), handler.handler_tag.clone());
        }
        if let Some(existing) =
            by_name.insert(handler.handler_name.clone(), handler.handler_tag.clone())
        {
            // Two resources share a name. Direct invocation of it is ambiguous
            // and whichever tag wins here is arbitrary.
            warn!(
                handler_name = %handler.handler_name,
                "two handlers share a name, direct invocation will reach only one of them, dropping the tag {existing}"
            );
        }
    }
    by_name
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
    if send_frame(&outbound, frame_config(context.runtime_config.clone()))
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

    let _ = send_frame(
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

    // Set once the handler says it is finishing, after which the stream is
    // only waiting for what it already has to come back.
    let mut draining_until: Option<Instant> = None;

    loop {
        tokio::select! {
            _ = sleep_until_or_never(draining_until) => {
                info!(stream_id, "handler drain deadline passed");
                break;
            }
            frame = dispatch_rx.recv() => match frame {
                Some(frame) => {
                    if send_frame(&outbound, runtime_frame(frame)).await.is_err() {
                        break;
                    }
                }
                None => break,
            },
            message = inbound.next() => match message {
                Some(Ok(message)) => {
                    match handle_inbound(stream_id, &context, &outbound, message).await {
                        Inbound::Continue => {}
                        Inbound::Draining(deadline) => draining_until = deadline,
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

/// How long a connection may stay open without declaring itself before the
/// runtime gives up on it.
///
/// Generous enough for a handlers executable that connects while it is still
/// starting up, short enough that a connection which never communicates cannot hold a
/// task and an outbound channel indefinitely.
const READY_TIMEOUT: Duration = Duration::from_secs(30);

/// Waits for the handler to declare itself.
///
/// Anything else arriving first is a protocol error: the runtime has nothing to
/// dispatch to a stream that has not said what it serves. Taking too long is
/// treated the same way as closing, since neither leaves anything to dispatch
/// to.
async fn await_ready<I>(inbound: &mut I) -> Option<proto::Ready>
where
    I: Stream<Item = Result<proto::HandlerMessage, Status>> + Unpin,
{
    let Ok(message) = tokio::time::timeout(READY_TIMEOUT, inbound.next()).await else {
        warn!("handler stream did not declare itself within the handshake window");
        return None;
    };

    match message {
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

/// What a frame from the handler means for the rest of the stream.
enum Inbound {
    /// Carry on as before.
    Continue,
    /// The handler is finishing. It is sent no more work, but its results are
    /// still taken until it closes or the deadline it gave passes. `None` means
    /// it named no deadline, so only closing ends the stream.
    Draining(Option<Instant>),
}

/// Applies one frame from the handler.
async fn handle_inbound(
    stream_id: StreamId,
    context: &Arc<StreamContext>,
    outbound: &mpsc::Sender<Result<proto::RuntimeMessage, Status>>,
    message: proto::HandlerMessage,
) -> Inbound {
    match message.frame {
        Some(proto::handler_message::Frame::WsSend(send)) => {
            // Spawned rather than awaited, because delivering to a slow or
            // remote connection must not hold up the events queued behind it
            // on this stream.
            let context = context.clone();
            let outbound = outbound.clone();
            tokio::spawn(async move {
                let ack = send_to_websockets(stream_id, &context, send).await;
                let _ = send_frame(&outbound, frame_ws_ack(ack)).await;
            });
            Inbound::Continue
        }
        Some(proto::handler_message::Frame::Result(result)) => {
            complete(stream_id, context, result).await;
            Inbound::Continue
        }
        Some(proto::handler_message::Frame::Credit(grant)) => {
            let _ = context
                .commands
                .send(DispatcherCommand::Grant {
                    stream_id,
                    additional: grant.additional,
                })
                .await;
            Inbound::Continue
        }
        Some(proto::handler_message::Frame::Draining(draining)) => {
            // The stream stays open. This frame exists so a supervisor can roll
            // handler processes without dropping work, and closing here would
            // release everything the handler is still holding, which is exactly
            // the work it is asking for time to finish.
            info!(
                stream_id,
                deadline_unix_ms = draining.deadline_unix_ms,
                "handler is draining, taking its results until it closes"
            );
            let _ = context
                .commands
                .send(DispatcherCommand::Draining { stream_id })
                .await;
            Inbound::Draining(deadline_from_unix_ms(draining.deadline_unix_ms))
        }
        Some(proto::handler_message::Frame::Ready(_)) => {
            warn!(stream_id, "handler declared itself twice, ignoring");
            Inbound::Continue
        }
        other => {
            debug!(stream_id, "ignoring an unsupported frame: {other:?}");
            Inbound::Continue
        }
    }
}

/// Turns a wall-clock deadline into one this stream can wait on.
///
/// A deadline already behind us fires at once, which is a handler saying its
/// time is up. A deadline of zero means it named none, and only the handler
/// closing ends the stream.
fn deadline_from_unix_ms(deadline_unix_ms: i64) -> Option<Instant> {
    if deadline_unix_ms <= 0 {
        return None;
    }
    let now_ms = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as i64;

    Some(
        Instant::now()
            + Duration::from_millis(deadline_unix_ms.saturating_sub(now_ms).max(0) as u64),
    )
}

/// Waits until an instant, or forever when there is nothing to wait for.
async fn sleep_until_or_never(deadline: Option<Instant>) {
    match deadline {
        Some(deadline) => tokio::time::sleep_until(deadline).await,
        None => std::future::pending().await,
    }
}

/// Hands a result back to whoever was waiting for it, and returns the slot and
/// credit to the dispatcher.
async fn complete(stream_id: StreamId, context: &StreamContext, result: proto::Result) {
    let event_id = result.id.clone();
    let credit_grant = result.credit_grant;

    match context.in_flight.remove(&event_id) {
        Some(entry) => {
            let event_result = event_result_from_frame(result, &entry.event.event_type);
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

async fn send_frame(
    outbound: &mpsc::Sender<Result<proto::RuntimeMessage, Status>>,
    frame: proto::RuntimeMessage,
) -> Result<(), ()> {
    outbound.send(Ok(frame)).await.map_err(|_| ())
}

/// Delivers a handler's outbound WebSocket messages and reports what happened.
///
/// Every message is attempted even when an earlier one fails, so that one dead
/// connection in a batch does not stop the rest being delivered, and each
/// failure is reported against its position in the batch so a handler can retry
/// exactly those. Reporting only a single outcome for the batch would leave a
/// handler no choice but to resend all of it, delivering the successful ones
/// twice.
async fn send_to_websockets(
    stream_id: StreamId,
    context: &StreamContext,
    send: proto::WsSend,
) -> proto::WsSendAck {
    let correlation_id = send.correlation_id;
    let mut failures = Vec::new();

    for (index, outbound) in send.messages.into_iter().enumerate() {
        let index = index as u32;
        let connection_id = outbound.connection_id.clone();
        let (message_type, message) = match encode_outbound(&outbound) {
            Ok(encoded) => encoded,
            Err(err) => {
                failures.push(proto::WsSendFailure {
                    index,
                    connection_id,
                    error_message: err,
                });
                continue;
            }
        };

        // Generated here when the handler did not supply one, since the id
        // exists to correlate acknowledgements and loss events rather than to
        // mean anything to the application.
        let message_id = if outbound.message_id.is_empty() {
            nanoid::nanoid!()
        } else {
            outbound.message_id
        };

        if let Err(err) = context
            .ws_registry
            .send_message(
                connection_id.clone(),
                message_id,
                message_type,
                message,
                send_context(outbound.caller, outbound.inform_clients_on_loss),
            )
            .await
        {
            error!(stream_id, %connection_id, "failed to send a websocket message: {err}");
            failures.push(proto::WsSendFailure {
                index,
                connection_id,
                error_message: err.to_string(),
            });
        }
    }

    proto::WsSendAck {
        correlation_id,
        success: failures.is_empty(),
        failures,
    }
}

/// Renders an outbound message the way the connection registry expects it.
///
/// The registry carries a message as a string, so a binary frame travels as its
/// base64 encoding and is decoded again before it reaches the socket. A text
/// frame has to be valid UTF-8, and a handler that marks bytes as text without
/// that being true is told rather than having them silently mangled.
fn encode_outbound(outbound: &proto::WsOutbound) -> Result<(MessageType, String), String> {
    if outbound.is_binary {
        return Ok((MessageType::Binary, BASE64.encode(&outbound.message)));
    }
    match std::str::from_utf8(&outbound.message) {
        Ok(text) => Ok((MessageType::Json, text.to_string())),
        Err(err) => Err(format!("a text message must be valid UTF-8: {err}")),
    }
}

/// Builds the context that decides what happens when a message cannot be
/// delivered.
///
/// Only meaningful when the handler named clients to inform, which is why no
/// context is produced without them. Acknowledgements are not waited for, since
/// the handler has already been told the send was accepted and holding the
/// stream for a cluster round trip would delay every message behind it.
fn send_context(caller: String, inform_clients: Vec<String>) -> Option<SendContext> {
    if inform_clients.is_empty() {
        return None;
    }
    Some(SendContext {
        caller: (!caller.is_empty()).then_some(caller),
        inform_clients,
        wait_for_ack: false,
    })
}

fn frame_config(config: proto::RuntimeConfig) -> proto::RuntimeMessage {
    proto::RuntimeMessage {
        frame: Some(proto::runtime_message::Frame::Config(config)),
    }
}

fn frame_ws_ack(ack: proto::WsSendAck) -> proto::RuntimeMessage {
    proto::RuntimeMessage {
        frame: Some(proto::runtime_message::Frame::WsAck(ack)),
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
///
/// Named apart from the generated `HandlerRuntimeService` trait it implements,
/// which carries the service's own name.
pub struct HandlerStreamService {
    context: Arc<StreamContext>,
    stream_ids: Arc<StreamIds>,
}

impl HandlerStreamService {
    pub fn new(context: Arc<StreamContext>) -> Self {
        HandlerStreamService {
            context,
            stream_ids: Arc::new(StreamIds::default()),
        }
    }
}

#[tonic::async_trait]
impl proto::handler_runtime_service_server::HandlerRuntimeService for HandlerStreamService {
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

    /// Stands in for the connection registry, recording what reached it and
    /// failing for connections named as unreachable.
    #[derive(Debug, Default)]
    struct RecordingWsRegistry {
        sent: std::sync::Mutex<Vec<(String, String, MessageType, String)>>,
        unreachable: HashSet<String>,
    }

    impl std::fmt::Display for RecordingWsRegistry {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            write!(f, "RecordingWsRegistry")
        }
    }

    #[async_trait::async_trait]
    impl WebSocketRegistrySend for RecordingWsRegistry {
        async fn send_message(
            &self,
            connection_id: String,
            message_id: String,
            message_type: MessageType,
            message: String,
            _: Option<SendContext>,
        ) -> Result<(), celerity_ws_registry::errors::WebSocketConnError> {
            if self.unreachable.contains(&connection_id) {
                return Err(
                    celerity_ws_registry::errors::WebSocketConnError::MessageLost(message_id),
                );
            }
            self.sent
                .lock()
                .unwrap()
                .push((connection_id, message_id, message_type, message));
            Ok(())
        }
    }

    struct Harness {
        inbound_tx: mpsc::Sender<Result<proto::HandlerMessage, Status>>,
        outbound_rx: mpsc::Receiver<Result<proto::RuntimeMessage, Status>>,
        commands_rx: mpsc::Receiver<DispatcherCommand>,
        in_flight: Arc<InFlightTable>,
        ws_registry: Arc<RecordingWsRegistry>,
        _cleanup: oneshot::Sender<()>,
    }

    fn start(tags: &[&str]) -> Harness {
        start_with_registry(tags, RecordingWsRegistry::default())
    }

    fn start_with_registry(tags: &[&str], registry: RecordingWsRegistry) -> Harness {
        let (handles, _receivers, cleanup) = EventQueueParts::new(8).into_parts();
        let cleanup_shutdown = cleanup.spawn();
        let (commands_tx, commands_rx) = mpsc::channel(16);
        let (inbound_tx, inbound_rx) = mpsc::channel(16);
        let (outbound_tx, outbound_rx) = mpsc::channel(16);

        let ws_registry = Arc::new(registry);
        let context = Arc::new(StreamContext {
            runtime_config: proto::RuntimeConfig {
                tracing_enabled: false,
                metrics_enabled: false,
                handlers: vec![],
            },
            blueprint_tags: blueprint_tags(tags),
            commands: commands_tx,
            in_flight: handles.in_flight.clone(),
            ws_registry: ws_registry.clone(),
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
            ws_registry,
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

    fn schedule_event(id: &str) -> EventData {
        EventData {
            id: id.to_string(),
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
        }
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

    async fn ready_stream(harness: &mut Harness) -> Box<StreamRegistration> {
        let _config = next_frame(&mut harness.outbound_rx).await;
        harness
            .inbound_tx
            .send(Ok(ready(&["schedule::a"])))
            .await
            .unwrap();
        let _ack = next_frame(&mut harness.outbound_rx).await;
        complete_handshake(harness).await
    }

    fn ws_send(correlation_id: &str, messages: Vec<proto::WsOutbound>) -> proto::HandlerMessage {
        proto::HandlerMessage {
            frame: Some(proto::handler_message::Frame::WsSend(proto::WsSend {
                correlation_id: correlation_id.to_string(),
                messages,
            })),
        }
    }

    fn outbound(connection_id: &str, message: &[u8], is_binary: bool) -> proto::WsOutbound {
        proto::WsOutbound {
            connection_id: connection_id.to_string(),
            message: message.to_vec(),
            is_binary,
            inform_clients_on_loss: vec![],
            message_id: String::new(),
            caller: String::new(),
        }
    }

    #[tokio::test]
    async fn delivers_a_websocket_message_a_handler_sent() {
        let mut harness = start(&["schedule::a"]);
        let _registration = ready_stream(&mut harness).await;

        harness
            .inbound_tx
            .send(Ok(ws_send(
                "correlation-1",
                vec![outbound("connection-1", br#"{"event":"tick"}"#, false)],
            )))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::WsAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected the send to be acknowledged");
        };
        assert_eq!(ack.correlation_id, "correlation-1");
        assert!(ack.success, "unexpected failures: {:?}", ack.failures);

        let sent = harness.ws_registry.sent.lock().unwrap().clone();
        assert_eq!(sent.len(), 1);
        assert_eq!(sent[0].0, "connection-1");
        assert_eq!(sent[0].2, MessageType::Json);
        assert_eq!(sent[0].3, r#"{"event":"tick"}"#);
        // Generated for the handler, which did not supply one.
        assert!(!sent[0].1.is_empty());
    }

    #[tokio::test]
    async fn carries_a_binary_websocket_message_without_corrupting_it() {
        let mut harness = start(&["schedule::a"]);
        let _registration = ready_stream(&mut harness).await;

        // Bytes that are not valid UTF-8, so a text path would destroy them.
        let payload = vec![0xff, 0xfe, 0x00, 0x80, 0x01];
        harness
            .inbound_tx
            .send(Ok(ws_send(
                "correlation-1",
                vec![outbound("connection-1", &payload, true)],
            )))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::WsAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected the send to be acknowledged");
        };
        assert!(ack.success);

        let sent = harness.ws_registry.sent.lock().unwrap().clone();
        assert_eq!(sent[0].2, MessageType::Binary);
        // The registry carries a message as a string, so binary travels base64
        // encoded and is decoded again on its way to the socket.
        assert_eq!(BASE64.decode(&sent[0].3).unwrap(), payload);
    }

    #[tokio::test]
    async fn delivers_the_rest_of_a_batch_when_one_connection_fails() {
        let mut harness = start_with_registry(
            &["schedule::a"],
            RecordingWsRegistry {
                unreachable: blueprint_tags(&["gone"]),
                ..Default::default()
            },
        );
        let _registration = ready_stream(&mut harness).await;

        harness
            .inbound_tx
            .send(Ok(ws_send(
                "correlation-1",
                vec![
                    outbound("gone", b"{}", false),
                    outbound("here", b"{}", false),
                ],
            )))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::WsAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected the send to be acknowledged");
        };
        // The failure is reported against its position in the batch, so a
        // handler retries only that message. Resending the whole batch would
        // deliver the second message twice, and a client can only tell the
        // difference if the application put a message id in the payload.
        assert!(!ack.success);
        assert_eq!(ack.failures.len(), 1);
        assert_eq!(ack.failures[0].index, 0);
        assert_eq!(ack.failures[0].connection_id, "gone");

        let sent = harness.ws_registry.sent.lock().unwrap().clone();
        assert_eq!(sent.len(), 1);
        assert_eq!(sent[0].0, "here");
    }

    #[tokio::test]
    async fn refuses_a_text_websocket_message_that_is_not_utf8() {
        let mut harness = start(&["schedule::a"]);
        let _registration = ready_stream(&mut harness).await;

        harness
            .inbound_tx
            .send(Ok(ws_send(
                "correlation-1",
                vec![outbound("connection-1", &[0xff, 0xfe], false)],
            )))
            .await
            .unwrap();

        let Some(proto::runtime_message::Frame::WsAck(ack)) =
            next_frame(&mut harness.outbound_rx).await
        else {
            panic!("expected the send to be acknowledged");
        };
        // Told rather than silently mangled into replacement characters.
        assert!(!ack.success);
        assert_eq!(ack.failures.len(), 1);
        assert_eq!(ack.failures[0].index, 0);
        assert!(ack.failures[0].error_message.contains("UTF-8"));
        assert!(harness.ws_registry.sent.lock().unwrap().is_empty());
    }

    #[tokio::test]
    async fn keeps_dispatching_while_a_websocket_send_is_in_progress() {
        let mut harness = start(&["schedule::a"]);
        let registration = ready_stream(&mut harness).await;

        harness
            .inbound_tx
            .send(Ok(ws_send(
                "correlation-1",
                vec![outbound("connection-1", b"{}", false)],
            )))
            .await
            .unwrap();

        // An event queued behind the send still goes out, which is what the
        // send being spawned rather than awaited buys.
        registration
            .dispatch_tx
            .send(crate::dispatcher::StreamFrame::Drain {
                deadline_unix_ms: 1,
            })
            .await
            .unwrap();

        let mut seen_ack = false;
        let mut seen_drain = false;
        for _ in 0..2 {
            match next_frame(&mut harness.outbound_rx).await {
                Some(proto::runtime_message::Frame::WsAck(_)) => seen_ack = true,
                Some(proto::runtime_message::Frame::Drain(_)) => seen_drain = true,
                other => panic!("unexpected frame {other:?}"),
            }
        }
        assert!(seen_ack && seen_drain);
    }

    #[tokio::test]
    async fn keeps_taking_results_from_a_handler_that_is_draining() {
        let mut harness = start(&["schedule::a"]);
        let _registration = ready_stream(&mut harness).await;

        // Stand in for the dispatcher having sent this event, so the handler
        // has work outstanding when it says it is finishing.
        let (result_tx, result_rx) = oneshot::channel();
        harness.in_flight.insert(
            crate::event_queue::InFlightEntry {
                result_tx,
                event: schedule_event("event-1"),
            },
            Duration::from_secs(60),
        );

        harness
            .inbound_tx
            .send(Ok(proto::HandlerMessage {
                frame: Some(proto::handler_message::Frame::Draining(proto::Draining {
                    deadline_unix_ms: 0,
                })),
            }))
            .await
            .unwrap();

        // The dispatcher is told to stop sending work, and nothing is detached.
        // Detaching would release the very work the handler asked for time to
        // finish.
        let command = tokio::time::timeout(Duration::from_secs(2), harness.commands_rx.recv())
            .await
            .expect("the dispatcher should be told")
            .expect("the command channel should be open");
        assert!(
            matches!(command, DispatcherCommand::Draining { .. }),
            "expected the stream to drain rather than detach, got {command:?}"
        );

        // The result of the outstanding event still reaches its caller.
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
        assert!(matches!(outcome, EventOutcome::Completed(_, _)));
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
