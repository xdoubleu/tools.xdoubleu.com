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
requirement, etc. all live there — don't re-derive them here). Which
auto-merge rule to hand it branches first on whether the tracking issue
carries the `feature` label — check the issue's labels before choosing a
branch, don't infer it from the diff's shape. See
[`docs/convention-feature-review-policy.md`](../../../docs/convention-feature-review-policy.md)
for the full rationale behind the two tracks below; this section is just
the mechanics.

### Branch A — issue is `feature`-labeled: quality gate, then unconditional auto-merge

No human reviews this PR at all, regardless of size or how many of Branch
B's "larger/architectural" signals it would otherwise trip — skip the
`## Manual review needed` section entirely on this branch. In its place, an
automated quality gate stands in for the reviewer. Run all of the following
and fix everything each one turns up **before opening the PR** — or, if
something only surfaces after the PR is already open, before enabling
auto-merge:

1. Run the `code-review` skill against the diff and apply its findings —
   the same skill a human reviewer would otherwise have been asked to run
   by hand.
2. Run the architecture-boundary lints for whichever side changed: `cd api
   && make lint` (Go `depguard` app-isolation rules) and/or `cd web && npm
   run lint` (`dependency-cruiser` server/client and route-segregation
   rules). Fix every violation.
3. Regenerate the dependency diagrams for whichever side changed: `cd api
   && make arch/diagram` and/or `cd web && npm run arch:diagram`. These
   feed the Slack summary in step 6a below.
4. Run the diff-scoped mutation-testing targets for whichever side
   changed: `cd api && make test/mutation/diff` and/or `cd web && npm run
   test:mutation:diff`. Kill every surviving mutant the run reports — a
   passing coverage percentage alone doesn't prove a new test asserts
   anything, and nothing else in this pipeline checks that once no human
   reads the diff.

Once the gate is clean, tell `ship-pr` to enable auto-merge unconditionally
— its own Step 2 already covers doing this in the same breath as creating
the PR.

### Branch B — issue is not `feature`-labeled: the existing tiered rule, unchanged

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

## 5. "Needs you": a deferred manual step skips straight-to-Done

Before handing `ship-pr` a closing keyword in step 4, check whether the
tracking issue's body has a `## Follow-up: your turn` heading — the
convention `refine-feature`/`refine-issue` issues use to flag a step only the
user can do (registering with a third-party service, generating an API key,
creating a webhook, setting a deploy secret via the `wizard` skill). Freeform
prose elsewhere in the body (e.g. buried in `## Scope`) doesn't count — the
heading is what makes this detectable at all, so don't try to infer intent
from prose that doesn't use it.

If that heading is present:

- Give `ship-pr` a **non-closing** reference for this issue instead of a
  closing keyword — `Part of #123` or `Relates to #123`, never `Fixes #123`/
  `Closes #123` — so merging the PR does not auto-close it. Say explicitly in
  the PR body that the issue stays open pending the step under its own
  `## Follow-up: your turn` heading.
- Once CI is green and the PR has merged, set the issue's board `Status`
  field to **"Needs you"** rather than leaving it on whatever it was:
  ```
  gh project item-edit --id <ITEM_ID> --field-id PVTSSF_lAHOAzw7nc4BdsAmzhYLzDw \
    --project-id <PROJECT_ID> --single-select-option-id f83bddc3
  ```
  This board is project #8 — a **personal** project, not org-owned, which is
  why the GitHub MCP server's own field tools can't resolve or write it (see
  `docs/convention-mcp-gap-first.md`'s #1357 entry, which covers the same
  read-side gap `get_project_issues_by_status` works around). `<PROJECT_ID>`
  comes from `gh project view 8 --owner xdoubleu --format json`; `<ITEM_ID>`
  is *this issue's* item id on that board, from `gh project item-list 8
  --owner xdoubleu --format json` matched on `content.number`. `Status`
  field id `PVTSSF_lAHOAzw7nc4BdsAmzhYLzDw` and the "Needs you" option id
  `f83bddc3` are fixed — they don't need re-discovering each time.
  Without `gh` (a Claude Code on the web session), the same effect is the
  `updateProjectV2ItemFieldValue` GraphQL mutation `gh project item-edit`
  wraps, called directly against `https://api.github.com/graphql`; if
  nothing in the session can authenticate that call, say so explicitly in
  the report rather than leaving the board silently unchanged — the same
  as this skill already does for a missing auto-merge tool below.
- Leave the issue **open** at "Needs you". Closing it to "Done" once the
  actual capability is live (the webhook provisioned, the secret set) is the
  job of whatever walks the user through that step — today that's the
  `wizard` skill, once it exists in this repo's enabled plugins — not this
  skill.

If the heading is absent, none of this applies: the issue closes to "Done"
the normal way via the PR's closing keyword, unchanged from before.

## 6. Resolve linked Sentry issues once merged

Once `ship-pr` reports the PR merged, check the tracking issue's body for
Sentry permalinks (`https://xdoubleu.sentry.io/issues/<id>/`) — issues filed
by `sentry-triage` or `refine-issue` often list them under a "Sentry
permalinks" heading. For each one found, mark it resolved with the
`resolve_sentry_issue` MCP tool (`issue_id` = the numeric id from the
permalink). Closing the GitHub issue does not resolve Sentry on its own —
this step is easy to forget and was missed for issues #770 and #775.

## 6a. Feature epic completion: the Slack summary

Only applies when the tracking issue just merged is `feature`-labeled (see
step 4's Branch A) **and** has a parent issue. Once `ship-pr` reports the
PR merged, check whether any other sub-issues under that same parent are
still open — via the project board, or the parent issue's own sub-issue
listing (`sub_issue_write`/`issue_read`'s `get_sub_issues`). If at least one
sibling is still open, stop here: nothing more to do until the last one
closes.

If none are still open — this was the last sub-issue of the feature epic —
call the `notify_slack` MCP tool (`message` required, `title` optional)
with a summary covering: what was built across the epic, the key design
decisions (pull these from the sub-issues' own PRs/commits, not just this
one), and how to try it. Include the two generated Mermaid diagrams from
step 4's Branch A (`make arch/diagram` / `npm run arch:diagram` output) when
they're available for this epic's changes.

This is a **completion** checkpoint, distinct from `refine-feature`'s
checkpoint at the *start* of a feature epic (the exhaustive grilling
interview during issue refinement, before any code is written). The two
never overlap: `refine-feature` never sends a Slack message, and this step
never re-runs a grilling interview. The summary is sent exactly once per
epic, by `finish-task`, when the last sibling sub-issue closes — never
per-sub-issue-PR, and never re-sent by any other skill.

## 7. Run the session retro

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
