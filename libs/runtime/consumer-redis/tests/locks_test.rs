use celerity_consumer_redis::locks::MessageLocks;
use celerity_helpers::redis::{get_redis_connection, ConnectionConfig};

#[test_log::test(tokio::test)]
async fn test_single_message_lock_for_concurrent_consumers() {
    let service_name = "test-service".to_string();

    let nodes = vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()];
    let conn_config = ConnectionConfig {
        nodes,
        password: None,
        cluster_mode: false,
    };
    let connection = get_redis_connection(&conn_config, None).await.unwrap();

    // Worker 1 should be able to acquire the initial lock for the message.
    let worker_1 = "consumer-worker-1".to_string();
    let mut locks_for_worker_1 =
        MessageLocks::new(service_name.clone(), worker_1, connection.clone());
    let acquired = locks_for_worker_1
        // Prefix with ts1 to avoid conflict with other tests.
        .acquire_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![true],
        "worker 1 should be able to acquire the lock"
    );

    // Worker 2 should fail to acquire the lock for the message.
    let worker_2 = "consumer-worker-2".to_string();
    let mut locks_for_worker_2 = MessageLocks::new(service_name, worker_2, connection.clone());
    let acquired = locks_for_worker_2
        .acquire_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![false],
        "worker 2 should fail to acquire the lock owned by worker 1"
    );

    // Worker 1 should be able to release the lock for the message.
    let released = locks_for_worker_1
        .release_locks(&["ts1-message-id-1"])
        .await
        .unwrap();
    assert_eq!(
        released,
        vec![true],
        "worker 1 should be able to release the lock"
    );

    // Worker 2 should be able to acquire the lock for the message.
    let acquired = locks_for_worker_2
        .acquire_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![true],
        "worker 2 should be able to acquire the lock"
    );

    // Worker 2 should be able to extend the lock for the message.
    let extended = locks_for_worker_2
        .extend_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        extended,
        vec![true],
        "worker 2 should be able to extend the lock"
    );

    // Worker 1 should fail to acquire the extended lock.
    let acquired = locks_for_worker_1
        .acquire_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![false],
        "worker 1 should fail to acquire the extended lock owned by worker 2"
    );

    // Worker 1 should not be able to extend the lock for a message it does not own.
    let extended = locks_for_worker_1
        .extend_locks(&["ts1-message-id-1"], 10000)
        .await
        .unwrap();
    assert_eq!(
        extended,
        vec![false],
        "worker 1 should not be able to extend the lock for a message it does not own"
    );
}

#[test_log::test(tokio::test)]
async fn test_multiple_message_locks_for_concurrent_consumers() {
    let service_name = "test-service".to_string();
    let nodes = vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()];
    let conn_config = ConnectionConfig {
        nodes,
        password: None,
        cluster_mode: false,
    };
    let connection = get_redis_connection(&conn_config, None).await.unwrap();

    // Worker 1 should be able to acquire the locks for some messages.
    let worker_1 = "consumer-worker-1".to_string();
    let mut locks_for_worker_1 =
        MessageLocks::new(service_name.clone(), worker_1, connection.clone());
    let acquired = locks_for_worker_1
        // Prefix with ts2 to avoid conflict with other tests.
        .acquire_locks(&["ts2-message-id-1", "ts2-message-id-3"], 10000)
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![true, true],
        "worker 1 should be able to acquire the locks"
    );

    // Worker 2 should be able to acquire the locks for messages not owned by worker 1 (2,4)
    // but not for the messages owned by worker 1 (1,3).
    let worker_2 = "consumer-worker-2".to_string();
    let mut locks_for_worker_2 = MessageLocks::new(service_name, worker_2, connection.clone());
    let acquired = locks_for_worker_2
        .acquire_locks(
            &[
                "ts2-message-id-1",
                "ts2-message-id-2",
                "ts2-message-id-3",
                "ts2-message-id-4",
            ],
            10000,
        )
        .await
        .unwrap();
    assert_eq!(
        acquired,
        vec![false, true, false, true],
        "worker 2 should be able to acquire the locks for messages 2 and 4 but not for messages 1 and 3"
    );

    // Worker 1 should be able to extend the locks for the messages it owns (1,3)
    // but not for the messages not owned by worker 1 (2,4).
    let extended = locks_for_worker_1
        .extend_locks(
            &[
                "ts2-message-id-1",
                "ts2-message-id-2",
                "ts2-message-id-3",
                "ts2-message-id-4",
            ],
            10000,
        )
        .await
        .unwrap();
    assert_eq!(
        extended,
        vec![true, false, true, false],
        "worker 1 should be able to extend the locks for messages 1 and 3 but not for messages 2 and 4"
    );

    // Worker 2 should be able to release the locks for the messages it owns (2,4)
    // but not for the messages not owned by worker 2 (1,3).
    let released = locks_for_worker_2
        .release_locks(&[
            "ts2-message-id-1",
            "ts2-message-id-2",
            "ts2-message-id-3",
            "ts2-message-id-4",
        ])
        .await
        .unwrap();
    assert_eq!(
        released,
        vec![false, true, false, true],
        "worker 2 should be able to release the locks for messages 2 and 4 but not for messages 1 and 3"
    );
}
