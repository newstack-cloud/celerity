#!/bin/bash
set -e

RUNTIME_DIR="/opt/celerity"

case "${1}" in
  start)
    # Production mode: straight execution with OTel early init
    exec node \
      --import "${RUNTIME_DIR}/register-hooks.mjs" \
      --import @celerity-sdk/telemetry/setup \
      "${RUNTIME_DIR}/index.mjs"
    ;;
  dev)
    # Dev mode: file watching with restart on changes + tsx for TypeScript support
    APP_DIR="${CELERITY_APP_DIR:-${RUNTIME_DIR}/app}"
    echo "[celerity-runtime] Dev mode: watching ${APP_DIR} for changes"
    exec node \
      --import "${RUNTIME_DIR}/register-hooks.mjs" \
      --import tsx \
      --import @celerity-sdk/telemetry/setup \
      --watch-path="${APP_DIR}" \
      "${RUNTIME_DIR}/index.mjs"
    ;;
  *)
    # Pass-through for arbitrary commands (e.g., /bin/bash for debugging)
    exec "$@"
    ;;
esac
