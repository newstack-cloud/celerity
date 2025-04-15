# blueprint state

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=coverage)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)

A library that provides a collection of state container implementations to be used in the deploy engine or other applications built on top of the blueprint framework.

## Implementations

- Postgres - A state container backed by a Postgres database that is modelled in a normalised, relational way.
- In-memory with file persistence - A state container backed by an in-process, in-memory store that uses files on disk for persistence. This implementation is mostly useful for single node deployments of the deploy engine or for managing deployments from a developer's machine.

## Usage

### Postgres

A set of database migrations are provided to manage the schema of the database required for the Postgres state container.
See [Postgres migrations](./docs/POSTGRES_MIGRATIONS.md) for more information.

#### Requirements

- A postgres database/cluster that is using Postgres 17.0 and above.
- Only UUIDs are supported for blueprint entity IDs, this means only `core.IDGenerator` imlpementations that generate UUIDs can be used.

#### Example

```go
package main

import (
    "os"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/two-hundred/celerity/libs/blueprint/core"
    "github.com/two-hundred/celerity/libs/blueprint/state"
    "github.com/two-hundred/celerity/libs/blueprint-state/postgres"
)

func main() {
    stateContainer, err := setupStateContainer()
    if err != nil {
        panic(err)
    }

    // Use the state container ...
    // For example, you could wire up the state container with a blueprint loader to carry out deployments.
}

func setupStateContainer() (state.Container, error) {
    connPool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
    if err != nil {
        panic(err)
    }
    logger := core.NewNopLogger()
    // You'll generally want to name the logger to allow for filtering logs
    // that get displayed based on scope or better debugging when an error occurs.
    return postgres.LoadStateContainer(".deploy_state", connPool, logger.Named("state"))
}
```

### In-memory with file persistence

```go
package main

import (
    "github.com/spf13/afero"
    "github.com/two-hundred/celerity/libs/blueprint/core"
    "github.com/two-hundred/celerity/libs/blueprint/state"
    "github.com/two-hundred/celerity/libs/blueprint-state/memfile"
)

func main() {
    stateContainer, err := setupStateContainer()
    if err != nil {
        panic(err)
    }

    // Use the state container ...
    // For example, you could wire up the state container with a blueprint loader to carry out deployments.
}

func setupStateContainer() (state.Container, error) {
    fs := afero.NewOsFs()
    logger := core.NewNopLogger()
    // You'll generally want to name the logger to allow for filtering logs
    // that get displayed based on scope or better debugging when an error occurs.
    return memfile.LoadStateContainer(".deploy_state", fs, logger.Named("state"))
}
```

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
