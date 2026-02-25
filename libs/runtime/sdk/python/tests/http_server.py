import importlib
import pathlib
import sys
from collections.abc import Callable
from os import path

from celerity_runtime_sdk import (
    CoreRuntimeApplication,
    CoreRuntimeConfigBuilder,
    RuntimePlatform,
)


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/http-api.blueprint.yaml",
            service_name="python-sdk-http-test",
            server_port=22360,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_platform(RuntimePlatform.LOCAL)
        .set_test_mode(True)
        .build()
    )
    config = app.setup()

    if config.api and config.api.http:
        for http_handler in config.api.http.handlers:
            handler_fn = _load_handler(http_handler.location, http_handler.handler)
            timeout = 1 if http_handler.path == "/slow" else None
            app.register_http_handler(
                path=http_handler.path,
                method=http_handler.method,
                handler=handler_fn,
                timeout_seconds=timeout,
            )

    app.run()


def _load_handler(location: str, handler_path: str) -> Callable:
    full_path = pathlib.Path(path.dirname(__file__), location)
    sys.path.append(str(full_path))

    segments = handler_path.rsplit(".", 1)
    if len(segments) != 2:
        raise ValueError(f"Invalid handler path: {handler_path}")

    module_name, function_name = segments
    module = importlib.import_module(module_name)
    return getattr(module, function_name)


if __name__ == "__main__":
    run_server()
