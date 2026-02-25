import asyncio

from typing import Any

from celerity_runtime_sdk import (
    ConsumerEventInput,
    CoreRuntimeApplication,
    CoreRuntimeConfigBuilder,
    EventResult,
    Request,
    RequestContext,
    Response,
    ResponseBuilder,
    RuntimePlatform,
    ScheduleEventInput,
)


async def health_check(req: Request, ctx: RequestContext) -> Response:
    return (
        ResponseBuilder()
        .set_status(200)
        .set_json_body({"status": "ok"})
        .build()
    )


async def handle_order(event: ConsumerEventInput) -> EventResult:
    return EventResult(success=True)


async def handle_cleanup(event: ScheduleEventInput) -> EventResult:
    return EventResult(success=True)


async def handle_utility_slow(payload: Any) -> dict[str, bool]:
    await asyncio.sleep(5)
    return {"should_not_reach": True}


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfigBuilder(
            blueprint_config_path="tests/consumer-schedule.blueprint.yaml",
            service_name="python-sdk-slow-consumer-test",
            server_port=22349,
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
            app.register_http_handler(
                path=http_handler.path,
                method=http_handler.method,
                handler=health_check,
            )

    # Register consumer handlers.
    if config.consumers:
        for consumer in config.consumers.consumers:
            for event_handler in consumer.handlers:
                app.register_consumer_handler(
                    handler_tag=event_handler.name,
                    handler=handle_order,
                    timeout_seconds=event_handler.timeout,
                )

    # Register schedule handlers.
    if config.schedules:
        for schedule in config.schedules.schedules:
            for event_handler in schedule.handlers:
                app.register_schedule_handler(
                    handler_tag=event_handler.name,
                    handler=handle_cleanup,
                    timeout_seconds=event_handler.timeout,
                )

    # Register custom handler with 1-second timeout and slow implementation.
    if config.custom_handlers:
        for custom_handler in config.custom_handlers.handlers:
            app.register_custom_handler(
                handler_name=custom_handler.name,
                handler=handle_utility_slow,
                timeout_seconds=1,
            )

    app.run()


if __name__ == "__main__":
    run_server()
