#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

POSITIONAL=()
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    -h|--help)
    HELP=yes
    shift # past argument
    ;;
    --update-snapshots)
    UPDATE_SNAPSHOTS=yes
    shift # past argument
    ;;
    --no-infra)
    NO_INFRA=yes
    shift # past argument
    ;;
    *)    # unknown option
    POSITIONAL+=("$1") # save it in an array for later
    shift # past argument
    ;;
esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

function help {
  cat << EOF
Test runner
Runs tests for the CLI:

Usage:
  bash scripts/run-tests.sh [options]

Options:
  -h, --help           Show this help message
  --update-snapshots   Update test snapshots
  --no-infra           Skip starting Docker test dependencies
                       (useful when they are already running or not needed)

Note: By default, this script starts Docker test dependencies
(DynamoDB Local, MinIO, PostgreSQL, Valkey) for integration tests.
Use --no-infra to skip this step if they are already running.

Examples:
  # Run all tests (starts infrastructure automatically)
  bash scripts/run-tests.sh

  # Run tests without starting infrastructure (already running)
  bash scripts/run-tests.sh --no-infra
EOF
}

if [ -n "$HELP" ]; then
  help
  exit 0
fi

set -e

cd "$CLI_DIR"

# Create coverage directory
mkdir -p coverage

if [ -z "$NO_INFRA" ]; then
  echo "Starting test dependencies (DynamoDB Local, MinIO, PostgreSQL, Valkey)..."
  docker compose --env-file "$CLI_DIR/.env.test" -f "$CLI_DIR/docker-compose.test-deps.yml" up -d --wait

  cleanup() {
    echo "Stopping test dependencies..."
    docker compose --env-file "$CLI_DIR/.env.test" -f "$CLI_DIR/docker-compose.test-deps.yml" down
  }
  trap cleanup EXIT
fi

# Export environment variables for integration tests
echo "Exporting environment variables for test suite..."
set -a
source "$CLI_DIR/.env.test"
set +a

# Run tests with coverage
echo "Running tests..."
echo "" > coverage.txt
go test -count=1 -timeout 30000ms -race -coverprofile=coverage.txt -coverpkg=./... -covermode=atomic $(go list ./... | grep -v '/testutils$')

if [ -z "$GITHUB_ACTION" ]; then
  # We are on a dev machine so produce html output of coverage
  # to get a visual to better reveal uncovered lines.
  go tool cover -html=coverage.txt -o coverage.html
  echo ""
  echo "Coverage report: coverage.html"
fi

if [ -n "$GITHUB_ACTION" ]; then
  # We are in a CI environment so run tests again to generate JSON report.
  go test -timeout 30000ms -json -tags "$TEST_TYPES" $(go list ./... | grep -v '/testutils$') > report.json
fi

echo ""
echo "Tests complete!"
