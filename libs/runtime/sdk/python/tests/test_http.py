import json
import os
import subprocess
import time

import pytest
import requests
from dotenv import dotenv_values
from celerity_runtime_sdk import CoreRuntimeApplication, CoreRuntimeConfigBuilder

HTTP_SERVER_PORT = 22360
BASE = f"http://localhost:{HTTP_SERVER_PORT}"


def _config_for_port(port: int):
    return (
        CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/http-api.blueprint.yaml",
            service_name="python-sdk-http-config-test",
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
def http_server():
    env = {
        **os.environ,
        **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
    }
    log_file = open("test-http-server.log", "w")
    server_proc = subprocess.Popen(
        ["uv", "run", "python", "tests/http_server.py"],
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
# 1. Config test
# ---------------------------------------------------------------------------


def test_setup_returns_handler_definitions():
    app = CoreRuntimeApplication(runtime_config=_config_for_port(22361))
    config = app.setup()
    assert config.api is not None
    assert config.api.http is not None

    handlers = config.api.http.handlers
    assert len(handlers) == 8

    routes = sorted(f"{h.method} {h.path}" for h in handlers)
    assert routes == [
        "DELETE /items/{itemId}",
        "GET /items",
        "GET /items/{itemId}",
        "GET /slow",
        "POST /binary-echo",
        "POST /custom-status",
        "POST /items",
        "PUT /items/{itemId}",
    ]

    for h in handlers:
        assert h.timeout == 60
        assert h.handler
        assert h.location


# ---------------------------------------------------------------------------
# 2-5. Basic HTTP methods
# ---------------------------------------------------------------------------


def test_get_request(http_server):
    response = requests.get(f"{BASE}/items")
    assert response.status_code == 200
    echo = response.json()
    assert echo["method"] == "GET"


def test_post_request_with_json_body(http_server):
    response = requests.post(
        f"{BASE}/items",
        json={"name": "widget"},
    )
    assert response.status_code == 200
    echo = response.json()
    assert echo["method"] == "POST"
    assert echo["contentType"] == "application/json"
    assert json.loads(echo["textBody"]) == {"name": "widget"}


def test_put_request(http_server):
    response = requests.put(
        f"{BASE}/items/1",
        json={"name": "updated"},
    )
    assert response.status_code == 200
    echo = response.json()
    assert echo["method"] == "PUT"


def test_delete_request(http_server):
    response = requests.delete(f"{BASE}/items/1")
    assert response.status_code == 200
    echo = response.json()
    assert echo["method"] == "DELETE"


# ---------------------------------------------------------------------------
# 6. Custom response status and headers
# ---------------------------------------------------------------------------


def test_custom_response_status_and_headers(http_server):
    response = requests.post(f"{BASE}/custom-status")
    assert response.status_code == 201
    assert response.headers.get("x-custom-header") == "custom-value"
    body = response.json()
    assert body == {"created": True}


# ---------------------------------------------------------------------------
# 7-12. Request getters
# ---------------------------------------------------------------------------


def test_path_params(http_server):
    response = requests.get(f"{BASE}/items/42")
    echo = response.json()
    assert echo["path"] == "/items/42"
    assert echo["pathParams"] == {"itemId": "42"}


def test_query_parameters(http_server):
    response = requests.get(f"{BASE}/items", params={"color": "red", "tag": ["a", "b"]})
    echo = response.json()
    assert echo["query"]["color"] == ["red"]
    assert echo["query"]["tag"] == ["a", "b"]


def test_multi_valued_headers(http_server):
    response = requests.get(
        f"{BASE}/items",
        headers={"x-custom": "a, b"},
    )
    echo = response.json()
    custom_values = echo["headers"].get("x-custom", [])
    joined = ", ".join(custom_values)
    assert "a" in joined
    assert "b" in joined


def test_cookie_parsing(http_server):
    response = requests.get(
        f"{BASE}/items",
        cookies={"session": "abc123", "theme": "dark"},
    )
    echo = response.json()
    assert echo["cookies"]["session"] == "abc123"
    assert echo["cookies"]["theme"] == "dark"


def test_request_metadata(http_server):
    response = requests.get(
        f"{BASE}/items",
        headers={"User-Agent": "test-agent/1.0"},
    )
    echo = response.json()
    assert echo["requestId"]
    assert len(echo["requestId"]) > 0
    assert echo["requestTime"]
    assert isinstance(echo["clientIp"], str)
    assert isinstance(echo["userAgent"], str)


def test_auth_is_null(http_server):
    response = requests.get(f"{BASE}/items")
    echo = response.json()
    assert echo["auth"] is None


# ---------------------------------------------------------------------------
# 13-14. Body handling
# ---------------------------------------------------------------------------


def test_binary_request_body(http_server):
    binary_payload = bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A])
    response = requests.post(
        f"{BASE}/binary-echo",
        data=binary_payload,
        headers={"Content-Type": "application/octet-stream"},
    )
    assert response.status_code == 200
    body = response.json()
    assert body["binaryLength"] == 6


def test_binary_response_body(http_server):
    response = requests.post(
        f"{BASE}/binary-echo",
        data="give-me-binary",
        headers={"Content-Type": "text/plain"},
    )
    assert response.status_code == 200
    assert response.headers.get("content-type") == "application/octet-stream"
    assert list(response.content) == [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]


# ---------------------------------------------------------------------------
# 15-16. x-request-id
# ---------------------------------------------------------------------------


def test_auto_generated_request_id(http_server):
    response = requests.get(f"{BASE}/items")
    req_id = response.headers.get("x-request-id")
    assert req_id
    assert len(req_id) > 0


def test_client_provided_request_id(http_server):
    response = requests.get(
        f"{BASE}/items",
        headers={"x-request-id": "test-req-123"},
    )
    assert response.headers.get("x-request-id") == "test-req-123"
    echo = response.json()
    assert echo["requestId"] == "test-req-123"


# ---------------------------------------------------------------------------
# 17. Timeout
# ---------------------------------------------------------------------------


def test_handler_timeout_returns_504(http_server):
    response = requests.get(f"{BASE}/slow")
    assert response.status_code == 504
    body = response.json()
    assert "handler timed out" in body["message"]


# ---------------------------------------------------------------------------
# 18-19. CORS
# ---------------------------------------------------------------------------


def test_cors_preflight(http_server):
    response = requests.options(
        f"{BASE}/items",
        headers={
            "Origin": "https://example.com",
            "Access-Control-Request-Method": "GET",
        },
    )
    assert response.status_code == 200
    assert response.headers.get("access-control-allow-origin") == "https://example.com"
    assert response.headers.get("access-control-allow-methods")


def test_cors_disallowed_origin(http_server):
    response = requests.options(
        f"{BASE}/items",
        headers={
            "Origin": "https://evil.com",
            "Access-Control-Request-Method": "GET",
        },
    )
    assert response.headers.get("access-control-allow-origin") is None
