import subprocess
from typing import List
import os
import time
import json

import pytest
import requests
from dotenv import dotenv_values
from websocket import create_connection
from celerity_runtime_sdk import CoreRuntimeConfigBuilder, RuntimePlatform


def test_http_endpoint(runtime_server):
    response = requests.get("http://localhost:22346/orders/2393483", json={})
    assert response.status_code == 200
    assert response.json() == {"message": "Order received"}


def test_websockets(runtime_server):
    ws = create_connection("ws://localhost:22346/ws")
    ws.send(json.dumps({"action": "processOrderUpdate", "data": {"orderId": "1234"}}))
    msg = ws.recv()
    assert msg == '{"action": "processedOrderUpdate", "message": "Order received"}'
    ws.close()


def test_core_runtime_config_builder_with_defaults():
    builder = CoreRuntimeConfigBuilder(
        blueprint_config_path="tests/blueprint.yaml",
        service_name="test-service",
        server_port=22346,
    ).set_api_resource("testApi")

    config = builder.build()
    assert config.blueprint_config_path == "tests/blueprint.yaml"
    assert config.service_name == "test-service"
    assert config.server_port == 22346
    assert config.trace_otlp_collector_endpoint == "http://otelcollector:4317"
    assert config.runtime_max_diagnostics_level == "info"
    assert config.platform == RuntimePlatform.OTHER
    assert config.test_mode is False
    assert config.api_resource == "testApi"
    assert config.resource_store_verify_tls is True
    assert config.resource_store_cache_entry_ttl == 600
    assert config.resource_store_cleanup_interval == 3600

    # Defaults for the following fields will be selected by the runtime
    # application that the config is passed to.
    assert config.server_loopback_only is None
    assert config.use_custom_health_check is None
    assert config.consumer_app is None
    assert config.schedule_app is None


def test_core_runtime_config_builder_with_overrides():
    builder = CoreRuntimeConfigBuilder(
        blueprint_config_path="tests/blueprint.yaml",
        service_name="test-service",
        server_port=22346,
    )
    config = (
        builder
        .set_server_loopback_only(True)
        .set_use_custom_health_check(True)
        .set_trace_otlp_collector_endpoint("http://otelcollector-alt:44317")
        .set_runtime_max_diagnostics_level("debug")
        .set_platform(RuntimePlatform.AWS)
        .set_test_mode(True)
        .set_api_resource("testApi")
        .set_consumer_app("testConsumer")
        .set_schedule_app("testSchedule")
        .set_resource_store_verify_tls(False)
        .set_resource_store_cache_entry_ttl(1200)
        .set_resource_store_cleanup_interval(7200)
        .build()
    )
    assert config.blueprint_config_path == "tests/blueprint.yaml"
    assert config.service_name == "test-service"
    assert config.server_port == 22346
    assert config.server_loopback_only is True
    assert config.use_custom_health_check is True
    assert config.trace_otlp_collector_endpoint == "http://otelcollector-alt:44317"
    assert config.runtime_max_diagnostics_level == "debug"
    assert config.platform == RuntimePlatform.AWS
    assert config.test_mode is True
    assert config.api_resource == "testApi"
    assert config.consumer_app == "testConsumer"
    assert config.schedule_app == "testSchedule"
    assert config.resource_store_verify_tls is False
    assert config.resource_store_cache_entry_ttl == 1200
    assert config.resource_store_cleanup_interval == 7200


@pytest.fixture(scope="session")
def runtime_server(command_args: List[str]):
    with open("test-server.log", "w") as log_file:
        server_proc = subprocess.Popen(
            command_args,
            stdout=log_file,
            stderr=log_file,
            env={
                **os.environ,
                **{k: v for k, v in dotenv_values(dotenv_path=".env.test").items() if v is not None},
            }
        )
    # Give the server time to start up.
    time.sleep(2)

    yield server_proc
    server_proc.terminate()


@pytest.fixture(name="command_args", scope="session")
def fixture_command_args() -> List[str]:
    return [
        "uv",
        "run",
        "python",
        "tests/server.py",
    ]
