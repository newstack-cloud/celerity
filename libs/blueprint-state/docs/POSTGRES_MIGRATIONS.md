# Postgres migrations

Migrations for the Postgres implementation can be found in the `postgres/migrations` directory. These migrations need to be applied to the database before the state container can be used. Each update to the migrations will need to be applied every time you upgrade the blueprint state library to a new version, for applications like the deploy engine this should be a part of a streamlined upgrade process.

For contributors, the golang-migrate cli tool is used to manage the migrations, you will need to install it following the instructions that can be found [here](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate). For library users, you will need to ensure that the migrations are applied as part of your application's installation and upgrade processes.

## Create migration

Prepare a new migration file by running the following command:

```bash
migrate create -ext sql -dir postgres/migrations -seq <migration_name>
```

Add the SQL statements to create tables or modify existing tables in the newly created `.up.sql` migration file.

Add the SQL statements to revert the changes in the `.down.sql` migration file.

## Running & rolling back migrations

Before running/rolling back migrations, prepare and export the database connection URL as an environment variable:

```bash
export POSTGRES_URL="pgx5://{user}:{password}@{hostname}:{port}/{dbname}?sslmode=disable"
```

### Running migrations

```bash
migrate -path postgres/migrations -database $POSTGRES_URL up
```

### Rolling back migrations

```bash
migrate -path postgres/migrations -database $POSTGRES_URL down
```

### Migrations in applications such as the deploy engine

When using the blueprint state library in an application such as the deploy engine, migrations should run as part of the tooling used in the application's install and upgrade processes. This ensures that the database schema is always up-to-date with the version of the blueprint state library being used.

The `golang-migrate` tool provides a library API to programatically run migrations in Go applications. The deploy engine, for example, has a helper Go program that uses this library to run migrations as part of the installation and upgrade processes.

## Bringing up the test postgres environment

To bring up a test postgres database service, you can bring up the docker compose stack by running the following command:

```bash
docker compose -f docker-compose.postgres.yml up
```

_Run this command in a separate terminal window to keep the database service running while you work on the migrations._
