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
            blueprint_config_path="tests/consumer-schedule.blueprint.yaml",
            service_name="python-sdk-consumer-test",
            server_port=22348,
        )
        .set_server_loopback_only(True)
        .set_trace_otlp_collector_endpoint("")
        .set_platform(RuntimePlatform.LOCAL)
        .set_test_mode(True)
        .build()
    )
    config = app.setup()

    # Register HTTP handlers.
    if config.api and config.api.http:
        for http_handler in config.api.http.handlers:
            print(f"Registering HTTP handler: {http_handler.path} {http_handler.method}")
            handler_fn = _load_handler(http_handler.location, http_handler.handler)
            app.register_http_handler(
                path=http_handler.path,
                method=http_handler.method,
                handler=handler_fn,
            )

    # Register consumer handlers.
    if config.consumers:
        for consumer in config.consumers.consumers:
            for event_handler in consumer.handlers:
                print(f"Registering consumer handler: {event_handler.name}")
                handler_fn = _load_handler(event_handler.location, event_handler.handler)
                app.register_consumer_handler(
                    handler_tag=event_handler.name,
                    handler=handler_fn,
                    timeout_seconds=event_handler.timeout,
                )

    # Register schedule handlers.
    if config.schedules:
        for schedule in config.schedules.schedules:
            for event_handler in schedule.handlers:
                print(f"Registering schedule handler: {event_handler.name}")
                handler_fn = _load_handler(event_handler.location, event_handler.handler)
                app.register_schedule_handler(
                    handler_tag=event_handler.name,
                    handler=handler_fn,
                    timeout_seconds=event_handler.timeout,
                )

    # Register custom handlers.
    if config.custom_handlers:
        for custom_handler in config.custom_handlers.handlers:
            print(f"Registering custom handler: {custom_handler.name}")
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
