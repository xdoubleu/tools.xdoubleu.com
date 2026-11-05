# Task Lifecycle

The harness-neutral rules for starting and finishing a task. Harness
mechanisms reference this file and add only the *how*:

- Claude Code: the `start-task`/`finish-task` skills and the `PreToolUse`/
  `Stop`/`ExitPlanMode` hooks ([`adr-0014`](adr-0014-start-finish-task-enforcement.md)).
- OpenCode: the same skills (via compatibility discovery, dependencies under
  `.agents/skills/`) and `.opencode/plugins/repo-guard`.

## Factory jobs

A job on the factory runner (`FACTORY_JOB` set) runs unattended in a sandbox on a
branch the host cloned. It skips the worktree, claim and PR steps: it commits
locally and writes its outcome to `$FACTORY_OUTCOME_PATH`; the host checks the work,
pushes it and opens the PR ([`adr-0045`](adr-0045-factory-github-app.md)). The edit
guard stands aside when `FACTORY_JOB` is set; the sandbox is the boundary.

## Factory jobs

A job on the factory runner (`FACTORY_JOB` set) runs unattended in a sandbox on a
branch the host cloned. It skips the worktree, claim and PR steps and the retro,
and doesn't push, move the task or resolve Sentry issues. It does the lint,
coverage and build steps as far as the offline sandbox allows, commits locally, and
writes its outcome to `$FACTORY_OUTCOME_PATH`: the first line is the PR title, then
what it did and what is left. The host checks the work, pushes it and opens the PR
([`adr-0045`](adr-0045-factory-github-app.md)). The edit guard (both harnesses) stands aside when
`FACTORY_JOB` is set; the sandbox is the boundary.

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
   keyword) and moves it to In review; a merge moves it to Testing, and the owner moves it to
   Done. Pushed without a PR is not finished. Work started from a GitHub issue (the sweeps,
   until #2779) closes it with `Fixes #123` instead.
6. **Never merge on red CI; never push to `main`.**
7. **Watch CI to green** and fix what it finds.
