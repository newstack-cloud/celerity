import importlib
from typing import Callable, Awaitable
from os import path
import sys
import pathlib

from celerity_runtime_sdk import (
    CoreRuntimeApplication,
    CoreRuntimeConfigBuilder,
    Response,
    Request,
    RequestContext,
    WebSocketMessageInfo,
)
import shared


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/hybrid-api.blueprint.yaml",
            service_name="test-service",
            server_port=22346,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("http://localhost:4317")
        .build()
    )
    runtime_app_config = app.setup()
    ws_registry = app.websocket_registry()
    shared.set_ws_registry(ws_registry)

    if runtime_app_config.api is None or runtime_app_config.api.http is None:
        raise ValueError("No HTTP API configuration found in blueprint")

    for handler in runtime_app_config.api.http.handlers:
        print(handler.path, handler.method, handler.location, handler.handler)
        http_handler_callable = load_http_handler(handler.location, handler.handler)
        app.register_http_handler(
            path=handler.path,
            method=handler.method,
            handler=http_handler_callable,
        )

    if runtime_app_config.api.websocket is None:
        raise ValueError("No WebSocket API configuration found in blueprint")

    for ws_handler in runtime_app_config.api.websocket.handlers:
        print(ws_handler.route, ws_handler.handler)
        ws_handler_callable = load_ws_handler(ws_handler.location, ws_handler.handler)
        app.register_websocket_handler(
            route=ws_handler.route,
            handler=ws_handler_callable,
        )

    app.run()


WebSocektHandler = Callable[[WebSocketMessageInfo], Awaitable[None]]


def load_ws_handler(location: str, handler: str) -> WebSocektHandler:
    return _load_handler(location, handler)


HttpHandler = Callable[[Request, RequestContext], Awaitable[Response]]


def load_http_handler(location: str, handler: str) -> HttpHandler:
    return _load_handler(location, handler)


def _load_handler(location: str, handler: str) -> Callable:
    full_path = pathlib.Path(path.dirname(__file__), location)
    sys.path.append(str(full_path))

    segments = handler.rsplit(".", 1)
    if len(segments) != 2:
        raise ValueError(f"Invalid handler path: {handler}")

    module_name, function_name = segments
    module = importlib.import_module(module_name)
    return getattr(module, function_name)


if __name__ == "__main__":
    run_server()
