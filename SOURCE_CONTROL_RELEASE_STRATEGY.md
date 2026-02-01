# Source Control & Release Strategy

## Source control & development workflow

- Development work by core contributes should be carried out on the main branch for most contributions, with the exception being longer projects (weeks or months worth of work) or experimental new versions of a package or application. For the exceptions, feature/hotfix branches should be used.
- All development work by non-core contributes should be carried out on feature/hotfix branches on your fork, pull requests should be utilised for code reviews and merged (**rebase!**) back into the main branch of the primary repo.
- All commits should follow the [commit guidelines](./COMMIT_GUIDELINES.md).
- Work should be commited in small, specific commits where it makes sense to do so.

## Release strategy

To allow for a high degree of flexibility, every key component has its own version. Releases are automated via [release-please](https://github.com/googleapis/release-please), which creates release PRs with changelogs based on conventional commits.

Release-please creates tags in the format `{component}/v{version}` for all components:

```
{component}/v{MAJOR}.{MINOR}.{PATCH}

e.g. cli/v0.1.0
     runtime-core/v1.0.0
     runtime-consumers/v1.0.0
     runtime-ws/v1.0.0
     runtime-sdk-ffi/v1.0.0
     runtime-sdk-node/v0.2.0
```

**Go modules** are the only exception that require additional tag processing. The Go module proxy (`proxy.golang.org`) requires tags that match the module's directory path within the repository, not the release-please component name. For Go components, the post-process-tags job creates a path-based tag and re-associates the GitHub release:

```
Component tag (release-please): cli/v0.1.0
Go module tag (for proxy):      apps/cli/v0.1.0
```

Non-Go components do not need path-based tags — the component tag created by release-please is sufficient. Their release workflows are triggered via `workflow_dispatch` from the post-process-tags job.

**_A key component consists of one or more related applications and libraries. (e.g. the nodejs runtime which consists of an application and multiple NPM packages)_**

### Key components

| Component | Path | Description |
|---|---|---|
| `cli` | `apps/cli` | CLI for test/build/package/deploy tooling |
| `runtime-core` | `libs/runtime` | Core runtime libraries (core, workflow, signature, helpers, blueprint parser) |
| `runtime-consumers` | `libs/runtime/consumers` | Message consumer crates (SQS, Redis, Kinesis, Azure Service Bus, Azure Events Hub, GCloud Pub/Sub, GCloud Tasks) |
| `runtime-ws` | `libs/runtime/ws` | WebSocket crates (ws-registry, ws-redis) |
| `runtime-sdk-ffi` | `libs/runtime/sdk` | FFI binding crates for Java/.NET. A release triggers the downstream Java/.NET SDK build and publish. |
| `runtime-sdk-node` | `libs/runtime/sdk/node` | Node.js runtime SDK bindings |
| `runtime-sdk-python` | `libs/runtime/sdk/python` | Python runtime SDK bindings |
| `runtime-nodejs` | `apps/runtime/nodejs` | Node.js runtime wrapper application (Docker image published to GHCR) |
| `runtime-python` | `apps/runtime/python` | Python runtime wrapper application (Docker image published to GHCR) |

For details on how commit scopes map to component groups, see the [commit guidelines](./COMMIT_GUIDELINES.md#how-commit-scopes-relate-to-releases).

## Release workflow

Releases are managed automatically by release-please. Each component gets its own release PR, providing individual review and control over each release.

1. Merge changes into the trunk (main) using [conventional commits](./COMMIT_GUIDELINES.md).
2. Release-please automatically creates a release PR for each affected component. The PR includes a generated changelog based on commits since the last release.
3. Review and edit the release PR notes as needed.
4. Merge the release PR into main. This triggers:
   - A git tag in the format `{component}/v{version}` is created by release-please
   - The `post-process-tags` job runs and:
     - For **Go components only**: creates an additional path-based tag (`{path}/v{version}`), re-associates the GitHub release with it, and indexes the module with the Go module proxy
     - For components with release workflows: dispatches the component-specific release workflow via `workflow_dispatch` to build and publish artifacts

### Release configuration

- `release-please-config.json` — Defines the component groups, their paths, and release types
- `.release-please-manifest.json` — Tracks the current version of each component
- `.github/workflows/release-please.yml` — Orchestrates the release-please process and post-processing
