---
name: start-task
description: Set up a fresh worktree off up-to-date main and create/refine the GitHub tracking issue before starting a code change on tools.xdoubleu.com. Use when beginning a new coding task, feature, bug fix, or any change here that doesn't yet have a tracking issue and worktree, and pair with finish-task when the work wraps up.
---

# Start Task

Opening half of every task, paired with `finish-task`; implements
[`docs/convention-task-lifecycle.md`](../../../docs/convention-task-lifecycle.md)
on top of `task-worktree` (`git-task-flow` plugin).

## 1. Check for a prior attempt first

Before any setup, if the issue exists:

- `gh issue view <n> --json state,stateReason,comments` — the live state and
  comments are the truth. A merged PR proves nothing; the issue may have been
  reopened. Never report "already fixed" from PR history alone.
- `gh pr list --search "<n>" --state all` and `git log --all --grep=<n>`.

If a previous attempt exists (even merged and closed), read its diff, review
comments and CI failures, and record a short "Why attempt #N failed" on the
issue (via `refine-issue`, in or before `## Plan`) before writing code. The
new attempt must differ in response to that analysis.

## 2. Fresh worktree off up-to-date main

Run `task-worktree` (pull latest `main`, completely fresh worktree; never
the main checkout or an existing branch/worktree). Don't rely on the
`SessionStart` fetch hook for freshness.

**Session's worktree already on a stale/foreign branch:** a second worktree
won't help — the `PreToolUse` edit guard scopes to one path per session.
Reset in place: confirm `git status --porcelain` is clean (else `git stash
push -u -m <tag>`), then `git fetch origin main && git checkout -B
<new-branch> origin/main`.

**OpenCode only:** after creating the worktree, re-point the session with
`tools.opencode.session_move` before the first edit. If it refuses a second
re-point, apply changes there with `git apply` from the current session.

## 3. Create (or find) the tracking issue

Always via `refine-issue` (`github-issue-triage` plugin, config in
`.claude/github-triage.config.json`), never a bare `gh issue create` — even
for tooling/doc work.

- **Empty body** (title only): stop and ask the user what they want before
  planning or editing; record the answer in the body via `refine-issue`.
- **Grilling**: run the `grilling` skill on the scope and plan, feeding its
  settled design tree into `## Plan` — unless the plan was already grilled
  in plan mode (root `CLAUDE.md`), then say so in one line.
- Record the plan in `## Plan` before the first edit and move Status to
  "In progress".

## 4. Bug fix: reproduce production state before planning

- Pull the affected production rows via MCP read tools (`feeds_list_items`,
  `get_sentry_issues`, `prom_query`, …). The symptom is often stale data a
  code fix won't remove, so the plan may need a cleanup too.
- External input (third-party HTML, upstream API): capture the real bytes
  **through the app's own HTTP client** and commit them gzipped under
  `testdata/`. A synthetic or plain-`curl` fixture is not a reproduction.
- Name the cleanup and real-bytes fixture in `## Plan`.

## Board Status: "In progress"

Project #8 is personal, so GitHub MCP field tools can't write it:

```
gh project item-edit --id <ITEM_ID> \
  --field-id PVTSSF_lAHOAzw7nc4BdsAmzhYLzDw \
  --project-id PVT_kwHOAzw7nc4BdsAm \
  --single-select-option-id 47fc9ee4
```

`<ITEM_ID>`: `gh project item-list 8 --owner xdoubleu --format json`,
matched on `content.number`. Without `gh`, use the
`updateProjectV2ItemFieldValue` GraphQL mutation.

## Missing skills or `gh`

Check both (`command -v gh`); a Claude Code on the web session has neither
the plugins nor `gh`.

- **No `task-worktree`**: create the branch off up-to-date `origin/main`
  yourself (`EnterWorktree` or `git worktree add`); needs no `gh`.
- **`refine-issue` loaded, no `gh`**: follow its "When `gh` isn't available"
  section.
- **No `refine-issue`**: still create/find the issue and record `## Plan`
  before the first edit (via `mcp__github__*` if no `gh`); report what
  couldn't be set (board fields).

`refine-issue` owns labels, Priority and board config — don't redefine them
here. Hand off to `finish-task` when done.
