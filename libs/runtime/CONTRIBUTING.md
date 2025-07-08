# Contributing to celerity runtime packages

## Prerequisites

- [Rust](https://www.rust-lang.org/tools/install) >=1.76.0
- Clippy (`rustup component add clippy`)
- [Cargo workspaces](https://crates.io/crates/cargo-workspaces) >=0.3.2 - tool to managing multiple rust crates in a single repository
- [Pipenv](https://pypi.org/project/pipenv/) >=2022.8.5 - Python package manager for integration test harness
- Docker Engine >=25.0.3 - For running integration test dependencies (Comes with Docker Desktop)
- Docker Compose >=2.24.6 - For running integration test dependencies (Comes with Docker Desktop)
- [cargo-llvm-cov](https://crates.io/crates/cargo-llvm-cov) >=0.6.11 - For generating code coverage reports
- [cargo-insta](https://crates.io/crates/cargo-insta) - For snapshot test reviews
- _Optional_ - [dotnet](https://dotnet.microsoft.com/download) >=8.0.7 - For building the C#/.NET runtime SDK and running tests
- _Optional_ - [java development kit](https://www.oracle.com/uk/java/technologies/downloads/) >=21.0.1 - For building the Java runtime SDK and running tests

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

## SDK Generation and Testing (Java and C#)

[oo_bindgen](https://github.com/stepfunc/oo_bindgen) is used for generating the Java and C# runtime SDKs from the Rust runtime packages.

To build the SDKs and run the accompanying tests, you can run the following script in unix-based systems:

```bash
./scripts/build-test-sdk-bindgen.sh
```

For Windows you can run:

```powershell
.\scripts\build-test-sdk-bindgen.ps1
```

## Releasing

See the specific release process and instructions for each individual SDK packages:

- [Python](sdk/python/CONTRIBUTING.md)
- [Node.js](sdk/node/CONTRIBUTING.md)
- [Java](sdk/bindings/java/CONTRIBUTING.md)
- [C#/.NET](sdk/bindings/dotnet/CONTRIBUTING.md)

## Commit scope

**runtime-libs**

Example commit:

```bash
git commit -m 'test(runtime-libs): update runtime-libs combined test runner'
```

Each individual SDK package has its own, more granular commit scope that should be used for commits that are specific to that package.
