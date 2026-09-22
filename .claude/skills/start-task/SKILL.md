---
name: start-task
description: Set up a fresh worktree off up-to-date main and create/refine the GitHub tracking issue before starting a code change on tools.xdoubleu.com. Use when beginning a new coding task, feature, bug fix, or any change here that doesn't yet have a tracking issue and worktree, and pair with finish-task when the work wraps up.
---

# Start Task

The opening half of every task in this repo, paired with `finish-task`. This
repo layers one project-specific step (a tracking issue) on top of the
generic `task-worktree` skill from the `git-task-flow` plugin
(`xdoubleu/skills` marketplace — see root `CLAUDE.md`'s "Docs
Impact" note if that plugin isn't installed yet).

## 1. Fresh worktree off up-to-date main

Run `task-worktree` first — it covers pulling latest `main` and creating a
completely fresh worktree (never edit in the main checkout or reuse an
existing branch/worktree, even one from earlier in this same session). A
`SessionStart` hook in `.claude/settings.json` also runs `git fetch origin
main` once at the start of every session as a backstop, but that only
covers freshness as of session start, not a long session that keeps
exploring for hours — still run `task-worktree`'s own fetch, don't assume
the hook already covered it.

**If the session's own worktree is already checked out to a stale/foreign
branch** — e.g. a prior task's branch, left over from an earlier session in
the same lineage — running `task-worktree`/`git worktree add` to create a
*separate* fresh worktree does not help: the `PreToolUse` hook that denies
`Edit`/`Write`/`NotebookEdit` outside the active worktree scopes to one
fixed path per session, so an `Edit` inside a second worktree gets denied
even though the worktree itself was created successfully. Reset the
existing worktree **in place** instead: confirm `git status --porcelain`
is clean (stash with `-u` first if not), then `git fetch origin main &&
git checkout -B <new-branch-name> origin/main` inside that same directory.
This satisfies "fresh branch off up-to-date main" without violating the
hook's single-path scope.

## 2. Create (or find) the tracking issue

Before editing, always create a tracking GitHub issue for the work via the
`refine-issue` skill (from the `github-issue-triage` plugin — its config for
this repo lives in `.claude/github-triage.config.json`), not a bare `gh
issue create`, so Priority/Status/labels get set — do this even for work
that wasn't explicitly requested as an "issue", e.g. tooling/doc changes.

**An issue with an empty body is never self-explanatory.** If the tracking issue
exists but has no body — only a title — stop and ask the user what they actually
want before planning, exploring further, or editing anything. A title states a
topic, not a scope, and the readings it permits usually differ enough to produce
materially different work. Record their answer in the issue body via
`refine-issue` so the next session doesn't have to ask again.

Always run the `grilling` skill against the issue's scope and plan before
recording anything — this is unconditional (see root `CLAUDE.md`, "Always
grill the scope before an issue is treated as refined"), even for a one-line
issue with an already-approved plan. Feed its settled design tree into the
`## Plan` section.

If a finalized plan exists (from plan mode or otherwise), record it in the
issue's `## Plan` section via `refine-issue` before the first edit, and move
Status to "In progress" at that point.

## When a delegated skill isn't installed, or `gh` isn't available

Two distinct things can be missing, and neither implies the other — a
Claude Code **on the web** session has neither the `task-worktree`/
`refine-issue` marketplace plugins nor a `gh` binary (GitHub access there is
via `mcp__github__*` tools instead), but check both explicitly (`command -v
gh`) rather than assuming one from the other; a future environment could
have one without the other.

- **No `task-worktree`** (plugin not installed): create the branch yourself
  off up-to-date `origin/main` (`EnterWorktree`, or `git worktree add`),
  never editing the main checkout or reusing an existing branch.
  `task-worktree` never needed `gh` in the first place — branch/worktree
  creation is plain `git` either way — so a missing `gh` alone doesn't
  affect this step.
- **`refine-issue` loaded but `gh` is missing**: follow `refine-issue`'s own
  "When `gh` isn't available" section (`github-issue-triage` plugin) for the
  MCP-tool mapping (issue create/edit/label) rather than re-deriving it
  here — it also calls out explicitly when the project-board Priority/Status
  fields can't be set because no MCP tool for them is mounted.
- **`refine-issue` itself not installed** (plugin absent): still create/find
  a tracking issue and record the plan in its `## Plan` section before the
  first edit, using `mcp__github__*` tools directly if `gh` is also missing
  — same capability mapping `refine-issue`'s own fallback section describes,
  since the plugin not loading doesn't change which MCP tools exist. Note
  in your reply whatever couldn't be done (project board fields, in
  particular) so it can be finished from a local session later.

## Notes

- `refine-issue` owns the repo/project-board config, label lists, and
  Priority (P0/P1/P2) rule (via `.claude/github-triage.config.json`) — don't
  redefine any of that here.
- Once the work is done, hand off to `finish-task` for lint/coverage/build/PR/CI.
