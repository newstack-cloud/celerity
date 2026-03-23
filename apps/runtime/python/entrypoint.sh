#!/bin/bash
set -e

RUNTIME_DIR="/opt/celerity"
APP_DIR="${CELERITY_APP_DIR:-${RUNTIME_DIR}/app}"

# Install app dependencies into the shared venv if the app has a
# pyproject.toml or requirements.txt. This ensures resource extras
# (e.g. celerity-sdk[cache], celerity-sdk[resources-sql]) that the
# app declares are available at runtime.
install_app_deps() {
  if [ -f "${APP_DIR}/pyproject.toml" ]; then
    echo "[celerity-runtime] Installing app dependencies from ${APP_DIR}/pyproject.toml"
    uv pip install --quiet "${APP_DIR}" || {
      echo "[celerity-runtime] Warning: failed to install app dependencies" >&2
    }
  elif [ -f "${APP_DIR}/requirements.txt" ]; then
    echo "[celerity-runtime] Installing app dependencies from ${APP_DIR}/requirements.txt"
    uv pip install --quiet -r "${APP_DIR}/requirements.txt" || {
      echo "[celerity-runtime] Warning: failed to install app dependencies" >&2
    }
  fi
}

case "${1}" in
  start)
    # Production mode: straight execution
    # App deps should already be installed in the derivative image.
    exec python "${RUNTIME_DIR}/main.py"
    ;;
  dev)
    # Dev mode: install app deps then auto-reload with watchfiles
    install_app_deps
    echo "[celerity-runtime] Dev mode: watching ${APP_DIR}/src for changes"
    exec watchfiles "python ${RUNTIME_DIR}/main.py" "${APP_DIR}/src"
    ;;
  *)
    # Pass-through for arbitrary commands (e.g., /bin/bash for debugging)
    exec "$@"
    ;;
esac
