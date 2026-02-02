# Contributing to Celerity Runtime SDK for Python

## Prerequisites

- [Rust](https://www.rust-lang.org/tools/install) >=1.76.0
- Clippy (`rustup component add clippy`)
- [Cargo workspaces](https://crates.io/crates/cargo-workspaces) >=0.3.2 - tool to managing multiple rust crates in a single repository
- [uv](https://docs.astral.sh/uv/) >=0.5 - Python package and project manager for test harness

## Installation

Test harness dependencies need to be installed with uv:

```bash
uv sync --group dev
```

## Running tests

```bash
./scripts/run-tests.sh
```

## Releasing

To release a new version of the Python Runtime SDK, follow these steps:

### Pre-Release Checklist:

1. **Update Version in Cargo.toml**

   ```bash
   # Edit libs/runtime/sdk/python/Cargo.toml
   # Change the version field to match your release
   version = "1.2.3"  # Remove the 'v' prefix for Cargo.toml
   ```

2. **Commit the Version Change**

   ```bash
   git add libs/runtime/sdk/python/Cargo.toml
   git commit -m "chore(lib-rt-sdk-python): bump version to 1.2.3"
   git push origin main
   ```

3. **Create and Push Release Tag**
   ```bash
   # Note: Use 'v' prefix in the tag name
   git tag -a libs/runtime/sdk/python-v1.2.3 -m "Release Celerity Runtime SDK for Python v1.2.3"
   git push origin libs/runtime/sdk/python-v1.2.3
   ```

### Version Format Guidelines:

- **Cargo.toml**: Use `1.2.3` (no 'v' prefix)
- **Git Tags**: Use `libs/runtime/sdk/python-v1.2.3` (with 'v' prefix)
- **PyPI Package**: Will be published as `celerity-runtime-sdk 1.2.3`

### What Happens After Tagging:

1. **CI/CD Pipeline**: The GitHub Actions workflow will automatically:

   - Build wheels for all supported platforms
   - Run tests across multiple platforms
   - Extract version from tag (e.g., `v1.2.3` from `libs/runtime/sdk/python-v1.2.3`)
   - Publish to PyPI as `celerity-runtime-sdk 1.2.3`

2. **Verification**: Check that the package is available on PyPI:
   ```bash
   pip install celerity-runtime-sdk==1.2.3
   ```

### Important Notes:

- ✅ All tests must pass before the package is published
- ✅ Cross-platform compatibility is verified automatically
- ✅ The published package version will be clean (no monorepo prefixes)
- ✅ Version changes are tracked in git history for audit purposes
