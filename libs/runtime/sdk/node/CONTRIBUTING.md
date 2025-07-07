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

To release a new version of the Node.js Runtime SDK, follow these steps:

### Pre-Release Checklist:

1. **Update Version in package.json**
   ```bash
   # Edit libs/runtime/sdk/node/package.json
   # Change the version field to match your release
   "version": "1.2.3"
   ```

2. **Update Version in Cargo.toml**
   ```bash
   # Edit libs/runtime/sdk/node/Cargo.toml
   # Change the version field to match your release
   version = "1.2.3"
   ```

3. **Commit the Version Changes**
   ```bash
   git add libs/runtime/sdk/node/package.json libs/runtime/sdk/node/Cargo.toml
   git commit -m "chore(lib-rt-sdk-node): bump version to 1.2.3"
   git push origin main
   ```

4. **Create and Push Release Tag**
   ```bash
   # Note: Use 'v' prefix in the tag name
   git tag -a libs/runtime/sdk/node-v1.2.3 -m "Release Celerity Runtime SDK for Node.js v1.2.3"
   git push origin libs/runtime/sdk/node-v1.2.3
   ```

### Version Format Guidelines:

- **package.json**: Use `"1.2.3"` (no 'v' prefix)
- **Cargo.toml**: Use `1.2.3` (no 'v' prefix)
- **Git Tags**: Use `libs/runtime/sdk/node-v1.2.3` (with 'v' prefix)
- **NPM Package**: Will be published as `@celerity-sdk/runtime 1.2.3`

### What Happens After Tagging:

1. **CI/CD Pipeline**: The GitHub Actions workflow will automatically:
   - Build native modules for all supported platforms
   - Run tests across multiple Node.js versions and platforms
   - Extract version from tag (e.g., `v1.2.3` from `libs/runtime/sdk/node-v1.2.3`)
   - Publish to NPM as `@celerity-sdk/runtime 1.2.3`

2. **Verification**: Check that the package is available on NPM:
   ```bash
   npm install @celerity-sdk/runtime@1.2.3
   ```

### Supported Platforms:

The SDK is built and tested for the following platforms:

- **macOS**: x86_64, aarch64 (Apple Silicon)
- **Linux**: x86_64 (GNU), x86_64 (musl), aarch64 (GNU), aarch64 (musl)
- **Windows**: x86_64, aarch64
- **FreeBSD**: x86_64

### Important Notes:

- All tests must pass before the package is published
- Cross-platform compatibility is verified automatically
- The published package version will be clean (no monorepo prefixes)
