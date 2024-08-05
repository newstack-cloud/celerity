# Contributing to Celerity Runtime SDK for Python

## Prerequisites

- [Rust](https://www.rust-lang.org/tools/install) >=1.76.0
- Clippy (`rustup component add clippy`)
- [Cargo workspaces](https://crates.io/crates/cargo-workspaces) >=0.3.2 - tool to managing multiple rust crates in a single repository
- [Pipenv](https://pypi.org/project/pipenv/) >=2022.8.5 - Python package manager for test harness

## Installation

Test harness dependencies need to be installed with pipenv:

```bash
pipenv install -d
```

## Running tests

```bash
./scripts/run-tests.sh
```
