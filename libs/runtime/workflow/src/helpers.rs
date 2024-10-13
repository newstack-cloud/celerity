use celerity_blueprint_config_parser::blueprint::CelerityWorkflowRetryConfig;
use rand::Rng;

/// Calculate the wait time in milliseconds for a retry attempt.
/// This uses exponential backoff; jitter is also used if configured.
/// See: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
pub fn calculate_retry_wait_time_ms(
    retry_config: &CelerityWorkflowRetryConfig,
    retry_attempt: i64,
    default_interval_seconds: i64,
    default_backoff_rate: f64,
) -> u64 {
    // Interval is configured in seconds, convert to milliseconds to allow
    // for millisecond precision for fractional backoff rates.
    let interval_ms = retry_config.interval.unwrap_or(default_interval_seconds) * 1000;
    let multiplier = retry_config.backoff_rate.unwrap_or(default_backoff_rate);
    let mut computed_wait_time_ms = interval_ms as f64 * multiplier.powf(retry_attempt as f64);

    if let Some(max_delay) = retry_config.max_delay {
        computed_wait_time_ms = computed_wait_time_ms.min(max_delay as f64 * 1000.0);
    }

    if retry_config.jitter.unwrap_or(false) {
        rand::thread_rng()
            .gen_range(0.0..computed_wait_time_ms)
            .trunc() as u64
    } else {
        computed_wait_time_ms as u64
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_calculates_retry_wait_time_ms() {
        let retry_config = CelerityWorkflowRetryConfig {
            match_errors: vec![], // Not used to calculate wait time
            max_attempts: None,   // Not used to calculate wait time
            interval: Some(2),
            backoff_rate: Some(1.5),
            max_delay: Some(14),
            jitter: Some(false),
        };

        // First retry attempt should be 2 seconds using interval.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 0, 2, 1.5);
        assert_eq!(wait_time, 2000);

        // Second retry attempt should be 3 seconds using interval * backoff_rate^retry_attempt_number.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 1, 2, 1.5);
        assert_eq!(wait_time, 3000);

        // Third retry attempt should be 4.5 seconds using interval * backoff_rate^retry_attempt_number.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 2, 2, 1.5);
        assert_eq!(wait_time, 4500);

        // Fourth retry attempt should be 6.75 seconds using interval * backoff_rate^retry_attempt_number.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 3, 2, 1.5);
        assert_eq!(wait_time, 6750);

        // Fifth retry attempt should be 10.125 seconds using interval * backoff_rate^retry_attempt_number.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 4, 2, 1.5);
        assert_eq!(wait_time, 10125);

        // Sixth retry attempt should be 15.1875 seconds using interval * backoff_rate^retry_attempt_number.
        // However max_delay is set to 14
        // so the wait time should be capped at 14 seconds.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 5, 2, 1.5);
        assert_eq!(wait_time, 14000);
    }

    #[test_log::test]
    fn test_calculates_retry_wait_time_ms_correctly_with_jitter() {
        let retry_config = CelerityWorkflowRetryConfig {
            match_errors: vec![], // Not used to calculate wait time
            max_attempts: None,   // Not used to calculate wait time
            interval: Some(3),
            backoff_rate: Some(2.0),
            max_delay: Some(80),
            jitter: Some(true),
        };

        // First retry attempt would be 3 seconds using interval * backoff_rate^retry_attempt_number,
        // therefore between 0 and 3 seconds with jitter.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 0, 3, 2.0);
        assert!(wait_time <= 3000);

        // Second retry attempt would be 6 seconds using interval * backoff_rate^retry_attempt_number,
        // therefore between 0 and 6 seconds with jitter.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 1, 3, 2.0);
        assert!(wait_time <= 6000);

        // Third retry attempt would be 12 seconds using interval * backoff_rate^retry_attempt_number,
        // therefore between 0 and 12 seconds with jitter.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 2, 3, 2.0);
        assert!(wait_time <= 12000);

        // Fourth retry attempt would be 24 seconds using interval * backoff_rate^retry_attempt_number,
        // therefore between 0 and 24 seconds with jitter.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 3, 3, 2.0);
        assert!(wait_time <= 24000);

        // Fifth retry attempt would be 48 seconds using interval * backoff_rate^retry_attempt_number,
        // therefore between 0 and 48 seconds with jitter.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 4, 3, 2.0);
        assert!(wait_time <= 48000);

        // Sixth retry attempt would be 96 seconds using interval * backoff_rate^retry_attempt_number,
        // However max_delay is set to 80
        // so the wait time should be capped at 80 seconds.
        let wait_time = calculate_retry_wait_time_ms(&retry_config, 5, 3, 2.0);
        assert!(wait_time <= 80000);
    }
}
