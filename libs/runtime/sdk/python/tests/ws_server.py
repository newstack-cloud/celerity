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
import shared


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/ws-only.blueprint.yaml",
            service_name="python-sdk-ws-test",
            server_port=22362,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_platform(RuntimePlatform.LOCAL)
        .set_test_mode(True)
        .build()
    )
    config = app.setup()

    ws_registry = app.websocket_registry()
    shared.set_ws_registry(ws_registry)

    if config.api and config.api.websocket:
        for ws_handler in config.api.websocket.handlers:
            print(f"Registering WS handler: {ws_handler.route} {ws_handler.handler}")
            handler_fn = _load_handler(ws_handler.location, ws_handler.handler)
            app.register_websocket_handler(
                route=ws_handler.route,
                handler=handler_fn,
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
