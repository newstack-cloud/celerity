use celerity_helpers::{redis::ConnectionWrapper, testing::redis_connection};

use celerity_ws_redis::node_group::{join_or_create, leave, node_key, NodeGroup, NodeGroupConfig};

/// A node's configuration under a key prefix of its own, so tests running
/// beside each other are not each other's cluster.
fn config(prefix: &str, node: &str, capacity: usize, node_ttl_ms: u64) -> NodeGroupConfig {
    NodeGroupConfig {
        server_node_name: node.to_string(),
        capacity,
        node_ttl_ms,
        key_prefix: prefix.to_string(),
    }
}

/// Clears anything a previous run left behind under a prefix. A test that fails
/// never reaches its own cleanup, and the next run would otherwise be looking at
/// the last run's cluster.
async fn clear(conn: &mut ConnectionWrapper, prefix: &str) {
    let index_key = format!("{prefix}:{{group-meta}}:node-groups");
    for group_id in conn.smembers(&index_key).await.unwrap() {
        let members_key = format!("{prefix}:{{group-meta}}:node-group-members:{group_id}");
        for member in conn.smembers(&members_key).await.unwrap() {
            conn.del(&node_key(prefix, &member)).await.unwrap();
        }
        conn.del(&members_key).await.unwrap();
    }
    conn.del(&index_key).await.unwrap();
}

async fn members(conn: &mut ConnectionWrapper, prefix: &str, group: &NodeGroup) -> Vec<String> {
    conn.smembers(&format!(
        "{prefix}:{{group-meta}}:node-group-members:{}",
        group.id
    ))
    .await
    .unwrap()
}

async fn group_ids(conn: &mut ConnectionWrapper, prefix: &str) -> Vec<String> {
    conn.smembers(&format!("{prefix}:{{group-meta}}:node-groups"))
        .await
        .unwrap()
}

/// Nodes fill a group before starting another one.
#[test_log::test(tokio::test)]
async fn test_a_group_is_filled_before_the_next_one_is_started() {
    let mut conn = redis_connection().await;
    let prefix = "test-fills-first";
    clear(&mut conn, prefix).await;

    let first = join_or_create(&mut conn, &config(prefix, "node-1", 2, 10_000))
        .await
        .unwrap();
    let second = join_or_create(&mut conn, &config(prefix, "node-2", 2, 10_000))
        .await
        .unwrap();
    assert_eq!(
        first, second,
        "a group with room should take the next node rather than a new group being started"
    );

    let third = join_or_create(&mut conn, &config(prefix, "node-3", 2, 10_000))
        .await
        .unwrap();
    assert_ne!(
        third.id, first.id,
        "a group at capacity should send the next node to a group of its own"
    );

    // Named from the group's id, which is what a node subscribes to and what
    // another node publishes to.
    assert_eq!(third.channel, format!("{prefix}:node-group:{}", third.id));
    assert_eq!(
        third.ack_channel,
        format!("{prefix}:node-group-ack:{}", third.id)
    );

    for node in ["node-1", "node-2"] {
        leave(&mut conn, &config(prefix, node, 2, 10_000), &first)
            .await
            .unwrap();
    }
    leave(&mut conn, &config(prefix, "node-3", 2, 10_000), &third)
        .await
        .unwrap();
}

/// A node that stops saying it is running stops holding a place in its group.
///
/// This is the case a subscriber count cannot see. The node is still in the
/// member set, and only its liveness key having expired tells the next node
/// that the place is free.
#[test_log::test(tokio::test)]
async fn test_a_node_that_stopped_running_gives_up_its_place() {
    let mut conn = redis_connection().await;
    let prefix = "test-expired-node";
    clear(&mut conn, prefix).await;

    let group = join_or_create(&mut conn, &config(prefix, "gone-node", 1, 200))
        .await
        .unwrap();
    assert!(conn.exists(&node_key(prefix, "gone-node")).await.unwrap());

    // Long enough for the liveness key to run out without it being refreshed,
    // which is what a node dying looks like from anywhere else.
    tokio::time::sleep(std::time::Duration::from_millis(400)).await;

    let joined = join_or_create(&mut conn, &config(prefix, "live-node", 1, 10_000))
        .await
        .unwrap();
    assert_eq!(
        joined, group,
        "a group whose only node has gone should have room again rather than being full"
    );

    let members = members(&mut conn, prefix, &group).await;
    assert_eq!(
        members,
        vec!["live-node".to_string()],
        "the node that stopped running should have been dropped from the member set"
    );

    leave(&mut conn, &config(prefix, "live-node", 1, 10_000), &group)
        .await
        .unwrap();
}

