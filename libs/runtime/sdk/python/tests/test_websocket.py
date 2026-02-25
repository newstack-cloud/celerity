import json
import os
import shutil
import subprocess
import tempfile
import time

import pytest
from dotenv import dotenv_values
from websocket import create_connection
from celerity_runtime_sdk import CoreRuntimeApplication, CoreRuntimeConfigBuilder

WS_SERVER_PORT = 22362
WS_URL = f"ws://localhost:{WS_SERVER_PORT}/ws"


def _config_for_port(port: int):
    return (
        CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/ws-only.blueprint.yaml",
            service_name="python-sdk-ws-config-test",
            server_port=port,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_test_mode(True)
        .build()
    )


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def ws_server():
    results_dir = tempfile.mkdtemp(prefix="celerity-ws-test-")
    env = {
        **os.environ,
        **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
        "TEST_RESULTS_DIR": results_dir,
    }
    log_file = open("test-ws-server.log", "w")
    server_proc = subprocess.Popen(
        ["uv", "run", "python", "tests/ws_server.py"],
        stdout=log_file,
        stderr=subprocess.STDOUT,
        env=env,
    )
    time.sleep(3)
    yield server_proc, results_dir
    server_proc.terminate()
    server_proc.wait(timeout=5)
    log_file.close()
    shutil.rmtree(results_dir, ignore_errors=True)


def _ws_connect(query: str = ""):
    url = f"{WS_URL}?{query}" if query else WS_URL
    return create_connection(url, origin="https://example.com")


def _wait_for_file(path: str, timeout: float = 10.0):
    start = time.time()
    while time.time() - start < timeout:
        if os.path.exists(path):
            with open(path) as f:
                return json.load(f)
        time.sleep(0.5)
    raise TimeoutError(f"File {path} not created within {timeout}s")


# ---------------------------------------------------------------------------
# 1. Config test
# ---------------------------------------------------------------------------


def test_setup_returns_websocket_config():
    app = CoreRuntimeApplication(runtime_config=_config_for_port(22363))
    config = app.setup()
    assert config.api is not None
    assert config.api.websocket is not None

    handlers = config.api.websocket.handlers
    assert len(handlers) == 4

    routes = sorted(h.route for h in handlers)
    assert routes == ["$connect", "$default", "$disconnect", "echo"]

    for h in handlers:
        assert h.timeout == 60
        assert h.handler
        assert h.location


# ---------------------------------------------------------------------------
# 2. Connect triggers handler
# ---------------------------------------------------------------------------


def test_ws_connect_triggers_handler(ws_server):
    ws = _ws_connect()
    try:
        msg = json.loads(ws.recv())
        assert msg["event"] == "connected"
        assert msg["connectionId"]
        assert "CONNECT" in msg["eventType"]
    finally:
        ws.close()


# ---------------------------------------------------------------------------
# 3. JSON message routes to echo
# ---------------------------------------------------------------------------


def test_ws_json_message_routes_to_echo(ws_server):
    ws = _ws_connect()
    try:
        # Consume the connect message first.
        ws.recv()

        ws.send(json.dumps({"action": "echo", "data": "hello"}))
        msg = json.loads(ws.recv())
        assert msg["event"] == "echo"
        assert msg["body"]["data"] == "hello"
        assert msg["connectionId"]
    finally:
        ws.close()


# ---------------------------------------------------------------------------
# 4. Default route
# ---------------------------------------------------------------------------


def test_ws_default_route(ws_server):
    ws = _ws_connect()
    try:
        # Consume the connect message.
        ws.recv()

        ws.send(json.dumps({"action": "unknownAction", "data": "test"}))
        msg = json.loads(ws.recv())
        assert msg["event"] == "default"
        assert msg["connectionId"]
    finally:
        ws.close()


# ---------------------------------------------------------------------------
# 5. Disconnect triggers handler
# ---------------------------------------------------------------------------


def test_ws_disconnect_triggers_handler(ws_server):
    _, results_dir = ws_server
    disconnect_file = os.path.join(results_dir, "disconnect_result.json")

    # Clear any previous disconnect result.
    if os.path.exists(disconnect_file):
        os.remove(disconnect_file)

    ws = _ws_connect()
    # Consume the connect message.
    ws.recv()
    ws.close()

    result = _wait_for_file(disconnect_file)
    assert result["event"] == "disconnected"
    assert result["connectionId"]


# ---------------------------------------------------------------------------
# 6. Echo handler sends via registry
# ---------------------------------------------------------------------------


def test_ws_registry_send_message(ws_server):
    ws = _ws_connect()
    try:
        # Consume the connect message.
        ws.recv()

        ws.send(json.dumps({"action": "echo", "payload": {"key": "value"}}))
        msg = json.loads(ws.recv())
        # The echo handler uses ws_registry.send_message to send back.
        assert msg["event"] == "echo"
        assert msg["body"]["payload"] == {"key": "value"}
    finally:
        ws.close()


# ---------------------------------------------------------------------------
# 7. Request context
# ---------------------------------------------------------------------------


def test_ws_request_context(ws_server):
    ws = _ws_connect(query="token=abc")
    try:
        # Consume the connect message.
        ws.recv()

        ws.send(json.dumps({"action": "echo", "data": "ctx-test"}))
        msg = json.loads(ws.recv())
        assert msg["requestContext"] is not None
        assert msg["requestContext"]["requestId"]
        assert msg["requestContext"]["clientIp"]
        # The path field contains only the URI path (no query string).
        # Query params are stored separately in the request context.
        assert msg["requestContext"]["path"] == "/ws"
    finally:
        ws.close()
