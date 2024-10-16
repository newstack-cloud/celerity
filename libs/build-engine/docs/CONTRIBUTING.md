# Contributing to the celerity build engine

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

## Generating gRPC protobuf code

The build engine uses gRPC for the provider and transform plugin system.

1. Follow the instructions [here](https://grpc.io/docs/protoc-installation/#install-using-a-package-manager) to install the `protoc` compiler.

2. Install the Go protoc plugins:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

3. Run the following command from the `libs/build-engine` directory to generate the gRPC protobuf code for the plugin service that plugins register with:

```bash
protoc --proto_path=.. --go_out=.. --go_opt=paths=source_relative \
  --go-grpc_out=.. --go-grpc_opt=paths=source_relative \
  build-engine/plugin/pluginservice/service.proto
```

4. Run the following command from the `libs/build-engine` directory to generate the gRPC protobuf code for provider plugins:

```bash
protoc --proto_path=.. --go_out=.. --go_opt=paths=source_relative \
  --go-grpc_out=.. --go-grpc_opt=paths=source_relative \
  build-engine/plugin/providerserverv1/provider.proto
```

5. Run the following command from the `libs/build-engine` directory to generate the gRPC protobuf code for transform plugins:

```bash
protoc --proto_path=.. --go_out=.. --go_opt=paths=source_relative \
  --go-grpc_out=.. --go-grpc_opt=paths=source_relative \
  build-engine/plugin/transformerserverv1/transformer.proto
```

## Releasing

To release a new version of the library, you need to create a new tag and push it to the repository.

The format must be `libs/build-engine/vX.Y.Z` where `X.Y.Z` is the semantic version number.
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
git tag -a libs/build-engine/v0.2.0 -m "chore(build-engine): Release v0.2.0"
git push --tags
```

Be sure to add a release for the tag with notes following this template:

Title: `Build Engine - v0.2.0`

```markdown
## Fixed:

- Corrects claims handling for JWT middleware.

## Added

- Adds dihandlers-compatible middleware for access control.
```

3. Prompt Go to update its index of modules with the new release:

```bash
GOPROXY=proxy.golang.org go list -m github.com/two-hundred/celerity/libs/build-engine@v0.2.0
```

## Commit scope

**blueprint**

Example commit:

```bash
git commit -m 'fix(build-engine): correct cyclic dependency bug'
```
