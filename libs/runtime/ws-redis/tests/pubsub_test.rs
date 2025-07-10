use std::collections::HashMap;

use celerity_ws_redis::pubsub::{connect, ConnectionConfig};
use celerity_ws_registry::types::WebSocketMessage;
use tokio::sync::mpsc::{channel, Receiver, Sender};

#[test_log::test(tokio::test)]
async fn test_publish_and_subscribe_to_redis_channel() {
    let nodes = vec!["redis://127.0.0.1:6379/?protocol=resp3".to_string()];
    let (client1_tx, mut client1_rx) = connect(ConnectionConfig {
        server_node_name: "api-node-1".to_string(),
        channel_name: "celerity_ws_messages".to_string(),
        nodes: nodes.clone(),
        password: None,
        cluster_mode: false,
    })
    .await
    .unwrap();

    let (client2_tx, mut client2_rx) = connect(ConnectionConfig {
        server_node_name: "api-node-2".to_string(),
        channel_name: "celerity_ws_messages".to_string(),
        nodes: nodes.clone(),
        password: None,
        cluster_mode: false,
    })
    .await
    .unwrap();

    let (collect_client1_tx, mut collect_client1_rx) = channel(1024);
    let (collect_client2_tx, mut collect_client2_rx) = channel(1024);

    tokio::spawn(async move {
        // Client 1 sends messages that are intended for client 2.
        send_messages_and_listen(
            vec!["2".to_string()],
            client1_tx,
            &mut client1_rx,
            collect_client1_tx,
        )
        .await;
    });

    tokio::spawn(async move {
        // Client 2 sends messages that are intended for client 1.
        send_messages_and_listen(
            vec!["1".to_string()],
            client2_tx,
            &mut client2_rx,
            collect_client2_tx,
        )
        .await;
    });

    let mut collected = HashMap::<String, Vec<String>>::new();
    let mut collected_count = 0;
    while collected_count < 200 {
        tokio::select! {
            Some(message) = collect_client1_rx.recv() => {
                collected.entry(message.connection_id).or_default().push(message.message);
                collected_count += 1;
            }
            Some(message) = collect_client2_rx.recv() => {
                collected.entry(message.connection_id).or_default().push(message.message);
                collected_count += 1;
            }

        }
    }

    assert_eq!(collected.len(), 2);
    assert_eq!(collected["1"], build_message_list("1", 100));
    assert_eq!(collected["2"], build_message_list("2", 100));
}

async fn send_messages_and_listen(
    dst_connection_ids: Vec<String>,
    src_client_tx: Sender<WebSocketMessage>,
    src_client_rx: &mut Receiver<WebSocketMessage>,
    collect_tx: Sender<WebSocketMessage>,
) {
    for connection_id in dst_connection_ids {
        for i in 0..100 {
            src_client_tx
                .send(WebSocketMessage {
                    connection_id: connection_id.clone(),
                    message: format!("This is message {i} for {connection_id}"),
                })
                .await
                .unwrap();
        }
    }

    while let Some(message) = src_client_rx.recv().await {
        collect_tx.send(message).await.unwrap();
    }
}

fn build_message_list(connection_id: &str, count: usize) -> Vec<String> {
    (0..count)
        .map(|i| format!("This is message {i} for {connection_id}"))
        .collect()
}
