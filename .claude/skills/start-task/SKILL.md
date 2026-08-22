---
name: start-task
description: Set up a fresh worktree off up-to-date main and create/refine the GitHub tracking issue before starting a code change on tools.xdoubleu.com. Use when beginning a new coding task, feature, bug fix, or any change here that doesn't yet have a tracking issue and worktree, and pair with finish-task when the work wraps up.
---

# Start Task

The opening half of every task in this repo, paired with `finish-task`. Do
these steps *before* the first edit, not after.

## 1. Pull latest main

`git checkout main && git pull` from the repo root, or `git fetch origin
main` — don't explore or plan against a stale checkout, since another
session or the user may have merged changes since. A `SessionStart` hook in
`.claude/settings.json` already runs `git fetch origin main` once at the
start of every session as a backstop (this step had been skipped often
enough in practice that relying on it being followed wasn't reliable) — but
that only covers freshness as of session start, not a long session that
keeps exploring for hours, so still treat this as an explicit step, not
something to assume already happened.

## 2. Create a completely fresh worktree off up-to-date main

Never edit in the main checkout or reuse an existing branch/worktree — even
one from earlier in this same session; it may already be merged or based on
a stale `main`.

Prefer the `EnterWorktree` tool. If it's unavailable (e.g. the session is
already inside a worktree and can't nest another with `name`), fall back to
manual `git worktree add` from the current worktree without `cd`-ing to the
shared checkout, then switch in with `EnterWorktree`'s `path` parameter:

```bash
git fetch origin main
git worktree add ../<descriptive-branch-name> -b <descriptive-branch-name> origin/main
```

then `EnterWorktree({ path: "<repo-root>/.claude/worktrees/<name>" })` if
that worktree lives under `.claude/worktrees/`, or note the path for the
session to use directly otherwise.

**After `EnterWorktree` (or the fallback) returns, every subsequent
Read/Edit/Write absolute path must be rebased onto the new worktree
directory it reports** — do not keep reusing an absolute path prefix from
earlier in the session (the original checkout, or a prior worktree). Nothing
rewrites old paths automatically: a stale prefix silently edits the wrong
checkout, and a Bash `cd` doesn't persist between tool calls either, so
`pwd` alone won't catch it. If this happens, recover by diffing the
wrongly-edited files (`git diff --cached`/`git diff`), restoring that
checkout to clean, and applying the diff (`git apply`) in the correct
worktree — don't just re-run the edits from memory, since that risks drift
from what was actually tested.

## 3. Create (or find) the tracking issue

Before editing, always create a tracking GitHub issue for the work via the
`refine-issue` skill (not a bare `gh issue create`), so Priority/Status/
labels get set — do this even for work that wasn't explicitly requested as
an "issue", e.g. tooling/doc changes.

If a finalized plan exists (from plan mode or otherwise), record it in the
issue's `## Plan` section via `refine-issue` before the first edit, and move
Status to "In progress" at that point.

## Notes

- The `refine-issue` skill owns the repo/project-board config, label lists,
  and Priority (P0/P1/P2) rule — don't redefine any of that here.
- Once the work is done, hand off to `finish-task` for lint/coverage/build/PR/CI.
