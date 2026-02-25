import json
import os

from celerity_runtime_sdk import (
    WebSocketMessageInfo,
    WebSocketMessageType,
)
import shared


async def on_connect(msg: WebSocketMessageInfo) -> None:
    ws_registry = shared.get_ws_registry()
    await ws_registry.send_message(
        connection_id=msg.connection_id,
        message_id=msg.message_id,
        message_type=WebSocketMessageType.JSON,
        message=json.dumps({
            "event": "connected",
            "connectionId": msg.connection_id,
            "eventType": str(msg.event_type),
        }),
        ctx=None,
    )


async def on_default(msg: WebSocketMessageInfo) -> None:
    ws_registry = shared.get_ws_registry()
    await ws_registry.send_message(
        connection_id=msg.connection_id,
        message_id=msg.message_id,
        message_type=WebSocketMessageType.JSON,
        message=json.dumps({
            "event": "default",
            "connectionId": msg.connection_id,
        }),
        ctx=None,
    )


async def on_disconnect(msg: WebSocketMessageInfo) -> None:
    results_dir = os.environ.get("TEST_RESULTS_DIR", "/tmp/celerity-ws-test-results")
    os.makedirs(results_dir, exist_ok=True)
    result_path = os.path.join(results_dir, "disconnect_result.json")
    with open(result_path, "w") as f:
        json.dump({
            "event": "disconnected",
            "connectionId": msg.connection_id,
        }, f)


async def echo(msg: WebSocketMessageInfo) -> None:
    ws_registry = shared.get_ws_registry()
    request_context = None
    if msg.request_context:
        request_context = {
            "requestId": msg.request_context.request_id,
            "path": msg.request_context.path,
            "clientIp": msg.request_context.client_ip,
        }
    await ws_registry.send_message(
        connection_id=msg.connection_id,
        message_id=msg.message_id,
        message_type=WebSocketMessageType.JSON,
        message=json.dumps({
            "event": "echo",
            "body": msg.json_body,
            "messageType": str(msg.type),
            "eventType": str(msg.event_type),
            "connectionId": msg.connection_id,
            "requestContext": request_context,
        }),
        ctx=None,
    )
