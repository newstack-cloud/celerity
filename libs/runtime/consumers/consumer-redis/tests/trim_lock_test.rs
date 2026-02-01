use celerity_consumer_redis::trim_lock::TrimLock;
use celerity_helpers::redis::{get_redis_connection, ConnectionConfig};

#[test_log::test(tokio::test)]
async fn test_stream_trim_lock_for_concurrent_consumers() {
    let service_name = "test-service".to_string();
    let consumer_1_name = "consumer-1".to_string();
    let consumer_2_name = "consumer-2".to_string();
    let stream = "test-stream".to_string();

    let nodes = vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()];
    let conn_config = ConnectionConfig {
        nodes,
        password: None,
        cluster_mode: false,
    };
    let connection = get_redis_connection(&conn_config, None).await.unwrap();

    let mut consumer_1_trim_lock: TrimLock = TrimLock::new(
        service_name.clone(),
        consumer_1_name,
        stream.clone(),
        connection.clone(),
    );
    let mut consumer_2_trim_lock = TrimLock::new(service_name, consumer_2_name, stream, connection);

    let lock_acquired_by_consumer_1 = consumer_1_trim_lock.acquire(10000).await;
    assert!(lock_acquired_by_consumer_1.is_ok());
    assert!(
        lock_acquired_by_consumer_1.unwrap(),
        "consumer 1 should be able to acquire the lock"
    );

    let lock_acquired_by_consumer_2 = consumer_2_trim_lock.acquire(10000).await;
    assert!(lock_acquired_by_consumer_2.is_ok());
    assert!(
        !lock_acquired_by_consumer_2.unwrap(),
        "consumer 2 should not be able to acquire the lock"
    );

    let released_by_consumer_1 = consumer_1_trim_lock.release().await;
    assert!(released_by_consumer_1.is_ok());
    assert!(
        released_by_consumer_1.unwrap(),
        "consumer 1 should be able to release the lock"
    );

    let lock_acquired_by_consumer_2 = consumer_2_trim_lock.acquire(10000).await;
    assert!(lock_acquired_by_consumer_2.is_ok());
    assert!(
        lock_acquired_by_consumer_2.unwrap(),
        "consumer 2 should be able to acquire the lock"
    );

    let released_by_consumer_2 = consumer_2_trim_lock.release().await;
    assert!(released_by_consumer_2.is_ok());
    assert!(
        released_by_consumer_2.unwrap(),
        "consumer 2 should be able to release the lock"
    );
}
