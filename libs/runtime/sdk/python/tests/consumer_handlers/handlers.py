import json
import os
import tempfile
from typing import Any

from celerity_runtime_sdk import (
    ConsumerEventInput,
    EventResult,
    Request,
    RequestContext,
    Response,
    ResponseBuilder,
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
    results_dir = os.environ.get("TEST_RESULTS_DIR", "/tmp/celerity-test-results")
    os.makedirs(results_dir, exist_ok=True)
    data = {
        "handler_tag": event.handler_tag,
        "messages": [
            {"message_id": m.message_id, "body": m.body}
            for m in event.messages
        ],
    }
    _write_result(results_dir, "consumer_result.json", data)
    return EventResult(success=True)


async def handle_cleanup(event: ScheduleEventInput) -> EventResult:
    results_dir = os.environ.get("TEST_RESULTS_DIR", "/tmp/celerity-test-results")
    os.makedirs(results_dir, exist_ok=True)
    data = {
        "handler_tag": event.handler_tag,
        "message_id": event.message_id,
        "schedule_id": event.schedule_id,
    }
    _write_result(results_dir, "schedule_result.json", data)
    return EventResult(success=True)


async def handle_utility(payload: Any) -> dict[str, Any]:
    return {"received": payload, "echoed": True}


def _write_result(results_dir: str, filename: str, data: dict) -> None:
    """Write JSON result atomically using a temp file + rename."""
    target = os.path.join(results_dir, filename)
    fd, tmp_path = tempfile.mkstemp(dir=results_dir, suffix=".tmp")
    try:
        with os.fdopen(fd, "w") as f:
            json.dump(data, f)
        os.replace(tmp_path, target)
    except Exception:
        os.unlink(tmp_path)
        raise
