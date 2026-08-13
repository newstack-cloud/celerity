//! The bounded event queue and in-flight event tracking used by the
//! IPC runtime call mode.
//!
//! Producers (HTTP routes, WebSocket routing, consumers and schedules) push an
//! event onto a bounded channel and await a result on a oneshot channel. The
//! handlers executable takes events off the channel, at which point the event
//! moves into the in-flight table until its result comes back.
//!
//! Two independent mechanisms bound how long that can take:
//!
//! - The producer applies its own timeout while awaiting the oneshot, so a
//!   caller is never blocked indefinitely by a handler that never responds.
//! - The cleanup task removes in-flight entries whose deadline has passed. This is
//!   the only thing that can release an entry the handlers executable has taken
//!   but will never return a result for, such as when the process dies.

use std::{
    collections::HashMap,
    future::poll_fn,
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::Duration,
};

use tokio::{
    sync::{mpsc, oneshot},
    time::Instant,
};
use tokio_util::time::{delay_queue, DelayQueue};
use tracing::{debug, info, warn};

use crate::{
    config::{AppConfig, EventConfig},
    consts::{
        DEFAULT_HANDLER_TIMEOUT, EVENT_QUEUE_ADMISSION_WAIT_DIVISOR,
        MAX_EVENT_QUEUE_ADMISSION_WAIT_SECS,
    },
    types::{CancelReason, CancelRequest, EventData, EventOutcome, EventTuple},
};

/// The reason an event could not be handed to the handlers executable.
#[derive(Debug, PartialEq)]
pub enum EventQueueError {
    /// The queue is full; the handlers executable is not keeping up.
    /// Producers should manage load rather than wait indefinitely.
    QueueFull,
    /// The receiving end of the queue has gone away, meaning the runtime
    /// is shutting down or the local API was never started.
    Closed,
}

impl std::fmt::Display for EventQueueError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            EventQueueError::QueueFull => write!(f, "event queue is full"),
            EventQueueError::Closed => write!(f, "event queue is closed"),
        }
    }
}

impl std::error::Error for EventQueueError {}

/// The producer half of the bounded event queue.
///
/// Cloning is cheap and yields another producer for the same queue.
#[derive(Debug, Clone)]
pub struct EventQueue {
    tx: mpsc::Sender<EventTuple>,
    cancel_tx: mpsc::UnboundedSender<CancelRequest>,
}

impl EventQueue {
    /// Pushes an event onto the queue and returns the receiver that will yield
    /// the result once a handler has processed it.
    ///
    /// When the queue is at capacity this waits up to `admission_wait` for room
    /// rather than rejecting immediately, so that a burst is absorbed instead of
    /// being pushed back to the event's source. Only sustained overload, where
    /// no room appears within that window, returns
    /// [`EventQueueError::QueueFull`].
    ///
    /// The wait is deliberately bounded and comes out of the event's own time
    /// budget (see [`admission_wait`]): time spent queueing is time the handler
    /// no longer has, so waiting indefinitely would just produce events that
    /// are certain to time out.
    pub async fn enqueue(
        &self,
        event: EventData,
        admission_wait: Duration,
    ) -> Result<oneshot::Receiver<EventOutcome>, EventQueueError> {
        let (tx, rx) = oneshot::channel();
        self.tx
            .send_timeout((tx, event), admission_wait)
            .await
            .map_err(|err| match err {
                mpsc::error::SendTimeoutError::Timeout(_) => EventQueueError::QueueFull,
                mpsc::error::SendTimeoutError::Closed(_) => EventQueueError::Closed,
            })
            .map(|()| rx)
    }

    /// Guards an event so that a caller which goes away mid-request tells the
    /// handler to stop.
    ///
    /// A dropped future is the only way a client disconnect reaches this code,
    /// so the cancellation has to hang off `Drop` rather than off a branch a
    /// caller could return through. Every path that stops waiting for a reason
    /// the runtime already knows about, a result, a closed channel or the
    /// deadline passing, should [`disarm`](CancelOnDrop::disarm) the guard
    /// instead.
    ///
    /// Only the HTTP path uses this. A WebSocket message is closer to a queue
    /// message than to a request and response, so a closing connection does not
    /// make the work pointless, and the connection loop awaits its handlers
    /// inline rather than dropping them.
    pub fn cancel_on_drop(&self, event_id: String) -> CancelOnDrop {
        CancelOnDrop {
            event_id: Some(event_id),
            cancel_tx: self.cancel_tx.clone(),
        }
    }
}

