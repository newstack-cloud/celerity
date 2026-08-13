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
use std::time::Duration;

use tokio::{
    sync::{mpsc, oneshot},
    time::Instant,
};
use tracing::{debug, info, warn};

use crate::{
    consts::{HANDLER_ATTACH_GRACE_SECS, MAX_DERIVED_DRAIN_TIMEOUT_SECS},
    event_queue::{EventQueueReceivers, HandlerTimeouts, InFlightEntry, InFlightTable},
    types::{CancelReason, CancelRequest, EventData, EventOutcome, EventTuple, UnservableReason},
};

/// Identifies one attached handler stream.
pub type StreamId = u64;

/// An event on its way to a handler, with the deadline the runtime will hold it
/// to.
///
/// The deadline travels with the event because the dispatcher is what resolves
/// it, and a handler is expected to observe the same deadline the runtime
/// enforces rather than deriving its own.
#[derive(Debug)]
pub struct DispatchedEvent {
    pub event: EventData,
    pub deadline_unix_ms: i64,
}

/// What the dispatcher sends down an attached stream.
///
/// Cancellation and drain share the event channel rather than having their own,
/// so that a handler sees them in order relative to the events they concern.
#[derive(Debug)]
pub enum StreamFrame {
    Dispatch(Box<DispatchedEvent>),
    /// Stop work on an event nobody is waiting for. This is advisory, the handler is
    /// still expected to return a result, which is what returns its credit.
    Cancel {
        event_id: String,
        reason: CancelReason,
    },
    /// The runtime has stopped dispatching and is waiting for what is already
    /// in flight.
    Drain {
        deadline_unix_ms: i64,
    },
}

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
    /// Where dispatched events and the frames concerning them are sent.
    pub dispatch_tx: mpsc::Sender<StreamFrame>,
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
    ///
    /// Sent for every result, including one for an event whose deadline had
    /// already passed. The credit for that event was still consumed, so a
    /// result the caller can no longer use must still return it, or the window
    /// shrinks by one every time a handler answers late and eventually stalls
    /// the stream.
    Completed {
        stream_id: StreamId,
        event_id: String,
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
    /// The events this stream is holding, by event id and the tag they were
    /// dispatched for, so that a result identifies which per-tag count to
    /// release and a departing stream releases everything it still holds.
    holding: HashMap<String, String>,
    dispatch_tx: mpsc::Sender<StreamFrame>,
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
    /// Which stream holds each dispatched event, so a cancellation can be
    /// routed without searching every stream.
    holders: HashMap<String, StreamId>,
    in_flight: Arc<InFlightTable>,
    timeouts: HandlerTimeouts,
    /// How long a shutdown waits for in-flight events before abandoning them.
    drain_timeout: Duration,
}

/// Waits until an instant, or forever when there is nothing to wait for.
///
/// A `select!` branch has to produce a future either way, and a branch that
/// never completes is how "no deadline" is expressed without disabling the
/// branch and losing it when a deadline appears.
async fn sleep_until_or_never(deadline: Option<Instant>) {
    match deadline {
        Some(deadline) => tokio::time::sleep_until(deadline).await,
        None => std::future::pending().await,
    }
}

/// The wall-clock instant a timeout from now lands on, in milliseconds.
fn unix_millis_from_now(timeout: std::time::Duration) -> i64 {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default();
    now.saturating_add(timeout).as_millis() as i64
}

/// An event waiting for a stream that can take it.
struct QueuedEvent {
    result_tx: oneshot::Sender<EventOutcome>,
    event: EventData,
    /// When this joined the queue, which starts the grace window allowed for a
    /// stream serving its tag to attach.
    queued_at: Instant,
}

