from typing import Optional, List, Callable, Awaitable, Dict


def sum_as_string(a: int, b: int) -> str:
    ...


class CoreRuntimeConfig:
    blueprint_config_path: str
    server_port: int
    server_loopback_only: Optional[bool]

    def __init__(
        self,
        blueprint_config_path: str,
        server_port: int,
        server_loopback_only: Optional[bool]
    ):
        ...


class CoreHttpHandlerDefinition:
    path: str
    method: str
    location: str
    handler: str


class CoreHttpConfig:
    handlers: List[CoreHttpHandlerDefinition]


class CoreWebSocketConfig:
    ...


class CoreApiConfig:
    http: Optional[CoreHttpConfig]
    websocket: Optional[CoreWebSocketConfig]


class CoreRuntimeAppConfig:
    api: Optional[CoreApiConfig]


class Response:
    status: int
    headers: Optional[Dict[str, str]]
    body: Optional[str]

    def __init__(
        self,
        status: int,
        headers: Optional[Dict[str, str]],
        body: Optional[str]
    ):
        ...


class CoreRuntimeApplication:
    def __init__(self, runtime_config: CoreRuntimeConfig):
        ...

    def setup(self) -> CoreRuntimeAppConfig:
        ...

    def run(self) -> None:
        ...

    def register_http_handler(self, path: str, method: str, handler: Callable[[], Awaitable[Response]]) -> None:
        ...
