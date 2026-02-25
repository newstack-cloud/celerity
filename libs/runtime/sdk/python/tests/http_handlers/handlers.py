import asyncio
import json

from celerity_runtime_sdk import (
    Request,
    RequestContext,
    Response,
    ResponseBuilder,
)


async def echo(req: Request, ctx: RequestContext) -> Response:
    return (
        ResponseBuilder()
        .set_status(200)
        .set_headers({"content-type": "application/json"})
        .set_json_body({
            "method": req.method,
            "path": req.path,
            "pathParams": req.path_params,
            "query": req.query,
            "headers": req.headers,
            "cookies": req.cookies,
            "contentType": req.content_type,
            "requestId": ctx.request_id,
            "requestTime": str(ctx.request_time),
            "auth": ctx.auth,
            "clientIp": ctx.client_ip,
            "traceContext": ctx.trace_context,
            "userAgent": req.user_agent,
            "matchedRoute": ctx.matched_route,
            "textBody": req.text_body,
            "httpVersion": str(req.protocol_version),
            "hasBinaryBody": req.binary_body is not None,
            "binaryBodyLength": len(req.binary_body) if req.binary_body else 0,
        })
        .build()
    )


async def custom_status(req: Request, ctx: RequestContext) -> Response:
    return (
        ResponseBuilder()
        .set_status(201)
        .set_headers({
            "content-type": "application/json",
            "x-custom-header": "custom-value",
        })
        .set_json_body({"created": True})
        .build()
    )


async def slow(req: Request, ctx: RequestContext) -> Response:
    await asyncio.sleep(3)
    return (
        ResponseBuilder()
        .set_status(200)
        .set_json_body({"message": "should not reach"})
        .build()
    )


async def binary_echo(req: Request, ctx: RequestContext) -> Response:
    if req.binary_body:
        return (
            ResponseBuilder()
            .set_status(200)
            .set_headers({"content-type": "application/json"})
            .set_json_body({"binaryLength": len(req.binary_body)})
            .build()
        )
    return (
        ResponseBuilder()
        .set_status(200)
        .set_headers({"content-type": "application/octet-stream"})
        .set_binary_body(bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]))
        .build()
    )
