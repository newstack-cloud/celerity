import json
import os
import subprocess
import time

import pytest
import requests
from dotenv import dotenv_values

CUSTOM_HANDLER_SERVER_PORT = 22356
SLOW_SERVER_PORT = 22349


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def custom_handler_server():
    env = {
        **os.environ,
        **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
    }
    log_file = open("test-custom-handler-server.log", "w")
    server_proc = subprocess.Popen(
        ["uv", "run", "python", "tests/custom_handler_server.py"],
        stdout=log_file,
        stderr=subprocess.STDOUT,
        env=env,
    )
    time.sleep(3)
    yield server_proc
    server_proc.terminate()
    server_proc.wait(timeout=5)
    log_file.close()


@pytest.fixture(scope="module")
def slow_custom_handler_server():
    env = {
        **os.environ,
        **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
    }
    log_file = open("test-slow-custom-handler-server.log", "w")
    server_proc = subprocess.Popen(
        ["uv", "run", "python", "tests/slow_custom_handler_server.py"],
        stdout=log_file,
        stderr=subprocess.STDOUT,
        env=env,
    )
    time.sleep(3)
    yield server_proc
    server_proc.terminate()
    server_proc.wait(timeout=5)
    log_file.close()


# ---------------------------------------------------------------------------
# Custom handler integration tests
# ---------------------------------------------------------------------------


def test_custom_handler_invocation(custom_handler_server):
    response = requests.post(
        f"http://localhost:{CUSTOM_HANDLER_SERVER_PORT}/runtime/handlers/invoke",
        json={
            "handlerName": "utilityHandler",
            "invocationType": "requestResponse",
            "payload": {"input": "test-data"},
        },
    )
    assert response.status_code == 200
    body = response.json()
    assert body["message"] == "Handler invoked successfully"
    data = json.loads(body["data"])
    assert data["received"] == {"input": "test-data"}
    assert data["echoed"] is True


def test_custom_handler_timeout(slow_custom_handler_server):
    response = requests.post(
        f"http://localhost:{SLOW_SERVER_PORT}/runtime/handlers/invoke",
        json={
            "handlerName": "utilityHandler",
            "invocationType": "requestResponse",
            "payload": {},
        },
    )
    assert response.status_code == 500
    body = response.json()
    assert "timed out" in body["message"]
