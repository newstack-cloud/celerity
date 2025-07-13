use std::collections::HashMap;

use celerity_ws_redis::pubsub::{connect, ConnectionConfig};
use celerity_ws_registry::types::{AckMessage, Message, WebSocketMessage};
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
            "api-node-2".to_string(),
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
            "api-node-1".to_string(),
            collect_client2_tx,
        )
        .await;
    });

    let mut collected = HashMap::<String, Vec<String>>::new();
    let mut collected_count = 0;
    let mut collected_acks = HashMap::<String, AckMessage>::new();
    let mut ack_count = 0;
    while collected_count < 200 || ack_count < 2 {
        tokio::select! {
            Some(message) = collect_client1_rx.recv() => {
                match message {
                    Message::WebSocket(message) => {
                        collected.entry(message.connection_id).or_default().push(message.message);
                        collected_count += 1;
                    }
                    Message::Ack(ack_message) => {
                        collected_acks.insert("node1".to_string(), ack_message);
                        ack_count += 1;
                    }
                }
            }
            Some(message) = collect_client2_rx.recv() => {
                match message {
                    Message::WebSocket(message) => {
                        collected.entry(message.connection_id).or_default().push(message.message);
                        collected_count += 1;
                    }
                    Message::Ack(ack_message) => {
                        collected_acks.insert("node2".to_string(), ack_message);
                        ack_count += 1;
                    }
                }
            }

        }
    }

    assert_eq!(collected.len(), 2);
    assert_eq!(collected["1"], build_message_list("1", 100));
    assert_eq!(collected["2"], build_message_list("2", 100));
    assert_eq!(collected_acks.len(), 2);
    // Node 1 should have received an acknowledgement for the message sent to node 2
    // that was forwarded for connection 2.
    assert_eq!(
        collected_acks["node1"],
        AckMessage {
            message_id: "conn-2-msg-99".to_string(),
            message_node: "api-node-1".to_string(),
        }
    );
    // Node 2 should have received an acknowledgement for the message sent to node 1
    // that was forwarded for connection 1.
    assert_eq!(
        collected_acks["node2"],
        AckMessage {
            message_id: "conn-1-msg-99".to_string(),
            message_node: "api-node-2".to_string(),
        }
    );
}

async fn send_messages_and_listen(
    dst_connection_ids: Vec<String>,
    src_client_tx: Sender<Message>,
    src_client_rx: &mut Receiver<Message>,
    other_node_name: String,
    collect_tx: Sender<Message>,
) {
    for connection_id in dst_connection_ids {
        for i in 0..100 {
            src_client_tx
                .send(Message::WebSocket(WebSocketMessage {
                    connection_id: connection_id.clone(),
                    message_id: format!("conn-{connection_id}-msg-{i}"),
                    source_node: "node1".to_string(),
                    message: format!("This is message {i} for {connection_id}"),
                    inform_clients_on_loss: None,
                    caller: None,
                }))
                .await
                .unwrap();
        }
    }

    while let Some(message) = src_client_rx.recv().await {
        if let Message::WebSocket(message) = message.clone() {
            if message.message_id.contains("msg-99") {
                let _ = src_client_tx
                    .send(Message::Ack(AckMessage {
                        message_id: message.message_id,
                        message_node: other_node_name.clone(),
                    }))
                    .await;
            }
        }
        collect_tx.send(message).await.unwrap();
    }
}

fn build_message_list(connection_id: &str, count: usize) -> Vec<String> {
    (0..count)
        .map(|i| format!("This is message {i} for {connection_id}"))
        .collect()
}
