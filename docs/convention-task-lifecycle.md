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
3. **A tracking issue with a real scope before the first edit.** A title is
   not a scope: if the body is empty, find out what's wanted first. Record
   the plan in the issue body.

## Finishing

1. **Lint** what changed (`AGENTS.md`'s Commands), plus `make lint/docs`.
2. **≥80% coverage on changed code** — the diff-scoped targets
   (`make test/cov/diff`, `npm run test:cov:diff`) approximate CI's gate.
3. **`npm run build`** for any web change (only it checks the server/client
   boundary).
4. **Proto changed:** `make proto/check` (`api/`) and `npm run
   generate:check` (`web/`).
5. **Non-draft PR** whose body closes the issue with a keyword
   (`Fixes #123`). Pushed without a PR is not finished.
6. **Never merge on red CI; never push to `main`.**
7. **Watch CI to green** and fix what it finds.