/// Cancels an event if it is dropped before being disarmed.
///
/// See [`EventQueue::cancel_on_drop`].
pub struct CancelOnDrop {
    /// Taken when the guard is disarmed, so that `Drop` can tell the two apart
    /// without a second field.
    event_id: Option<String>,
    cancel_tx: mpsc::UnboundedSender<CancelRequest>,
}

impl CancelOnDrop {
    /// Stops the guard cancelling, for a caller that stopped waiting for a
    /// reason the runtime already knows about.
    pub fn disarm(mut self) {
        self.event_id = None;
    }
}

impl Drop for CancelOnDrop {
    fn drop(&mut self) {
        let Some(event_id) = self.event_id.take() else {
            return;
        };
        // Unbounded because this runs in `Drop`, which cannot wait, and because
        // dropping a cancellation leaves a handler working on something nobody
        // wants. A failed send means the dispatcher has already stopped.
        let _ = self.cancel_tx.send(CancelRequest {
            event_id,
            reason: CancelReason::CallerGone,
        });
    }
}

/// How long a producer should wait for queue capacity before shedding an event.
///
/// Taken as a fraction of the event's own timeout, so that a short-deadline
/// event is not held waiting for a slot it could never use, and capped so that
/// a producer with a long timeout does not stall its own loop under sustained
/// overload, a consumer blocked here is not polling its source.
pub fn admission_wait(timeout: Duration) -> Duration {
    (timeout / EVENT_QUEUE_ADMISSION_WAIT_DIVISOR)
        .min(Duration::from_secs(MAX_EVENT_QUEUE_ADMISSION_WAIT_SECS))
}

/// The tag identifying an HTTP handler, as it appears on dispatched events.
pub fn http_handler_tag(method: &str, path: &str) -> String {
    format!("{method}::{path}")
}

/// The tag identifying a WebSocket message handler.
pub fn websocket_handler_tag(route_key: &str, route: &str) -> String {
    format!("{route_key}::{route}")
}

/// The tag identifying a consumer or schedule handler, keyed by its source.
pub fn source_handler_tag(source_id: &str, handler_name: &str) -> String {
    format!("source::{source_id}::{handler_name}")
}

/// The tag identifying a custom handler.
pub fn custom_handler_tag(handler_name: &str) -> String {
    format!("custom::{handler_name}")
}

