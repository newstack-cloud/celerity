# Contributing to the plugin docgen tool

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

## Building the plugin docgen binary

```bash
go generate ./...
go build -o plugin-docgen ./cmd/main.go
```

For releases, you will need to make sure that the generated code includes the new version number in the generated versions.go file.
You can do this by running the following command:

```bash
export PLUGIN_DOCGEN_APPLICATION_VERSION="v0.2.0"
go generate ./...
go build -o plugin-docgen ./cmd/main.go
```

Replace `v0.2.0` with the version you are releasing, keeping the `v` prefix.

The `versions.go` file is committed to version control so that the `go install` command can be used to install the binary directly from source.
The trunk should contain the `versions.go` file generated for the latest release.

## Releasing

To release a new version of the server, you need to create a new tag and push it to the repository.

The format must be `tools/plugin-docgen/vX.Y.Z` where `X.Y.Z` is the semantic version number.
The reason for this is that Go's mechanism for picking up modules from multi-repo packages is based on the sub-directory path being in the version tag.

This is a binary application but follows the same convention as a library for consistency.

See [here](https://go.dev/wiki/Modules#publishing-a-release).

1. add a change log entry to the `CHANGELOG.md` file following the template below:

```markdown
## [0.2.0] - 2024-06-05

### Fixed:

- Adds correction for missing config field generation.

### Added

- Adds support for generating metadata for the plugin docgen tool.
```

2. Create and push the new tag prefixed by sub-directory path:

```bash
git tag -a tools/plugin-docgen/v0.2.0 -m "chore(plugin-docgen): Release v0.2.0"
git push --tags
```

Be sure to add a release for the tag with notes following this template:

Title: `Plugin JSON Doc Generator - v0.2.0`

```markdown
## Fixed:

- Adds correction for missing config field generation.

## Added

- Adds support for generating metadata for the plugin docgen tool.
```

## Commit scope

**plugin-docgen**

Example commit:

```bash
git commit -m 'fix(plugin-docgen): add correction to config field generation'
```
