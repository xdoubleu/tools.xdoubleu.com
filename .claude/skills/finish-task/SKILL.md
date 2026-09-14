---
name: finish-task
description: Run the required final steps to ship a code change on tools.xdoubleu.com — lint, coverage, build, open the PR (with the right auto-merge call), watch CI to green, and reflect on doc/tooling gaps. Use whenever code changes in this repo are complete — committed, or ready to commit — whether or not the user asked for a PR; also when asked to "finish up", "wrap this up", "open a PR", "ship this". Always prefer this over calling `ship-pr` directly: this skill wraps it with the repo's own lint/coverage/build steps and auto-merge rule.
---

# Finish Task

The closing half of every task in this repo, paired with `start-task`. Run
these steps in order — don't skip ahead to opening the PR before lint/coverage
pass, and don't stop at "CI is running" as if that were done.

**Opening the PR is standing pre-authorized workflow here — it is not an
outward-facing action to hold back on pending a request.** Run this skill when
the code is done, whether or not the user asked for a PR; pushing the branch
and reporting it "ready for a PR" is not a finished task. Sessions that did
that are the reason issue #1236 exists.

This repo layers its own lint/coverage/build steps and auto-merge threshold
on top of the generic `ship-pr` skill from the `git-task-flow` plugin
(`xdoubleu/xdoubleu-claude-plugins` marketplace).

## 1. Lint

- API changes confined to a single package under `api/apps/<pkg>`: `cd api
  && make lint/fix/pkg PKG=apps/<pkg>` instead of the repo-wide target —
  golines' repo-wide pass in plain `make lint/fix` can reformat unrelated,
  already-merged files elsewhere in the tree that have no actual lint
  failure (`api/CLAUDE.md`'s Lint section), which then has to be caught and
  reverted before the diff is clean. Use plain `cd api && make lint/fix`
  only when the change spans multiple packages or touches `api/internal`.
- `cd web && npm run lint` for web changes.
- `cd kobo-gateway && make lint/fix` for kobo-gateway changes.
- `cd sentrytools && make lint/fix` for sentrytools changes — and re-run `go mod
  tidy` in `api` afterward if sentrytools' public API changed, since `api`
  depends on it via a local `replace`.

## 2. Coverage

Target ≥80% on changed code.

- API: `cd api && docker-compose up -d && make test/cov/report`. **Leave the
  container running** — do not `docker-compose down` afterwards. Every
  worktree shares one Postgres container (see `api/CLAUDE.md`'s Testing
  Notes), so stopping it kills the database any concurrent session is
  mid-test against, which surfaces as unrelated failures in whatever suite
  that session happened to be running (issue #1205).
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
  signals below apply: tell `ship-pr` this is a small, self-contained change
  — its own Step 2 already covers enabling auto-merge in the same breath as
  creating the PR, so there's no need to re-run those `gh` commands here.
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

  Whenever this branch applies, once `ship-pr` has opened the PR, append a
  `## Manual review needed` section to its body (fetch the existing
  `--fill`-generated body first with `gh pr view --json body -q .body` and
  append to it via `gh pr edit <number> --body "$(cat <<EOF
  ...
  EOF
  )"` — never overwrite it) naming every specific signal that triggered
  manual review (e.g. "touches `.github/workflows/main.yml`", "diff is 312
  lines / 11 files", "adds a new DB migration under
  `api/apps/feeds/migrations`", "spans both `api/apps/feeds` and `web/`")
  and a short, signal-specific "what to double check" line for each (a CI
  workflow change: verify it doesn't break `ci-pass`'s required-check
  gating; a migration: confirm it's backward-compatible with the
  currently-deployed code; a dependency bump: check the changelog for
  breaking changes; a multi-app/`api/internal/*` change: confirm the
  affected apps/packages stay consistent with each other). List every
  triggered signal, not just the first match — a generic disclaimer with no
  named signal isn't good enough.

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

## When a delegated skill isn't installed, or `gh` isn't available

Two distinct things can be missing, and neither implies the other — a
Claude Code **on the web** session has neither the `ship-pr`/`session-retro`
marketplace plugins nor a `gh` binary (GitHub access there is via
`mcp__github__*` tools instead), but check both explicitly (`command -v gh`)
rather than assuming one from the other. If a skill fails to load, or `gh`
is missing, don't silently skip or improvise the mechanics — say so
explicitly, then fall back:

- **`ship-pr` loaded but `gh` is missing**: follow `ship-pr`'s own "When
  `gh` isn't available" section (`git-task-flow` plugin) for the exact
  MCP-tool mapping (PR creation with the `Fixes #123` body, arming
  auto-merge, polling CI status) instead of re-deriving it here — apply
  this skill's own auto-merge rule (step 4 above) as the *decision*, the
  same either way; only the mechanics that enact it change. It also calls
  out explicitly when auto-merge can't be armed because no MCP tool for it
  is mounted — don't fake that by merging early or leaving it silently
  unset.
- **`ship-pr` itself not installed** (plugin absent): rebase the branch on
  latest `origin/main`, push, and open a **non-draft** PR whose body closes
  the tracking issue with a keyword (`Fixes #123`), then watch CI to green
  — using `mcp__github__*` tools directly if `gh` is also missing, per the
  same capability mapping `ship-pr`'s own fallback section describes (the
  plugin not loading doesn't change which MCP tools exist). Apply this
  skill's auto-merge rule above yourself, and say explicitly in your report
  if anything (auto-merge, in particular) couldn't be set.
- **No `session-retro`**: still run the retro *analysis* by hand — review
  this session's tool calls, retries, and CI runs for a concrete
  inefficiency (a doc gap, a missing target, an under-triggered
  skill/tool). Only if something real turns up, ship it as its own issue
  and PR, never stacked on this one. `session-retro`'s own fix, if any
  turns up, goes through `start-task`/`finish-task` again and inherits this
  same fallback.

The `Stop` hook in `.claude/settings.json` can't confirm a PR exists
without `gh`, so in a web session it treats "can't tell" as "don't block"
(issue #1400) — meaning nothing external forces the PR to get opened there.
That makes doing it yourself, unprompted, the only guardrail.

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
