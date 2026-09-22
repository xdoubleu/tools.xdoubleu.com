# Task Lifecycle

The harness-neutral shape of "start a task" and "finish a task" in this
repository. This is a **convention**, not an ADR — it states current
imperative rules, not a decision-with-alternatives. Any coding agent working
here should follow it; the mechanism that enforces or automates each step is
documented separately per harness:

- Claude Code: `CLAUDE.md`'s "Starting a Task"/"Finishing a Task" sections,
  the `start-task`/`finish-task` skills, and the `PreToolUse`/`Stop`/
  `ExitPlanMode` hooks in `.claude/settings.json`
  ([`adr-0014`](adr-0014-start-finish-task-enforcement.md)).
- OpenCode: the same `start-task`/`finish-task` skills (read from
  `.claude/skills/` via compatibility discovery; their generic marketplace
  dependencies installed under `.agents/skills/`, tracked in
  `skills-lock.json`) and the `.opencode/plugins/repo-guard` plugin, which
  ports the worktree-scope edit guard, the session-start main ff-merge, and
  the unshipped-work check (as an idle nudge rather than a blocking stop).

Nothing here should be duplicated into either harness adapter — they should
reference this file for *what* the workflow requires and only add *how*
their own harness carries it out.

## Starting

1. **Fetch and start from up-to-date `main`.** Never build on a stale base,
   and never edit in a checkout that's on `main` itself or on another
   task's branch.
2. **Use a fresh branch/worktree per task.** Never reuse an existing
   branch or worktree from an earlier task, even one from earlier in the
   same session.
3. **A tracking GitHub issue must exist, with a real scope, before the
   first edit.** A title alone is not a scope — if the issue exists but
   its body is empty, resolve what's actually wanted before touching code.
   Record the plan (however it was arrived at — a planning conversation, a
   direct instruction, or refinement on the issue itself) in the issue
   body before starting.

## Finishing

1. **Lint** whatever changed — see `AGENTS.md`'s Commands section for the
   per-module targets (`make lint`/`make lint/fix` for `api/`, `npm run
   lint`/`npm run lint:fix` for `web/`, `make lint/fix` for
   `kobo-gateway/`).
2. **≥80% coverage on changed code.** `api/AGENTS.md`'s `make
   test/cov/report`/`make test/cov/diff` and `web/AGENTS.md`'s `npm run
   test:cov`/`npm run test:cov:diff` approximate what CI's coverage gate
   checks — run the diff-scoped variant before pushing to catch a gap
   locally instead of after a CI round trip.
3. **`npm run build` for any web change.** Next.js's server/client
   boundary check is enforced only by the production build, not
   `tsc`/ESLint/Jest.
4. **Regenerate proto stubs if any `.proto` file changed** — `make
   proto/check` (`api/`) and `npm run generate:check` (`web/`); see
   `AGENTS.md`'s Commands section for why there's no ordering dependency
   with lint.
5. **Open a non-draft pull request** whose body closes the tracking issue
   with a GitHub closing keyword (`Fixes #123`, `Closes #123`) — a bare
   issue reference doesn't auto-close on merge. A branch that is merely
   committed and pushed, with no PR opened, is not a finished task.
6. **Never merge on red CI**, and **never push directly to `main`** — `main`
   deploys on every push without re-testing, so only a PR whose CI passed
   reaches it.
7. **Watch CI to green** before considering the task done, and resolve
   anything CI turns up rather than treating a red run as someone else's
   problem.
