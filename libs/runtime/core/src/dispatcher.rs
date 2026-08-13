//! Chooses which queued event goes to which handler stream, and when.
//!
//! Events arrive on one bounded queue but are not dispatched from it directly.
//! A single line shared by every handler tag is head-of-line blocking waiting
//! to happen, an application with a one millisecond health check and a five
//! second report endpoint puts both in the same queue, so fifty report requests
//! at the head hold up every health check behind them. That is how a slow
//! endpoint takes down a liveness probe.
//!
//! Events are therefore partitioned into one queue per handler tag, and two
//! independent limits decide what may be sent:
//!
//! - **Credit**, which a handler stream grants and the runtime consumes. It
//!   bounds total in-flight work for that stream and is sized to the handler's
//!   worker pool, where throughput saturates.
//! - **Per-tag concurrency caps**, optional, which stop one slow tag consuming
//!   the entire credit window and starving the others.
//!
//! Neither is sufficient alone, credit sizes the pool correctly, caps stop a
//! single tag monopolising it. Among the tags that can be served, selection is
//! round-robin, which is enough while scheduling policy stays runtime-side.
//!
//! Nothing here knows about specific transport. A stream is somewhere events can be sent and
//! a source of credit, so the dispatcher can be exercised without a transport.

use std::collections::{BTreeMap, HashMap, HashSet, VecDeque};
use std::sync::Arc;

use tokio::sync::{mpsc, oneshot};
use tracing::{debug, warn};

use crate::{
    event_queue::{EventQueueReceiver, HandlerTimeouts, InFlightEntry, InFlightTable},
    types::EventData,
};

/// Identifies one attached handler stream.
pub type StreamId = u64;

/// What a handler stream declares about itself when it attaches.
#[derive(Debug)]
pub struct StreamRegistration {
    /// The handler tags this stream serves. Events for any other tag are never
    /// sent to it.
    pub handler_tags: Vec<String>,
    /// How many events may be in flight to this stream at once.
    pub initial_credit: u32,
    /// Optional per-tag concurrency caps. A tag with no entry is bounded only
    /// by the credit window.
    pub limits: HashMap<String, u32>,
    /// Where dispatched events are sent.
    pub dispatch_tx: mpsc::Sender<EventData>,
}

/// Tells the dispatcher about something that happened outside its own loop.
#[derive(Debug)]
pub enum DispatcherCommand {
    /// A handler stream finished its handshake and is ready for work.
    ///
    /// `registered` fires once the stream is attached, so that a caller can
    /// acknowledge the handshake only after the dispatcher will actually
    /// consider it. Without it there is a window where a stream believes it is
    /// serving traffic that is still going elsewhere.
    Attach {
        stream_id: StreamId,
        registration: Box<StreamRegistration>,
        registered: oneshot::Sender<()>,
    },
    /// A handler stream went away. Anything it was still holding is released.
    Detach { stream_id: StreamId },
    /// A result came back, freeing a slot and returning whatever credit the
    /// handler chose to give back.
    Completed {
        stream_id: StreamId,
        handler_tag: String,
        credit_grant: u32,
    },
    /// Credit returned outside a result, used to resize the window or to resume
    /// after a handler deliberately withheld.
    ///
    /// There is one window per stream, covering every tag it serves. Isolation
    /// between tags comes from the concurrency caps declared at attach.
    Grant {
        stream_id: StreamId,
        additional: u32,
    },
}

/// One attached stream, as the dispatcher sees it.
struct StreamState {
    tags: HashSet<String>,
    credit: u32,
    limits: HashMap<String, u32>,
    /// How many events are in flight to this stream, per tag.
    in_flight: HashMap<String, u32>,
    /// The events this stream is holding, so they can be released if it goes.
    holding: HashSet<String>,
    dispatch_tx: mpsc::Sender<EventData>,
}

impl StreamState {
    /// Whether this stream could take an event for the given tag right now.
    fn can_take(&self, handler_tag: &str) -> bool {
        if self.credit == 0 || !self.tags.contains(handler_tag) {
            return false;
        }
        match self.limits.get(handler_tag) {
            Some(cap) => self.in_flight.get(handler_tag).copied().unwrap_or(0) < *cap,
            None => true,
        }
    }
}

