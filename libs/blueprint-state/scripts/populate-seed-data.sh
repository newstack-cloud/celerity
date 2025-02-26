#!/bin/bash

echo "Exporting environment variables for seed data population ..."
set -a
source .env.test
set +a

echo "Preparing new-line delimited json files for seeding the test postgres database ..."

mkdir -p postgres/__testdata/seed/tmp

for file in postgres/__testdata/seed/*.json; do
  jq -c '.[]' $file > postgres/__testdata/seed/tmp/$(basename "$file" .json).nd.json
done

echo "Seeding the test postgres database ..."

export PGPASSWORD=$POSTGRES_PASSWORD
psql -U $POSTGRES_USER -h $POSTGRES_HOST -p $POSTGRES_PORT -d $POSTGRES_DB -a -f postgres/__testdata/seed/load-data.sql \
  > seed-output.log 2>&1
