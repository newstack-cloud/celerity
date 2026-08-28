# Releasing the Python Runtime

## Version Strategy

The runtime version tracks `celerity-sdk` on **major.minor**, and owns its patch component.
The Celerity CLI resolves the SDK version a project declares to the runtime image's minor
tag, so it maps between the two without a lookup table.

```
runtime major.minor == SDK major.minor      patch belongs to the runtime
```

The patch component is held out of the mapping so the runtime can ship a rebuild of its
own, for a base image CVE or a fix in the wrapper, without waiting on an SDK release. A
project declaring `celerity-sdk[runtime]>=0.3.0` resolves to the `dev-0.3` tag, which
carries the newest runtime patch on that minor, so it picks the rebuild up without editing
a pin.

The two version numbers are no longer readable as "this image contains exactly that SDK".
Runtime `0.3.2` may carry SDK `0.3.1`. The exact set is recorded in `runtime-manifest.json`,
written at image build time by `generate_manifest.py`, and logged at startup by `main.py`.

This rests on the SDK keeping breaking changes out of patch releases. Under `0.x` that is a
convention rather than something semver guarantees, so a breaking `0.3.x` patch would need
a runtime minor bump to keep the two in step.

- `celerity-runtime-sdk` (the PyO3 package) is versioned independently — it lives in the
  `libs/runtime/sdk/python` directory of this repo and has its own release-please component
  (`runtime-sdk-python`).
- `celerity-sdk` (the handler framework) is versioned in the
  [celerity-python-sdk](https://github.com/newstack-cloud/celerity-python-sdk) repo.

## Updating SDK Dependencies

### A new SDK minor or major (e.g. `0.3.x` → `0.4.0`)

The runtime has to land on that exact minor, so the `Release-As` footer forces it:

```bash
cd apps/runtime/python

# 1. Update celerity-sdk dependency in pyproject.toml
#    celerity-sdk[runtime] → >=0.4.0

# 2. Commit with Release-As footer to move the runtime onto the new minor
git commit -m "deps(runtime-python): update celerity-sdk to 0.4.0

Release-As: 0.4.0"

# 3. Push to main (or open a PR)
```

### An SDK patch, or a runtime-only fix

This covers a patch-level SDK release, a base image rebuild for a CVE, and any fix to the
wrapper itself. The runtime's patch number is its own, so there is no version to force:

```bash
cd apps/runtime/python

# 1. Refresh the lock file (the declared floor already admits the new SDK patch)
uv lock --upgrade-package celerity-sdk

# 2. Commit with no Release-As footer, letting release-please bump the patch
git commit -m "deps(runtime-python): bump up celerity-sdk 0.3.4"
```

Do not add a `Release-As` footer here. Pinning the runtime to the SDK's patch number is what
makes the sequence unreleasable: a runtime already at `0.3.1` from a CVE rebuild cannot then
be released as the SDK's `0.3.1`.

## Release Flow

1. **release-please** detects the commit on `main` and creates a release PR that bumps the
   version, to the version named by a `Release-As` footer if there is one and by the
   conventional commit bump rules otherwise.
2. **Merge** the release PR — release-please creates tag `runtime-python/v0.4.0`.
3. The `release-please.yml` `post-process-tags` job dispatches `app-runtime-python-release.yml`
   with the tag.
4. The release workflow builds each architecture, scans it, and only then assembles the
   manifest that carries the tags, so an image that fails its scan is never tagged:
   - Production: `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:0.4.0`, `:0.4`, `:latest`
   - Dev: `ghcr.io/newstack-cloud/celerity-runtime-python-3-13:dev-0.4.0`, `:dev-0.4`, `:dev-latest`
5. Images are signed (cosign keyless) and attested (SBOM + build provenance). Signing runs
   off the published manifests, which only exist if every scan passed.

`:dev-0.4` is the tag the CLI resolves to. `:dev-0.4.0` stays available for pinning an
exact runtime patch. A prerelease publishes no minor tag, and the CLI resolves such a
version to its full `dev-` tag instead.

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
(configured in `release-please-config.json`). On the commit that moves the runtime onto a
new SDK minor, the `Release-As` footer overrides the conventional commit bump rules so the
runtime lands on that minor exactly.