/// How long a shutdown should wait for in-flight events before abandoning them.
///
/// A configured value is taken as given, since an operator setting it is
/// matching the deployment's own grace period. Otherwise it comes from the
/// longest handler timeout the blueprint configures, so an application of short
/// handlers stops promptly and one with a long running handler is given the
/// time that handler was told it had, bounded because a deployment cannot wait
/// out an hour long handler.
pub fn drain_timeout(configured: Option<u64>, timeouts: &HandlerTimeouts) -> Duration {
    match configured {
        Some(seconds) => Duration::from_secs(seconds),
        None => timeouts
            .longest()
            .min(Duration::from_secs(MAX_DERIVED_DRAIN_TIMEOUT_SECS)),
    }
}

impl Dispatcher {
    pub fn new(
        in_flight: Arc<InFlightTable>,
        timeouts: HandlerTimeouts,
        drain_timeout: Duration,
    ) -> Self {
        Dispatcher {
            queues: BTreeMap::new(),
            cursor: 0,
            streams: BTreeMap::new(),
            holders: HashMap::new(),
            in_flight,
            timeouts,
            drain_timeout,
        }
    }

    /// Runs dispatch until the event queue closes or shutdown is signalled,
    /// then drains what is already in flight.
    pub async fn run(
        mut self,
        receivers: EventQueueReceivers,
        mut command_rx: mpsc::Receiver<DispatcherCommand>,
        mut shutdown_rx: oneshot::Receiver<()>,
    ) {
        let EventQueueReceivers {
            mut events,
            mut cancellations,
        } = receivers;

        loop {
            // Recomputed each pass because attaching a stream can make a queue
            // servable again, and dispatching can empty one.
            let shed_at = self.next_shed_deadline();

            tokio::select! {
                _ = &mut shutdown_rx => break,
                _ = sleep_until_or_never(shed_at) => self.shed_unservable(),
                taken = events.recv() => match taken {
                    Some((result_tx, event)) => {
                        self.enqueue(QueuedEvent {
                            result_tx,
                            event,
                            queued_at: Instant::now(),
                        });
                        self.dispatch_ready().await;
                    }
                    None => break,
                },
                request = cancellations.recv() => match request {
                    Some(request) => self.cancel(request),
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

        self.drain(&mut events, &mut command_rx).await;
        debug!("dispatcher stopping");
    }

    fn enqueue(&mut self, queued: QueuedEvent) {
        self.queues
            .entry(queued.event.handler_tag.clone())
            .or_default()
            .push_back(queued);
    }

    /// Tells whichever stream holds an event to stop working on it.
    ///
    /// A cancellation for an event no stream holds is expected rather than
    /// an exception, it may have completed in the moment before the caller went
    /// away, or its stream may have detached.
    fn cancel(&mut self, request: CancelRequest) {
        let CancelRequest { event_id, reason } = request;
        let Some(stream) = self
            .holders
            .get(&event_id)
            .and_then(|stream_id| self.streams.get(stream_id))
        else {
            return;
        };

        // A caller that went away leaves an entry nobody will ever read, so it
        // is released here rather than being held until its deadline, which
        // would also produce a second cancellation for the same event. A
        // deadline that has already passed took its own entry out.
        if reason == CancelReason::CallerGone {
            self.in_flight.remove(&event_id);
        }

        debug!(event_id = %event_id, ?reason, "cancelling an in-flight event");
        if stream
            .dispatch_tx
            .try_send(StreamFrame::Cancel {
                event_id: event_id.clone(),
                reason,
            })
            .is_err()
        {
            debug!(event_id = %event_id, "could not deliver a cancellation to the handler stream");
        }
    }

    /// Stops dispatching, tells attached streams to finish, and waits for what
    /// is already in flight.
    ///
    /// Queued events are shed rather than held, nothing will dispatch them now,
    /// so waiting out their own timeouts would only delay the callers finding
    /// out.
    async fn drain(
        &mut self,
        events: &mut mpsc::Receiver<EventTuple>,
        command_rx: &mut mpsc::Receiver<DispatcherCommand>,
    ) {
        let deadline = Instant::now() + self.drain_timeout;
        let deadline_unix_ms = unix_millis_from_now(self.drain_timeout);

        // Closing first stops producers adding more, then everything already
        // accepted is taken so its callers get an answer. Left in the channel
        // they would instead see it close under them, which is indistinguishable
        // from the handlers executable dying mid-request.
        events.close();
        while let Ok((result_tx, event)) = events.try_recv() {
            self.enqueue(QueuedEvent {
                result_tx,
                event,
                queued_at: Instant::now(),
            });
        }
        self.shed_queued(UnservableReason::ShuttingDown);
        for stream in self.streams.values() {
            let _ = stream
                .dispatch_tx
                .try_send(StreamFrame::Drain { deadline_unix_ms });
        }

        if self.holders.is_empty() {
            return;
        }
        info!(
            in_flight = self.holders.len(),
            drain_timeout = ?self.drain_timeout,
            "waiting for in-flight events before stopping the dispatcher"
        );

        while !self.holders.is_empty() {
            tokio::select! {
                _ = tokio::time::sleep_until(deadline) => {
                    warn!(
                        in_flight = self.holders.len(),
                        "drain deadline passed with events still in flight, abandoning them"
                    );
                    self.abandon_in_flight();
                    return;
                }
                command = command_rx.recv() => match command {
                    // A stream attaching now would be given no work, and the
                    // dropped confirmation tells it to stop rather than sit
                    // idle believing it is serving traffic.
                    Some(DispatcherCommand::Attach { stream_id, .. }) => {
                        debug!(stream_id, "refusing a handler stream that attached during drain");
                    }
                    Some(command) => self.apply(command),
                    None => return,
                },
            }
        }
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
                        holding: HashMap::new(),
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
                event_id,
                credit_grant,
            } => {
                self.holders.remove(&event_id);
                if let Some(stream) = self.streams.get_mut(&stream_id) {
                    if let Some(handler_tag) = stream.holding.remove(&event_id) {
                        if let Some(count) = stream.in_flight.get_mut(&handler_tag) {
                            *count = count.saturating_sub(1);
                        }
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
        for event_id in stream.holding.into_keys() {
            self.holders.remove(&event_id);
            self.in_flight.remove(&event_id);
        }
    }

    /// Whether any attached stream serves a handler tag at all, regardless of
    /// whether it could take an event right now.
    ///
    /// Distinct from [`StreamState::can_take`]: a stream that serves the tag
    /// but has no credit left is busy, and its events wait for their own
    /// timeout. A tag no stream serves has nothing to wait for.
    fn is_served(&self, handler_tag: &str) -> bool {
        self.streams
            .values()
            .any(|stream| stream.tags.contains(handler_tag))
    }

    /// When the oldest event waiting on a tag nothing serves has surpassed
    /// the grace period.
    fn next_shed_deadline(&self) -> Option<Instant> {
        let grace = Duration::from_secs(HANDLER_ATTACH_GRACE_SECS);
        self.queues
            .iter()
            .filter(|(handler_tag, _)| !self.is_served(handler_tag))
            .filter_map(|(_, queue)| queue.front().map(|queued| queued.queued_at + grace))
            .min()
    }

    /// Fails the events that have waited out their grace period on a tag nothing
    /// serves.
    ///
    /// Callers are told now rather than at the end of a handler timeout they
    /// were never going to survive, so a request to an application whose
    /// handlers are not running fails in seconds rather than in a minute.
    fn shed_unservable(&mut self) {
        let now = Instant::now();
        let grace = Duration::from_secs(HANDLER_ATTACH_GRACE_SECS);
        let unserved: Vec<String> = self
            .queues
            .keys()
            .filter(|handler_tag| !self.is_served(handler_tag))
            .cloned()
            .collect();

        for handler_tag in unserved {
            let Some(queue) = self.queues.get_mut(&handler_tag) else {
                continue;
            };

            let mut shed = 0;
            while queue
                .front()
                .is_some_and(|queued| queued.queued_at + grace <= now)
            {
                let queued = queue.pop_front().expect("the front was just inspected");
                let _ = queued
                    .result_tx
                    .send(EventOutcome::Unservable(UnservableReason::NoHandler));
                shed += 1;
            }

            if shed > 0 {
                warn!(
                    %handler_tag,
                    shed,
                    "shedding events, no attached handler stream serves this tag"
                );
            }
        }
    }

    /// Tells the handlers still holding events that the runtime has stopped
    /// waiting for them.
    ///
    /// Only reached when the drain deadline passes, at which point the process
    /// is going away regardless; the cancellations are what stop a handler
    /// that outlives it from finishing work whose result has nowhere to go.
    fn abandon_in_flight(&mut self) {
        for (event_id, stream_id) in std::mem::take(&mut self.holders) {
            let Some(stream) = self.streams.get(&stream_id) else {
                continue;
            };
            let _ = stream.dispatch_tx.try_send(StreamFrame::Cancel {
                event_id,
                reason: CancelReason::Shutdown,
            });
        }
    }

    /// Fails everything still queued, for a runtime that will not dispatch
    /// again.
    fn shed_queued(&mut self, reason: UnservableReason) {
        let mut shed = 0;
        for queue in self.queues.values_mut() {
            for queued in queue.drain(..) {
                let _ = queued.result_tx.send(EventOutcome::Unservable(reason));
                shed += 1;
            }
        }
        if shed > 0 {
            warn!(shed, %reason, "shedding queued events");
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
        let deadline_unix_ms = unix_millis_from_now(timeout);

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

        if let Err(err) =
            stream
                .dispatch_tx
                .try_send(StreamFrame::Dispatch(Box::new(DispatchedEvent {
                    event: queued.event,
                    deadline_unix_ms,
                })))
        {
            // The stream went away between being chosen and being sent to.
            // Releasing the entry lets the caller fail now rather than wait.
            warn!(stream_id, %handler_tag, "failed to send to handler stream: {err}");
            self.in_flight.remove(&event_id);
            self.detach(stream_id);
            return false;
        }

        stream.credit = stream.credit.saturating_sub(1);
        *stream.in_flight.entry(handler_tag.to_string()).or_insert(0) += 1;
        stream
            .holding
            .insert(event_id.clone(), handler_tag.to_string());
        self.holders.insert(event_id, stream_id);
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
        shutdown: Option<oneshot::Sender<()>>,
        _cleanup_shutdown: oneshot::Sender<()>,
    }

    impl Harness {
        /// Signals shutdown, which is what puts the dispatcher into its drain.
        fn stop(&mut self) {
            if let Some(shutdown) = self.shutdown.take() {
                let _ = shutdown.send(());
            }
        }
    }

    fn start(capacity: usize) -> Harness {
        let (handles, receivers, cleanup) = EventQueueParts::new(capacity).into_parts();
        let cleanup_shutdown = cleanup.spawn();
        let (command_tx, command_rx) = mpsc::channel(16);
        let (shutdown_tx, shutdown_rx) = oneshot::channel();

        let dispatcher = Dispatcher::new(
            handles.in_flight.clone(),
            timeouts(),
            Duration::from_secs(30),
        );
        tokio::spawn(dispatcher.run(receivers, command_rx, shutdown_rx));

        Harness {
            queue: handles.queue.clone(),
            commands: command_tx,
            shutdown: Some(shutdown_tx),
            _cleanup_shutdown: cleanup_shutdown,
        }
    }

    async fn attach(
        harness: &Harness,
        stream_id: StreamId,
        tags: &[&str],
        credit: u32,
        limits: HashMap<String, u32>,
    ) -> mpsc::Receiver<StreamFrame> {
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

    async fn recv(rx: &mut mpsc::Receiver<StreamFrame>) -> Option<StreamFrame> {
        tokio::time::timeout(Duration::from_secs(2), rx.recv())
            .await
            .ok()
            .flatten()
    }

    /// The next frame, when it is expected to be an event rather than a
    /// cancellation or a drain.
    async fn recv_dispatch(rx: &mut mpsc::Receiver<StreamFrame>) -> Option<DispatchedEvent> {
        match recv(rx).await {
            Some(StreamFrame::Dispatch(dispatched)) => Some(*dispatched),
            Some(other) => panic!("expected an event, got {other:?}"),
            None => None,
        }
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

        let dispatched = recv_dispatch(&mut stream)
            .await
            .expect("the event should arrive");
        assert_eq!(dispatched.event.id, "event-1");
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

        assert!(recv_dispatch(&mut stream).await.is_some());
        assert!(recv_dispatch(&mut stream).await.is_some());
        // Credit is exhausted, so the rest wait rather than being sent.
        assert!(recv_dispatch(&mut stream).await.is_none());

        // Returning credit with a result releases exactly one more.
        harness
            .commands
            .send(DispatcherCommand::Completed {
                stream_id: 1,
                event_id: "event-0".to_string(),
                credit_grant: 1,
            })
            .await
            .unwrap();

        assert!(recv_dispatch(&mut stream).await.is_some());
        assert!(recv_dispatch(&mut stream).await.is_none());
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
        while let Some(dispatched) = recv_dispatch(&mut stream).await {
            seen.push(dispatched.event.id);
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
        while recv_dispatch(&mut first).await.is_some() {
            first_count += 1;
        }
        while recv_dispatch(&mut second).await.is_some() {
            second_count += 1;
        }

        assert_eq!(first_count + second_count, 4);
        assert!(
            first_count > 0 && second_count > 0,
            "both streams should have been given work, got {first_count} and {second_count}"
        );
    }

    #[tokio::test(start_paused = true)]
    async fn sheds_events_for_a_tag_nothing_serves_once_the_grace_window_passes() {
        let harness = start(8);
        let _stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let result_rx = harness
            .queue
            .enqueue(
                event("event-1", "schedule::b"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        // The caller is told in seconds rather than being held for the whole
        // sixty second handler timeout it was never going to survive.
        let outcome = tokio::time::timeout(
            Duration::from_secs(HANDLER_ATTACH_GRACE_SECS + 2),
            result_rx,
        )
        .await
        .expect("the caller should be woken")
        .expect("an outcome should arrive");
        assert!(matches!(
            outcome,
            EventOutcome::Unservable(UnservableReason::NoHandler)
        ));
    }

    #[tokio::test]
    async fn dispatches_an_event_that_arrives_before_the_stream_serving_it() {
        let harness = start(8);

        harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        // Attaching within the grace window is what a handlers executable that
        // is still starting up looks like, and the event waits for it rather
        // than being shed on arrival.
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let dispatched = recv_dispatch(&mut stream)
            .await
            .expect("the event should arrive");
        assert_eq!(dispatched.event.id, "event-1");
    }

    #[tokio::test]
    async fn routes_a_cancellation_to_the_stream_holding_the_event() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let _result_rx = harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        let dispatched = recv_dispatch(&mut stream)
            .await
            .expect("the event should arrive");

        // Stands in for the guard an HTTP route holds while it waits, which
        // drops when the response future does.
        drop(harness.queue.cancel_on_drop(dispatched.event.id));

        let frame = recv(&mut stream).await;
        let Some(StreamFrame::Cancel { event_id, reason }) = frame else {
            panic!("expected a cancellation, got {frame:?}");
        };
        assert_eq!(event_id, "event-1");
        assert_eq!(reason, CancelReason::CallerGone);
    }

    #[tokio::test]
    async fn ignores_a_cancellation_for_an_event_no_stream_holds() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        drop(harness.queue.cancel_on_drop("never-dispatched".to_string()));

        assert!(recv(&mut stream).await.is_none());
    }

    #[test]
    fn a_configured_drain_timeout_is_taken_as_given() {
        let timeouts = HandlerTimeouts::new(HashMap::new(), Duration::from_secs(60));

        // An operator setting this is matching the deployment's own grace
        // period, so it is not second-guessed against the handler timeouts.
        assert_eq!(
            drain_timeout(Some(600), &timeouts),
            Duration::from_secs(600)
        );
    }

    #[test]
    fn an_unconfigured_drain_timeout_comes_from_the_longest_handler() {
        let timeouts = HandlerTimeouts::new(
            HashMap::from([
                ("health".to_string(), Duration::from_secs(1)),
                ("report".to_string(), Duration::from_secs(120)),
            ]),
            Duration::from_secs(60),
        );

        // A handler told it had two minutes is not abandoned after thirty
        // seconds, and the short handler alongside it does not shorten that.
        assert_eq!(drain_timeout(None, &timeouts), Duration::from_secs(120));
    }

    #[test]
    fn a_derived_drain_timeout_is_bounded() {
        let timeouts = HandlerTimeouts::new(
            HashMap::from([("slow".to_string(), Duration::from_secs(3600))]),
            Duration::from_secs(60),
        );

        // A deployment cannot wait out an hour long handler, so past the bound
        // the work is abandoned rather than the shutdown being held open.
        assert_eq!(
            drain_timeout(None, &timeouts),
            Duration::from_secs(MAX_DERIVED_DRAIN_TIMEOUT_SECS)
        );
    }

    #[tokio::test]
    async fn tells_attached_streams_to_drain_and_sheds_what_is_still_queued() {
        let mut harness = start(8);
        // One credit, so the second event is still queued when shutdown lands.
        let mut stream = attach(&harness, 1, &["schedule::a"], 1, HashMap::new()).await;

        let _first = harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        assert!(recv_dispatch(&mut stream).await.is_some());

        let second = harness
            .queue
            .enqueue(
                event("event-2", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        harness.stop();

        let frame = recv(&mut stream).await;
        assert!(
            matches!(frame, Some(StreamFrame::Drain { .. })),
            "the attached stream should be told to drain, got {frame:?}"
        );

        // Nothing will dispatch the queued event now, so its caller is told
        // rather than left waiting out a timeout.
        let outcome = tokio::time::timeout(Duration::from_secs(2), second)
            .await
            .expect("the caller should be woken")
            .expect("an outcome should arrive");
        assert!(matches!(
            outcome,
            EventOutcome::Unservable(UnservableReason::ShuttingDown)
        ));
    }

    #[tokio::test]
    async fn returns_credit_for_a_result_that_arrives_after_its_caller_gave_up() {
        let harness = start(8);
        // A single credit, so the stream stalls if the first event never
        // returns it.
        let mut stream = attach(&harness, 1, &["schedule::a"], 1, HashMap::new()).await;

        let first = harness
            .queue
            .enqueue(
                event("event-1", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        assert!(recv_dispatch(&mut stream).await.is_some());

        // The caller gives up, which releases the in-flight entry, so the late
        // result has nobody to go to.
        drop(first);
        drop(harness.queue.cancel_on_drop("event-1".to_string()));
        assert!(matches!(
            recv(&mut stream).await,
            Some(StreamFrame::Cancel { .. })
        ));

        harness
            .commands
            .send(DispatcherCommand::Completed {
                stream_id: 1,
                event_id: "event-1".to_string(),
                credit_grant: 1,
            })
            .await
            .unwrap();

        let _second = harness
            .queue
            .enqueue(
                event("event-2", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        let dispatched = recv_dispatch(&mut stream)
            .await
            .expect("the returned credit should let the next event through");
        assert_eq!(dispatched.event.id, "event-2");
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
        assert!(recv_dispatch(&mut stream).await.is_some());

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
