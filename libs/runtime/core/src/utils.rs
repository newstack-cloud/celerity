use std::time::{SystemTime, UNIX_EPOCH};

pub fn get_epoch_seconds() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}
