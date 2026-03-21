# Releasing the Python Runtime

## Version Strategy

The runtime version tracks `celerity-sdk` package versions so the Celerity CLI can map
between SDK and runtime image versions without a lookup table.

- `celerity-runtime-sdk` (the PyO3 package) is versioned independently — it lives in the
  `libs/runtime/sdk/python` directory of this repo and has its own release-please component
  (`runtime-sdk-python`).
- `celerity-sdk` (the handler framework) is versioned in the
  [celerity-python-sdk](https://github.com/newstack-cloud/celerity-python-sdk) repo.

## Updating SDK Dependencies

When a new `celerity-sdk` version is released (e.g., `0.3.0`):

```bash
cd apps/runtime/python

# 1. Update celerity-sdk dependency in pyproject.toml
#    celerity-sdk[runtime] → >=0.3.0

# 2. Commit with Release-As footer to force version alignment
git commit -m "deps(runtime-python): update celerity-sdk to 0.3.0

Release-As: 0.3.0"

# 3. Push to main (or open a PR)
```

## Release Flow

1. **release-please** detects the `deps` commit on `main` and creates a release PR that
   bumps the version to `0.3.0` (the `Release-As` footer forces exact version).
2. **Merge** the release PR — release-please creates tag `runtime-python/v0.3.0`.
3. The `release-please.yml` `post-process-tags` job dispatches `app-runtime-python-release.yml`
   with the tag.
4. The release workflow builds and pushes Docker images to GHCR:
   - Production: `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:0.3.0`, `:0.3`, `:latest`
   - Dev: `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-0.3.0`, `:dev-latest`
5. Images are scanned (Trivy), signed (cosign keyless), and attested (SBOM + build provenance).

## Image Verification

```bash
# Verify cosign signature
cosign verify \
  ghcr.io/newstack-cloud/celerity-runtime-python-3-13:0.3.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Verify SBOM attestation
cosign verify-attestation --type spdxjson \
  ghcr.io/newstack-cloud/celerity-runtime-python-3-13:0.3.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Changelog

The `deps` commit type appears under "Dependencies" in the auto-generated changelog
(configured in `release-please-config.json`). The `Release-As` footer ensures the version
number matches the SDK, regardless of conventional commit bump rules.
