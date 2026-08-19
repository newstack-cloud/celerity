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
    sync::{mpsc, oneshot, watch},
    time::Instant,
};
use tracing::{debug, info, warn};

use crate::{
    consts::{
        HANDLER_ATTACH_GRACE_SECS, HANDLER_CANCEL_RECLAIM_GRACE_SECS, HANDLER_RECLAIM_MEMORY_SECS,
        MAX_DERIVED_DRAIN_TIMEOUT_SECS,
    },
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
    /// Optional per-tag concurrency caps. A tag with no entry is bounded by the
    /// credit window and by a default cap, which keeps a place for the other
    /// tags so that no one of them can take the window entirely.
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
    /// A handler stream is finishing. It takes no more work, but keeps
    /// everything it already has, so results can still come back.
    Draining { stream_id: StreamId },
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
    /// What a tag may hold when the handler declared no limit for it.
    ///
    /// Leaves room for every other tag to place one event, so no tag can take
    /// the window and leave the rest of them unable to dispatch at all.
    default_limit: u32,
    /// How many events are in flight to this stream, per tag.
    in_flight: HashMap<String, u32>,
    /// The events this stream is holding, by event id and the tag they were
    /// dispatched for, so that a result identifies which per-tag count to
    /// release and a departing stream releases everything it still holds.
    holding: HashMap<String, String>,
    /// Whether the handler has said it is finishing. It keeps what it has and
    /// its results are still taken, but nothing further is sent to it.
    draining: bool,
    dispatch_tx: mpsc::Sender<StreamFrame>,
}

impl StreamState {
    /// Whether this stream could take an event for the given tag right now.
    fn can_take(&self, handler_tag: &str) -> bool {
        if self.draining || self.credit == 0 || !self.tags.contains(handler_tag) {
            return false;
        }
        let cap = self
            .limits
            .get(handler_tag)
            .copied()
            .unwrap_or(self.default_limit);
        self.in_flight.get(handler_tag).copied().unwrap_or(0) < cap
    }
}

/// What a tag may hold when the handler declared no limit for it.
///
/// A window is one per stream and is refused to every tag once it runs out, so
/// without this a single tag can take all of it and the others are not just slow,
/// they are stopped. That matters most for the tags nothing else can stand in
/// for. A connection's disconnect handler shares a window with the messages
/// that connection sent, and credit comes back only when a handler answers, so
/// handlers that never answer would otherwise leave the connection unable to
/// finish tearing down.
///
/// Holds back a place for each of the other tags, but never more than half the
/// window. What is being bought is that no tag can take all of it, and a
/// handler serving many tags on a small window cannot give each of them a place
/// anyway. Without the halving, an application with twenty handlers and a window
/// of eight would cap every one of them at a single event, which is a worse
/// trade than the starvation it avoids for whichever handler carries the
/// traffic.
///
/// A handler that wants a different split declares its own limits, which are
/// used as given.
fn default_tag_limit(credit: u32, tags: usize) -> u32 {
    let held_back = (tags.saturating_sub(1) as u32).min(credit / 2);
    credit.saturating_sub(held_back).max(1)
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
    /// Publishes whether any stream is attached, for readiness probes and for
    /// the startup deadline a supervising runtime applies.
    readiness: watch::Sender<bool>,
    /// Events that have been cancelled and the moment the runtime stops keeping
    /// room for an answer to them.
    ///
    /// Every entry is given the same grace period, so they are added in the order they
    /// fall due and the earliest is always the front.
    reclaims: VecDeque<(Instant, String)>,
    /// How long a cancelled event is left with its handler before what it holds
    /// is taken back.
    reclaim_grace: Duration,
    /// Events whose place has been taken back, kept so that a handler answering
    /// late can still withhold the place rather than be sent more work.
    ///
    /// Ordered by when they stop being remembered, which is the same span for
    /// each, so the earliest is always the front.
    reclaimed: VecDeque<(Instant, String)>,
    /// The same events, for looking one up when a late answer names it.
    reclaimed_ids: HashSet<String>,
    /// How long a taken back event is remembered. Shortened by tests.
    reclaim_memory: Duration,
}

