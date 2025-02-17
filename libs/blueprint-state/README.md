# blueprint state

[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=coverage)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=two-hundred_celerity-blueprint-state&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=two-hundred_celerity-blueprint-state)

A library that provides a collection of state container implementations to be used in the deploy engine or other applications built on top of the blueprint framework.

## Implementations

- Postgres - A state container backed by a Postgres database that is modelled in a normalised, relational way.
- Redis OSS - A state container backed by Redis OSS (<=v7.2) or a Redis OSS compatible database such as [Valkey](https://valkey.io/).
- Cassandra - A state container backed by a Cassandra database, this is modelled in a way that is tailored towards the queries in the `state.Container` interface.
- In-memory with file persistence - A state container backed by an in-process, in-memory store that uses files on disk for persistence. This implementation is mostly useful for single node deployments of the deploy engine or for managing deployments from a developer's machine.

## Usage

### Postgres

Migrations for the Postgres implementation can be found in the `postgres/migrations` directory. These migrations need to be applied to the database before the state container can be used. Each update to the migrations will need to be applied every time you upgrade the blueprint state library to a new version, for applications like the deploy engine this should be a part of a streamlined upgrade process.

### Redis OSS

### Cassandra

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
    // Set up code before ...
    logger := setupLogger()
    fs := afero.NewOsFs()
    stateContainer, err := setupStateContainer(
        // You'll generally want to name the logger to allow for filtering logs
        // that get displayed based on scope or better debugging when an error occurs.
        logger.Named("deployStateContainer"),
        fs,
    )
    if err != nil {
        panic(err)
    }

    // Use the state container ...
    // For example, you could wire up the state container with a blueprint loader to carry out deployments.
}

func setupStateContainer() (state.Container, error) {
    fs := afero.NewOsFs()
    logger := core.NewNopLogger()
    return memfile.LoadStateContainer(".deploy_state", fs, logger.Named("state"))
}
```

## Additional documentation

- [Contributing](docs/CONTRIBUTING.md)
