---
name: finish-task
description: Run the required final steps to ship a code change on tools.xdoubleu.com — lint, coverage, build, open the PR (with the right auto-merge call), watch CI to green, and reflect on doc/tooling gaps. Use when a task's code changes are complete and ready to commit, or when asked to "finish up", "wrap this up", "open a PR", "ship this".
---

# Finish Task

The closing half of every task in this repo, paired with `start-task`. Run
these steps in order — don't skip ahead to opening the PR before lint/coverage
pass, and don't stop at "CI is running" as if that were done.

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

## 4. Open the PR yourself — don't wait to be asked

```bash
git push -u origin HEAD
gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base main
```

Never push to `main` directly; never open as `--draft`. Reference the
tracking issue from `start-task` in the PR body using a closing keyword
(`Fixes #123`, `Closes #123`) so it auto-closes on merge — a bare `#123` or
"Related to #123" leaves it open even after merge (this happened with issue
#727 / PR #728).

Then decide on auto-merge:

- **Small, code-only changes** — no `CLAUDE.md`, Makefile/npm-script, lint
  config, CI workflow, or script edits, AND none of the "larger/architectural"
  signals below apply: enable auto-merge right away, in the same breath as
  creating the PR — `gh pr merge --auto --squash` only merges once checks
  pass, so there's no reason to wait for green first:
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

## 5. Monitor CI until green, fixing it yourself if it isn't

```bash
gh pr checks --watch
gh pr view --json mergeable,mergeStateStatus,statusCheckRollup
```

A red PR or non-`MERGEABLE` state is not "done" — diagnose the actual failure
(don't just re-run blindly) and repeat from step 1. Once green + mergeable,
report the PR URL. Auto-merge was already armed in step 4 for small
code-only changes; for tooling/harness or larger/architectural changes, stop
here and wait for review.

## 6. Run the session retro

Once CI is green, always run the `session-retro` skill. It reviews this
session's own tool-call/commit/CI history for concrete inefficiencies
(redundant reads, avoidable CI back-and-forth, a missing or under-triggered
skill/MCP tool, a doc gap) and, only when something concrete turned up,
ships the fix as its own tracking issue and independent PR via
`start-task`/`finish-task` — never stacked on this PR. Running the analysis
is mandatory every time; most runs should find nothing worth acting on, and
that's expected.

## Notes

- Never skip hooks (`--no-verify`) or bypass signing (`--no-gpg-sign`) unless
  the user has explicitly asked for it. If a hook fails, investigate and fix
  the underlying issue.
- If a `.proto` file changed this session, use the `proto-generate` skill
  before this one's lint step — `make proto/generate` must be the last thing
  touching `api/gen`/`web/lib/gen` before committing, and `make lint/fix`
  above would re-stale it if run after.
