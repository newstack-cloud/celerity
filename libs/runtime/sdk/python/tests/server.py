import json
from celerity_runtime_sdk import CoreRuntimeApplication, CoreRuntimeConfig, Response


def run_server() -> None:
    app = CoreRuntimeApplication(
        runtime_config=CoreRuntimeConfig(
            blueprint_config_path="tests/http-api.blueprint.yaml",
            server_port=22346,
            server_loopback_only=True,
        )
    )
    runtime_app_config = app.setup()

    async def test_handler() -> Response:
        return Response(status=200, headers={}, body=json.dumps({"message": "Order received"}))

    if runtime_app_config.api is None or runtime_app_config.api.http is None:
        raise ValueError("No HTTP API configuration found in blueprint")

    for handler in runtime_app_config.api.http.handlers:
        print(handler.path, handler.method, handler.location, handler.handler)
        app.register_http_handler(
            path=handler.path,
            method=handler.method,
            handler=test_handler,
        )

    app.run()


if __name__ == "__main__":
    run_server()
