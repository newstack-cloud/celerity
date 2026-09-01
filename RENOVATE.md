# Dependency automation: Renovate (CLI / Bluelink chain) + Dependabot

Celerity is a mixed monorepo consisting of a Go CLI (`apps/cli`), local dev helper tools (e.g. `apps/local-events`), the Rust runtime libs
(`libs/runtime`), and the Node.js / Python runtime SDKs and wrappers. This is a
**focused Renovate rollout**: Renovate owns version updates for the **CLI's Go
modules only** which is a part of the Bluelink dependency chain, while Dependabot keeps managing
the Rust/Node/Python ecosystems (and security updates repo-wide) as before.

`apps/cli` is a **Bluelink consumer**. Its `go.mod` pulls in the core Bluelink
libs and a sibling module that rides the same release cascade:

- `github.com/newstack-cloud/bluelink/libs/blueprint`
- `github.com/newstack-cloud/bluelink/libs/blueprint-state`
- `github.com/newstack-cloud/bluelink/libs/common`
- `github.com/newstack-cloud/bluelink/libs/deploy-engine-client`
- `github.com/newstack-cloud/deploy-cli-sdk`

So Celerity joins `bluelink`, `celerity-provider-aws`, and `deploy-cli-sdk` as a
repo in the Bluelink graph, and its config is layered exactly like theirs:

```
newstack-cloud/renovate-config//default              ← org-wide policy; ALL repos
        ▲
        │ extends
newstack-cloud/bluelink//renovate/bluelink-consumers  ← Bluelink cascade only
        ▲
        │ extends
celerity  (scoped to apps/cli via includePaths)
```

- **Org layer** ([`newstack-cloud/renovate-config`](https://github.com/newstack-cloud/renovate-config))
  — universal, product-agnostic policy: schedule, the `deps` / `ci(deps)` commit
  convention, tiered auto-merge (minor/patch/digest auto-merge, majors held), and
  the Dependabot-security-only stance.
- **Bluelink layer** ([`bluelink//renovate/bluelink-consumers`](https://github.com/newstack-cloud/bluelink/blob/main/renovate/bluelink-consumers.json))
  - the cascade rule: auto-merge internal `github.com/newstack-cloud/*` bumps and
  let them bypass the weekly schedule so the chain reacts promptly.
- **Repo layer** ([`renovate.json`](renovate.json)) - `includePaths: ["apps/cli/**"]`
  keeps Renovate out of the Rust/Node/Python trees, plus a `deps(cli)` commit
  scope and a group so a component's simultaneous bumps land in one PR (no go.sum
  conflicts).

## Why scoped to the CLI

The org preset extends `config:recommended`, which would otherwise enable
Renovate across every ecosystem in the repo (cargo, npm, pip) and produce PRs that
compete with Dependabot's existing version updates for those trees. `includePaths`
narrows Renovate to `apps/cli/**` so this rollout is limited to the Bluelink chain.
The matching `gomod` block was removed from
[`.github/dependabot.yml`](.github/dependabot.yml) so only one tool bumps the CLI.

Broadening later (Renovate owns the whole repo, Dependabot → security-only, per
the org policy) is a matter of dropping `includePaths` and retiring the remaining
Dependabot version-update blocks.

## How the cascade works

- **Bump commits keep the `deps(cli): …` prefix** that release-please expects, so
  release behaviour is unchanged and only the tool that opens the PR differs.
- **Auto-merge tiers** (all CI-gated by `platformAutomerge`):
  - internal `github.com/newstack-cloud/*` bumps → auto-merge, bypassing the
    weekly schedule so the cascade reacts promptly;
  - third-party minor/patch/digest → auto-merge
  - majors → held for manual review (`major-update` label).
- **The human gate is the release PR.** Renovate lands the `deps(cli)` bump on
  `main`; release-please then opens the "release cli" PR for a person to review
  and merge.

```
bluelink lib (e.g. libs/blueprint) release published
  → deps(cli): bump blueprint (+ blueprint-state, common, deploy-engine-client)  [auto-merged]
    → release-please "release cli" PR  [human merges] → cli published
deploy-cli-sdk release published
  → deps(cli): bump deploy-cli-sdk  [auto-merged] → release PR → ...
```

## One-time setup

1. **`RENOVATE_TOKEN` secret** in this repo, a **GitHub App installation token or
   PAT**, *not* `GITHUB_TOKEN`. PRs opened by `GITHUB_TOKEN` don't trigger other
   workflows, so CI wouldn't run on Renovate PRs and auto-merge could never
   complete. The token needs **read access to `newstack-cloud/renovate-config`**
   (org preset) and **`newstack-cloud/bluelink`** (cascade preset). Reuse the
   org-level GitHub App already installed on the other Bluelink-graph repos.
2. The [`renovate.yml`](.github/workflows/renovate.yml) workflow runs hourly and on
   demand. Trigger a **dry run** first via *Actions → Renovate → Run workflow* with
   `dryRun: full` and confirm it only proposes `apps/cli` Go bumps.

## Validate before changes

```bash
npx --yes --package renovate renovate-config-validator renovate.json
```

## Follow-up

Two docs in the `bluelink` repo still list the cascade consumers as
`bluelink · celerity-provider-aws · deploy-cli-sdk` (its `RENOVATE.md` diagram and
the `renovate/bluelink-consumers.json` description). Add `celerity` there so the
consumer list stays accurate.
