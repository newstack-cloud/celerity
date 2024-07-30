# Contributing to celerity runtime packages

## Prerequisites

- [Rust](https://www.rust-lang.org/tools/install) >=1.76.0
- [Cargo workspaces](https://crates.io/crates/cargo-workspaces) >=0.3.2 - tool to managing multiple rust crates in a single repository
- [Pipenv](https://pypi.org/project/pipenv/) >=2022.8.5 - Python package manager for integration test harness
- Docker Engine >=25.0.3 - For running integration test dependencies (Comes with Docker Desktop)
- Docker Compose >=2.24.6 - For running integration test dependencies (Comes with Docker Desktop)
- [cargo-llvm-cov](https://crates.io/crates/cargo-llvm-cov) >=0.6.11 - For generating code coverage reports
- [cargo-insta](https://crates.io/crates/cargo-insta) - For snapshot test reviews

Package dependencies are managed with cargo and will be installed automatically when you first
run tests.

## Getting started

### Integration Test Harness Setup

This project uses a virtual environment managed by pipenv for running integration tests.

Run the following to install dependencies:

```bash
pipenv install
# dev dependencies
pipenv install -d
```

### Install cargo-llvm-cov on your machine

This tool is used to generate accurate code coverage reports when running tests.

```bash
cargo install cargo-llvm-cov
```

### Install cargo-insta for snapshot test reviews

This tool is used to provide an improved snapshot test review experience.

```bash
cargo install cargo-insta
```

### VSCode settings (Skip if not using VSCode)

Copy `.vscode/settings.json.example` to `.vscode/settings.json` and set `python.defaultInterpreterPath` to the absolute path of python in the virtualenv created by pipenv for running integration tests.

## Running tests

```bash
PIPENV_DOTENV_LOCATION=.env.test pipenv run python scripts/package-test-tools.py --localdeps
```

### Reviewing snapshot tests

When snapshot tests fail due to changes that you have made, you need to carefully review the changes before accepting them.
`cargo-insta` provides a tool to review the changes and accept them if they are correct, this is to be used after a test run that failed due to snapshot changes.

```bash
cargo insta review
```

## Test harness dependencies

Every time the dependencies in the Pipfile or Pipfile.lock are updated, the test harness `requirements.txt` file must be updated to reflect these changes.
This is because Pipenv is not used in the CI environments.

```bash
pipenv requirements > requirements.txt
```

## Releasing

To release a new version of the library, you need to create a new tag and push it to the repository.

The format must be `libs/blueprint/vX.Y.Z` where `X.Y.Z` is the semantic version number.
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
git tag -a libs/blueprint/v0.2.0 -m "chore(blueprint): Release v0.2.0"
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
GOPROXY=proxy.golang.org go list -m github.com/two-hundred/celerity/libs/blueprint@v0.2.0
```

## Commit scope

**blueprint**

Example commit:

```bash
git commit -m 'fix(blueprint): correct cyclic dependency bug'
```