/// Whether a handlers executable is attached and able to be given work.
///
/// A handler process that has died takes the runtime down with it, which an
/// orchestrator sees as a non-zero exit. This covers the other case: a process
/// that is alive but not serving, because it never completed its handshake, was
/// refused over a handler tag mismatch, or detached and never came back. None
/// of those exit, so without this they look healthy while every event is shed.
///
/// Readiness here means at least one stream is attached, not that every handler
/// tag the blueprint declares is served. A partially attached application can
/// still serve some of its routes, and refusing traffic outright would be a
/// worse answer than shedding the events that have nowhere to go.
#[derive(Debug, Clone)]
pub struct HandlerReadiness {
    ready: watch::Receiver<bool>,
}

impl HandlerReadiness {
    /// Whether a handlers executable is attached right now.
    pub fn is_ready(&self) -> bool {
        *self.ready.borrow()
    }

    /// Waits for a handlers executable to attach, returning whether one did.
    ///
    /// Returns `false` when the dispatcher has gone, since nothing will attach
    /// after that and a caller waiting on readiness would otherwise wait
    /// forever. A caller has to tell that apart from an attach, or it treats a
    /// runtime that is shutting down as one that is ready to serve.
    pub async fn wait_until_ready(&mut self) -> bool {
        // `wait_for` checks the current value before waiting, so an attach that
        // happened before this was called is not missed.
        self.ready.wait_for(|ready| *ready).await.is_ok()
    }
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
            readiness: watch::Sender::new(false),
            reclaims: VecDeque::new(),
            reclaim_grace: Duration::from_secs(HANDLER_CANCEL_RECLAIM_GRACE_SECS),
            reclaimed: VecDeque::new(),
            reclaimed_ids: HashSet::new(),
            reclaim_memory: Duration::from_secs(HANDLER_RECLAIM_MEMORY_SECS),
        }
    }

    /// A handle to whether a handlers executable is attached.
    ///
    /// Take this before [`Dispatcher::run`], which consumes the dispatcher.
    pub fn readiness(&self) -> HandlerReadiness {
        HandlerReadiness {
            ready: self.readiness.subscribe(),
        }
    }

    /// Publishes the current readiness, notifying only on a change so that a
    /// waiter is not woken by every attach on an application that has several.
    fn publish_readiness(&self) {
        let serving = !self.streams.is_empty();
        self.readiness.send_if_modified(|ready| {
            if *ready == serving {
                return false;
            }
            *ready = serving;
            true
        });
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
            let reclaim_at = match (self.reclaims.front(), self.reclaimed.front()) {
                (Some((due, _)), Some((forget, _))) => Some(*due.min(forget)),
                (Some((due, _)), None) => Some(*due),
                (None, Some((forget, _))) => Some(*forget),
                (None, None) => None,
            };

            tokio::select! {
                _ = &mut shutdown_rx => break,
                _ = sleep_until_or_never(shed_at) => self.shed_unservable(),
                _ = sleep_until_or_never(reclaim_at) => {
                    self.reclaim_abandoned();
                    self.dispatch_ready().await;
                }
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

        // Whatever the handler does with the cancellation, the runtime stops
        // keeping room for an answer to it once the grace period passes. A handler
        // that honours it and answers releases what it holds the ordinary way,
        // well within the grace period.
        self.reclaims
            .push_back((Instant::now() + self.reclaim_grace, event_id.clone()));

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

    /// Takes back what cancelled events are still holding, once the grace period for
    /// answering has passed.
    ///
    /// An event the handler answered in time is already gone from `holders`, so
    /// what is left here is what nothing came back for. Doing this is what
    /// keeps a handler that ignores cancellation from shrinking its stream's
    /// window one event at a time until it dispatches nothing at all.
    fn reclaim_abandoned(&mut self) {
        let now = Instant::now();
        while self
            .reclaims
            .front()
            .is_some_and(|(due_at, _)| *due_at <= now)
        {
            let Some((_, event_id)) = self.reclaims.pop_front() else {
                break;
            };
            let Some(stream_id) = self.holders.remove(&event_id) else {
                continue;
            };
            let Some(stream) = self.streams.get_mut(&stream_id) else {
                continue;
            };
            let Some(handler_tag) = stream.holding.remove(&event_id) else {
                continue;
            };
            if let Some(count) = stream.in_flight.get_mut(&handler_tag) {
                *count = count.saturating_sub(1);
            }
            stream.credit = stream.credit.saturating_add(1);
            warn!(
                stream_id,
                %event_id,
                %handler_tag,
                "no answer to a cancelled event within the grace, taking back the place \
                 it was holding"
            );
            // Remembered so that an answer arriving later can still withhold
            // the place, which is the handler saying it cannot take more.
            self.reclaimed
                .push_back((now + self.reclaim_memory, event_id.clone()));
            self.reclaimed_ids.insert(event_id);
        }

        while self
            .reclaimed
            .front()
            .is_some_and(|(forget_at, _)| *forget_at <= now)
        {
            if let Some((_, event_id)) = self.reclaimed.pop_front() {
                self.reclaimed_ids.remove(&event_id);
            }
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
                // Credit is the handler's own statement of how much it can take
                // at once, so it is not quietly reduced here. Past the channel's
                // capacity it stops being the binding limit though, and events
                // wait in their queues instead, which is worth saying rather
                // than leaving someone to wonder why a large pool did not help.
                let buffered = registration.dispatch_tx.max_capacity() as u32;
                if registration.initial_credit > buffered {
                    info!(
                        stream_id,
                        credit = registration.initial_credit,
                        buffered,
                        "declared credit is larger than the stream can buffer, \
                         throughput will be bounded by the buffer"
                    );
                }
                let tags: HashSet<String> = registration.handler_tags.into_iter().collect();
                let default_limit = default_tag_limit(registration.initial_credit, tags.len());
                self.streams.insert(
                    stream_id,
                    StreamState {
                        tags,
                        credit: registration.initial_credit,
                        limits: registration.limits,
                        default_limit,
                        in_flight: HashMap::new(),
                        holding: HashMap::new(),
                        draining: false,
                        dispatch_tx: registration.dispatch_tx,
                    },
                );
                self.publish_readiness();
                // A receiver that has gone away just means the caller stopped
                // waiting, which is not a reason to fail the attach.
                let _ = registered.send(());
            }
            DispatcherCommand::Detach { stream_id } => self.detach(stream_id),
            DispatcherCommand::Draining { stream_id } => {
                if let Some(stream) = self.streams.get_mut(&stream_id) {
                    debug!(
                        stream_id,
                        in_flight = stream.holding.len(),
                        "handler stream is draining, no more work will be sent to it"
                    );
                    stream.draining = true;
                }
            }
            DispatcherCommand::Completed {
                stream_id,
                event_id,
                credit_grant,
            } => {
                self.holders.remove(&event_id);
                let taken_back = self.reclaimed_ids.remove(&event_id);
                if let Some(stream) = self.streams.get_mut(&stream_id) {
                    if let Some(handler_tag) = stream.holding.remove(&event_id) {
                        if let Some(count) = stream.in_flight.get_mut(&handler_tag) {
                            *count = count.saturating_sub(1);
                        }
                        stream.credit = stream.credit.saturating_add(credit_grant);
                    } else if taken_back && credit_grant == 0 {
                        // The place is already back, so a grant would grow the
                        // window past what the handler declared and is dropped.
                        // No grant is a handler withholding though, and it means
                        // the same whenever it arrives, so the place goes back
                        // to the handler's account rather than the runtime's.
                        stream.credit = stream.credit.saturating_sub(1);
                        debug!(
                            stream_id,
                            %event_id,
                            "a late answer withheld the place that was taken back"
                        );
                    }
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
        self.publish_readiness();
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
    /// waiting for them, and releases the callers still waiting on those
    /// events.
    ///
    /// Only reached when the drain deadline passes, at which point the process
    /// is going away regardless; the cancellations are what stop a handler that
    /// outlives it from finishing work whose result has nowhere to go.
    ///
    /// Releasing the entry drops the sender the caller is waiting on, so it
    /// finds out now rather than waiting out its own timeout against a runtime
    /// that has stopped. It is deliberately not answered with an unservable
    /// outcome, as a queued event would be. This one was dispatched, and a
    /// handler may have applied some of it, so inviting a retry could apply it
    /// twice. A caller that cannot be told what happened should hear that,
    /// rather than hear that nothing happened.
    fn abandon_in_flight(&mut self) {
        for (event_id, stream_id) in std::mem::take(&mut self.holders) {
            // Before the stream is looked up, so an event whose stream has
            // already gone is still released rather than skipped.
            self.in_flight.remove(&event_id);

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
        let queued_at = queued.queued_at;
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

        match stream
            .dispatch_tx
            .try_send(StreamFrame::Dispatch(Box::new(DispatchedEvent {
                event: queued.event,
                deadline_unix_ms,
            }))) {
            Ok(()) => {}
            // The stream is behind, not gone. Putting the event back at the
            // head of its queue keeps it in line ahead of anything newer, and
            // the stream keeps everything it is already holding. Detaching here
            // would throw away a working stream, and every event on it, over a
            // buffer that is about to drain.
            Err(mpsc::error::TrySendError::Full(_)) => {
                debug!(
                    stream_id,
                    %handler_tag,
                    "handler stream is not keeping up, holding the event back"
                );
                if let Some(entry) = self.in_flight.remove(&event_id) {
                    self.queues
                        .entry(handler_tag.to_string())
                        .or_default()
                        .push_front(QueuedEvent {
                            result_tx: entry.result_tx,
                            event: entry.event,
                            queued_at,
                        });
                }
                return false;
            }
            // The stream went away between being chosen and being sent to.
            // Releasing the entry lets the caller fail now rather than wait.
            Err(mpsc::error::TrySendError::Closed(_)) => {
                warn!(stream_id, %handler_tag, "handler stream closed before the event was sent");
                self.in_flight.remove(&event_id);
                self.detach(stream_id);
                return false;
            }
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
        readiness: HandlerReadiness,
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
        start_with_drain_timeout(capacity, Duration::from_secs(30))
    }

    fn start_with_drain_timeout(capacity: usize, drain_timeout: Duration) -> Harness {
        let (handles, receivers, cleanup) = EventQueueParts::new(capacity).into_parts();
        let cleanup_shutdown = cleanup.spawn();
        let (command_tx, command_rx) = mpsc::channel(16);
        let (shutdown_tx, shutdown_rx) = oneshot::channel();

        let mut dispatcher = Dispatcher::new(handles.in_flight.clone(), timeouts(), drain_timeout);
        // The real grace is measured in seconds, which no test can wait out.
        dispatcher.reclaim_grace = Duration::from_millis(100);
        dispatcher.reclaim_memory = Duration::from_secs(30);
        let readiness = dispatcher.readiness();
        tokio::spawn(dispatcher.run(receivers, command_rx, shutdown_rx));

        Harness {
            queue: handles.queue.clone(),
            commands: command_tx,
            shutdown: Some(shutdown_tx),
            readiness,
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
        attach_with_buffer(harness, stream_id, tags, credit, limits, 64).await
    }

    async fn attach_with_buffer(
        harness: &Harness,
        stream_id: StreamId,
        tags: &[&str],
        credit: u32,
        limits: HashMap<String, u32>,
        buffer: usize,
    ) -> mpsc::Receiver<StreamFrame> {
        let (dispatch_tx, dispatch_rx) = mpsc::channel(buffer);
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
    fn a_derived_drain_timeout_covers_handlers_left_on_the_default() {
        let timeouts = HandlerTimeouts::new(
            HashMap::from([
                ("health".to_string(), Duration::from_secs(1)),
                ("quick".to_string(), Duration::from_secs(5)),
            ]),
            Duration::from_secs(60),
        );

        // A tag with no entry of its own is given the default, so an event can
        // run for a minute even though every configured handler is quicker.
        // Draining for five seconds would abandon it inside its own deadline.
        assert_eq!(drain_timeout(None, &timeouts), Duration::from_secs(60));
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
    async fn releases_callers_whose_events_are_abandoned_at_the_drain_deadline() {
        let mut harness = start_with_drain_timeout(8, Duration::from_millis(100));
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

        // Nothing answers, so the drain runs out of time with the event still
        // held. The caller should hear about it then, rather than waiting out
        // its own timeout against a runtime that has stopped.
        harness.stop();

        let outcome = tokio::time::timeout(Duration::from_secs(2), result_rx)
            .await
            .expect("the caller should be woken at the drain deadline");
        assert!(
            outcome.is_err(),
            "the caller should find the result channel closed, got {outcome:?}"
        );
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
    async fn holds_an_event_back_when_a_stream_is_behind_rather_than_dropping_it() {
        let harness = start(16);
        // One slot on the wire, more credit than that, so the second dispatch
        // finds the channel full while the stream is perfectly healthy.
        let mut stream =
            attach_with_buffer(&harness, 1, &["schedule::a"], 4, HashMap::new(), 1).await;

        let mut callers = Vec::new();
        for index in 0..2 {
            callers.push(
                harness
                    .queue
                    .enqueue(
                        event(&format!("event-{index}"), "schedule::a"),
                        admission_wait(Duration::from_secs(60)),
                    )
                    .await
                    .unwrap(),
            );
        }

        // The first fills the buffer. The second must not take the stream down
        // with it, nor be lost. A full channel is a stream that is behind, not
        // one that has gone.
        let first = recv_dispatch(&mut stream)
            .await
            .expect("the first event should be sent");
        assert_eq!(first.event.id, "event-0");

        // Room has appeared, so the held event goes out on the next pass. The
        // command is only there to prompt one.
        harness
            .commands
            .send(DispatcherCommand::Grant {
                stream_id: 1,
                additional: 0,
            })
            .await
            .unwrap();

        let second = recv_dispatch(&mut stream)
            .await
            .expect("the held event should be sent once there is room");
        assert_eq!(second.event.id, "event-1");

        // Neither caller was failed along the way.
        for caller in &mut callers {
            assert!(
                caller.try_recv().is_err(),
                "no caller should have been given an outcome yet"
            );
        }
    }

    #[tokio::test]
    async fn sends_no_more_work_to_a_draining_stream_but_keeps_what_it_holds() {
        let harness = start(8);
        let mut stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let held = harness
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
            .send(DispatcherCommand::Draining { stream_id: 1 })
            .await
            .unwrap();
        // Commands and events arrive on separate channels, so the dispatcher is
        // free to take either first. This waits for the command to have been
        // applied, rather than racing the next event against it.
        tokio::time::sleep(Duration::from_millis(50)).await;

        let mut held = held;
        harness
            .queue
            .enqueue(
                event("event-2", "schedule::a"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        // Nothing further is sent to it, even though it still has credit.
        assert!(recv_dispatch(&mut stream).await.is_none());
        // What it already has is untouched, so its caller is still waiting for
        // a result rather than having been failed.
        assert!(
            held.try_recv().is_err(),
            "the held event should not have been released"
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

    #[tokio::test]
    async fn is_not_ready_until_a_handler_stream_attaches() {
        let harness = start(16);
        assert!(
            !harness.readiness.is_ready(),
            "nothing has attached, so there is nothing to give work to"
        );

        let _stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        assert!(harness.readiness.is_ready());
    }

    #[tokio::test]
    async fn stops_being_ready_when_the_last_handler_stream_detaches() {
        let harness = start(16);
        let _stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;
        assert!(harness.readiness.is_ready());

        harness
            .commands
            .send(DispatcherCommand::Detach { stream_id: 1 })
            .await
            .unwrap();

        let mut readiness = harness.readiness.clone();
        tokio::time::timeout(Duration::from_secs(5), async {
            while readiness.is_ready() {
                readiness.ready.changed().await.unwrap();
            }
        })
        .await
        .expect("readiness should drop once the last stream detaches");
    }

    #[tokio::test]
    async fn stays_ready_while_another_handler_stream_is_still_attached() {
        let harness = start(16);
        let _first = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;
        let _second = attach(&harness, 2, &["schedule::b"], 4, HashMap::new()).await;

        harness
            .commands
            .send(DispatcherCommand::Detach { stream_id: 1 })
            .await
            .unwrap();
        // Round-trips a command so the detach above has certainly been applied.
        let _third = attach(&harness, 3, &["schedule::c"], 4, HashMap::new()).await;

        assert!(
            harness.readiness.is_ready(),
            "one stream going does not leave the application unable to serve"
        );
    }

    #[tokio::test]
    async fn wait_until_ready_returns_for_a_stream_that_attached_earlier() {
        let harness = start(16);
        let _stream = attach(&harness, 1, &["schedule::a"], 4, HashMap::new()).await;

        let mut readiness = harness.readiness.clone();
        let attached = tokio::time::timeout(Duration::from_secs(5), readiness.wait_until_ready())
            .await
            .expect("readiness should not be missed by a later waiter");

        assert!(attached);
    }

    #[tokio::test]
    async fn wait_until_ready_gives_up_once_the_dispatcher_has_gone() {
        let mut harness = start(16);
        let mut readiness = harness.readiness.clone();
        harness.stop();

        // Waiting has to end, and it has to end saying nothing attached. A
        // caller that read this as an attach would take a runtime on its way
        // out for one that is ready to serve.
        let attached = tokio::time::timeout(Duration::from_secs(5), readiness.wait_until_ready())
            .await
            .expect("a waiter should not outlive the dispatcher it waits on");

        assert!(!attached);
    }

    /// Every tag keeps a place, so no tag can take the window and leave the
    /// others unable to dispatch at all.
    #[test]
    fn a_tag_is_held_back_from_taking_the_whole_window() {
        assert_eq!(default_tag_limit(8, 3), 6);
        assert_eq!(default_tag_limit(8, 2), 7);
    }

    /// Nothing to starve, so nothing is held back.
    #[test]
    fn a_stream_serving_one_tag_gives_it_everything() {
        assert_eq!(default_tag_limit(8, 1), 8);
    }

    /// More tags than credit cannot leave a place for each, so every tag gets
    /// one and the window itself is what turns them away.
    #[test]
    fn a_window_too_small_to_share_still_lets_every_tag_try() {
        assert_eq!(default_tag_limit(2, 5), 1);
        assert_eq!(default_tag_limit(0, 3), 1);
    }

    /// Many handlers on a small window would otherwise cap each of them at a
    /// single event, which costs whichever one carries the traffic far more
    /// than the starvation it avoids.
    #[test]
    fn a_stream_serving_many_tags_still_keeps_half_the_window_for_one() {
        assert_eq!(default_tag_limit(8, 20), 4);
        assert_eq!(default_tag_limit(64, 100), 32);
    }

    /// A tag the handler declared no limit for still cannot take the window.
    ///
    /// The reclaim would eventually free the places a stalled tag holds, so
    /// this is what keeps the other tags dispatching in the meantime rather
    /// than waiting out a grace before they can be served at all.
    #[tokio::test]
    async fn a_tag_with_no_declared_limit_still_leaves_room_for_the_others() {
        let harness = start(64);
        let mut stream = attach(&harness, 1, &["hot", "cold"], 4, HashMap::new()).await;

        // More of the hot tag than the window holds, and none of them answered.
        for index in 0..8 {
            harness
                .queue
                .enqueue(
                    event(&format!("hot-{index}"), "hot"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }
        let mut hot = 0;
        while let Some(dispatched) = recv_dispatch(&mut stream).await {
            assert!(dispatched.event.id.starts_with("hot-"));
            hot += 1;
        }
        assert!(
            hot < 4,
            "the hot tag took {hot} of a window of 4, leaving nothing for the other tag"
        );

        // The cold tag has never been served and nothing has answered, so the
        // only reason it can be dispatched is the place held back for it.
        harness
            .queue
            .enqueue(
                event("cold-0", "cold"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        let cold = recv_dispatch(&mut stream)
            .await
            .expect("a tag that has taken nothing should still have a place");
        assert_eq!(cold.event.id, "cold-0");
    }

    /// A handler that never answers a cancellation does not keep what it was
    /// holding.
    ///
    /// Credit and a per tag place come back when a handler answers, so a
    /// handler that ignores a cancellation would otherwise leave a stream one
    /// place smaller for good, and enough of them would stop it dispatching
    /// anything at all.
    #[tokio::test]
    async fn takes_back_what_a_cancelled_event_was_holding_when_nothing_answers() {
        let harness = start(32);
        // One place, so what happens to it is the whole of what this observes.
        let mut stream = attach(&harness, 1, &["work"], 1, HashMap::new()).await;

        harness
            .queue
            .enqueue(
                event("event-1", "work"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        let dispatched = recv_dispatch(&mut stream)
            .await
            .expect("the first event should be dispatched");
        assert_eq!(dispatched.event.id, "event-1");

        harness
            .queue
            .enqueue(
                event("event-2", "work"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();
        assert!(
            recv_dispatch(&mut stream).await.is_none(),
            "the window is held by the first event, so the second should wait"
        );

        // The caller has gone. The handler is told to stop and never answers,
        // which is what a hung or crashed handler looks like from here.
        drop(harness.queue.cancel_on_drop("event-1".to_string()));
        match recv(&mut stream).await {
            Some(StreamFrame::Cancel { event_id, .. }) => assert_eq!(event_id, "event-1"),
            other => panic!("expected a cancellation, got {other:?}"),
        }

        tokio::time::sleep(Duration::from_millis(250)).await;

        let second = recv_dispatch(&mut stream)
            .await
            .expect("the place the cancelled event held should have come back");
        assert_eq!(second.event.id, "event-2");
    }

    /// An answer that arrives after the runtime gave up waiting does not put a
    /// place back that has already been taken back.
    ///
    /// The handler declares its own limits here, generously, so that credit is
    /// the only thing bounding what can be in flight. Without that the per tag
    /// limit hides an inflated window rather than the test finding it.
    #[tokio::test]
    async fn does_not_credit_an_answer_that_comes_after_the_place_was_taken_back() {
        let harness = start(32);
        let mut stream = attach(
            &harness,
            1,
            &["work"],
            2,
            HashMap::from([("work".to_string(), 10)]),
        )
        .await;

        for index in 0..2 {
            harness
                .queue
                .enqueue(
                    event(&format!("held-{index}"), "work"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }
        for _ in 0..2 {
            recv_dispatch(&mut stream)
                .await
                .expect("both events should be dispatched, which is the whole window");
        }

        drop(harness.queue.cancel_on_drop("held-0".to_string()));
        match recv(&mut stream).await {
            Some(StreamFrame::Cancel { .. }) => {}
            other => panic!("expected a cancellation, got {other:?}"),
        }
        tokio::time::sleep(Duration::from_millis(250)).await;

        // The handler answers late, returning the credit it believes it still
        // holds. That place is already back, so this must add nothing.
        harness
            .commands
            .send(DispatcherCommand::Completed {
                stream_id: 1,
                event_id: "held-0".to_string(),
                credit_grant: 1,
            })
            .await
            .unwrap();

        for index in 0..2 {
            harness
                .queue
                .enqueue(
                    event(&format!("later-{index}"), "work"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }

        assert!(
            recv_dispatch(&mut stream).await.is_some(),
            "the place the cancelled event held should have come back"
        );
        assert!(
            recv_dispatch(&mut stream).await.is_none(),
            "a late answer should not grow the window past what the handler declared"
        );
    }

    /// A handler answering late with no credit is still withholding, and the
    /// place the runtime took back on its behalf goes back to it.
    ///
    /// No credit on a result means stop dispatching until a later grant. The
    /// runtime returned that place when nothing answered in time, so honouring
    /// the withhold means undoing its own return rather than ignoring what the
    /// handler asked for and sending it work it said it could not take.
    #[tokio::test]
    async fn honours_a_withhold_that_arrives_after_the_place_was_taken_back() {
        let harness = start(32);
        let mut stream = attach(
            &harness,
            1,
            &["work"],
            2,
            HashMap::from([("work".to_string(), 10)]),
        )
        .await;

        for index in 0..2 {
            harness
                .queue
                .enqueue(
                    event(&format!("held-{index}"), "work"),
                    admission_wait(Duration::from_secs(60)),
                )
                .await
                .unwrap();
        }
        for _ in 0..2 {
            recv_dispatch(&mut stream)
                .await
                .expect("both events should be dispatched, which is the whole window");
        }

        drop(harness.queue.cancel_on_drop("held-0".to_string()));
        match recv(&mut stream).await {
            Some(StreamFrame::Cancel { .. }) => {}
            other => panic!("expected a cancellation, got {other:?}"),
        }
        tokio::time::sleep(Duration::from_millis(250)).await;

        // Late, and withholding. The place the runtime returned is the one the
        // handler is saying it does not want.
        harness
            .commands
            .send(DispatcherCommand::Completed {
                stream_id: 1,
                event_id: "held-0".to_string(),
                credit_grant: 0,
            })
            .await
            .unwrap();
        // Waited for rather than raced. A withhold governs what is dispatched
        // after it is read, and work already waiting when the place came back
        // may have gone out before it arrived, which is what being late costs.
        tokio::time::sleep(Duration::from_millis(100)).await;

        harness
            .queue
            .enqueue(
                event("later-0", "work"),
                admission_wait(Duration::from_secs(60)),
            )
            .await
            .unwrap();

        assert!(
            recv_dispatch(&mut stream).await.is_none(),
            "a handler that withheld should not be sent more work, however late it said so"
        );
    }
}
