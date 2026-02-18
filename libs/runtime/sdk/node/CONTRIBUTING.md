# Contributing to Celerity Runtime SDK for Node.js

## Prerequisites

- [Node.js](https://nodejs.org/) >=22.0.0
- [Rust](https://www.rust-lang.org/tools/install) >=1.76.0
- [Yarn](https://yarnpkg.com/) (will be installed via corepack)
- [Git](https://git-scm.com/)

## Installation

1. **Enable corepack (if not already enabled):**
   ```bash
   corepack enable
   ```

2. **Install dependencies:**
   ```bash
   yarn install
   ```

## Development

### Building

Build the native module for all platforms:

```bash
yarn build
```

Build for a specific target:

```bash
yarn build --target x86_64-apple-darwin
yarn build --target aarch64-unknown-linux-gnu
yarn build --target x86_64-pc-windows-msvc
```

### Running Tests

```bash
yarn test
```

### Development Build

For faster iteration during development:

```bash
yarn build:debug
```

## Releasing

Releases are managed by [release-please](https://github.com/googleapis/release-please) and triggered automatically through conventional commits.

### How It Works

1. **Merge a conventional commit** to `main` that touches files in `libs/runtime/sdk/node/`:
   ```
   feat(runtime-sdk-node): add support for custom middleware
   fix(runtime-sdk-node): correct handler timeout resolution
   ```

2. **release-please opens a release PR** that bumps `package.json` version, updates the changelog, and updates platform-specific `npm/*/package.json` versions.

3. **Merge the release PR** — release-please creates a Git tag (`runtime-sdk-node/v1.2.3`) and a GitHub release.

4. **CI automatically publishes** — the release workflow builds native modules for all supported platforms, runs tests, and publishes to NPM as `@celerity-sdk/runtime`.

### Core Library Changes

When a shared Rust crate (`runtime-core`, `runtime-consumers`, or `runtime-ws`) is released but no SDK binding code changed, the release-please workflow automatically cascades the release to all SDK packages. It commits a `.core-version` marker file into each SDK directory, which triggers release-please to open SDK release PRs on the next run.

No manual action is needed — core releases automatically propagate to the Node SDK, Python SDK, and FFI SDK.

### Verification

After the release workflow completes, verify the package is available on NPM:
```bash
npm info @celerity-sdk/runtime versions --json | tail -5
```

### Supported Platforms

The SDK is built and tested for the following platforms:

- **macOS**: x86_64, aarch64 (Apple Silicon)
- **Linux**: x86_64 (GNU), x86_64 (musl), aarch64 (GNU), aarch64 (musl)
- **Windows**: x86_64, aarch64
- **FreeBSD**: x86_64
