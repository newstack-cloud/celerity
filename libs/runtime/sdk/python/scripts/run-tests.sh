#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$SDK_DIR/../../docker-compose.test-deps.yml"

cleanup() {
  docker compose -f "$COMPOSE_FILE" down --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

# Start Valkey (reuses existing docker-compose.test-deps.yml).
docker compose -f "$COMPOSE_FILE" up -d valkey

# Wait for Valkey to be ready.
echo "Waiting for Valkey..."
for i in $(seq 1 30); do
  if docker exec valkey_celerity_runtime_tests redis-cli ping 2>/dev/null | grep -q PONG; then
    echo "Valkey ready."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Valkey did not become ready in time."
    exit 1
  fi
  sleep 1
done

# Build native module (debug, with local consumer support for Valkey).
echo "Building native module..."
cd "$SDK_DIR"
uv run maturin develop --features celerity_local_consumers

# Load test environment variables
set -a
# shellcheck disable=SC1091
source "$SDK_DIR/.env.test"
set +a

echo "Running tests..."
uv run python -m pytest -rA tests/ "$@"
