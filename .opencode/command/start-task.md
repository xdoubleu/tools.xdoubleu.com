---
description: Start a new task on tools.xdoubleu.com — fresh branch off main, tracking issue before the first edit
---

Follow `docs/convention-task-lifecycle.md`'s "Starting" section for the task: $ARGUMENTS

Concretely, in this repo:

1. `git fetch origin main`, then create a new branch off `origin/main` —
   never build on a stale base, and never reuse a branch/worktree from an
   earlier task.
2. Find or create a GitHub tracking issue for this work with `gh issue
   list`/`gh issue create` (or the GitHub MCP tools if `gh` isn't
   available). If an issue already exists but its body is empty, a title
   alone isn't a scope — figure out what's actually wanted before writing
   any code, asking the user if it's genuinely unclear.
3. Record the plan you're about to execute in the issue body (`gh issue
   edit --body-file ...` or an equivalent MCP call) before making the
   first edit.

Read `AGENTS.md` for the repository's architecture, commands, and
conventions before exploring further — it's the shared contract this
workflow assumes.
