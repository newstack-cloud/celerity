# Contributing to the celerity cli

## Getting set up

### Prerequisites

- [Go](https://golang.org/dl/) >=1.26
- [Docker](https://docs.docker.com/get-docker/) (for integration tests)

Dependencies are managed with Go modules (go.mod) and will be installed automatically when you first
run tests.

If you want to install dependencies manually you can run:

```bash
go mod download
```

## Running tests

### Unit tests only (no Docker required)

To run tests without starting Docker test infrastructure:

```bash
bash ./scripts/run-tests.sh --no-infra
```

### Full test suite (unit + integration)

The full test suite requires Docker for integration test dependencies
(DynamoDB Local, MinIO, PostgreSQL, Valkey).

```bash
bash ./scripts/run-tests.sh
```

This will:
1. Start test dependencies via `docker-compose.test-deps.yml`
2. Wait for all services to be healthy
3. Export environment variables from `.env.test`
4. Run all tests with coverage
5. Generate `coverage.html` for local inspection
6. Tear down test dependencies on exit

### Test infrastructure

Test dependencies are defined in `docker-compose.test-deps.yml` with pinned versions
matching the compose generator (`internal/compose/consts.go`):

| Service | Image | Host Port |
|---------|-------|-----------|
| DynamoDB Local | `amazon/dynamodb-local:3.3.0` | 48000 |
| MinIO | `minio/minio:RELEASE.2025-09-07T16-13-09Z` | 49000 |
| PostgreSQL | `postgres:17-alpine` | 45433 |
| Valkey | `valkey/valkey:8-alpine` | 46379 |

Environment variables for connecting to these services are in `.env.test`.

Integration tests should check for the relevant environment variable and skip
if it is not set:

```go
func (s *MySuite) SetupTest() {
    endpoint := testutils.RequireEnv(s.T(), "CELERITY_TEST_DYNAMODB_ENDPOINT")
    // ...
}
```

### Updating snapshots

```bash
bash ./scripts/run-tests.sh --update-snapshots
```

### Viewing coverage

After running tests, open the coverage report:

```bash
open coverage.html
```

## Test conventions

- Use `testify/suite` for test organisation
- Test only the **public API** of each package — do not test unexported functions directly
- Use `s.T().TempDir()` for filesystem fixtures
- Use `s.Require().NoError()` for preconditions, `s.Assert()` for test assertions
- Shared mocks live in `internal/testutils/`
- Integration tests that need Docker services should use `testutils.RequireEnv()`
  to skip gracefully when infrastructure is unavailable

## Releasing

TODO: Outline a more involved release process to ship binaries!

To release a new version of the library, you need to create a new tag and push it to the repository.

The format must be `apps/cli/vX.Y.Z` where `X.Y.Z` is the semantic version number.
The reason for this is that Go's mechanism for picking up modules from multi-repo packages is based on the sub-directory path being in the version tag.

See [here](https://go.dev/wiki/Modules#publishing-a-release).

1. add a change log entry to the `CHANGELOG.md` file following the template below:

```markdown
## [0.2.0] - 2024-06-05

### Fixed:

- Corrects error reporting for change staging.

### Added

- Adds retry behaviour to resource providers.
```

2. Create and push the new tag prefixed by sub-directory path:

```bash
git tag -a apps/blueprint/v0.2.0 -m "chore(cli): Release v0.2.0"
git push --tags
```

Be sure to add a release for the tag with notes following this template:

Title: `Blueprint Framework - v0.2.0`

```markdown
## Fixed:

- Corrects claims handling for JWT middleware.

## Added

- Adds dihandlers-compatible middleware for access control.
```

3. Prompt Go to update its index of modules with the new release:

```bash
GOPROXY=proxy.golang.org go list -m github.com/newstack-cloud/celerity/apps/cli@v0.2.0
```

## Commit scope

**cli**

Example commit:

```bash
git commit -m 'fix(cli): correct cyclic dependency bug'
```
