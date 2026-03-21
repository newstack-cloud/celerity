# Celerity Python Runtime Host — Entry Point
#
# Bridges the Rust PyO3 runtime binary (celerity-runtime-sdk) with developer
# application code via the celerity-sdk framework.
#
# See: entrypoint.sh start (production) / entrypoint.sh dev (with auto-reload)

from __future__ import annotations

import contextlib
import logging
import os
import signal
import sys
import threading

from celerity.bootstrap.runtime_orchestrator import start_runtime

from sdk_compat_check import check_sdk_version_alignment, get_runtime_sdk_versions

LOG_PREFIX = "[celerity-runtime]"

REQUIRED_ENV_VARS = [
    "CELERITY_BLUEPRINT",
    "CELERITY_RUNTIME_CALL_MODE",
    "CELERITY_SERVICE_NAME",
    "CELERITY_SERVER_PORT",
    "CELERITY_RUNTIME_PLATFORM",
    "CELERITY_MODULE_PATH",
]

logger = logging.getLogger("celerity.runtime")


def main() -> None:
    # 1. Validate required environment variables
    missing = [v for v in REQUIRED_ENV_VARS if not os.environ.get(v)]
    if missing:
        items = "\n".join(f"  - {v}" for v in missing)
        print(
            f"{LOG_PREFIX} Missing required environment variables:\n{items}"
            "\n\nSet these before starting the runtime. See .env.example for reference.",
            file=sys.stderr,
        )
        sys.exit(1)

    # 2. SDK version alignment check (best-effort — never blocks startup)
    with contextlib.suppress(Exception):
        check_sdk_version_alignment()

    # 3. Log startup info and SDK versions
    sdk_versions = get_runtime_sdk_versions()
    sdk_version_str = ", ".join(f"{pkg}=={ver}" for pkg, ver in sdk_versions.items())

    is_dev = os.environ.get("CELERITY_ENV") != "production"

    print(f"{LOG_PREFIX} Starting Celerity Python runtime")
    print(f"{LOG_PREFIX} Service:  {os.environ['CELERITY_SERVICE_NAME']}")
    print(f"{LOG_PREFIX} Port:     {os.environ['CELERITY_SERVER_PORT']}")
    print(f"{LOG_PREFIX} Platform: {os.environ['CELERITY_RUNTIME_PLATFORM']}")
    print(f"{LOG_PREFIX} Module:   {os.environ['CELERITY_MODULE_PATH']}")
    print(f"{LOG_PREFIX} Mode:     {'development' if is_dev else 'production'}")
    print(f"{LOG_PREFIX} SDK:      {sdk_version_str}")

    # 4. Graceful shutdown handling
    shutting_down = threading.Event()

    def _shutdown_handler(signum: int, _frame: object) -> None:
        if shutting_down.is_set():
            return
        shutting_down.set()
        sig_name = signal.Signals(signum).name
        print(f"{LOG_PREFIX} Received {sig_name}, shutting down...")
        # Force exit after 10 seconds if graceful shutdown stalls
        timer = threading.Timer(10.0, _force_exit)
        timer.daemon = True
        timer.start()

    def _force_exit() -> None:
        print(f"{LOG_PREFIX} Forced shutdown after timeout", file=sys.stderr)
        os._exit(1)

    signal.signal(signal.SIGTERM, _shutdown_handler)
    signal.signal(signal.SIGINT, _shutdown_handler)

    # 5. Start the runtime
    # start_runtime() is synchronous: app.setup() creates its own asyncio
    # event loop, bootstrap runs on that loop via run_until_complete(),
    # and app.run(block=True) calls run_forever() to process callbacks.
    start_runtime(block=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"{LOG_PREFIX} Fatal error: {exc}", file=sys.stderr)
        sys.exit(1)
