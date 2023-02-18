# common library

The common library provides common utility packages used in Celerity.

## Getting set up

### Prerequisites

- [Go](https://golang.org/dl/) >=1.20

Dependencies are managed with Go modules (go.mod) and will be installed automatically when you first
run tests.

If you want to install dependencies manually you can run:

```bash
go mod download
```

## Running tests

```bash
bash ./scripts/run-tests.sh --tags unit
```

## Release tag format

Release tags for the common library should be created in the following format:

```
common-MAJOR.MINOR.PATCH

e.g. common-0.1.0
```

## Commit scope

**common**

Example commit:

```bash
git commit -m 'fix(common): correct slice search util function'
```
