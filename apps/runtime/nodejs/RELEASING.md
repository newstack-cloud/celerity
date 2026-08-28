# Releasing the Node.js Runtime

## Version Strategy

The runtime version tracks `@celerity-sdk/*` on **major.minor**, and owns its patch
component. The Celerity CLI resolves a project's declared SDK range to the runtime image's
minor tag, so it maps between the two without a lookup table.

```
runtime major.minor == SDK major.minor      patch belongs to the runtime
```

The patch component is held out of the mapping so the runtime can ship a rebuild of its
own, for a base image CVE or a fix in the wrapper, without waiting on an SDK release. A
project declaring `^0.9.0` is asking for any `0.9.x`, which is exactly what the `dev-0.9`
tag resolves to, so it picks the rebuild up without editing a pin.

The two version numbers are no longer readable as "this image contains exactly that SDK".
Runtime `0.9.2` may carry SDK `0.9.1`. The exact set is recorded in `runtime-manifest.json`,
written at image build time by `generate-manifest.mjs`, and logged at startup by `index.mjs`.

This rests on the SDK keeping breaking changes out of patch releases. Under `0.x` that is a
convention rather than something semver guarantees, so a breaking `0.9.x` patch would need
a runtime minor bump to keep the two in step.

- `@celerity-sdk/runtime` (the NAPI package) is versioned independently, it lives in this
  repo and has its own release-please component (`runtime-sdk-node`).
- All other `@celerity-sdk/*` packages (`core`, `config`, `common`, `telemetry`, `types`)
  are versioned in unison in the [celerity-node-sdk](https://github.com/newstack-cloud/celerity-node-sdk) repo.

## Updating SDK Dependencies

### A new SDK minor or major (e.g. `0.9.x` → `0.10.0`)

The runtime has to land on that exact minor, so the `Release-As` footer forces it:

```bash
cd apps/runtime/nodejs

# 1. Update SDK dependencies in package.json
#    @celerity-sdk/core, /config, /common, /telemetry, /types → ^0.10.0
#    @celerity-sdk/runtime stays at its own version

# 2. Update lockfile
yarn install

# 3. Commit with Release-As footer to move the runtime onto the new minor
git commit -m "deps(runtime-nodejs): update @celerity-sdk/* to 0.10.0

Release-As: 0.10.0"

# 4. Push to main (or open a PR)
```

### An SDK patch, or a runtime-only fix

This covers a patch-level SDK release, a base image rebuild for a CVE, and any fix to the
wrapper itself. The runtime's patch number is its own, so there is no version to force:

```bash
cd apps/runtime/nodejs

# 1. Update the lockfile (a caret range already admits the new SDK patch)
yarn install

# 2. Commit with no Release-As footer, letting release-please bump the patch
git commit -m "deps(runtime-nodejs): bump up @celerity-sdk/* 0.9.4"
```

Do not add a `Release-As` footer here. Pinning the runtime to the SDK's patch number is what
makes the sequence unreleasable as a runtime already at `0.9.1` from a CVE rebuild cannot then
be released as the SDK's `0.9.1`.

## Release Flow

1. **release-please** detects the commit on `main` and creates a release PR that bumps
   `package.json`, to the version named by a `Release-As` footer if there is one and by the
   conventional commit bump rules otherwise.
2. **Merge** the release PR — release-please creates tag `runtime-nodejs/v0.10.0`.
3. The `release-please.yml` `post-process-tags` job dispatches `runtime-nodejs-release.yml`
   with the tag.
4. The release workflow builds each architecture, scans it, and only then assembles the
   manifest that carries the tags, so an image that fails its scan is never tagged:
   - Production: `ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:0.10.0`, `:0.10`, `:latest`
   - Dev: `ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:dev-0.10.0`, `:dev-0.10`, `:dev-latest`
5. Images are signed (cosign keyless) and attested (SBOM + build provenance). Signing runs
   off the published manifests, which only exist if every scan passed.

`:dev-0.10` is the tag the CLI resolves to. `:dev-0.10.0` stays available for pinning an
exact runtime patch. A prerelease publishes no minor tag, and the CLI resolves such a
version to its full `dev-` tag instead.

## Image Verification

```bash
# Verify cosign signature
cosign verify \
  ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:0.4.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"

# Verify SBOM attestation
cosign verify-attestation --type spdxjson \
  ghcr.io/newstack-cloud/celerity-runtime-nodejs-24:0.4.0 \
  --certificate-identity-regexp="github.com/newstack-cloud" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Changelog

The `deps` commit type appears under "Dependencies" in the auto-generated changelog
(configured in `release-please-config.json`). On the commit that moves the runtime onto a
new SDK minor, the `Release-As` footer overrides the conventional commit bump rules so the
runtime lands on that minor exactly.
