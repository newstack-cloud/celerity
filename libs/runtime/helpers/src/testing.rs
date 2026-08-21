//! Helpers for tests in the crates that build on this one.
//!
//! Behind the `test-support` feature, so nothing here is built into a runtime.

use crate::redis::{get_redis_connection, ConnectionConfig, ConnectionWrapper};

const SINGLE_NODE: &str = "redis://127.0.0.1:6379/?protocol=resp3";

/// The cluster `docker-compose.test-deps.yml` starts.
const CLUSTER_NODES: [&str; 6] = [
    "redis://127.0.0.1:7100/?protocol=resp3",
    "redis://127.0.0.1:7101/?protocol=resp3",
    "redis://127.0.0.1:7102/?protocol=resp3",
    "redis://127.0.0.1:7103/?protocol=resp3",
    "redis://127.0.0.1:7104/?protocol=resp3",
    "redis://127.0.0.1:7105/?protocol=resp3",
];

/// Which deployment a run is against.
///
/// Set `CELERITY_TEST_REDIS_CLUSTER` for the cluster, which is the only place
/// some decisions can be wrong: a script reaching across slots is refused, and
/// so is a request carrying keys that belong to different ones.
pub fn redis_config() -> ConnectionConfig {
    if std::env::var("CELERITY_TEST_REDIS_CLUSTER").is_ok() {
        return ConnectionConfig {
            nodes: CLUSTER_NODES.iter().map(|node| node.to_string()).collect(),
            password: None,
            cluster_mode: true,
        };
    }

    ConnectionConfig {
        nodes: vec![SINGLE_NODE.to_string()],
        password: None,
        cluster_mode: false,
    }
}

pub async fn redis_connection() -> ConnectionWrapper {
    get_redis_connection(&redis_config(), None)
        .await
        .expect("redis has to be running, see docker-compose.test-deps.yml")
}
