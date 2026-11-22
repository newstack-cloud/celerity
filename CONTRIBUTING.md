# Contributing to Celerity

## Setup

Ensure git uses the custom directory for git hooks so the pre-commit and commit-msg linting hooks
kick in.

```bash
git config core.hooksPath .githooks
```

### NPM dependencies

There are npm dependencies that provide tools that are used in git hooks and scripting that span
multiple applications and libraries.

Install dependencies from the root directory, so the commit-msg hook finds commitlint in a
`node_modules` instead of downloading it on every commit:
```bash
YARN_NODE_LINKER=node-modules yarn install
```

Yarn 4 otherwise installs in Plug'n'Play mode, without a `node_modules`.

## Projects

[docs/INDEX.md](./docs/INDEX.md) lists the released applications and libraries, with their
language and release component. Build, lint and test commands are in each project's
`CONTRIBUTING.md`: `apps/cli/docs/CONTRIBUTING.md`, `apps/local-events/docs/CONTRIBUTING.md`,
and `libs/runtime/CONTRIBUTING.md` for all the runtime crates. dev-auth has only `scripts/lint.sh`.

## Further documentation

- [Commit Guidelines](./COMMIT_GUIDELINES.md)
- [Source Control and Release Strategy](./SOURCE_CONTROL_RELEASE_STRATEGY.md)