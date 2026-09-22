---
name: task-worktree
description: Set up a fresh git worktree off up-to-date main/master before starting a new coding task. Use when beginning a new feature, bug fix, or any change in a git+GitHub repo that doesn't yet have a dedicated worktree — pair with the ship-pr skill when the work wraps up.
---

# Task Worktree

The opening move of any task: get a clean, current, isolated place to work
*before* the first edit. Never edit in the main checkout or reuse an
existing branch/worktree — even one from earlier in this same session; it
may already be merged or based on a stale default branch.

## 1. Pull latest default branch

Find the repo's default branch (`git symbolic-ref refs/remotes/origin/HEAD`,
falling back to `main` or `master`), then `git fetch origin <default>` — or
`git checkout <default> && git pull` from the repo root. Don't explore or
plan against a stale checkout: another session or teammate may have merged
changes since you last looked.

## 2. Create a completely fresh worktree

Prefer the `EnterWorktree` tool if available. Otherwise:

```bash
git fetch origin <default>
git worktree add ../<descriptive-branch-name> -b <descriptive-branch-name> origin/<default>
```

then switch into it (`EnterWorktree({ path: "<new-worktree-path>" })` if that
tool is available, or note the path for the session to use directly
otherwise).

**If `EnterWorktree` errors that it "cannot create a worktree from a
subagent with a cwd override"**, the session's working directory is already
pinned to some other worktree (e.g. subagent isolation, or a bridged/shared
worktree the orchestrator manages). Do **not** work around this by running
`git checkout -b <branch>` in place inside that pinned directory — a shared
or orchestrator-managed worktree can have its branch switched back out from
under you by a concurrent process at any point (observed in practice: a
`git checkout -b` there was silently reverted to the worktree's original
branch mid-session, costing several tool calls — `git reflog`, `git worktree
list`, restoring an unrelated file some other branch had modified — just to
notice and recover). Instead, use `git worktree add` with an **absolute**
path outside that pinned directory (e.g. under the repo's own
`.claude/worktrees/`, or a sibling of it) exactly as the snippet above
shows, and use that absolute path for every subsequent tool call. Never
reuse or branch-switch a directory this session did not create itself via
`EnterWorktree`/`git worktree add`.

**After switching, every subsequent file-editing tool call's absolute path
must be rebased onto the new worktree directory** — don't keep reusing an
absolute path prefix from earlier in the session (the original checkout, or
a prior worktree). Nothing rewrites old paths automatically: a stale prefix
silently edits the wrong checkout, and a shell `cd` doesn't persist between
tool calls either, so `pwd` alone won't catch it. If this happens, recover
by diffing the wrongly-edited files (`git diff --cached`/`git diff`),
restoring that checkout to clean, and applying the diff (`git apply`) in the
correct worktree — don't just re-run the edits from memory, since that risks
drift from what was actually tested.

## Notes

- If this repo tracks work in GitHub Issues (or an equivalent tracker) and
  has its own convention for creating/labeling one, follow that convention
  before the first edit — this skill only covers the worktree/branch
  mechanics, not issue tracking.
- Once the work is done, hand off to `ship-pr` for the rebase/push/PR/
  auto-merge/CI-watch flow.
- This skill never needs `gh` — branch creation and the local worktree are
  both plain `git`. `gh`'s absence (e.g. Claude Code on the web) only
  starts to matter once `ship-pr`/`refine-issue` need to reach the GitHub
  API; see their own "when `gh` isn't available" sections.
