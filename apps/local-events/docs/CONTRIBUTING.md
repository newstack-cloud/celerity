# Contributing to the Celerity Local Events Sidecar

## Getting set up

### Prerequisites

- [Go](https://golang.org/dl/) >=1.24
- [Docker](https://www.docker.com/get-started) (for running tests that require external services)

Dependencies are managed with Go modules (go.mod) and will be installed automatically when you first run tests.

If you want to install dependencies manually you can run:

```bash
go mod download
```

## Running tests

```bash
bash ./scripts/run-tests.sh
```

## Releasing

Releases are automated using [release-please](https://github.com/googleapis/release-please).

### How it works

1. **Conventional commits drive releases** - Commits with scopes matching this app (e.g., `feat(local-events): ...` or `fix(local-events): ...`) are tracked by release-please.

2. **Release PRs are created automatically** - When releasable commits land on `main`, release-please opens/updates a PR with:
   - Version bump based on commit types (feat = minor, fix = patch)
   - CHANGELOG.md updates

3. **Merging creates the release** - When the release PR is merged:
   - A GitHub release is created
   - Two git tags are created:
     - `local-events/v{version}` - Used internally by release-please for tracking. Do not use this tag.
     - `apps/local-events/v{version}` - The canonical tag. Use this for workflows and references.

### Build artifacts

When a release tag is pushed, separate workflows will build and publish artifacts (binaries). These workflows are triggered by tags matching `apps/local-events/v*`.

### Tag format

Tags follow the pattern: `apps/local-events/vX.Y.Z`

Example: `apps/local-events/v1.0.0`

## Commit scope

**local-events**

Example commit:

```bash
git commit -m 'feat(local-events): add support for azure cosmosdb change feed'
```
