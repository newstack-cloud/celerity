use std::{
    fmt::Debug,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use tokio::time::Instant;

/// A trait for a clock that can provide the current time
/// as a UNIX timestamp in seconds.
pub trait Clock {
    fn now(&self) -> u64;
    fn now_millis(&self) -> u64;
}

impl Debug for dyn Clock + Send + Sync {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "Clock")
    }
}

/// A default implementation of a clock that uses the system time.
///
/// # Examples
///
/// ```
/// # use celerity_helpers::time::DefaultClock;
/// # use std::time::SystemTime;
///
/// let clock = DefaultClock::new();
/// let now = clock.now();
/// ```
pub struct DefaultClock {}

impl DefaultClock {
    /// Creates a new instance of the default clock
    /// that uses system time.
    pub fn new() -> Self {
        DefaultClock {}
    }
}

impl Default for DefaultClock {
    fn default() -> Self {
        Self::new()
    }
}

impl Clock for DefaultClock {
    fn now(&self) -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("Time went backwards")
            .as_secs()
    }

    fn now_millis(&self) -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("Time went backwards")
            .as_millis() as u64
    }
}

/// Calculates the remaining wait time in milliseconds
/// before the next polling attempt for a polling loop
/// such as a message consumer.
pub fn calcuate_polling_wait_time(start_time: Instant, current_polling_wait_time_ms: u64) -> u64 {
    let elapsed = start_time.elapsed();
    let configured_wait_time = Duration::from_millis(current_polling_wait_time_ms);
    if elapsed < configured_wait_time {
        (configured_wait_time - elapsed).as_millis() as u64
    } else {
        0
    }
}
