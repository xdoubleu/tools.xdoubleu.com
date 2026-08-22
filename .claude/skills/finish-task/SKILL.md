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

## 4. Rebase on latest main, then open the PR yourself — don't wait to be asked

Before opening the PR — not on every later push, see the note below — bring
the branch up to date with `main`:

```bash
git fetch origin main
git rebase origin/main
```

If the rebase reports conflicts, resolve them the normal way (fix the
files, `git add`, `git rebase --continue`) — never `git rebase --abort` and
skip this step. If the rebase actually replayed any upstream commits (check
`git log --oneline origin/main..HEAD` before and after — a no-op rebase
changes nothing there), re-run whichever of steps 1–3 apply before pushing,
since the rebase can shift line numbers or interact with this task's own
changes.

Then push and open the PR:

```bash
git push -u origin HEAD --force-with-lease
gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base main
```

`--force-with-lease` (not `-f`) is safe here — this is this task's own
feature branch, not `main`, and it refuses to overwrite anything if someone
else pushed to the same branch since your last fetch. On a brand-new branch
(nothing pushed yet) it behaves like a normal push.

Only rebase+force-push right before the PR is first created. Once a PR is
open and under review (step 5's fix-and-repush loop), just push normally —
rebasing an already-reviewed branch rewrites commits a reviewer may have
already looked at.

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
- If a `.proto` file changed this session, run `cd api && make proto/check`
  and `cd web && npm run generate:check` and commit the result — order
  relative to this skill's lint step doesn't matter, since `api/gen`/
  `web/lib/gen` are fully excluded from every lint/fix tool in this repo
  (see root CLAUDE.md's Commands section).
- Prefer an existing `make`/`npm run` target over an ad-hoc equivalent for
  any of the checks in this skill. If the exact check needed doesn't have
  one, add it to the Makefile/`package.json` rather than improvising it
  inline — see `session-retro`'s "ad-hoc commands" category.
