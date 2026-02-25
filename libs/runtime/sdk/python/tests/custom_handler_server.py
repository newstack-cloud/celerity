import importlib
import pathlib
import sys
from collections.abc import Callable
from os import path

from celerity_runtime_sdk import (
    CoreRuntimeApplication,
    CoreRuntimeConfigBuilder,
    Request,
    RequestContext,
    Response,
    ResponseBuilder,
    RuntimePlatform,
)


async def health_check(req: Request, ctx: RequestContext) -> Response:
    return (
        ResponseBuilder()
        .set_status(200)
        .set_json_body({"status": "ok"})
        .build()
    )


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/consumer-schedule.blueprint.yaml",
            service_name="python-sdk-custom-handler-test",
            server_port=22356,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_platform(RuntimePlatform.LOCAL)
        .set_test_mode(True)
        .build()
    )
    config = app.setup()

    # Register HTTP handlers (required for server to start).
    if config.api and config.api.http:
        for http_handler in config.api.http.handlers:
            app.register_http_handler(
                path=http_handler.path,
                method=http_handler.method,
                handler=health_check,
            )

    # Register custom handlers.
    if config.custom_handlers:
        for custom_handler in config.custom_handlers.handlers:
            handler_fn = _load_handler(custom_handler.location, custom_handler.handler)
            app.register_custom_handler(
                handler_name=custom_handler.name,
                handler=handler_fn,
                timeout_seconds=custom_handler.timeout,
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
