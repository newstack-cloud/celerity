import json

from celerity_runtime_sdk import (
    Response,
    ResponseBuilder,
    Request,
    RequestContext,
    WebSocketMessageInfo,
    SendContext,
    WebSocketMessageType,
)
import shared


async def get_order(req: Request, ctx: RequestContext) -> Response:
    print("Request body:", req.text_body)
    print("Request trace context:", ctx.trace_context)
    return (
        ResponseBuilder()
        .set_status(200)
        .set_json_body({"message": "Order received"})
        .build()
    )


async def update_order(req: Request, ctx: RequestContext) -> Response:
    print("Request body:", req.text_body)
    print("Request trace context:", ctx.trace_context)
    return (
        ResponseBuilder()
        .set_status(200)
        .set_json_body({"message": "Order updated"})
        .build()
    )


async def process_order_update(ws_message_info: WebSocketMessageInfo) -> None:
    print("WebSocket message info:", {
        "type": ws_message_info.type,
        "connection_id": ws_message_info.connection_id,
        "message_id": ws_message_info.message_id,
        "trace_context": ws_message_info.trace_context,
        "request_context_trace_context": (
            ws_message_info.request_context.trace_context
            if ws_message_info.request_context else None
        ),
    })
    ws_registry = shared.get_ws_registry()
    ws_registry.send_message(
        connection_id=ws_message_info.connection_id,
        message_id="123",
        message_type=WebSocketMessageType.JSON,
        message=json.dumps({
            "action": "processedOrderUpdate",
            "message": "Order received",
        }),
        ctx=SendContext(
            caller="test_handler",
            wait_for_ack=False,
            inform_clients=[]
        ),
    )
