import json
import os
import shutil
import subprocess
import tempfile
import time

import pytest
from dotenv import dotenv_values
from celerity_runtime_sdk import CoreRuntimeApplication, CoreRuntimeConfigBuilder
import redis as redis_lib

REDIS_URL = os.environ.get("CELERITY_LOCAL_REDIS_URL", "redis://127.0.0.1:6379")


def _config_for_port(port: int):
    return (
        CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/consumer-schedule.blueprint.yaml",
            service_name="python-sdk-consumer-config-test",
            server_port=port,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_test_mode(True)
        .build()
    )


def _parse_redis_url(url: str):
    """Parse a redis:// URL into (host, port)."""
    from urllib.parse import urlparse
    parsed = urlparse(url)
    return parsed.hostname or "127.0.0.1", parsed.port or 6379


def _wait_for_file(path: str, timeout: float = 10.0):
    """Poll for a file to appear, then return its JSON contents."""
    start = time.time()
    while time.time() - start < timeout:
        if os.path.exists(path):
            with open(path) as f:
                return json.load(f)
        time.sleep(0.5)
    raise TimeoutError(f"File {path} not created within {timeout}s")


# ---------------------------------------------------------------------------
# Config-only tests (no external dependencies)
# ---------------------------------------------------------------------------


def test_setup_returns_consumer_config():
    app = CoreRuntimeApplication(runtime_config=_config_for_port(22350))
    config = app.setup()
    assert config.consumers is not None
    consumers = config.consumers.consumers
    assert len(consumers) == 1
    assert consumers[0].source_id == "test-order-queue"
    assert consumers[0].batch_size == 10
    assert len(consumers[0].handlers) > 0
    assert consumers[0].handlers[0].name == "orderHandler"


def test_setup_returns_schedule_config():
    app = CoreRuntimeApplication(runtime_config=_config_for_port(22351))
    config = app.setup()
    assert config.schedules is not None
    schedules = config.schedules.schedules
    assert len(schedules) == 1
    assert schedules[0].schedule_value == "rate(1 day)"
    assert len(schedules[0].handlers) > 0
    assert schedules[0].handlers[0].name == "cleanupHandler"


# ---------------------------------------------------------------------------
# Fixtures for integration tests (require Valkey/Redis)
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def consumer_server():
    results_dir = tempfile.mkdtemp(prefix="celerity-consumer-test-")
    env = {
        **os.environ,
        **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
        "TEST_RESULTS_DIR": results_dir,
    }
    log_file = open("test-consumer-server.log", "w")
    server_proc = subprocess.Popen(
        ["uv", "run", "python", "tests/consumer_server.py"],
        stdout=log_file,
        stderr=subprocess.STDOUT,
        env=env,
    )
    # Wait for server + consumer polling to initialise.
    time.sleep(3)
    yield server_proc, results_dir
    server_proc.terminate()
    server_proc.wait(timeout=5)
    log_file.close()
    shutil.rmtree(results_dir, ignore_errors=True)


# ---------------------------------------------------------------------------
# Consumer integration tests
# ---------------------------------------------------------------------------


def test_consumer_receives_messages(consumer_server):
    _, results_dir = consumer_server
    host, port = _parse_redis_url(REDIS_URL)
    r = redis_lib.Redis(host=host, port=port)
    try:
        timestamp = str(int(time.time()))
        r.xadd(
            "celerity:queue:test-order-queue",
            {
                "body": json.dumps({"orderId": "order-1", "total": 42.5}),
                "timestamp": timestamp,
                "message_type": "0",
            },
        )

        result = _wait_for_file(os.path.join(results_dir, "consumer_result.json"))
        assert result["handler_tag"] == "source::test-order-queue::orderHandler"
        assert len(result["messages"]) > 0
        body = json.loads(result["messages"][0]["body"])
        assert body["orderId"] == "order-1"
        assert body["total"] == 42.5
    finally:
        r.close()


# ---------------------------------------------------------------------------
# Schedule integration tests
# ---------------------------------------------------------------------------


def test_schedule_handler_receives_trigger(consumer_server):
    _, results_dir = consumer_server
    # Get schedule_id from config.
    app = CoreRuntimeApplication(runtime_config=_config_for_port(22355))
    config = app.setup()
    assert config.schedules is not None
    schedule_id = config.schedules.schedules[0].schedule_id

    host, port = _parse_redis_url(REDIS_URL)
    r = redis_lib.Redis(host=host, port=port)
    try:
        timestamp = str(int(time.time()))
        r.xadd(
            f"celerity:schedules:{schedule_id}",
            {
                "body": json.dumps({"triggered": True}),
                "timestamp": timestamp,
                "message_type": "0",
            },
        )

        result = _wait_for_file(os.path.join(results_dir, "schedule_result.json"))
        assert result["handler_tag"] == "source::dailyCleanup::cleanupHandler"
        assert result["message_id"] is not None
        assert result["schedule_id"] is not None
    finally:
        r.close()


# ---------------------------------------------------------------------------
# Consumer batch processing
# ---------------------------------------------------------------------------


def test_consumer_receives_batch_messages(consumer_server):
    _, results_dir = consumer_server
    result_file = os.path.join(results_dir, "consumer_result.json")

    # Clear any existing result from previous test.
    if os.path.exists(result_file):
        os.remove(result_file)

    # Small delay to let the consumer finish its current poll cycle.
    time.sleep(2)

    host, port = _parse_redis_url(REDIS_URL)
    r = redis_lib.Redis(host=host, port=port)
    try:
        timestamp = str(int(time.time()))
        # Publish 5 messages atomically via pipeline so the consumer sees them
        # all in a single XREAD response rather than picking them up one at a time.
        pipe = r.pipeline()
        for i in range(5):
            pipe.xadd(
                "celerity:queue:test-order-queue",
                {
                    "body": json.dumps({"orderId": f"batch-{i}", "index": i}),
                    "timestamp": timestamp,
                    "message_type": "0",
                },
            )
        pipe.execute()

        result = _wait_for_file(result_file)
        assert len(result["messages"]) >= 2, (
            f"Expected batch of >=2 messages, got {len(result['messages'])}"
        )
    finally:
        r.close()
