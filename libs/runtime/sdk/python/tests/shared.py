from celerity_runtime_sdk import WebSocketRegistry


_ws_registry: WebSocketRegistry | None = None


def set_ws_registry(registry: WebSocketRegistry) -> None:
    global _ws_registry
    _ws_registry = registry


def get_ws_registry() -> WebSocketRegistry:
    global _ws_registry
    if _ws_registry is None:
        raise ValueError("WebSocket registry not set")
    return _ws_registry
