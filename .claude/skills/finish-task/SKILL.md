---
name: finish-task
description: >-
  Ship a code change on tools.xdoubleu.com — lint, coverage, build, open the
  PR (with the right auto-merge call), watch CI to green, run the retro. Use
  whenever code changes here are complete, whether or not the user asked for
  a PR; also for "finish up", "wrap this up", "open a PR", "ship this".
  Prefer this over calling `ship-pr` directly.
---

# Finish Task

Closing half of every task, paired with `start-task`; implements
[`docs/convention-task-lifecycle.md`](../../../docs/convention-task-lifecycle.md).
Run the steps in order. Opening the PR is pre-authorized; a pushed branch
without a PR, or "CI is running", is not done.

## 1. Lint

- API, single package: `cd api && make lint/fix/pkg PKG=apps/<pkg>` (repo-wide
  golines reformats unrelated files). Multiple packages or `api/internal`:
  `cd api && make lint/fix`.
- Web: `cd web && npm run lint`.
- kobo-gateway: `cd kobo-gateway && make lint/fix`.
- sentrytools: `cd sentrytools && make lint/fix`; if its public API changed,
  re-run `go mod tidy` in `api`.
- Always: `make lint/docs` from the repo root must pass (rule:
  [`docs/convention-concise-docs-and-comments.md`](../../../docs/convention-concise-docs-and-comments.md)).
- Re-read the diff and cut any comment or doc text that restates the code or
  repeats something said elsewhere.
- `.proto` changed: `cd api && make proto/check` and `cd web && npm run
  generate:check`, commit the result.

## 2. Coverage (≥80% on changed code)

- API: `cd api && docker-compose up -d && make test/cov/report`. **Never
  `docker-compose down`** — every worktree shares that Postgres container.
- Web: `cd web && npm run test:cov`.

## 3. Build (web changes)

`cd web && npm run build` — the only check of the server/client boundary.

## 4. Decide the issue reference

Check the tracking issue before handing `ship-pr` a reference:

- **`## Follow-up: your turn` heading in the issue body** (only the heading
  counts, not prose): use `Part of #123`, never a closing keyword, and say
  in the PR body the issue stays open for that step. After merge, set its
  board Status to **"Needs you"** (step 5's command, option id `f83bddc3`)
  and leave it open.
- **Bug report**: a green suite doesn't close it. Verify the symptom is
  gone in production after deploy (MCP read tools or the UI); until then,
  name that check as a post-deploy step in the PR body. If unverifiable
  (third party, hardware), say so; the user's confirmation is acceptance.
- Otherwise: closing keyword (`Fixes #123`).

## 5. Ship via `ship-pr`

`ship-pr` owns rebase → push → non-draft PR → CI watch. Pick the auto-merge
rule from the issue's **labels**, not the diff
([`docs/convention-feature-review-policy.md`](../../../docs/convention-feature-review-policy.md)).

**A — `feature` label: quality gate, then unconditional auto-merge.** No
`## Manual review needed` section. Before opening the PR (or before arming
auto-merge, if found later) fix everything each of these turns up, for
whichever side changed:

1. `code-review` skill on the diff; apply findings.
2. `cd api && make lint` (depguard) / `cd web && npm run lint`
   (dependency-cruiser).
3. `cd api && make arch/diagram` / `cd web && npm run arch:diagram` (for step 7).
4. `cd api && make test/mutation/diff` / `cd web && npm run
   test:mutation:diff`; kill every surviving mutant.

Then tell `ship-pr` to enable auto-merge.

**B — no `feature` label: tiered rule.**

- **Small, code-only**: tell `ship-pr` it's small and self-contained; it
  enables auto-merge.
- **Tooling/harness or larger/architectural**: no auto-merge; wait for the
  user's review. Tooling/harness: `CLAUDE.md`, Makefile targets, lint config,
  `.github/workflows/*`, scripts, hooks. Larger/architectural: >~150–200
  changed lines or >8 files (`git diff --stat main`); a new/changed public
  interface; `api/internal/*`; a new/modified migration; a new app in
  `api/cmd/api/apps.go`; a new/changed proto RPC; changes spanning more than
  one app under `api/apps/*` or `web/`; any `go.mod`/`package.json`
  dependency change.

  Then append (never overwrite: `gh pr view --json body -q .body`, then
  `gh pr edit <n> --body ...`) a `## Manual review needed` section naming
  **every** triggered signal, each with a specific "what to double check"
  (e.g. migration: backward-compatible with deployed code).

Board Status edits (project #8 is personal, so GitHub MCP field tools can't
write it):

```
gh project item-edit --id <ITEM_ID> --field-id PVTSSF_lAHOAzw7nc4BdsAmzhYLzDw \
  --project-id PVT_kwHOAzw7nc4BdsAm --single-select-option-id <OPTION_ID>
```

`<ITEM_ID>`: `gh project item-list 8 --owner xdoubleu --format json`,
matched on `content.number`. Without `gh`, use the
`updateProjectV2ItemFieldValue` GraphQL mutation, or report that you couldn't.

## 6. Resolve linked Sentry issues once merged

For each `https://xdoubleu.sentry.io/issues/<id>/` permalink in the tracking
issue, call `resolve_sentry_issue` (`issue_id` = `<id>`).

## 7. Feature epic completion: Slack summary

Only for a merged `feature` issue with a parent. If any sibling sub-issue
is still open, stop. If this was the last, call `notify_slack` (`message`,
optional `title`) summarizing what the epic built, key design decisions
(from all sub-issues' PRs), how to try it, plus step 5's Mermaid diagrams
when available. Sent once per epic, only here.

## 8. Session retro

Once CI is green, always run `session-retro`. Any fix it finds ships as its
own issue and PR, never stacked on this one.

## Missing skills or `gh`

Check both (`command -v gh`); on the web there is neither, and the `Stop`
hook can't confirm a PR, so opening it is on you. Say what's missing, then:

- **`ship-pr` loaded, no `gh`**: follow its "When `gh` isn't available"
  section. If auto-merge can't be armed, say so — never merge early.
- **No `ship-pr`**: rebase on `origin/main`, push, open the PR, watch CI
  (via `mcp__github__*` if no `gh`); apply step 5 yourself.
- **No `session-retro`**: do the analysis by hand.