/// Leaving takes everything the node wrote with it, rather than leaving the
/// group looking fuller than it is until a TTL runs out.
#[test_log::test(tokio::test)]
async fn test_leaving_takes_the_membership_and_the_group_with_it() {
    let mut conn = redis_connection().await;
    let prefix = "test-graceful-leave";
    clear(&mut conn, prefix).await;
    let node_config = config(prefix, "node-1", 5, 10_000);

    let group = join_or_create(&mut conn, &node_config).await.unwrap();
    leave(&mut conn, &node_config, &group).await.unwrap();

    assert!(
        !conn.exists(&node_key(prefix, "node-1")).await.unwrap(),
        "a node that left should not still be saying it is running"
    );
    assert!(members(&mut conn, prefix, &group).await.is_empty());
    assert!(
        group_ids(&mut conn, prefix).await.is_empty(),
        "a group nobody is in should not be left for the next node to find"
    );
}

/// A node already in a group keeps the place it has.
///
/// This is what makes the join the heartbeat as well. Every beat runs it, and a
/// node that is still a member comes back to the same group rather than being
/// counted into another one.
#[test_log::test(tokio::test)]
async fn test_a_node_already_in_a_group_keeps_its_place() {
    let mut conn = redis_connection().await;
    let prefix = "test-keeps-place";
    clear(&mut conn, prefix).await;
    let node_config = config(prefix, "node-1", 5, 10_000);

    let joined = join_or_create(&mut conn, &node_config).await.unwrap();
    let again = join_or_create(&mut conn, &node_config).await.unwrap();
    assert_eq!(joined, again);
    assert_eq!(
        members(&mut conn, prefix, &joined).await,
        vec!["node-1".to_string()],
        "joining again should not count the node twice"
    );

    leave(&mut conn, &node_config, &joined).await.unwrap();
}

/// A node dropped from its group takes whatever place is free, which may be in
/// another group.
///
/// Nothing keeps a place for a node that stopped saying it was running, so
/// returning is an ordinary join and the group it lands in is whichever has
/// room.
#[test_log::test(tokio::test)]
async fn test_a_dropped_node_takes_a_free_place_wherever_it_is() {
    let mut conn = redis_connection().await;
    let prefix = "test-dropped-takes-free";
    clear(&mut conn, prefix).await;

    let first = join_or_create(&mut conn, &config(prefix, "node-1", 1, 10_000))
        .await
        .unwrap();
    // Dropped as another node would have dropped it, while a second node holds
    // the only place the group has.
    conn.srem(
        &format!("{prefix}:{{group-meta}}:node-group-members:{}", first.id),
        "node-1",
    )
    .await
    .unwrap();
    let second = join_or_create(&mut conn, &config(prefix, "node-2", 1, 10_000))
        .await
        .unwrap();
    assert_eq!(
        second, first,
        "the freed place should be taken by the next node"
    );

    let returned = join_or_create(&mut conn, &config(prefix, "node-1", 1, 10_000))
        .await
        .unwrap();
    assert_ne!(
        returned.id, first.id,
        "a node returning to a group that is full should take a place elsewhere"
    );

    leave(&mut conn, &config(prefix, "node-2", 1, 10_000), &first)
        .await
        .unwrap();
    leave(&mut conn, &config(prefix, "node-1", 1, 10_000), &returned)
        .await
        .unwrap();
}

/// Nodes starting together do not overfill a group.
///
/// The case capacity is most likely to meet, since a rolling deploy or a
/// scale-out starts nodes at the same moment rather than one at a time. Reading
/// the members and adding to them separately would let every one of these see
/// room that the others were about to take.
#[test_log::test(tokio::test)]
async fn test_nodes_starting_together_do_not_overfill_a_group() {
    let mut conn = redis_connection().await;
    let prefix = "test-concurrent-join";
    clear(&mut conn, prefix).await;

    let joins = (0..12).map(|index| {
        let node_config = config(prefix, &format!("node-{index}"), 2, 10_000);
        async move {
            let mut conn = redis_connection().await;
            join_or_create(&mut conn, &node_config).await.unwrap()
        }
    });
    let joined: Vec<_> = futures::future::join_all(joins).await;

    for group_id in group_ids(&mut conn, prefix).await {
        let members = conn
            .smembers(&format!(
                "{prefix}:{{group-meta}}:node-group-members:{group_id}"
            ))
            .await
            .unwrap();
        assert!(
            members.len() <= 2,
            "a group of capacity 2 should never hold {} nodes, it holds {members:?}",
            members.len()
        );
    }

    assert_eq!(
        joined.len(),
        12,
        "every node should have come away with a group"
    );

    for (index, group) in joined.iter().enumerate() {
        leave(
            &mut conn,
            &config(prefix, &format!("node-{index}"), 2, 10_000),
            group,
        )
        .await
        .unwrap();
    }
}
