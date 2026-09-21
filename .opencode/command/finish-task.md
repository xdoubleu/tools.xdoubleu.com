---
description: Ship the current change on tools.xdoubleu.com — lint, coverage, build, open the PR, watch CI to green
---

Follow `docs/convention-task-lifecycle.md`'s "Finishing" section: $ARGUMENTS

Concretely, in this repo:

1. Lint whatever changed — `make lint`/`make lint/fix` under `api/`, `npm
   run lint`/`npm run lint:fix` under `web/`, `make lint/fix` under
   `kobo-gateway/`. See `AGENTS.md`'s Commands section for the full list.
2. Check coverage on changed code (target ≥80%) — `make test/cov/report`
   or `make test/cov/diff` under `api/`, `npm run test:cov`/`npm run
   test:cov:diff` under `web/`.
3. If any web page/component changed, run `npm run build` in `web/` —
   Next.js's server/client boundary check only fails there, not in
   lint/typecheck/tests.
4. If any `.proto` file changed, run `make proto/check` in `api/` and `npm
   run generate:check` in `web/`, and commit the regenerated stubs.
5. Push the branch and open a non-draft PR with `gh pr create` (or the
   GitHub MCP tools) whose body closes the tracking issue with a closing
   keyword (`Fixes #123`) — never a bare reference.
6. Never push straight to `main`. Watch CI (`gh pr checks --watch` or
   equivalent) until it's green; if it's red, diagnose and fix rather than
   merging anyway or leaving it for someone else.

This is the same lint/coverage/build/PR/CI-green contract Claude Code's
`finish-task` skill enforces in this repo — see `docs/convention-task-lifecycle.md`
for the full rationale. OpenCode has no hook to block an unshipped change
the way Claude Code does, so treat this command as the actual last step of
the task, not an optional wrap-up.
