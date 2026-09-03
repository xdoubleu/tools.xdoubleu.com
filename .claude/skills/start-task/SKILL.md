---
name: start-task
description: Set up a fresh worktree off up-to-date main and create/refine the GitHub tracking issue before starting a code change on tools.xdoubleu.com. Use when beginning a new coding task, feature, bug fix, or any change here that doesn't yet have a tracking issue and worktree, and pair with finish-task when the work wraps up.
---

# Start Task

The opening half of every task in this repo, paired with `finish-task`. This
repo layers one project-specific step (a tracking issue) on top of the
generic `task-worktree` skill from the `git-task-flow` plugin
(`xdoubleu/xdoubleu-claude-plugins` marketplace — see root `CLAUDE.md`'s "Docs
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

If a finalized plan exists (from plan mode or otherwise), record it in the
issue's `## Plan` section via `refine-issue` before the first edit, and move
Status to "In progress" at that point.

## When a delegated skill isn't installed (e.g. Claude Code on the web)

`task-worktree` and `refine-issue` come from marketplace plugins that a
local CLI syncs automatically but a Claude Code **on the web** session does
not have, and that environment also has no `gh` (GitHub access is via the
`github` MCP tools). If either skill fails to load, don't silently improvise
the mechanics — say so explicitly, then fall back:

- **No `task-worktree`**: create the branch yourself off up-to-date
  `origin/main` (`EnterWorktree`, or `git worktree add`), never editing the
  main checkout or reusing an existing branch.
- **No `refine-issue`**: still create/find a tracking issue (via the
  `github` MCP tools) and record the plan in its `## Plan` section before
  the first edit. You can't set the project board's Priority/Status fields
  without `gh` — note that in your reply so it can be done from a local
  session later.

## Notes

- `refine-issue` owns the repo/project-board config, label lists, and
  Priority (P0/P1/P2) rule (via `.claude/github-triage.config.json`) — don't
  redefine any of that here.
- Once the work is done, hand off to `finish-task` for lint/coverage/build/PR/CI.
