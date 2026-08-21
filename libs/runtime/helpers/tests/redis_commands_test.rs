use celerity_helpers::redis::{get_redis_connection, ConnectionConfig, ConnectionWrapper};

async fn connection() -> ConnectionWrapper {
    let conn_config = ConnectionConfig {
        nodes: vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()],
        password: None,
        cluster_mode: false,
    };
    get_redis_connection(&conn_config, None)
        .await
        .expect("must be able to connect to redis for the command tests")
}

/// Set membership, which is how a node group knows who belongs to it.
#[test_log::test(tokio::test)]
async fn test_a_set_holds_its_members_until_they_are_removed() {
    let mut conn = connection().await;
    // Prefixed per test, since these run beside each other against one server.
    let key = "helpers-test:set-members";
    conn.del(key).await.unwrap();

    assert!(
        conn.sadd(key, "node-1").await.unwrap(),
        "a member that was not there should be reported as added"
    );
    assert!(
        !conn.sadd(key, "node-1").await.unwrap(),
        "a member that is already there should not be reported as added again"
    );
    conn.sadd(key, "node-2").await.unwrap();

    let mut members = conn.smembers(key).await.unwrap();
    members.sort();
    assert_eq!(members, vec!["node-1".to_string(), "node-2".to_string()]);

    assert!(conn.srem(key, "node-1").await.unwrap());
    assert!(
        !conn.srem(key, "node-1").await.unwrap(),
        "removing a member twice should say so the second time"
    );
    assert_eq!(
        conn.smembers(key).await.unwrap(),
        vec!["node-2".to_string()]
    );

    conn.del(key).await.unwrap();
    assert!(
        conn.smembers(key).await.unwrap().is_empty(),
        "a set that is gone should read as empty rather than fail"
    );
}

/// A key's presence and its removal, which is what a node's liveness comes down
/// to once the key carries an expiry.
#[test_log::test(tokio::test)]
async fn test_a_key_is_there_until_it_is_removed() {
    let mut conn = connection().await;
    let key = "helpers-test:key-presence";
    conn.del(key).await.unwrap();

    assert!(!conn.exists(key).await.unwrap());
    conn.pset_ex(key, "group-1", 10_000).await.unwrap();
    assert!(conn.exists(key).await.unwrap());

    assert!(conn.del(key).await.unwrap());
    assert!(
        !conn.del(key).await.unwrap(),
        "removing a key that has gone should say so rather than report a removal"
    );
    assert!(!conn.exists(key).await.unwrap());
}
