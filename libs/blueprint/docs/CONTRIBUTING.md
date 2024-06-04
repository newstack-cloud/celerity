# Contributing to the blueprint framework

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

# to re-generate snapshots (For spec/schema tests)
bash scripts/run-tests.sh --update-snapshots
```

## Generating protobuf code

The blueprint framework uses protobuf to store and transmit an expanded version of a blueprint. Expanded blueprints include AST-like expansions of substitutions that can be cached with an implementation of the `cache.BlueprintCache` interface.

1. Follow the instructions [here](https://grpc.io/docs/protoc-installation/#install-using-a-package-manager) to install the `protoc` compiler.

2. Install the Go protoc plugin:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

3. Run the following command from the `libs/blueprint` directory to generate the protobuf code:

```bash
protoc --go_out=./pkg ./schema.proto
```

## Release tag format

Release tags for the common library should be created in the following format:

```
blueprint-MAJOR.MINOR.PATCH

e.g. blueprint-0.1.0
```

## Commit scope

**blueprint**

Example commit:

```bash
git commit -m 'fix(blueprint): correct cyclic dependency bug'
```