/// Runs the dispatch loop until shutdown.
pub struct Dispatcher {
    /// One queue per handler tag. Ordered so that round-robin selection visits
    /// tags predictably rather than depending on hash iteration order.
    queues: BTreeMap<String, VecDeque<QueuedEvent>>,
    /// Where the last round-robin pass stopped, so the next starts after it.
    cursor: usize,
    streams: BTreeMap<StreamId, StreamState>,
    in_flight: Arc<InFlightTable>,
    timeouts: HandlerTimeouts,
}

/// An event waiting for a stream that can take it.
struct QueuedEvent {
    result_tx: oneshot::Sender<(EventData, crate::types::EventResult)>,
    event: EventData,
}

impl Dispatcher {
    pub fn new(in_flight: Arc<InFlightTable>, timeouts: HandlerTimeouts) -> Self {
        Dispatcher {
            queues: BTreeMap::new(),
            cursor: 0,
            streams: BTreeMap::new(),
            in_flight,
            timeouts,
        }
    }

    /// Runs dispatch until the event queue closes or shutdown is signalled.
    pub async fn run(
        mut self,
        event_rx: EventQueueReceiver,
        mut command_rx: mpsc::Receiver<DispatcherCommand>,
        mut shutdown_rx: oneshot::Receiver<()>,
    ) {
        let mut events = event_rx.lock().await;

        loop {
            tokio::select! {
                _ = &mut shutdown_rx => break,
                taken = events.recv() => match taken {
                    Some((result_tx, event)) => {
                        self.enqueue(QueuedEvent { result_tx, event });
                        self.dispatch_ready().await;
                    }
                    None => break,
                },
                command = command_rx.recv() => match command {
                    Some(command) => {
                        self.apply(command);
                        self.dispatch_ready().await;
                    }
                    None => break,
                },
            }
        }

        debug!("dispatcher stopping");
    }

    fn enqueue(&mut self, queued: QueuedEvent) {
        self.queues
            .entry(queued.event.handler_tag.clone())
            .or_default()
            .push_back(queued);
    }

    fn apply(&mut self, command: DispatcherCommand) {
        match command {
            DispatcherCommand::Attach {
                stream_id,
                registration,
                registered,
            } => {
                let registration = *registration;
                debug!(
                    stream_id,
                    credit = registration.initial_credit,
                    tags = registration.handler_tags.len(),
                    "handler stream attached"
                );
                self.streams.insert(
                    stream_id,
                    StreamState {
                        tags: registration.handler_tags.into_iter().collect(),
                        credit: registration.initial_credit,
                        limits: registration.limits,
                        in_flight: HashMap::new(),
                        holding: HashSet::new(),
                        dispatch_tx: registration.dispatch_tx,
                    },
                );
                // A receiver that has gone away just means the caller stopped
                // waiting, which is not a reason to fail the attach.
                let _ = registered.send(());
            }
            DispatcherCommand::Detach { stream_id } => self.detach(stream_id),
            DispatcherCommand::Completed {
                stream_id,
                handler_tag,
                credit_grant,
            } => {
                if let Some(stream) = self.streams.get_mut(&stream_id) {
                    if let Some(count) = stream.in_flight.get_mut(&handler_tag) {
                        *count = count.saturating_sub(1);
                    }
                    stream.credit = stream.credit.saturating_add(credit_grant);
                }
            }
            DispatcherCommand::Grant {
                stream_id,
                additional,
            } => {
                if let Some(stream) = self.streams.get_mut(&stream_id) {
                    stream.credit = stream.credit.saturating_add(additional);
                }
            }
        }
    }

    /// Releases everything a departed stream was holding.
    ///
    /// Dropping the result sender wakes whoever was waiting on the event with a
    /// closed channel, so a caller sees a failure immediately rather than
    /// waiting out a deadline for a handler that has gone.
    fn detach(&mut self, stream_id: StreamId) {
        let Some(stream) = self.streams.remove(&stream_id) else {
            return;
        };
        if stream.holding.is_empty() {
            debug!(stream_id, "handler stream detached");
            return;
        }

        warn!(
            stream_id,
            in_flight = stream.holding.len(),
            "handler stream detached while holding events, releasing them"
        );
        for event_id in stream.holding {
            self.in_flight.remove(&event_id);
        }
    }

    /// Sends as many queued events as the attached streams can currently take.
    async fn dispatch_ready(&mut self) {
        // Each pass sends at most one event per eligible tag, and stops once a
        // pass achieves nothing, so an unservable queue cannot spin.
        while self.dispatch_one_round().await > 0 {}
    }

