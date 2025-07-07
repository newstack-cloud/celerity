import subprocess
from typing import List
import os
import time

import pytest
import requests
from dotenv import dotenv_values


def test_http_endpoint(runtime_server):
    response = requests.post("http://localhost:22346/orders/2393483", json={})
    assert response.status_code == 200
    assert response.json() == {"message": "Order received"}


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
    if os.getenv("GITHUB_ACTIONS"):
        return [
            "python",
            "tests/server.py",
        ]
    return [
        "pipenv",
        "run",
        "python",
        "tests/server.py",
    ]
