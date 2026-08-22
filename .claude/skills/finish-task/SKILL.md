---
name: finish-task
description: Run the required final steps to ship a code change on tools.xdoubleu.com — lint, coverage, build, open the PR (with the right auto-merge call), watch CI to green, and reflect on doc/tooling gaps. Use when a task's code changes are complete and ready to commit, or when asked to "finish up", "wrap this up", "open a PR", "ship this".
---

# Finish Task

The closing half of every task in this repo, paired with `start-task`. Run
these steps in order — don't skip ahead to opening the PR before lint/coverage
pass, and don't stop at "CI is running" as if that were done.

This repo layers its own lint/coverage/build steps and auto-merge threshold
on top of the generic `ship-pr` skill from the `git-task-flow` plugin
(`xdoubleu/claude-plugins` marketplace).

## 1. Lint

- `cd api && make lint/fix` and/or `cd web && npm run lint` — whichever area changed.
- `cd kobo-gateway && make lint/fix` for kobo-gateway changes.
- `cd sentrytools && make lint/fix` for sentrytools changes — and re-run `go mod
  tidy` in `api` afterward if sentrytools' public API changed, since `api`
  depends on it via a local `replace`.

## 2. Coverage

Target ≥80% on changed code.

- API: `cd api && docker-compose up -d && make test/cov/report && docker-compose down`
  — always stop the DB after, even if the run fails.
- Web: `cd web && npm run test:cov`.

## 3. Build (web changes only)

`cd web && npm run build`. Next.js's server/client boundary check (a Server
Component importing anything from a file that pulls in client-only hooks) is
enforced **only** by `next build`, not `tsc --noEmit`, ESLint, or Jest — lint
and coverage passing does not mean the build passes. Put constants shared
across the boundary in a plain `lib/` module with no React imports.

## 4. Ship it via `ship-pr`

Run `ship-pr` for the rebase-on-main → push → open-PR → CI-watch mechanics
(force-with-lease reasoning, never `--draft`, issue closing-keyword
requirement, etc. all live there — don't re-derive them here). Give it this
repo's own auto-merge rule instead of its generic default:

- **Small, code-only changes** — no `CLAUDE.md`, Makefile/npm-script, lint
  config, CI workflow, or script edits, AND none of the "larger/architectural"
  signals below apply: enable auto-merge right away, in the same breath as
  creating the PR:
  ```bash
  gh pr create --fill --base main && gh pr merge --auto --squash
  ```
- **Tooling/harness changes, or larger/architectural changes** — do **not**
  enable auto-merge. Open a normal (non-draft) PR and wait for the user's own
  review. Tooling/harness means anything touching `CLAUDE.md`, Makefile
  targets, lint config, `.github/workflows/*`, scripts, or hooks.
  Larger/architectural means any of: a diff of roughly >150–200 changed lines
  or >8 files (check `git diff --stat` against `main` before opening the PR);
  a new/changed public interface, edits under a shared `api/internal/*`
  package, a new/modified DB migration, a new app registered in
  `api/cmd/api/apps.go`, a new/changed proto RPC, or changes spanning more
  than one app under `api/apps/*` or `web/`; or any `go.mod`/`package.json`
  dependency addition, removal, or version bump.

Reference the tracking issue from `start-task` in the PR body using a
closing keyword (`Fixes #123`, `Closes #123`) — `ship-pr` already covers the
mechanics of this, this is just a reminder it applies here too (this
happened to get missed once: issue #727 / PR #728 used a bare `#123`, which
doesn't auto-close on merge).

## 5. Resolve linked Sentry issues once merged

Once `ship-pr` reports the PR merged, check the tracking issue's body for
Sentry permalinks (`https://xdoubleu.sentry.io/issues/<id>/`) — issues filed
by `sentry-triage` or `refine-issue` often list them under a "Sentry
permalinks" heading. For each one found, mark it resolved with the
`resolve_sentry_issue` MCP tool (`issue_id` = the numeric id from the
permalink). Closing the GitHub issue does not resolve Sentry on its own —
this step is easy to forget and was missed for issues #770 and #775.

## 6. Run the session retro

Once CI is green, always run the `session-retro` skill (from the
`session-retro` plugin). It reviews this session's own tool-call/commit/CI
history for concrete inefficiencies (redundant reads, avoidable CI
back-and-forth, a missing or under-triggered skill/MCP tool, a doc gap) and,
only when something concrete turned up, ships the fix as its own tracking
issue and independent PR via `start-task`/`finish-task` — never stacked on
this PR. Running the analysis is mandatory every time; most runs should find
nothing worth acting on, and that's expected.

## Notes

- Never skip hooks (`--no-verify`) or bypass signing (`--no-gpg-sign`) unless
  the user has explicitly asked for it. If a hook fails, investigate and fix
  the underlying issue.
- If a `.proto` file changed this session, run `cd api && make proto/check`
  and `cd web && npm run generate:check` and commit the result — order
  relative to this skill's lint step doesn't matter, since `api/gen`/
  `web/lib/gen` are fully excluded from every lint/fix tool in this repo
  (see root CLAUDE.md's Commands section).
- Prefer an existing `make`/`npm run` target over an ad-hoc equivalent for
  any of the checks in this skill. If the exact check needed doesn't have
  one, add it to the Makefile/`package.json` rather than improvising it
  inline — see `session-retro`'s "ad-hoc commands" category.
