# Contributing to the blueprint language server

## Getting set up

### Prerequisites

- [Go](https://golang.org/dl/) >=1.22

Dependencies are managed with Go modules (go.mod) and will be installed automatically when you first
run tests.

If you want to install dependencies manually you can run:

```bash
go mod download
```

## Running tests

```bash
bash ./scripts/run-tests.sh
```

## Releasing

To release a new version of the server, you need to create a new tag and push it to the repository.

The format must be `tools/blueprint-ls/vX.Y.Z` where `X.Y.Z` is the semantic version number.
The reason for this is that Go's mechanism for picking up modules from multi-repo packages is based on the sub-directory path being in the version tag.

This is a binary application but follows the same convention as a library for consistency.

See [here](https://go.dev/wiki/Modules#publishing-a-release).

1. add a change log entry to the `CHANGELOG.md` file following the template below:

```markdown
## [0.2.0] - 2024-06-05

### Fixed:

- Fix syntax highlighting.

### Added

- Adds advanced autocomplete features for substitutions.
```

2. Create and push the new tag prefixed by sub-directory path:

```bash
git tag -a tools/blueprint-ls/v0.2.0 -m "chore(blueprint-ls): Release v0.2.0"
git push --tags
```

Be sure to add a release for the tag with notes following this template:

Title: `Blueprint Language Server - v0.2.0`

```markdown
## Fixed:

- Fix syntax highlighting.

## Added

- Adds advanced autocomplete features for substitutions.
```

## Commit scope

**blueprint-ls**

Example commit:

```bash
git commit -m 'fix(blueprint-ls): correct syntax highlighting bug'
```
