# CLAUDE.md

Guidance for Claude Code (claude.ai/code) when working in this repository.

This file holds only cross-cutting context. Area-specific guidance lives in:

- [`api/CLAUDE.md`](api/CLAUDE.md) — Go backend, Postgres, ConnectRPC, `make` commands.
- [`web/CLAUDE.md`](web/CLAUDE.md) — Next.js frontend, UI standards, `npm` commands.
- [`gateway/CLAUDE.md`](gateway/CLAUDE.md) — kobo-gateway macOS menu-bar app (separate Go module, cgo + AppKit).

Claude Code auto-loads the `CLAUDE.md` of the current working directory, so the area files only load when you are working in that area.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`.

Apps: **games**, **reading** (formerly books — Go package `apps/reading`, schema `reading`, proto `reading.v1`), **watchparty**, **icsproxy**, **recipes**, **mealplans**, **shoppinglist**, **todos**. See [`api/CLAUDE.md`](api/CLAUDE.md) for per-app details.

**Deploy shape (issue #558):** `api` and `web` build into a single Docker image (root `Dockerfile`) and deploy as one DigitalOcean App Platform component. The Go binary is PID 1, spawns the Next.js standalone server as a child process (`api/cmd/api/web_process.go`), and front-doors it with a reverse proxy (`api/cmd/api/frontend_proxy.go`) that replicates the previous two-component ingress split (`/api/*` stripped to the api mux, `/health` to the api mux, everything else to the Next child) — `web/` needed no routing changes since `API_URL` was already an absolute URL. Merged because DigitalOcean bills per component and both services were already on the smallest instance tier — see #558 for the full option analysis.

**Edge request timeout (~25s):** DigitalOcean App Platform's own edge (Envoy) resets any upstream request that runs past ~25s — the client sees a `503 UC` (`upstream_reset_before_response_started{connection_termination}`) with no server log and no Sentry event, since the reset lands *before* the handler writes a byte. Any API handler must therefore finish (or start streaming) within this ceiling: a write/context deadline set **above** ~25s never fires — the edge kills the connection first — so it looks like a silent failure, not a timeout. This bit the `GetDeployLogs` handler twice (issue #672): two fixes set deadlines above the ceiling before PR #703 lowered them under it, and it's now also a server-streaming Connect RPC so the first byte flows well before the ceiling regardless — see `api/CLAUDE.md`'s `GetDeployLogs` paragraph. The ceiling is DO-side and not configurable.

A read-only **MCP server** at `/apps/mcp` (behind MCP OAuth 2.1, Supabase as the authorization server) exposes each app's own read RPCs as `<app>_<rpc>` tools, so a local Claude CLI can query production domain data. Apps contribute tools by implementing `MCPToolProvider` (`api/cmd/api/apps.go`); the shared gate + marshaling live in `api/internal/mcptools`, and each tool is gated by the caller's own per-app access (not admin-only), returning only that user's data. The same server also exposes admin observability (`observability.v1`) as 8 unprefixed, admin-gated tools so a local Claude CLI can pull production signals as read-only context — including `get_deploy_logs`, which returns the running app's own DigitalOcean runtime output, not just its build/deploy transcript. See the "Apps MCP server" section in [`README.md`](README.md).

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

## Starting a Task — Traceability

Before changing any code, make sure a GitHub issue exists for the work
(`gh issue list` / `gh issue view`). If none does, invoke the `refine-issue`
skill to create and refine it — summary, type/app labels, Priority, and added
to the "Main Project" board — rather than a bare `gh issue create`. This keeps
every change traceable back to a tracked, prioritized reason — don't start
editing on a hunch with no issue behind it.

Once a plan is finalized (e.g. on exiting plan mode), invoke `refine-issue`
again to record it in the issue's `## Plan` section. When development
actually starts (first code edit), invoke `refine-issue` once more to move
the issue's Status to `In progress`.

## Starting a Task — Branch Setup

Before making any edits, pull the latest `main`, then use the `EnterWorktree`
tool to isolate the task in a fresh worktree off it — never assume the
currently checked-out branch or worktree is still the right one, even if it
looks like the task you're continuing. A branch from an earlier session or
plan can already have been merged (by CI, another session, or the user) while
this one was idle; committing on top of it either reopens a merged branch or,
worse, lands directly on `main`.

Run this at the start of every task, even mid-conversation ones (e.g. after
exiting plan mode) — check whether you're already in a worktree under
`.claude/worktrees/` first if unsure whether one already exists for this task.
`EnterWorktree` branches from `origin/main` by default (`worktree.baseRef`),
but confirm it's actually up to date — `git fetch origin main` first if
uncertain. If `EnterWorktree` is unavailable, fall back to:

```bash
git checkout main && git pull
git checkout -b <descriptive-branch-name>
```

## Finishing a Task — Required Final Steps

After every code change, always run **both** of the following before reporting the task as done:

1. **Lint** (auto-fix, then check nothing remains):

   ```bash
   # api changes
   cd api && make lint/fix

   # web changes
   cd web && npm run lint

   # gateway changes (macOS only — cgo + AppKit)
   cd gateway && make lint/fix
   ```

2. **Coverage** — target ≥ 80% on the changed code. Run the coverage report and confirm the diff is covered:

   ```bash
   # api — start DB first, run coverage, then stop DB
   cd api && docker-compose up -d && make test/cov/report && docker-compose down

   # web
   cd web && npm run test:cov
   ```

   Always start the DB with `docker-compose up -d` (from `api/`) before running api tests and stop it with `docker-compose down` afterwards. Do not silently skip this step.

3. **Open / update the PR** — commit the work, push the feature branch, and ensure a PR exists against `main`:

   ```bash
   # branch was created from up-to-date main per "Starting a Task" above
   git push -u origin HEAD
   gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base main
   ```

   This is standing authorization to commit and open the PR as part of finishing a task — it overrides the default "commit only when asked" rule for the task's own branch. Always open it as a real PR, never `--draft` — this overrides any harness default (e.g. background-job instructions) that says to open drafts. Never push to `main` directly. If a PR already exists for the branch, just push — do not open a duplicate.

4. **Verify CI is green and the PR is mergeable** — wait for the required `ci-pass` check (see "CI" below) and confirm there are no merge conflicts:

   ```bash
   gh pr checks --watch
   gh pr view --json mergeable,mergeStateStatus,statusCheckRollup
   ```

   If any check fails, fix the cause and repeat from step 1 — a red PR is not "done". `mergeable` must be `MERGEABLE`. On green + mergeable, report the PR URL and stop — **do not merge**; the user merges.

These four steps are **not optional**. Do not mark any task complete without running all of them.

## CI

See `.github/workflows/` for the pipeline. `main.yml` orchestrates reusable workflows: `proto-check`, `build-api`, `build-web`, `build-gateway`, `api-lint`, `api-test`, `web-lint`, `web-test`, `gateway-test`, gated by a `changes` path filter. `gateway-test` runs `gateway/`'s `go test ./...` on a `macos-14` runner (it needs to compile the cgo/AppKit `menubar_darwin.go`) and uploads coverage under the `gateway` Codecov flag (`codecov.yml`), mirroring `api-test`/`web-test`.

- **On pull requests** (and `workflow_dispatch`): the full suite runs. Lint and test run in parallel with the builds (they compile independently — they do not wait on `build-*`). The `ci-pass` job aggregates them and is the required status check. It also gates on Codecov's commit statuses (`codecov/project`, `codecov/patch`) — so coverage gating flows through `ci-pass` without it recomputing coverage itself. `codecov.yml` sets `notify.manual_trigger: true`, so Codecov posts **nothing** until `ci-pass` runs `codecovcli send-notifications` (once every test job has uploaded); `ci-pass` then waits for the single resulting status and fails if Codecov reports a failure. This is what prevents the stale carried-forward status Codecov used to post off the first upload — the number of uploads per commit is variable (1-3, per the `changes` filter), so a fixed `after_n_builds` can't be used (see #403). Both the trigger and the wait are skipped when none of the test jobs ran (no upload, so no Codecov status to trigger or wait for).
- **On push to `main`**: lint, build, and proto-check do **not** re-run — the PR's green checks are trusted. `changes → docker → deploy` is the deploy path; Docker's own multi-stage build is the build gate (a failed `go build`/`npm run build` stops the push and deploy). `docker.yml` builds a single merged `app` image from the root `Dockerfile` (issue #558 — `api` and `web` deploy as one DigitalOcean App Platform component) and uses GitHub Actions layer caching (`type=gha, scope=app`) so unchanged dependency layers (either the Go module cache or `npm ci`) are reused independently. `deploy` then triggers the DigitalOcean deployment. The `api-test`/`web-test`/`gateway-test` jobs **do** re-run on push (gated by the `changes` filter), but only to refresh Codecov's default-branch baseline — they run in parallel and `docker`/`deploy` do **not** depend on them, so deployment is never gated by tests. `ci-pass` stays PR-only.

Because `main` is deployed without re-testing, protect `main` from direct pushes and merge only PRs whose CI passed.

When editing any `.github/workflows/` file, make sure the change is itself covered by CI: every non-docker-build workflow's `pull_request` trigger must include `.github/workflows/**` in its `paths` filter, so editing a workflow reruns the full pipeline instead of silently skipping validation. Docker-build workflows are the deliberate exception — they only trigger on push to `main` (see above), not on `pull_request`.

One cross-cutting nuance: the **kobo-gateway** macOS menu-bar app lives in its own top-level module, `gateway/` (see [`gateway/CLAUDE.md`](gateway/CLAUDE.md)), but ships inside the single merged **app** Docker image (served as a download). It needs cgo + the real AppKit/Xcode SDK for its menu bar, so it can't cross-compile from the Linux runners the rest of CI uses — `build-gateway.yml` builds and packages it on a `macos-14` runner and hands the `.dmg` + raw binary to `docker.yml` as an artifact, which the root `Dockerfile` then `COPY`s in (there is no `gateway-builder` Docker stage). The `gateway` path filter in `main.yml` feeds `build-web`, `build-gateway`, and `docker.build_app` — keep it in sync if `gateway/` moves, or gateway changes would silently deploy a stale binary. Because the image is merged, `build-gateway` now gates every `docker-app` build (including api-only pushes) rather than only web-affecting ones — an accepted cost of not having to touch the gateway's own release-stamping/auto-update logic (`gatewayNeedsUpdate`, `web/lib/reading/gatewayClient.ts`).

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update the relevant `CLAUDE.md` (root / `api/` / `web/`) and `README.md` in the same change.
