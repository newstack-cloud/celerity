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
    sync::{Arc, Mutex},
    time::Duration,
};

use tokio::{
    sync::{mpsc, oneshot},
    task::JoinHandle,
    time::Instant,
};
use tokio_util::time::DelayQueue;
use tracing::{debug, info, warn};

use crate::{
    config::AppConfig,
    consts::{
        DEFAULT_HANDLER_TIMEOUT, EVENT_QUEUE_ADMISSION_WAIT_DIVISOR,
        MAX_EVENT_QUEUE_ADMISSION_WAIT_SECS,
    },
    types::{EventData, EventResult, EventTuple},
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
    ) -> Result<oneshot::Receiver<(EventData, EventResult)>, EventQueueError> {
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
                    seconds(handler.timeout),
                );
            }
        }
        if let Some(websocket) = &api.websocket {
            for handler in &websocket.handlers {
                by_tag.insert(
                    websocket_handler_tag(&handler.route_key, &handler.route),
                    seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(consumers) = &app_config.consumers {
        for consumer in &consumers.consumers {
            for handler in &consumer.handlers {
                by_tag.insert(
                    source_handler_tag(&consumer.source_id, &handler.name),
                    seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(schedules) = &app_config.schedules {
        for schedule in &schedules.schedules {
            for handler in &schedule.handlers {
                by_tag.insert(
                    source_handler_tag(&schedule.schedule_id, &handler.name),
                    seconds(handler.timeout),
                );
            }
        }
    }

    if let Some(custom) = &app_config.custom_handlers {
        for handler in &custom.handlers {
            by_tag.insert(custom_handler_tag(&handler.name), seconds(handler.timeout));
        }
    }

    HandlerTimeouts::new(by_tag, seconds(DEFAULT_HANDLER_TIMEOUT))
}

/// Converts a configured timeout in seconds to a duration, treating a
/// non-positive value as "unset" and falling back to the default.
fn seconds(timeout: i64) -> Duration {
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
}

/// An event that has been taken by the handlers executable and is awaiting
/// a result.
#[derive(Debug)]
pub struct InFlightEntry {
    pub result_tx: oneshot::Sender<(EventData, EventResult)>,
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
    arm_tx: mpsc::UnboundedSender<(String, Instant)>,
}

impl InFlightTable {
    /// Records an event as in-flight and arms its deadline with the cleanup task.
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
        if self.arm_tx.send((event_id, deadline)).is_err() {
            debug!("expired event cleanup task is not running, deadline will not be enforced");
        }
    }

    /// Removes an event from the table, returning its entry if it was present.
    ///
    /// A missing entry is expected rather than exceptional: the cleanup task may
    /// have already removed it, or a result may arrive twice.
    pub fn remove(&self, event_id: &str) -> Option<InFlightEntry> {
        self.entries
            .lock()
            .expect("in-flight table lock should not be poisoned")
            .remove(event_id)
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

/// Everything needed to run the event path in the IPC runtime call mode.
pub struct EventQueueParts {
    pub queue: EventQueue,
    pub receiver: EventQueueReceiver,
    pub in_flight: Arc<InFlightTable>,
    arm_rx: mpsc::UnboundedReceiver<(String, Instant)>,
}

impl EventQueueParts {
    /// Creates the bounded queue, the in-flight table and the channel that
    /// arms deadlines with the cleanup task.
    pub fn new(capacity: usize) -> Self {
        let (tx, rx) = mpsc::channel(capacity);
        let (arm_tx, arm_rx) = mpsc::unbounded_channel();
        EventQueueParts {
            queue: EventQueue { tx },
            receiver: Arc::new(tokio::sync::Mutex::new(rx)),
            in_flight: Arc::new(InFlightTable {
                entries: Mutex::new(HashMap::new()),
                arm_tx,
            }),
            arm_rx,
        }
    }

    /// Starts the cleanup task, consuming the arm channel receiver.
    ///
    /// Returns the handles the rest of the runtime needs, the cleanup task's
    /// handle and the sender that stops it.
    pub fn spawn_cleanup_task(self) -> (EventQueueHandles, JoinHandle<()>, oneshot::Sender<()>) {
        let (shutdown_tx, shutdown_rx) = oneshot::channel();
        let handle =
            spawn_expired_event_cleanup_task(self.in_flight.clone(), self.arm_rx, shutdown_rx);
        (
            EventQueueHandles {
                queue: self.queue,
                receiver: self.receiver,
                in_flight: self.in_flight,
            },
            handle,
            shutdown_tx,
        )
    }

    /// Starts the cleanup task without returning its task handle, for callers that
    /// stop it with the shutdown signal rather than by joining it.
    pub fn spawn_cleanup_task_detached(self) -> (EventQueueHandles, oneshot::Sender<()>) {
        let (handles, _task, shutdown_tx) = self.spawn_cleanup_task();
        (handles, shutdown_tx)
    }
}

/// The parts of the event path that outlive setup.
#[derive(Debug, Clone)]
pub struct EventQueueHandles {
    pub queue: EventQueue,
    pub receiver: EventQueueReceiver,
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
    mut arm_rx: mpsc::UnboundedReceiver<(String, Instant)>,
    mut shutdown_rx: oneshot::Receiver<()>,
) -> JoinHandle<()> {
    tokio::spawn(async move {
        let mut deadlines: DelayQueue<String> = DelayQueue::new();
        // Holds an armed deadline between the select that received it and the
        // top of the loop, so that no select branch borrows `deadlines`
        // while another is polling it.
        let mut pending_arm: Option<(String, Instant)> = None;

        loop {
            if let Some((event_id, deadline)) = pending_arm.take() {
                deadlines.insert_at(event_id, deadline);
            }

            if deadlines.is_empty() {
                tokio::select! {
                    _ = &mut shutdown_rx => break,
                    armed = arm_rx.recv() => match armed {
                        Some(armed) => pending_arm = Some(armed),
                        None => break,
                    },
                }
                continue;
            }

            let expired = tokio::select! {
                _ = &mut shutdown_rx => break,
                armed = arm_rx.recv() => {
                    match armed {
                        Some(armed) => pending_arm = Some(armed),
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
            if let Some(entry) = in_flight.remove(&event_id) {
                warn!(
                    event_id = %event_id,
                    handler_tag = %entry.event.handler_tag,
                    "event deadline exceeded before a result was returned, \
                     removing it from the in-flight table"
                );
            }
        }

        info!("received shutdown signal, stopping expired event cleanup task");
    })
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
        let parts = EventQueueParts::new(4);
        let receiver = parts.receiver.clone();
        let queue = parts.queue.clone();

        let _rx = queue
            .enqueue(test_event("event-1"), Duration::from_secs(1))
            .await
            .unwrap();

        let (_result_tx, event) = receiver.lock().await.recv().await.unwrap();
        assert_eq!(event.id, "event-1");
    }

    #[tokio::test(start_paused = true)]
    async fn enqueue_reports_queue_full_when_no_room_appears_within_the_admission_wait() {
        let parts = EventQueueParts::new(1);

        parts
            .queue
            .enqueue(test_event("event-1"), Duration::from_secs(1))
            .await
            .unwrap();
        let result = parts
            .queue
            .enqueue(test_event("event-2"), Duration::from_secs(1))
            .await;

        assert_eq!(result.err(), Some(EventQueueError::QueueFull));
    }

    #[tokio::test]
    async fn enqueue_waits_for_capacity_rather_than_shedding_immediately() {
        let parts = EventQueueParts::new(1);
        let receiver = parts.receiver.clone();

        parts
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

        let result = parts
            .queue
            .enqueue(test_event("event-2"), Duration::from_secs(5))
            .await;

        assert!(result.is_ok());
        assert!(drain.await.unwrap().is_some());
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
        let parts = EventQueueParts::new(4);
        let queue = parts.queue.clone();
        drop(parts);

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
        let parts = EventQueueParts::new(4);
        let (handles, _cleanup_task, _shutdown) = parts.spawn_cleanup_task();

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

    #[tokio::test(start_paused = true)]
    async fn cleanup_removes_entries_whose_deadline_has_passed() {
        let parts = EventQueueParts::new(4);
        let (handles, _cleanup_task, _shutdown) = parts.spawn_cleanup_task();

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
        let parts = EventQueueParts::new(4);
        let (handles, _cleanup_task, _shutdown) = parts.spawn_cleanup_task();

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