    async fn dispatch_one_round(&mut self) -> usize {
        let tags: Vec<String> = self.queues.keys().cloned().collect();
        if tags.is_empty() {
            return 0;
        }

        let mut sent = 0;
        for offset in 0..tags.len() {
            let tag = &tags[(self.cursor + offset) % tags.len()];
            if self.dispatch_from(tag).await {
                sent += 1;
            }
        }
        // Advance past the tag served first, so the next pass starts elsewhere
        // and no tag is repeatedly favoured.
        self.cursor = (self.cursor + 1) % tags.len();
        sent
    }

    /// Sends one event for a tag, if anything is queued and a stream can take
    /// it. Returns whether an event was sent.
    async fn dispatch_from(&mut self, handler_tag: &str) -> bool {
        if self
            .queues
            .get(handler_tag)
            .is_none_or(|queue| queue.is_empty())
        {
            return false;
        }

        let Some(stream_id) = self.choose_stream(handler_tag) else {
            return false;
        };

        let Some(queued) = self
            .queues
            .get_mut(handler_tag)
            .and_then(|queue| queue.pop_front())
        else {
            return false;
        };

        let event_id = queued.event.id.clone();
        let timeout = self.timeouts.for_tag(handler_tag);

        // Recorded as in flight before it is sent, so a result arriving
        // immediately finds the entry already there.
        self.in_flight.insert(
            InFlightEntry {
                result_tx: queued.result_tx,
                event: queued.event.clone(),
            },
            timeout,
        );

        let stream = self
            .streams
            .get_mut(&stream_id)
            .expect("the chosen stream should still be attached");

        if let Err(err) = stream.dispatch_tx.try_send(queued.event) {
            // The stream went away between being chosen and being sent to.
            // Releasing the entry lets the caller fail now rather than wait.
            warn!(stream_id, %handler_tag, "failed to send to handler stream: {err}");
            self.in_flight.remove(&event_id);
            self.detach(stream_id);
            return false;
        }

        stream.credit = stream.credit.saturating_sub(1);
        *stream.in_flight.entry(handler_tag.to_string()).or_insert(0) += 1;
        stream.holding.insert(event_id);
        true
    }

    /// Picks the attached stream that should take the next event for a tag.
    ///
    /// Among those that can take it, the one holding the fewest events for that
    /// tag wins, which spreads work rather than filling one stream first.
    fn choose_stream(&self, handler_tag: &str) -> Option<StreamId> {
        self.streams
            .iter()
            .filter(|(_, stream)| stream.can_take(handler_tag))
            .min_by_key(|(id, stream)| {
                (
                    stream.in_flight.get(handler_tag).copied().unwrap_or(0),
                    **id,
                )
            })
            .map(|(id, _)| *id)
    }
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use serde_json::json;

    use super::*;
    use crate::{
        event_queue::{admission_wait, EventQueueParts},
        types::{EventDataPayload, EventType, ScheduleEventData},
    };

