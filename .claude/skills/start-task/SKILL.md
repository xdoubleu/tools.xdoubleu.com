---
name: start-task
description: Set up a fresh worktree off up-to-date main and claim or create the factory board task before starting a code change on tools.xdoubleu.com. Use when beginning a new coding task, feature, bug fix, or any change here that doesn't yet have a task and worktree, and pair with finish-task when the work wraps up.
---

# Start Task

Opening half of every task, paired with `finish-task`; implements
[`docs/convention-task-lifecycle.md`](../../../docs/convention-task-lifecycle.md)
on top of `task-worktree` (`git-task-flow` plugin).

## 1. Check for a prior attempt first

Before any setup, if the task exists:

- `factory_get_task` — the live status and comments are the truth. A Done task
  or a merged PR proves nothing; the symptom may be back. Never report
  "already fixed" from history alone.
- Search PRs for the task id (`gh pr list --search "<id>" --state all`, or
  `mcp__github__search_pull_requests`) and `git log --all --grep=<id>`.

If a previous attempt exists (even merged), read its diff, review comments and
CI failures, and comment "Why attempt failed" on the task
(`factory_comment_task`) before writing code. The new attempt must differ in
response to that analysis.

## 2. Fresh worktree off up-to-date main

Run `task-worktree` (pull latest `main`, completely fresh worktree; never
the main checkout or an existing branch/worktree). Don't rely on the
`SessionStart` fetch hook for freshness.

**Session's worktree already on a stale/foreign branch:** a second worktree
won't help — the `PreToolUse` edit guard scopes to one path per session.
Reset in place: confirm `git status --porcelain` is clean (else `git stash
push -u -m <tag>`), then `git fetch origin main && git checkout -B
<new-branch> origin/main`.

**Actions routine (`GITHUB_ACTIONS` set):** no worktree. The sandbox user
can't write under `.claude/`, so `routine-sandbox npm ci` fails in
`.claude/worktrees/`, and the edit guard is off here. Work in the runner's
checkout (clean tree first; each issue in the run repeats this):
`git fetch origin main && git checkout -B <new-branch> origin/main`. Never
grant the sandbox write access to `.claude/` with `setfacl`.

**OpenCode only:** create the worktree under `<repo>/.claude/worktrees/` (the
`PreToolUse` edit guard only accepts edits there — a worktree at a sibling
path gets the first edit rejected), then re-point the session with
`tools.opencode.session_move` before the first edit. If it refuses a second
re-point, apply changes there with `git apply` from the current session.

## 3. Claim (or create) the board task

The board is the tracker ([#2701](https://github.com/xdoubleu/tools.xdoubleu.com/issues/2701)).
The `factory_*` tools come from your agent account's MCP server
(`tools-apps-agent`, [adr-0039](../../../docs/adr-0039-agent-accounts.md) Setup).
**Without them stop and tell the user**; never fall back to a GitHub issue
unless they say to proceed without a task.

- **A named task:** `factory_get_task`. Empty body: stop and ask what they
  want. Already In progress: ask whether someone else is on it.
- **No task:** create one with `refine-task` (interactive mode); its body
  carries the plan. Even tooling and doc work gets one.
- **Grilling:** run the `grilling` skill on the scope and plan, feeding its
  settled design tree into `## Plan` — unless the plan was already grilled in
  plan mode (root `CLAUDE.md`), then say so in one line.
- **Plan:** agents can't edit a body, so on an existing task comment the
  `## Plan` (`factory_comment_task`) before the first edit. That comment is for
  people and this session; the sandbox runner's prompt is the body only
  ([#2701](https://github.com/xdoubleu/tools.xdoubleu.com/issues/2701) decision
  15), so a plan it must follow belongs in the body.
- **Claim:** `factory_move_task(id, "in_progress")`. You can't move a task
  into Ready; that is the human's approval.

**GitHub-issue mode (until #2779 retires it):** `ready-issues-sweep`,
`subissue-sweep` and `red-pr-repair` still work from GitHub issues. When the
prompt names one, use `gh issue view <n> --json state,stateReason,comments` in
step 1, skip the board, and set Project #8 Status to In progress (`gh project
item-edit --id <ITEM_ID> --field-id PVTSSF_lAHOAzw7nc4BdsAmzhYLzDw --project-id
PVT_kwHOAzw7nc4BdsAm --single-select-option-id 47fc9ee4`; `<ITEM_ID>` from
`gh project item-list 8 --owner xdoubleu --format json`; unattended, the
App token can't write it, so leave it).

## 4. Bug fix: reproduce production state before planning

- Pull the affected production rows via MCP read tools (`feeds_list_items`,
  `get_sentry_issues`, `prom_query`, …). The symptom is often stale data a
  code fix won't remove, so the plan may need a cleanup too.
- External input (third-party HTML, upstream API): capture the real bytes
  **through the app's own HTTP client** and commit them gzipped under
  `testdata/`. A synthetic or plain-`curl` fixture is not a reproduction.
- Name the cleanup and real-bytes fixture in `## Plan`.

## Missing skills or `gh`

Check both (`command -v gh`); a Claude Code on the web session has neither
the plugins nor `gh`.

- **Cloud session pinned to a branch, in the main checkout:** `git fetch
  origin main && git checkout --detach origin/main && git branch -f <branch>
  origin/main && git worktree add .claude/worktrees/<name> <branch>`, then
  `cd` there before the first edit. The guard doesn't cover Bash writes, so
  don't edit before this.
- **No `task-worktree`**: create the branch off up-to-date `origin/main`
  yourself (`EnterWorktree` or `git worktree add`); needs no `gh`.
- **No `refine-task`**: create the task with `factory_create_task` yourself
  (Backlog, plan in the body) and report what you couldn't set.

`refine-task` owns labels and priority — don't redefine them here. Hand off to
`finish-task` when done.
