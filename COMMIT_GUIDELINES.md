# Commit Guidelines

## 1) Conventional Commits

For all contributions to this repo, you must use the conventional commits standard defined [here](https://www.conventionalcommits.org/en/v1.0.0/).

This is used to generate automated change logs, allow for tooling to decide semantic versions for packages and applications,
provide a rich and meaningful commit history along with providing
a base for more advanced tooling to allow for efficient searches for decisions and context related to commits and code.

### Commit types

**The following commit types are supported in the Celerity project:**

- `fix:` - Should be used for any bug fixes.
- `build:` - Should be used for functionality related to building an application.
- `revert:` - Should be used for any commits that revert changes.
- `wip:` - Should be used for commits that contain work in progress.
- `feat:` - Should be used for any new features added, regardless of the size of the feature.
- `chore:` - Should be used for tasks such as releases or patching dependencies.
- `ci:` - Should be used for any work on GitHub Action workflows or scripts used in CI.
- `docs:`- Should be used for adding or modifying documentation.
- `style:` - Should be used for code formatting commits and linting fixes.
- `refactor:` - Should be used for any type of refactoring work that is not a part of a feature or bug fix.
- `perf:` - Should be used for a commit that represents performance improvements.
- `test:` - Should be used for commits that are purely for automated tests.
- `instr:` - Should be used for commit that are for instrumentation purposes. (e.g. logs, trace spans and telemetry configuration)

### Commit scopes

**The following commit scopes are supported:**

This list will evolve as more applications and libraries are added to Celerity.

#### Applications

- `cli` - CLI for test/build/package/deploy tooling (`apps/cli`)
- `runtime-nodejs` - Node.js runtime wrapper application (`apps/runtime/nodejs`)
- `runtime-python` - Python runtime wrapper application (`apps/runtime/python`)

#### Core runtime libraries (`libs/runtime`)

- `lib-rt-core` - Core Rust runtime library (`libs/runtime/core`)
- `lib-rt-workflow` - Workflow orchestration runtime (`libs/runtime/workflow`)
- `lib-rt-blueprint-parser` - Blueprint YAML/JSON configuration parsing (`libs/runtime/blueprint-config-parser`)
- `lib-rt-signature` - Header signing authentication method using API key + secret (`libs/runtime/signature`)
- `lib-rt-helpers` - Shared utilities and helper functions (`libs/runtime/helpers`)
- `lib-rt-aws-helpers` - AWS SDK helper utilities (`libs/runtime/aws-helpers`)

#### Message consumer crates (`libs/runtime/consumers`)

- `lib-rt-consumer-sqs` - AWS SQS message consumer
- `lib-rt-consumer-kinesis` - AWS Kinesis stream consumer
- `lib-rt-consumer-redis` - Redis Streams consumer
- `lib-rt-consumer-gcloud-pubsub` - Google Cloud Pub/Sub consumer
- `lib-rt-consumer-gcloud-tasks` - Google Cloud Tasks consumer
- `lib-rt-consumer-aeh` - Azure Event Hubs consumer
- `lib-rt-consumer-asb` - Azure Service Bus consumer

#### WebSocket crates (`libs/runtime/ws`)

- `lib-rt-ws-registry` - WebSocket connection registry
- `lib-rt-ws-redis` - Redis-backed WebSocket pub/sub

#### SDK crates (`libs/runtime/sdk`)

- `lib-rt-sdk-ffi` - C FFI bindings generation (`libs/runtime/sdk/bindgen-ffi`)
- `lib-rt-sdk-ffi-java` - Java JNI bindings generation (`libs/runtime/sdk/bindgen-ffi-java`)
- `lib-rt-sdk-schema` - Schema definitions for FFI bindings (`libs/runtime/sdk/bindgen-schema`)
- `runtime-bindings` - Bindings CLI tool for packaging and testing (`libs/runtime/sdk/runtime-bindings`)
- `lib-rt-sdk-node` - Node.js native bindings via NAPI-RS (`libs/runtime/sdk/node`)
- `lib-rt-sdk-python` - Python native bindings via PyO3 (`libs/runtime/sdk/python`)
- `lib-rt-sdk-java` - Generated Java language bindings (`libs/runtime/sdk/bindings/java`)
- `lib-rt-sdk-dotnet` - Generated .NET/C# language bindings (`libs/runtime/sdk/bindings/dotnet`)

The commit scope can be omitted for changes that cut across these scopes.
However, it's best to check in commits that map to a specific scope where possible.

### How commit scopes relate to releases

Celerity uses [release-please](https://github.com/googleapis/release-please) to automate changelog generation and release PR creation. Release-please determines which **component group** a commit belongs to based on the **file paths changed** in the commit, not the commit scope. The commit scope is preserved as a label in the generated changelog entry for readability.

This means fine-grained commit scopes (e.g. `lib-rt-core`, `lib-rt-consumer-sqs`) serve two purposes:
1. **Git history filtering** — You can search and filter the git log by scope to find changes related to a specific crate or module (e.g. `git log --grep="lib-rt-core"`)
2. **Changelog readability** — Scopes appear as labels in release notes, so readers can see exactly which sub-component changed within a broader release

Release-please routes commits to component groups by **longest file path match**. For example, a commit that changes files in `libs/runtime/consumers/consumer-sqs/` matches the `runtime-consumers` component (path `libs/runtime/consumers`) rather than the broader `runtime-core` component (path `libs/runtime`). Similarly, `libs/runtime/sdk/node/` matches `runtime-sdk-node` (path `libs/runtime/sdk/node`) rather than `runtime-sdk-ffi` (path `libs/runtime/sdk`).

#### Component groups and scope mapping

| Component Group | Release-Please Path | Publishes To | Commit Scopes |
|---|---|---|---|
| `cli` | `apps/cli` | GitHub Release | `cli` |
| `runtime-core` | `libs/runtime` | Internal | `lib-rt-core`, `lib-rt-workflow`, `lib-rt-blueprint-parser`, `lib-rt-signature`, `lib-rt-helpers`, `lib-rt-aws-helpers` |
| `runtime-consumers` | `libs/runtime/consumers` | Internal | `lib-rt-consumer-sqs`, `lib-rt-consumer-redis`, `lib-rt-consumer-kinesis`, `lib-rt-consumer-asb`, `lib-rt-consumer-aeh`, `lib-rt-consumer-gcloud-pubsub`, `lib-rt-consumer-gcloud-tasks` |
| `runtime-ws` | `libs/runtime/ws` | Internal | `lib-rt-ws-registry`, `lib-rt-ws-redis` |
| `runtime-sdk-ffi` | `libs/runtime/sdk` | Triggers Java/.NET release | `lib-rt-sdk-ffi`, `lib-rt-sdk-ffi-java`, `lib-rt-sdk-schema`, `runtime-bindings`, `lib-rt-sdk-java`, `lib-rt-sdk-dotnet` |
| `runtime-sdk-node` | `libs/runtime/sdk/node` | NPM | `lib-rt-sdk-node` |
| `runtime-sdk-python` | `libs/runtime/sdk/python` | PyPI | `lib-rt-sdk-python` |
| `runtime-nodejs` | `apps/runtime/nodejs` | GHCR | `runtime-nodejs` |
| `runtime-python` | `apps/runtime/python` | GHCR | `runtime-python` |

> **Note:** Only Go components (e.g. `cli`) receive an additional path-based tag (`{path}/v{version}`) for the Go module proxy. All other components use only the release-please component tag (`{component}/v{version}`). See the [release strategy](./SOURCE_CONTROL_RELEASE_STRATEGY.md#release-strategy) for details.

> **Note:** Scopes for workflow SDK variants (`lib-rt-workflow-sdk-*`), plugin scopes (`lib-rt-plugin-*`), and runtime app variants (`runtime-csharp`, `runtime-java`, `runtime-workflow-*`) will be mapped to component groups as those packages are added.

#### Example: how a commit flows through to release notes

A developer fixes a retry bug in the SQS consumer crate:

```bash
git commit -m 'fix(lib-rt-consumer-sqs): correct backoff delay calculation for failed messages

The exponential backoff was using a linear multiplier instead of
the configured base delay, causing retries to happen too quickly.

GitHubIssue: #412
'
```

This commit changes files in `libs/runtime/consumers/consumer-sqs/src/`. Here is what happens:

1. **Commit scope** `lib-rt-consumer-sqs` is recorded in git history — useful for `git log --grep` filtering
2. **Release-please** sees file paths under `libs/runtime/consumers/` and attributes the commit to the **`runtime-consumers`** component group (longest path match)
3. **Release PR** is created for `runtime-consumers` with the changelog entry:

```markdown
## [1.3.0](https://github.com/...) (2025-02-15)

### Bug Fixes

* **lib-rt-consumer-sqs:** correct backoff delay calculation for failed messages ([#413](https://github.com/.../pull/413))
```

4. Merging the release PR creates tag `runtime-consumers/v1.3.0` and triggers the release workflow

The fine-grained scope (`lib-rt-consumer-sqs`) is preserved in the changelog entry, giving readers immediate visibility into which specific crate was affected within the broader `runtime-consumers` release.

### Commit footers

**The following custom footers are supported:**

- `GitHubIssue: #xxx` - This footer must be provided when a commit pertains to some work where there is a GitHub issue. 
  This helps with tooling that links GitHub issues to commits providing a way to easily get extra context and requirements
  that are related to a commit. You can also use the `#xxx` reference in the body of the message to reference GitHub issues.

### Example commit

#### With commit scope

```bash
git commit -m 'feat(cli): deprecate opentofu as the deployment backend

Bluelink now fully supports deploying resources required to deploy Celerity applications
across Cloud providers. This means that the CLI no longer needs to use OpenTofu as the deployment backend.

GitHubIssue: #2391
'
```

#### Without commit scope

```bash
git commit -m 'fix: correct default server configuration for all runtime applications'
```

## 2) You must use the imperative mood for commit headers.

https://cbea.ms/git-commit/#imperative

The imperative mood simply means naming the subject of the commit as if it is a unit of work that can be applied instead of reporting facts about work done.

If applied, this commit will **your subject line here**.

Read the article above to find more examples and tips for using the imperative mood.