/// Builds the handler tag to timeout mapping for an application.
///
/// Timeouts are resolved per handler when the blueprint is transformed, and are
/// keyed here by the same tags that events carry, so that the timeout applied
/// to an event is the one configured for the handler that will run it.
pub fn collect_handler_timeouts(app_config: &AppConfig) -> HandlerTimeouts {
    let mut by_tag = HashMap::new();

    if let Some(api) = &app_config.api {
        if let Some(http) = &api.http {
            for handler in &http.handlers {
                by_tag.insert(
                    http_handler_tag(&handler.method, &handler.path),
                    timeout_from_seconds(handler.timeout),
                );
            }
        }
        if let Some(websocket) = &api.websocket {
            for handler in &websocket.handlers {
                by_tag.insert(
                    websocket_handler_tag(&handler.route_key, &handler.route),
                    timeout_from_seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(consumers) = &app_config.consumers {
        for consumer in &consumers.consumers {
            for handler in &consumer.handlers {
                by_tag.insert(
                    source_handler_tag(&consumer.source_id, &handler.name),
                    timeout_from_seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(schedules) = &app_config.schedules {
        for schedule in &schedules.schedules {
            for handler in &schedule.handlers {
                by_tag.insert(
                    source_handler_tag(&schedule.schedule_id, &handler.name),
                    timeout_from_seconds(handler.timeout),
                );
            }
        }
    }

    // Event handlers are dispatched with a source tag built from the stream or
    // queue they are fed by, so they are keyed the same way here.
    if let Some(events) = &app_config.events {
        for event in &events.events {
            let (source_id, handlers) = match event {
                EventConfig::Stream(config) => (&config.stream_id, &config.handlers),
                EventConfig::EventTrigger(config) => (&config.queue_id, &config.handlers),
            };
            for handler in handlers {
                by_tag.insert(
                    source_handler_tag(source_id, &handler.name),
                    timeout_from_seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(custom) = &app_config.custom_handlers {
        for handler in &custom.handlers {
            by_tag.insert(
                custom_handler_tag(&handler.name),
                timeout_from_seconds(handler.timeout),
            );
        }
    }

    HandlerTimeouts::new(by_tag, timeout_from_seconds(DEFAULT_HANDLER_TIMEOUT))
}

/// Converts a configured timeout in seconds to a duration, treating a
/// non-positive value as "unset" and falling back to the default.
pub fn timeout_from_seconds(timeout: i64) -> Duration {
    if timeout <= 0 {
        Duration::from_secs(DEFAULT_HANDLER_TIMEOUT as u64)
    } else {
        Duration::from_secs(timeout as u64)
    }
}

/// Resolves the timeout that applies to an event from its handler tag.
///
/// Timeouts are resolved per handler when the blueprint is transformed into
/// application config, so this carries those values to the point where an
/// event is enqueued, where only the tag is known.
#[derive(Debug, Clone)]
pub struct HandlerTimeouts {
    by_tag: Arc<HashMap<String, Duration>>,
    fallback: Duration,
}

impl HandlerTimeouts {
    pub fn new(by_tag: HashMap<String, Duration>, fallback: Duration) -> Self {
        HandlerTimeouts {
            by_tag: Arc::new(by_tag),
            fallback,
        }
    }

    /// The timeout for a handler tag, falling back to the default when the tag
    /// has no configured timeout.
    ///
    /// An unknown tag is not an error: consumer and schedule tags are derived
    /// at runtime from source and handler names, so a tag can legitimately have
    /// no entry.
    pub fn for_tag(&self, handler_tag: &str) -> Duration {
        self.by_tag
            .get(handler_tag)
            .copied()
            .unwrap_or(self.fallback)
    }

    /// The longest any configured handler may run for.
    ///
    /// Used to decide how long a shutdown should wait for in-flight work: a
    /// handler that was told it had ten minutes should not be abandoned after
    /// thirty seconds. Falls back to the default timeout when no handler
    /// configures one, which covers an application with no handlers at all.
    pub fn longest(&self) -> Duration {
        self.by_tag.values().copied().max().unwrap_or(self.fallback)
    }
}

/// Tells the cleanup task to start or stop watching an event's deadline.
enum DeadlineMessage {
    Watch { event_id: String, deadline: Instant },
    Unwatch { event_id: String },
}

/// An event that has been taken by the handlers executable and is awaiting
/// a result.
#[derive(Debug)]
pub struct InFlightEntry {
    pub result_tx: oneshot::Sender<EventOutcome>,
    pub event: EventData,
}

/// Tracks events that are being processed by the handlers executable.
///
/// Entries are removed either by a matching result arriving, or by the cleanup task
/// once their deadline passes. Without the latter, an entry whose handler never
/// responds would be held for the lifetime of the process.
#[derive(Debug)]
pub struct InFlightTable {
    entries: Mutex<HashMap<String, InFlightEntry>>,
    deadline_tx: mpsc::UnboundedSender<DeadlineMessage>,
    /// How many deadlines the cleanup task is currently watching. Kept in step
    /// by the task itself, and useful both as an operational signal and to
    /// confirm that completed events stop being watched.
    watched_deadlines: Arc<AtomicUsize>,
}

impl InFlightTable {
    /// Records an event as in-flight and has the cleanup task watch its deadline.
    ///
    /// The deadline is resolved to an absolute instant here rather than being
    /// sent as a duration, so that time spent waiting for the cleanup task to pick
    /// the message up does not extend the deadline.
    pub fn insert(&self, entry: InFlightEntry, timeout: Duration) {
        let event_id = entry.event.id.clone();
        let deadline = Instant::now() + timeout;
        self.entries
            .lock()
            .expect("in-flight table lock should not be poisoned")
            .insert(event_id.clone(), entry);

        // A send failure means the cleanup task has stopped, which happens only
        // during shutdown. The entry is still tracked and will be dropped with
        // the table, so this is not worth failing the event over.
        if self
            .deadline_tx
            .send(DeadlineMessage::Watch { event_id, deadline })
            .is_err()
        {
            debug!("expired event cleanup task is not running, deadline will not be enforced");
        }
    }

    /// Removes an event from the table, returning its entry if it was present.
    ///
    /// A missing entry is expected rather than exceptional: the cleanup task may
    /// have already removed it, or a result may arrive twice.
    pub fn remove(&self, event_id: &str) -> Option<InFlightEntry> {
        let entry = self.take(event_id);
        if entry.is_some() {
            // Stop watching a deadline that can no longer be missed. Without
            // this the cleanup task holds an entry per completed event until
            // its deadline elapses, which at any real rate is a large amount
            // of memory spent watching events that already finished.
            let _ = self.deadline_tx.send(DeadlineMessage::Unwatch {
                event_id: event_id.to_string(),
            });
        }
        entry
    }

    /// Removes an entry without stopping the deadline watch, for the cleanup task,
    /// which has already taken the deadline out of its queue.
    fn take(&self, event_id: &str) -> Option<InFlightEntry> {
        self.entries
            .lock()
            .expect("in-flight table lock should not be poisoned")
            .remove(event_id)
    }

    /// How many deadlines are currently being watched.
    pub fn watched_deadlines(&self) -> usize {
        self.watched_deadlines.load(Ordering::Relaxed)
    }

    /// The number of events currently awaiting a result.
    pub fn len(&self) -> usize {
        self.entries
            .lock()
            .expect("in-flight table lock should not be poisoned")
            .len()
    }

    /// Whether any events are currently awaiting a result.
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

/// The receiving half of the bounded event queue.
///
/// The queue has exactly one consumer. Sharing it behind a mutex is a
/// transitional measure for the polling local runtime API, whose endpoint can
/// be called concurrently and which is currently the only thing draining the
/// queue. That API is being replaced by a gRPC bidirectional stream, and the
/// dispatcher that serves it will own the receiver outright — at which point
/// the mutex has no remaining purpose and this alias should collapse to a
/// plain `mpsc::Receiver`.
///
/// The local API is removed last, after the stream can carry consumer and
/// schedule events, so this shape has to survive until then.
pub type EventQueueReceiver = Arc<tokio::sync::Mutex<mpsc::Receiver<EventTuple>>>;

/// The receiving half of the cancellation channel.
///
/// Wrapped the same way as [`EventQueueReceiver`] only so that the handles can
/// stay cloneable while the local API still exists; the dispatcher is the sole
/// consumer, and this collapses to a plain receiver with the rest of that
/// transitional shape.
pub type CancelReceiver = Arc<tokio::sync::Mutex<mpsc::UnboundedReceiver<CancelRequest>>>;

/// Everything needed to run the event path in the IPC runtime call mode.
pub struct EventQueueParts {
    handles: EventQueueHandles,
    cleanup_task: EventCleanupTask,
}

impl EventQueueParts {
    /// Creates the bounded queue, the in-flight table and the channel the
    /// cleanup task watches deadlines through.
    ///
    /// Nothing is spawned here, so this is safe to call outside a tokio
    /// runtime. Starting the cleanup task is a separate step.
    pub fn new(capacity: usize) -> Self {
        let (tx, rx) = mpsc::channel(capacity);
        let (deadline_tx, deadline_rx) = mpsc::unbounded_channel();
        let (cancel_tx, cancel_rx) = mpsc::unbounded_channel();
        let watched_deadlines = Arc::new(AtomicUsize::new(0));
        let in_flight = Arc::new(InFlightTable {
            entries: Mutex::new(HashMap::new()),
            deadline_tx,
            watched_deadlines: watched_deadlines.clone(),
        });

        EventQueueParts {
            handles: EventQueueHandles {
                queue: EventQueue {
                    tx,
                    cancel_tx: cancel_tx.clone(),
                },
                receiver: Arc::new(tokio::sync::Mutex::new(rx)),
                cancellations: Arc::new(tokio::sync::Mutex::new(cancel_rx)),
                in_flight: in_flight.clone(),
            },
            cleanup_task: EventCleanupTask {
                in_flight,
                deadline_rx,
                cancel_tx,
                watched_deadlines,
            },
        }
    }

    /// Separates the handles the rest of the runtime uses from the cleanup task
    /// that has yet to be started.
    ///
    /// The two have different lifetimes, the handles are needed while the
    /// application is being set up, because every HTTP route in this mode holds
    /// a producer, while the task can only be started once there is a runtime
    /// to spawn it on.
    pub fn into_parts(self) -> (EventQueueHandles, EventCleanupTask) {
        (self.handles, self.cleanup_task)
    }
}

/// The cleanup task, created but not yet running.
pub struct EventCleanupTask {
    in_flight: Arc<InFlightTable>,
    deadline_rx: mpsc::UnboundedReceiver<DeadlineMessage>,
    cancel_tx: mpsc::UnboundedSender<CancelRequest>,
    watched_deadlines: Arc<AtomicUsize>,
}

impl EventCleanupTask {
    /// Starts the task, returning the sender that stops it.
    ///
    /// Must be called from within a tokio runtime.
    pub fn spawn(self) -> oneshot::Sender<()> {
        let (shutdown_tx, shutdown_rx) = oneshot::channel();
        spawn_expired_event_cleanup_task(
            self.in_flight,
            self.deadline_rx,
            self.cancel_tx,
            self.watched_deadlines,
            shutdown_rx,
        );
        shutdown_tx
    }
}

/// The parts of the event path that outlive setup.
#[derive(Debug, Clone)]
pub struct EventQueueHandles {
    pub queue: EventQueue,
    pub receiver: EventQueueReceiver,
    pub cancellations: CancelReceiver,
    pub in_flight: Arc<InFlightTable>,
}

/// Removes in-flight entries whose deadline has passed.
///
/// Dropping an entry drops its oneshot sender, which wakes the producer with a
/// closed-channel error. Producers apply their own timeout as well, so the
/// cleanup task's job is to stop the table growing without bound rather than to
/// deliver the timeout to the caller.
fn spawn_expired_event_cleanup_task(
    in_flight: Arc<InFlightTable>,
    mut deadline_rx: mpsc::UnboundedReceiver<DeadlineMessage>,
    cancel_tx: mpsc::UnboundedSender<CancelRequest>,
    watched_deadlines: Arc<AtomicUsize>,
    mut shutdown_rx: oneshot::Receiver<()>,
) {
    tokio::spawn(async move {
        let mut deadlines: DelayQueue<String> = DelayQueue::new();
        // The queue key for each event being watched, so that an event which
        // completes can have its deadline taken back out. The keys never leave
        // this task, which is why unwatching is a message rather than something
        // the in-flight table does directly.
        let mut watched: HashMap<String, delay_queue::Key> = HashMap::new();
        // Holds a message between the select that received it and the top of
        // the loop, so that no select branch borrows `deadlines` while another
        // is polling it.
        let mut pending: Option<DeadlineMessage> = None;

        loop {
            match pending.take() {
                Some(DeadlineMessage::Watch { event_id, deadline }) => {
                    let key = deadlines.insert_at(event_id.clone(), deadline);
                    watched.insert(event_id, key);
                    watched_deadlines.store(watched.len(), Ordering::Relaxed);
                }
                Some(DeadlineMessage::Unwatch { event_id }) => {
                    if let Some(key) = watched.remove(&event_id) {
                        deadlines.remove(&key);
                        watched_deadlines.store(watched.len(), Ordering::Relaxed);
                    }
                }
                None => {}
            }

            if deadlines.is_empty() {
                tokio::select! {
                    _ = &mut shutdown_rx => break,
                    message = deadline_rx.recv() => match message {
                        Some(message) => pending = Some(message),
                        None => break,
                    },
                }
                continue;
            }

            let expired = tokio::select! {
                _ = &mut shutdown_rx => break,
                message = deadline_rx.recv() => {
                    match message {
                        Some(message) => pending = Some(message),
                        None => break,
                    }
                    continue;
                }
                expired = poll_fn(|cx| deadlines.poll_expired(cx)) => expired,
            };

            let Some(expired) = expired else {
                continue;
            };
            let event_id = expired.into_inner();
            watched.remove(&event_id);
            watched_deadlines.store(watched.len(), Ordering::Relaxed);

            // `take` rather than `remove`, since the deadline has already left
            // the queue and the watch is already gone.
            if let Some(entry) = in_flight.take(&event_id) {
                warn!(
                    event_id = %event_id,
                    handler_tag = %entry.event.handler_tag,
                    "event deadline exceeded before a result was returned, \
                     removing it from the in-flight table"
                );
                // Nobody is waiting for this any more, so tell whichever handler
                // holds it to stop rather than leaving it to finish work whose
                // result will be discarded.
                let _ = cancel_tx.send(CancelRequest {
                    event_id,
                    reason: CancelReason::DeadlineExceeded,
                });
            }
        }

        info!("received shutdown signal, stopping expired event cleanup task");
    });
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use serde_json::json;

    use super::*;
    use crate::types::{EventDataPayload, EventType, ScheduleEventData};

    fn test_event(id: &str) -> EventData {
        EventData {
            id: id.to_string(),
            event_type: EventType::ScheduleMessage,
            handler_tag: "schedule::test".to_string(),
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

    #[tokio::test]
    async fn enqueue_yields_the_event_to_the_receiver() {
        let (handles, _cleanup) = EventQueueParts::new(4).into_parts();
        let receiver = handles.receiver.clone();
        let queue = handles.queue.clone();

        let _rx = queue
            .enqueue(test_event("event-1"), Duration::from_secs(1))
            .await
            .unwrap();

        let (_result_tx, event) = receiver.lock().await.recv().await.unwrap();
        assert_eq!(event.id, "event-1");
    }

    #[tokio::test(start_paused = true)]
    async fn enqueue_reports_queue_full_when_no_room_appears_within_the_admission_wait() {
        let (handles, _cleanup) = EventQueueParts::new(1).into_parts();

        handles
            .queue
            .enqueue(test_event("event-1"), Duration::from_secs(1))
            .await
            .unwrap();
        let result = handles
            .queue
            .enqueue(test_event("event-2"), Duration::from_secs(1))
            .await;

        assert_eq!(result.err(), Some(EventQueueError::QueueFull));
    }

    #[tokio::test]
    async fn enqueue_waits_for_capacity_rather_than_shedding_immediately() {
        let (handles, _cleanup) = EventQueueParts::new(1).into_parts();
        let receiver = handles.receiver.clone();

        handles
            .queue
            .enqueue(test_event("event-1"), Duration::from_secs(1))
            .await
            .unwrap();

        // Free a slot shortly after the second enqueue starts waiting. Without
        // the admission wait this would have been shed on the spot.
        let drain = tokio::spawn(async move {
            tokio::time::sleep(Duration::from_millis(20)).await;
            receiver.lock().await.recv().await
        });

        let result = handles
            .queue
            .enqueue(test_event("event-2"), Duration::from_secs(5))
            .await;

        assert!(result.is_ok());
        assert!(drain.await.unwrap().is_some());
    }

    #[test]
    fn collects_timeouts_for_event_handlers_from_both_source_kinds() {
        use crate::config::{
            EventConfig, EventHandlerDefinition, EventTriggerConfig, EventsConfig, StreamConfig,
            StreamSourceType,
        };

        fn handler(name: &str, timeout: i64) -> EventHandlerDefinition {
            EventHandlerDefinition {
                name: name.to_string(),
                location: String::new(),
                handler: String::new(),
                timeout,
                tracing_enabled: false,
                route: None,
            }
        }

        let app_config = AppConfig {
            api: None,
            consumers: None,
            schedules: None,
            events: Some(EventsConfig {
                events: vec![
                    EventConfig::Stream(StreamConfig {
                        consumer_name: "orders-consumer".to_string(),
                        source_type: StreamSourceType::DataStream,
                        stream_id: "orders-stream".to_string(),
                        batch_size: None,
                        partial_failures: None,
                        start_from_beginning: None,
                        handlers: vec![handler("ProcessOrders", 45)],
                    }),
                    EventConfig::EventTrigger(EventTriggerConfig {
                        consumer_name: "uploads-consumer".to_string(),
                        event_type: "created".to_string(),
                        queue_id: "uploads-queue".to_string(),
                        batch_size: None,
                        visibility_timeout: None,
                        wait_time_seconds: None,
                        partial_failures: None,
                        handlers: vec![handler("ProcessUpload", 120)],
                    }),
                ],
            }),
            custom_handlers: None,
        };

        let timeouts = collect_handler_timeouts(&app_config);

        assert_eq!(
            timeouts.for_tag("source::orders-stream::ProcessOrders"),
            Duration::from_secs(45)
        );
        assert_eq!(
            timeouts.for_tag("source::uploads-queue::ProcessUpload"),
            Duration::from_secs(120)
        );
        // An unknown tag still falls back to the default.
        assert_eq!(
            timeouts.for_tag("source::unknown::Handler"),
            Duration::from_secs(DEFAULT_HANDLER_TIMEOUT as u64)
        );
    }

    #[test]
    fn admission_wait_is_a_fraction_of_the_timeout_and_capped() {
        // A short deadline gets a proportionally short wait, so an event is not
        // held waiting for a slot it could never use.
        assert_eq!(
            admission_wait(Duration::from_secs(4)),
            Duration::from_secs(1)
        );
        // A long deadline is capped so a producer is not held off its own loop.
        assert_eq!(
            admission_wait(Duration::from_secs(600)),
            Duration::from_secs(MAX_EVENT_QUEUE_ADMISSION_WAIT_SECS)
        );
    }

    #[tokio::test]
    async fn enqueue_reports_closed_when_the_receiver_is_dropped() {
        let (handles, cleanup) = EventQueueParts::new(4).into_parts();
        let queue = handles.queue.clone();
        drop(handles);
        drop(cleanup);

        assert_eq!(
            queue
                .enqueue(test_event("event-1"), Duration::from_secs(1))
                .await
                .err(),
            Some(EventQueueError::Closed)
        );
    }

    #[tokio::test]
    async fn in_flight_entries_are_removed_by_a_matching_result() {
        let (handles, cleanup) = EventQueueParts::new(4).into_parts();
        let _shutdown = cleanup.spawn();

        let (result_tx, _result_rx) = oneshot::channel();
        handles.in_flight.insert(
            InFlightEntry {
                result_tx,
                event: test_event("event-1"),
            },
            Duration::from_secs(60),
        );
        assert_eq!(handles.in_flight.len(), 1);

        assert!(handles.in_flight.remove("event-1").is_some());
        assert!(handles.in_flight.is_empty());
    }

    #[tokio::test]
    async fn a_completed_event_stops_being_watched() {
        let (handles, cleanup) = EventQueueParts::new(4).into_parts();
        let _shutdown = cleanup.spawn();

        let (result_tx, _result_rx) = oneshot::channel();
        handles.in_flight.insert(
            InFlightEntry {
                result_tx,
                event: test_event("event-1"),
            },
            // Long enough that the deadline could not have fired on its own.
            Duration::from_secs(3600),
        );
        wait_for(|| handles.in_flight.watched_deadlines() == 1).await;

        // A result arriving takes the deadline back out, rather than leaving it
        // to occupy the queue until it elapses.
        assert!(handles.in_flight.remove("event-1").is_some());
        wait_for(|| handles.in_flight.watched_deadlines() == 0).await;
    }

    /// Waits for the cleanup task to catch up with a message sent to it.
    async fn wait_for(condition: impl Fn() -> bool) {
        for _ in 0..100 {
            if condition() {
                return;
            }
            tokio::task::yield_now().await;
        }
        panic!("the cleanup task did not reach the expected state");
    }

    #[tokio::test(start_paused = true)]
    async fn cleanup_removes_entries_whose_deadline_has_passed() {
        let (handles, cleanup) = EventQueueParts::new(4).into_parts();
        let _shutdown = cleanup.spawn();

        let (result_tx, mut result_rx) = oneshot::channel();
        handles.in_flight.insert(
            InFlightEntry {
                result_tx,
                event: test_event("event-1"),
            },
            Duration::from_millis(50),
        );
        assert_eq!(handles.in_flight.len(), 1);

        tokio::time::sleep(Duration::from_millis(100)).await;

        assert!(handles.in_flight.is_empty());
        // Dropping the entry drops the sender, so a producer still awaiting
        // the result observes a closed channel.
        assert!(result_rx.try_recv().is_err());
    }

    #[tokio::test(start_paused = true)]
    async fn cleanup_leaves_entries_that_are_still_within_their_deadline() {
        let (handles, cleanup) = EventQueueParts::new(4).into_parts();
        let _shutdown = cleanup.spawn();

        let (result_tx, _result_rx) = oneshot::channel();
        handles.in_flight.insert(
            InFlightEntry {
                result_tx,
                event: test_event("event-1"),
            },
            Duration::from_secs(60),
        );

        tokio::time::sleep(Duration::from_secs(1)).await;

        assert_eq!(handles.in_flight.len(), 1);
    }
}
