---
name: finish-task
description: >-
  Ship a code change on tools.xdoubleu.com — lint, coverage, build, open the
  PR with auto-merge armed, watch CI to green and the merge, run the retro. Use
  whenever code changes here are complete, whether or not the user asked for
  a PR; also for "finish up", "wrap this up", "open a PR", "ship this".
  Prefer this over calling `ship-pr` directly.
---

# Finish Task

Closing half of every task (pairs with `start-task`); implements
[`docs/convention-task-lifecycle.md`](../../../docs/convention-task-lifecycle.md).
**Factory job (`FACTORY_JOB` set):** follow [Factory jobs](../../../docs/convention-task-lifecycle.md) instead.

Run the steps in order. Opening the PR is pre-authorized; a pushed branch
without a PR, or running CI, isn't done.

## 1. Lint

- API, single package: `cd api && make lint/fix/pkg PKG=apps/<pkg>`. Multiple packages or `api/internal`:
  `cd api && make lint/fix`.
- Web: `cd web && npm run lint`.
- kobo-gateway: `cd kobo-gateway && make lint/fix`.
- sentrytools: `cd sentrytools && make lint/fix`; if its public API changed,
  re-run `go mod tidy` in `api`.
- `infra/grafana` changed: `make lint/grafana` and `make grafana/verify`.
- Always: `make lint/docs` from the repo root must pass (rule:
  [`docs/convention-concise-docs-and-comments.md`](../../../docs/convention-concise-docs-and-comments.md)).
- Cut comment/doc text that restates the code.
- `.proto` changed: `cd api && make proto/check` and `cd web && npm run
  generate:check`, commit the result.

## 2. Coverage (≥80% on changed code)

- API: `cd api && docker-compose up -d && make test/cov/report`. **Never
  `docker-compose down`** — every worktree shares that Postgres container.
- Web: `cd web && npm run test:cov`.

**Sandboxed (routine) runs:** `routine-sandbox` points `GOLANGCI_LINT_CACHE`
and the coverage profile at the sandbox home; run lint read-only and
hand-apply golines/gci diffs. `make grafana/verify` needs Docker: skip it,
rely on CI's Build Grafana job, and say so in the PR body.

## 3. Build (web changes)

`cd web && npm run build` checks the server/client boundary.

## 4. Link the task

The PR links the board task with **no closing keyword**. Put `Task: <title> (<id>)` at the top of the PR body, and after
`ship-pr` opens it `factory_move_task(id, "in_review")`. Once the PR has merged move it
to `testing` (the runner moves only its own PRs); the owner checks it there and moves it to Done.

- **Epic or child** (`epic:<slug>`): `factory_list_tasks(label)` lists the
  tree; move every task this PR delivered or superseded like its own.
- **`## Follow-up: your turn` in the task body** (only the heading counts, not
  prose): say in the PR body that the task stays open for that step. Move it to
  In review as usual, then to `needs_you` once the PR has merged.
- **Bug report**: a green suite doesn't close it. Verify the symptom is gone in
  production after deploy (MCP read tools or UI); name that check as a
  post-deploy step in the PR body. If unverifiable, say so.

**GitHub-issue mode (until #2779):** when the prompt named a GitHub issue,
reference it as before: `Part of #123` under a follow-up heading (then, after
merge, set its Project #8 Status to "Needs you", option id `f83bddc3`),
otherwise `Fixes #123`. The Status command is in `start-task`'s GitHub-issue
mode; unattended, leave Status as-is.

## 4b. Check for supersession

Before pushing: `git fetch origin main`, diff the touched files, re-run the PR
search. If deleted or already resolved, close as superseded.

## 5. Ship via `ship-pr`

`ship-pr` owns rebase → push → non-draft PR → CI watch. **Always arm
auto-merge** (`gh pr merge --auto --squash`) on a PR targeting `main`: green `ci-pass` merges it
([`docs/convention-feature-review-policy.md`](../../../docs/convention-feature-review-policy.md),
[`docs/convention-unattended-agent-trust.md`](../../../docs/convention-unattended-agent-trust.md)).
This overrides `ship-pr`'s "larger or tooling changes: don't arm" default: arm every PR right after creating it, whatever its size. Reference the task as in step 4.

**A — `feature` label: run the quality gate first.** No
`## Manual review needed` section. Before opening the PR fix everything each
of these turns up, for whichever side changed:

1. `code-review` skill on the diff; apply findings.
2. `cd api && make lint` (depguard) / `cd web && npm run lint`
   (dependency-cruiser).
3. `cd api && make test/mutation/diff` / `cd web && npm run
   test:mutation:diff`; kill every surviving mutant.

Nothing else checks the diff before it merges
([`docs/convention-feature-review-policy.md`](../../../docs/convention-feature-review-policy.md)).

**B — no `feature` label: flag what needs a closer look.** Append (never
overwrite: `gh pr view --json body -q .body`, then `gh pr edit <n> --body
...`) a `## Manual review needed` section naming **every** triggered signal,
each with a specific "what to double check", when the change is tooling/harness or
larger/architectural:

- Tooling/harness: `CLAUDE.md`, Makefile targets, lint config,
  `.github/workflows/*`, scripts, hooks.
- Larger/architectural: >~150–200 changed lines or >8 files (`git diff --stat
  main`); a new/changed public interface; `api/internal/*`; a new/modified
  migration; a new app in `api/cmd/api/apps.go`; a new/changed proto RPC;
  changes spanning more than one app under `api/apps/*` or `web/`; any
  `go.mod`/`package.json` dependency change.

## 6. Resolve linked Sentry issues once merged

For each `https://xdoubleu.sentry.io/issues/<id>/` permalink in the task body
(`factory_get_task`), call `resolve_sentry_issue` (`issue_id` = `<id>`).

## 7. Session retro

Once CI is green, run `session-retro`; its fixes ship as their own board task
(file it with `refine-task`, not a GitHub issue) and PR.

## Missing skills or `gh`

Check both (`command -v gh`); the web has neither. Say what's missing:

- **`ship-pr` loaded, no `gh`**: follow its "When `gh` isn't available"
  section. Arm auto-merge as in step 5; never merge early.
- **No `ship-pr`**: rebase on `origin/main`, push, open the PR, watch CI
  (via `mcp__github__*` if no `gh`); apply step 5 yourself.
- **No `session-retro`**: analyse by hand.
