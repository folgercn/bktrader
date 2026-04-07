# bktrader Collaboration Guide

## Goal

Keep `main` stable, reviewable, and runnable while allowing fast parallel work between core trading development and ops/deployment work.

## Roles

### `wuyaocheng`

Primary owner of:

- `internal/service/**`
- `internal/domain/**`
- strategy logic
- signal / decision / intent / dispatch flow
- trading behavior
- business tests and strategy tests

Rule of thumb:
If the change answers "why does the system trade this way?", it is core work and should be led by `wuyaocheng`.

### `folgercn`

Primary owner of:

- `.github/**`
- `Dockerfile`
- `deployments/**`
- `scripts/**`
- env templates and runtime configuration
- CI/CD
- deployment and ops documentation
- release, safety, and environment setup

Rule of thumb:
If the change answers "how does the system run, deploy, and stay healthy?", it is ops work and should be led by `folgercn`.

## Shared Files

These areas are easy to conflict on and should be announced before editing:

- `README.md`
- `.gitignore`
- `.dockerignore`
- shared config files
- public interfaces and request/response shapes
- large cross-cutting files such as `internal/service/live.go`
- default behavior flags such as `dispatchMode`

## Branch Rules

### Stable branch

- `main`: protected integration branch, PR merge only, no direct push

### Personal working branches

- `dev/core-*`: core and strategy work
- `dev/ops-*`: ops, infra, CI/CD, deploy work

### Other branch types

- `feature/*`: longer-running integration branches
- `codex/*`: AI-assisted or experimental branches that still require human review

Examples:

- `dev/core-order-dispatch`
- `dev/ops-github-actions`
- `feature/runtime-integration-testnet`
- `codex/add-signal-tests`

## Daily Workflow

1. Start from latest `main`.
2. Create one branch for one task.
3. Keep one PR focused on one change.
4. Review `git diff --stat` and `git diff` before committing.
5. Re-sync with `main` before merge.

Recommended:

- use `rebase` for small personal branches
- use `merge` for shared long-running integration branches

## PR Ownership

- core feature PRs: `wuyaocheng` implements, `folgercn` reviews
- ops / deploy / CI PRs: `folgercn` implements, `wuyaocheng` reviews
- risky default behavior changes: both review before merge

## High-Risk Changes

The following must be isolated in a dedicated PR and explicitly called out:

- default `dispatchMode` changes
- manual review to auto-dispatch changes
- mock to real execution changes
- testnet to mainnet changes
- memory to Postgres or other state backend changes
- live trading path changes
- environment variable meaning changes
- schema or runtime model changes

Do not hide these inside checkpoint or mixed-purpose PRs.

## Codex Rules

Codex should only be used for small, bounded tasks.

Allowed examples:

- fix one function
- add tests for one module
- update one document
- adjust one CI step

Avoid delegating:

- large refactors across unrelated modules
- mixed business + deploy + docs changes in one pass
- silent default behavior changes

When using Codex, always constrain the scope clearly, for example:

- only edit `internal/service/live.go` and matching tests
- do not modify deploy, Docker, or GitHub Actions
- do not change default `dispatchMode`

Codex output must still be reviewed by a human before merge to `main`.

## Merge Rules

- `main` should remain runnable
- checkpoint PRs are acceptable, but they must state unfinished areas clearly
- if a PR changes default behavior or live-trading semantics, both owners should confirm before merge

## Validation Rules

Before opening a PR:

- inspect `git diff --stat`
- inspect `git diff`
- run the most relevant local validation available
- document anything not verified

## PR Writing

Every PR should cover:

- purpose
- change scope
- risks
- validation

If the PR affects defaults or runtime behavior, state that explicitly.
