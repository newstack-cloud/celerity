# Contributing to Celerity

## Setup

Ensure git uses the custom directory for git hooks so the pre-commit and commit-msg linting hooks
kick in.

```bash
git config core.hooksPath .githooks
```

### NPM dependencies

There are npm dependencies that provide tools that are used in git hooks and scripting that span
multiple applications and libraries.

Install dependencies from the root directory by simply running:
```bash
yarn
```

## Build-time

Tools and libraries that make up the Celerity "build-time engine" that handles everything involved in validating source code/configuration, parsing, packaging and deploying Celerity backend services.

### Binary applications

- [Celerity CLI](./apps/cli)
- [Celerity API](./apps/api)

### Libraries

- [Blueprint](./libs/blueprint)
- [Build Engine](./libs/build-engine)
- [Common](./libs/common)

### Validators

## Runtime

The runtime powers applications built with Celerity across all the supported languages. The core runtime is Rust with FFI/C API interfaces exposed through libraries in each language supported by Celerity.
The runtime provides a framework to run a HTTP/WebSocket API server and async message consumers (polling).

The runtime is designed to be used in local emulator environments and for containerised deployments of a Celerity backend application.

### Libraries

- [Runtime Core](./runtime/core)

### Runtimes

The runtime directories host instances of the core Rust runtime embedded into thin wrapper applications along with automatically generated SDKs for supported Celerity languages.

Each runtime has an accompanying docker image that are used in the build-time phase in deploying applications as containerised services. 

- [Go](./runtime/go)
- [Java](./runtime/java)
- [NodeJS](./runtime/nodejs)
- [Python](./runtime/python)
- [Rust](./runtime/rust)

## Templates

Templates provide sample applications built with the Celerity framework that are used as boilerplates when creating new Celerity projects.

- [Go Templates](./templates/go)
- [Java Templates](./templates/java)
- [NodeJS Templates](./templates/nodejs)
- [Python Templates](./templates/python)
- [Rust Templates](./templates/rust)

## Tools

Tools provide test harnesses along with the packaging/release tooling for all the components that make up the Celerity framework.

- [Releaser CLI](./tools/releaser) - The releaser CLI used to manage releases for Celerity libraries and applications.
- [Test Runner CLI](./tools/test-runner) - The test runner CLI handles creating test environments for integration/integrated tests for Celerity libraries and applications.

## Further documentation

- [Commit Guidelines](./COMMIT_GUIDELINES.md)
- [Source Control and Release Strategy](./SOURCE_CONTROL_RELEASE_STRATEGY.md)