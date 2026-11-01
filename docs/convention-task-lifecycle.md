# Task Lifecycle

The harness-neutral rules for starting and finishing a task. Harness
mechanisms reference this file and add only the *how*:

- Claude Code: the `start-task`/`finish-task` skills and the `PreToolUse`/
  `Stop`/`ExitPlanMode` hooks ([`adr-0014`](adr-0014-start-finish-task-enforcement.md)).
- OpenCode: the same skills (via compatibility discovery, dependencies under
  `.agents/skills/`) and `.opencode/plugins/repo-guard`.

## Starting

1. **Start from freshly fetched `main`** — never edit on `main` or another
   task's branch.
2. **Fresh branch/worktree per task** — never reuse one, even from earlier in
   the same session.
3. **A board task with a real scope before the first edit.** A title is
   not a scope: if the body is empty, find out what's wanted first. Record
   the plan in the task (its body, or a `## Plan` comment: agents can't edit a
   body) and move it to In progress.

## Finishing

1. **Lint** what changed (`AGENTS.md`'s Commands), plus `make lint/docs`.
2. **≥80% coverage on changed code** — the diff-scoped targets
   (`make test/cov/diff`, `npm run test:cov:diff`) approximate CI's gate.
3. **`npm run build`** for any web change (only it builds the production
   bundle).
4. **Proto changed:** `make proto/check` (`api/`) and `npm run
   generate:check` (`web/`).
5. **Non-draft PR** that links the task (`Task: <title> (<id>)`, no closing
   keyword) and moves it to In review; the owner moves it to Done. Pushed
   without a PR is not finished. Work started from a GitHub issue (the sweeps,
   until #2779) closes it with `Fixes #123` instead.
6. **Never merge on red CI; never push to `main`.**
7. **Watch CI to green** and fix what it finds.