    fn event(id: &str, handler_tag: &str) -> EventData {
        EventData {
            id: id.to_string(),
            event_type: EventType::ScheduleMessage,
            handler_tag: handler_tag.to_string(),
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

    fn timeouts() -> HandlerTimeouts {
        HandlerTimeouts::new(HashMap::new(), Duration::from_secs(60))
    }

    struct Harness {
        queue: crate::event_queue::EventQueue,
        commands: mpsc::Sender<DispatcherCommand>,
        _shutdown: oneshot::Sender<()>,
        _cleanup_shutdown: oneshot::Sender<()>,
    }

    fn start(capacity: usize) -> Harness {
        let (handles, cleanup) = EventQueueParts::new(capacity).into_parts();
        let cleanup_shutdown = cleanup.spawn();
        let (command_tx, command_rx) = mpsc::channel(16);
        let (shutdown_tx, shutdown_rx) = oneshot::channel();

        let dispatcher = Dispatcher::new(handles.in_flight.clone(), timeouts());
        let receiver = handles.receiver.clone();
        tokio::spawn(dispatcher.run(receiver, command_rx, shutdown_rx));

        Harness {
            queue: handles.queue.clone(),
            commands: command_tx,
            _shutdown: shutdown_tx,
            _cleanup_shutdown: cleanup_shutdown,
        }
    }

    async fn attach(
        harness: &Harness,
        stream_id: StreamId,
        tags: &[&str],
        credit: u32,
        limits: HashMap<String, u32>,
    ) -> mpsc::Receiver<EventData> {
        let (dispatch_tx, dispatch_rx) = mpsc::channel(64);
        let (registered_tx, registered_rx) = oneshot::channel();
        harness
            .commands
            .send(DispatcherCommand::Attach {
                stream_id,
                registration: Box::new(StreamRegistration {
                    handler_tags: tags.iter().map(|tag| tag.to_string()).collect(),
                    initial_credit: credit,
                    limits,
                    dispatch_tx,
                }),
                registered: registered_tx,
            })
            .await
            .unwrap();
        registered_rx
            .await
            .expect("the dispatcher should confirm the attach");
        dispatch_rx
    }

    async fn recv(rx: &mut mpsc::Receiver<EventData>) -> Option<EventData> {
        tokio::time::timeout(Duration::from_secs(2), rx.recv())
            .await
            .ok()
            .flatten()
    }

    #[tokio::test]
    async fn dispatches_an_event_to_a_stream_that_serves_its_tag() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        let dispatched = recv(&mut stream).await.expect("the event should arrive");
        assert_eq!(dispatched.id, "event-1");
    }

    #[tokio::test]
    async fn does_not_dispatch_a_tag_no_attached_stream_serves() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        harness
            .queue
            .enqueue(
                event("event-1", "schedule::b"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        assert!(recv(&mut stream).await.is_none());
    }

    #[tokio::test]
    async fn stops_dispatching_when_credit_runs_out() {
        let harness = start(16);
        let mut stream = attach(&harness, 1, &["schedule::a"], 2, HashMap::new()).await;

        for index in 0..5 {
            harness
                .queue
                .enqueue(
                    event(&format!("event-{index}"), "schedule::a"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }

        assert!(recv(&mut stream).await.is_some());
        assert!(recv(&mut stream).await.is_some());
        // Credit is exhausted, so the rest wait rather than being sent.
        assert!(recv(&mut stream).await.is_none());

        // Returning credit with a result releases exactly one more.
        harness
            .commands
            .send(DispatcherCommand::Completed {
                stream_id: 1,
                handler_tag: "schedule::a".to_string(),
                credit_grant: 1,
            })
            .await
            .unwrap();

        assert!(recv(&mut stream).await.is_some());
        assert!(recv(&mut stream).await.is_none());
    }

    #[tokio::test]
    async fn a_per_tag_cap_stops_one_tag_consuming_the_whole_window() {
        let harness = start(32);
        let mut stream = attach(
            &harness,
            1,
            &["fast", "slow"],
            8,
            HashMap::from([("slow".to_string(), 1)]),
        )
        .await;

        // Queue the slow tag first and deeply, which without a cap would take
        // the entire credit window and leave nothing for the fast tag.
        for index in 0..6 {
            harness
                .queue
                .enqueue(
                    event(&format!("slow-{index}"), "slow"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }
        harness
            .queue
            .enqueue(
                event("fast-0", "fast"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        let mut seen = Vec::new();
        while let Some(dispatched) = recv(&mut stream).await {
            seen.push(dispatched.id);
        }

        // The cap allows one slow event at a time, so the fast one is not stuck
        // behind the five still queued.
        assert_eq!(seen.iter().filter(|id| id.starts_with("slow-")).count(), 1);
        assert!(seen.contains(&"fast-0".to_string()));
    }

    #[tokio::test]
    async fn spreads_events_across_streams_serving_the_same_tag() {
        let harness = start(16);
        let mut first = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;
        let mut second = attach(&harness, 2, &["schedule::a"], 4, HashMap::new()).await;

        for index in 0..4 {
            harness
                .queue
                .enqueue(
                    event(&format!("event-{index}"), "schedule::a"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }

        let mut first_count = 0;
        let mut second_count = 0;
        while recv(&mut first).await.is_some() {
            first_count += 1;
        }
        while recv(&mut second).await.is_some() {
            second_count += 1;
        }

        assert_eq!(first_count + second_count, 4);
        assert!(
            first_count > 0 && second_count > 0,
            "both streams should have been given work, got {first_count} and {second_count}"
        );
    }

    #[tokio::test]
    async fn releases_events_held_by_a_stream_that_detaches() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let result_rx = harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        assert!(recv(&mut stream).await.is_some());

        harness
            .commands
            .send(DispatcherCommand::Detach { stream_id: 1 })
            .await
            .unwrap();

        // The caller finds out now rather than waiting out the deadline of a
        // handler that has gone.
        let outcome = tokio::time::timeout(Duration::from_secs(2), result_rx)
            .await
            .expect("the caller should be woken");
        assert!(outcome.is_err(), "the result channel should have closed");
    }
}
