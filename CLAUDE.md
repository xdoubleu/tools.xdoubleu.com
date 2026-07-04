# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

This file holds only cross-cutting context. Area-specific guidance lives in:

- [`api/CLAUDE.md`](api/CLAUDE.md) — Go backend, Postgres, ConnectRPC, `make` commands.
- [`web/CLAUDE.md`](web/CLAUDE.md) — Next.js frontend, UI standards, `npm` commands.

Claude Code auto-loads the `CLAUDE.md` of the current working directory, so the area files only load when you are working in that area.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`.

Apps: **games**, **books**, **watchparty**, **icsproxy**, **recipes**, **mealplans**, **shoppinglist**, **todos**. See [`api/CLAUDE.md`](api/CLAUDE.md) for per-app details.

## Code Navigation (ast-grep)

**Prefer `ast-grep` over `grep` for any code search.** It understands syntax trees so results are exact — no false positives from comments or strings.

```bash
# Go
ast-grep run --pattern '$$.FunctionName($$$)' --lang go
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go

# TypeScript
ast-grep run --pattern 'functionName($$$)' --lang typescript
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript

# Scope to a subtree
ast-grep run --pattern '...' --lang go api/apps/recipes/
```

Key patterns: `$NAME` matches any single node; `$$$` matches zero or more nodes; `$$` matches a single complex expression.

## Proto Code Generation

When a `.proto` file changes, **both** generators must run — a change without both leaves one side stale.

```bash
# From api/
make proto/generate     # regenerates api/gen/ Go stubs

# From web/
npm run generate        # regenerates web/lib/gen/ TypeScript clients
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed; CI regenerates them automatically via `build.yml`.

**Do not read `api/gen/`, `api/internal/mocks/`, `api/apps/*/internal/mocks/`, or `web/lib/gen/`** to discover field names, message types, RPC signatures, or mock method signatures. Read the corresponding `.proto` file in `proto/` or the interface definition in the source package instead — it is much smaller and is the source of truth. Use `ast-grep` on `.proto` files for navigation.

## File Reading Efficiency

When **exploring** (finding a symbol, understanding structure, checking a type): read with `limit=50`.
When **implementing or editing**: read the full file only when you need to place edits accurately.

Never read generated or mock files — the warning in "Proto Code Generation" applies to all sessions. Alternatives:

- Field names / RPC signatures → read the `.proto` file in `proto/`
- Mock method signatures → read the interface definition in the source package (not `internal/mocks/`)

## Finishing a Task — Required Final Steps

After every code change, always run **both** of the following before reporting the task as done:

1. **Lint** (auto-fix, then check nothing remains):

   ```bash
   # api changes
   cd api && make lint/fix

   # web changes
   cd web && npm run lint
   ```

2. **Coverage** — target ≥ 80% on the changed code. Run the coverage report and confirm the diff is covered:

   ```bash
   # api — start DB first, run coverage, then stop DB
   cd api && docker-compose up -d && make test/cov/report && docker-compose down

   # web
   cd web && npm run test:cov
   ```

   Always start the DB with `docker-compose up -d` (from `api/`) before running api tests and stop it with `docker-compose down` afterwards. Do not silently skip this step.

These two steps are **not optional**. Do not mark any task complete without running both.

## CI

See `.github/workflows/` for the pipeline. `main.yml` orchestrates reusable workflows: `proto-check`, `build-api`, `build-web`, `api-lint`, `api-test`, `web-lint`, `web-test`, gated by a `changes` path filter.

- **On pull requests** (and `workflow_dispatch`): the full suite runs. Lint and test run in parallel with the builds (they compile independently — they do not wait on `build-*`). The `ci-pass` job aggregates them and is the required status check. It also waits for Codecov to post its commit statuses (`codecov/project`, `codecov/patch`) and fails if Codecov reports a failure — so coverage gating flows through `ci-pass` without it recomputing coverage itself. The wait is skipped when neither test job ran (no upload, so no Codecov status to wait for).
- **On push to `main`**: lint, build, and proto-check do **not** re-run — the PR's green checks are trusted. `changes → docker → deploy` is the deploy path; Docker's own multi-stage build is the build gate (a failed `go build`/`npm run build` stops the push and deploy). `docker.yml` uses GitHub Actions layer caching (`type=gha`, scoped per image) so unchanged dependency layers are reused. `deploy` then triggers the DigitalOcean deployment. The `api-test`/`web-test` jobs **do** re-run on push (gated by the `changes` filter), but only to refresh Codecov's default-branch baseline — they run in parallel and `docker`/`deploy` do **not** depend on them, so deployment is never gated by tests. `ci-pass` stays PR-only.

Because `main` is deployed without re-testing, protect `main` from direct pushes and merge only PRs whose CI passed.

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update the relevant `CLAUDE.md` (root / `api/` / `web/`) and `README.md` in the same change.
