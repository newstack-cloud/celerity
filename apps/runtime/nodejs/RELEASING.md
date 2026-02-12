# Releasing the Node.js Runtime

## Version Strategy

The runtime version tracks `@celerity-sdk/*` package versions so the Celerity CLI can map
between SDK and runtime image versions without a lookup table.

- `@celerity-sdk/runtime` (the NAPI package) is versioned independently — it lives in this
  repo and has its own release-please component (`runtime-sdk-node`).
- All other `@celerity-sdk/*` packages (`core`, `config`, `common`, `telemetry`, `types`)
  are versioned in unison in the [celerity-node-sdk](https://github.com/newstack-cloud/celerity-node-sdk) repo.

## Updating SDK Dependencies

When a new `@celerity-sdk/*` version is released (e.g., `0.4.0`):

```bash
cd apps/runtime/nodejs

# 1. Update SDK dependencies in package.json
#    @celerity-sdk/core, /config, /common, /telemetry, /types → ^0.4.0
#    @celerity-sdk/runtime stays at its own version

# 2. Update lockfile
yarn install

# 3. Commit with Release-As footer to force version alignment
git commit -m "deps(runtime-nodejs): update @celerity-sdk/* to 0.4.0

Release-As: 0.4.0"

# 4. Push to main (or open a PR)
```

## Release Flow

1. **release-please** detects the `deps` commit on `main` and creates a release PR that
   bumps `package.json` version to `0.4.0` (the `Release-As` footer forces exact version).
2. **Merge** the release PR — release-please creates tag `runtime-nodejs/v0.4.0`.
3. The `release-please.yml` `post-process-tags` job dispatches `runtime-nodejs-release.yml`
   with the tag.
4. The release workflow builds and pushes Docker images to GHCR:
   - Production: `ghcr.io/newstack-cloud/celerity-runtime-nodejs:0.4.0`, `:0.4`, `:latest`
   - Dev: `ghcr.io/newstack-cloud/celerity-runtime-nodejs:dev-0.4.0`, `:dev-latest`
5. Images are scanned (Trivy), signed (cosign keyless), and attested (SBOM + build provenance).

## Image Verification

```bash
# Verify cosign signature
cosign verify \
  ghcr.io/newstack-cloud/celerity-runtime-nodejs:0.4.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Verify SBOM attestation
cosign verify-attestation --type spdxjson \
  ghcr.io/newstack-cloud/celerity-runtime-nodejs:0.4.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Changelog

The `deps` commit type appears under "Dependencies" in the auto-generated changelog
(configured in `release-please-config.json`). The `Release-As` footer ensures the version
number matches the SDK, regardless of conventional commit bump rules.
