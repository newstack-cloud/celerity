#!/bin/bash
# Prepares a filtered copy of libs/runtime/ for use as a Docker build context.
# Excludes target/ (40+ GB), node_modules (5+ GB), .venv, and other artifacts
# that are not needed for building the Python runtime SDK wheel.
#
# Usage:
#   ./scripts/prepare-runtime-sdk-context.sh [source_dir] [dest_dir]
#
# Defaults:
#   source_dir: $HOME/projects2023/celerity/libs/runtime
#   dest_dir:   /tmp/celerity-runtime-sdk-context

set -euo pipefail

SOURCE="${1:-${HOME}/projects2023/celerity/libs/runtime}"
DEST="${2:-/tmp/celerity-runtime-sdk-context}"

echo "[prepare-context] Syncing ${SOURCE} → ${DEST} (excluding build artifacts)"

# Clean destination first — rsync --delete does not remove previously synced
# files that are now excluded.
rm -rf "${DEST}"
mkdir -p "${DEST}"

rsync -a \
  --exclude='target/' \
  --exclude='node_modules/' \
  --exclude='.yarn/' \
  --exclude='.venv/' \
  --exclude='.mypy_cache/' \
  --exclude='__pycache__/' \
  --exclude='.pytest_cache/' \
  --exclude='*.so' \
  --exclude='*.dylib' \
  --exclude='*.node' \
  --exclude='docs/' \
  --exclude='api-docs/' \
  --exclude='tests/' \
  --exclude='sdk/bindings/' \
  "${SOURCE}/" "${DEST}/"

SIZE=$(du -sh "${DEST}" | cut -f1)
echo "[prepare-context] Done — context size: ${SIZE}"
echo "${DEST}"
