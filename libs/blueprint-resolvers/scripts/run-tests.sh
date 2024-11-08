#!/usr/bin/env bash

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
Runs tests for the library:
bash scripts/run-tests.sh
EOF
}

if [ -n "$HELP" ]; then
  help
  exit 0
fi

finish() {
  echo "Taking down test dependencies docker compose stack ..."
  docker-compose -f docker-compose.test-deps.yml down
  docker-compose -f docker-compose.test-deps.yml rm -v -f
}

trap finish EXIT

echo "Bringing up docker compose stack for test dependencies ..."

docker-compose -f docker-compose.test-deps.yml up -d

echo "Waiting for LocalStack to be ready ..."
start=$EPOCHSECONDS
completed="false"
while [ "$completed" != "true" ]; do
  sleep 5
  completed=$(curl -s localhost:4579/_localstack/init/ready | jq .completed)
  if (( EPOCHSECONDS - start > 60 )); then break; fi 
done

# Fake GCS Server takes a lot less time to start up than LocalStack,
# so it is safe to run tests against it after LocalStack is ready.

echo "Populating S3 with test data ..."

aws --endpoint-url=http://localhost:4579 s3 mb s3://test-bucket --region eu-west-2
aws --endpoint-url=http://localhost:4579 s3 cp __testdata/s3/data/test-bucket/s3.test.blueprint.yml s3://test-bucket/s3.test.blueprint.yml --region eu-west-2

set -e
echo "" > coverage.txt

go test -timeout 30000ms -race -coverprofile=coverage.txt -coverpkg=./... -covermode=atomic `go list ./... | egrep -v '(/(testutils))$'`

if [ -z "$GITHUB_ACTION" ]; then
  # We are on a dev machine so produce html output of coverage
  # to get a visual to better reveal uncovered lines.
  go tool cover -html=coverage.txt -o coverage.html
fi

if [ -n "$GITHUB_ACTION" ]; then
  # We are in a CI environment so run tests again to generate JSON report.
  go test -timeout 30000ms -json -tags "$TEST_TYPES" `go list ./... | egrep -v '(/(testutils))$'` > report.json
fi
