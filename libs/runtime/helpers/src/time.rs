use std::time::{SystemTime, UNIX_EPOCH};

/// A trait for a clock that can provide the current time
/// as a UNIX timestamp in seconds.
pub trait Clock {
    fn now(&self) -> u64;
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

impl Clock for DefaultClock {
    fn now(&self) -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("Time went backwards")
            .as_secs()
    }
}
