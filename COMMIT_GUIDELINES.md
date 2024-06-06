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

This list will evolve as more applications are added to Celerity.

- `blueprint` - This commit scope should be used for a commit that represents work that pertains to the blueprint library/framework.
- `blueprint-ls` - This commit scope should be used for a commit that represents work that pertains to the Blueprint Language Server.
- `build-engine` - This commit scope should be used for a commit that represents work that pertains to the build engine that sits at the core of the test/build/package/deploy tooling.
- `common` - This commit scope should be used for a commit that represents work that pertains to the common Go library.
- `runtime-core` - This commit scope should be used for a commit that represents work that pertains to the core Rust runtime library.
- `runtime-go` - This commit scope should be used for a commit that represents work that pertains to the Go wrapper application for the core runtime and the supporting Go SDK.
- `runtime-java` - This commit scope should be used for a commit that represents work that pertains to the Java wrapper application for the core runtime and the supporting Java SDK.
- `runtime-nodejs` - This commit scope should be used for a commit that represents work that pertains to the NodeJS wrapper application for the core runtime and the supporting NodeJS SDK.
- `runtime-python` - This commit scope should be used for a commit that represents work that pertains to the Python wrapper application for the core runtime and the supporting Python SDK.
- `runtime-rust` - This commit scope should be used for a commit that represents work that pertains to the Rust wrapper application for the core runtime.
- `api` - This commit scope should be used for a commit that represents work that pertains to the API that allows for remote build/package/deploy resource orchestration.
- `cli` - This commit scope should be used for a commit that represents work that pertains to the CLI for the test/build/package/deploy tooling.
- `tool-releaser` - This commit scope should be used for a commit that represents work that pertains to the internal releaser tool used for publishing Celerity libraries and applications.
- `tool-test-runner` - This commit scope should be used for a commit that represents work that pertains to the internal test runner tool used for integration/integrated tests for Celerity libraries and applications.
- `validator-go` - This commit scope should be used for a commit that represents work that pertains to the Go Celerity source code validator.
- `validator-java` - This commit scope should be used for a commit that represents work that pertains to the Java Celerity source code validator.
- `validator-nodejs` - This commit scope should be used for a commit that represents work that pertains to the NodeJS/TypeScript Celerity source code validator.
- `validator-python` - This commit scope should be used for a commit that represents work that pertains to the Python Celerity source code validator.
- `validator-rust` - This commit scope should be used for a commit that represents work that pertains to the Rust Celerity source code validator.
- `templates-go` - This commit scope should be used for a commit that represents work that pertains to Go template/example projects that can be used to bootstrap new Go Celerity services.
- `templates-java` - This commit scope should be used for a commit that represents work that pertains to Java template/example projects that can be used to bootstrap new Java Celerity services.
- `templates-nodejs` - This commit scope should be used for a commit that represents work that pertains to NodeJS template/example projects that can be used to bootstrap new NodeJS Celerity services.
- `templates-python` - This commit scope should be used for a commit that represents work that pertains to Python template/example projects that can be used to bootstrap new Python Celerity services.
- `templates-rust` - This commit scope should be used for a commit that represents work that pertains to Rust template/example projects that can be used to bootstrap new Rust Celerity services.

The commit scope can be omitted for changes that cut across these scopes.
However, it's best to check in commits that map to a specific scope where possible.


### Commit footers

**The following custom footers are supported:**

- `GitHubIssue: #xxx` - This footer must be provided when a commit pertains to some work where there is a GitHub issue. 
  This helps with tooling that links GitHub issues to commits providing a way to easily get extra context and requirements
  that are related to a commit. You can also use the `#xxx` reference in the body of the message to reference GitHub issues.

### Example commit

#### With commit scope

```bash
git commit -m 'feat(blueprint): add celerity pub/sub resource types

Adds some pub/sub resource types including AWS SNS + SQS and Google Cloud Pub/Sub.

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
