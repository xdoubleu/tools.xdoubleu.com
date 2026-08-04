# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 / React 19 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`.

Apps: **games**, **books** (Go package `apps/books`, schema `books`, proto `books.v1`), **watchparty**, **icsproxy**, **recipes**, **mealplans**, **shoppinglist**, **todos**. All apps are registered in `api/cmd/api/apps.go` (implements the `App` interface: `Routes`, `ApplyMigrations`, `GetName`, `GetDisplayName`, `GetDomain`, `Start`) — migrations run sequentially in registration order, so schema dependencies between apps (e.g. books adopting the old backlog schema before games drops it) dictate the list order.

Shared Go code lives in `api/internal/` (auth, config, encryption, contacts, observability, digitalocean, github, sentryapi, mailer, oauthconn, mcptools, repositories, testhelper). Each app under `api/apps/<name>/` follows: `internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, and (where relevant) `pkg/`.

**Deploy shape (issue #558):** `api` and `web` build into a single Docker image (root `Dockerfile`) and deploy as one DigitalOcean App Platform component (DO bills per component; both used to run on separate smallest-tier instances). The Go binary is PID 1, spawns the Next.js standalone server as a child process (`api/cmd/api/web_process.go`, `WEB_ENABLED=true`), and reverse-proxies every request (`api/cmd/api/frontend_proxy.go`) — stripping `/api` for the Go mux, routing everything else to the Next child, replicating the old two-component ingress split.

**Edge request timeout (~25s):** DigitalOcean's own edge (Envoy) resets any upstream request past ~25s with a silent `503 UC` — no server log, no Sentry event, because the reset lands before the handler writes a byte. Any handler must finish (or start streaming) under this ceiling; a deadline set *above* it never fires since the edge kills the connection first. This bit `GetDeployLogs` twice (issue #672) before deadlines were lowered under it (`deployLogsCtxTimeout` in `api/cmd/api/routes.go` = 20s, `liveLogDeadline` in `api/internal/digitalocean/logs_live.go` = 8s). The ceiling is DO-side, not configurable, and enforced only by convention/comments today — no test asserts these constants stay under it.

A largely read-only **MCP server** at `/apps/mcp` (MCP OAuth 2.1, Supabase as the authorization server) exposes each app's own read RPCs as `<app>_<rpc>` tools plus 9 unprefixed admin observability tools, so a local Claude CLI can pull production domain data and system health as context. Apps opt in by implementing `MCPToolProvider` (`api/cmd/api/apps.go`); shared gating/marshaling lives in `api/internal/mcptools`. App tools are gated by the caller's own per-app access and return only that user's data; observability tools require admin. One observability tool, `resolve_sentry_issue`, is a deliberate exception to "read-only" — it lets an admin-authenticated agent close out a Sentry issue it just filed a fix for. See the "Apps MCP server" section in `README.md` for the full auth flow and setup.

**MCP coverage gaps:** if the user describes a production issue and there's no MCP tool that surfaces it, or an existing tool returns wrong/incomplete data, fix that gap first (add/correct the tool) before investigating the issue itself — otherwise the same blind spot just recurs next time. Note the gap and fix in this file when it happens.

## Code Navigation (ast-grep)

**Prefer `ast-grep` over `grep`/`rg` for code searches** — it understands syntax trees, so results are exact (no false positives from comments/strings). Reserve `grep`/`rg` for non-code files (logs, configs, docs).

```bash
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript
ast-grep run --pattern '...' --lang go api/apps/recipes/   # scope to a subtree
```

`$NAME` matches one node, `$$$` matches zero or more, `$$` matches one complex expression.

**Do not read `api/gen/`, `api/internal/mocks/`, `api/apps/*/internal/mocks/`, or `web/lib/gen/`** to discover field names, RPC signatures, or mock signatures — read the corresponding `.proto` file in `proto/` or the source interface instead; it's smaller and is the source of truth.

## Commands

```bash
# API (from api/)
docker-compose up -d       # start Postgres (needed before running/testing)
make run                   # go run ./cmd/api
make test                  # go test -p 1 ./...
make lint                  # golangci-lint + sqlfluff + buf lint
make lint/fix               # auto-fix (golines, golangci-lint --fix, gci, sqlfluff, buf lint)
make test/cov/report        # coverage report
make build                  # go build ./cmd/api
make scaffold NAME=x [DB=true] [JOBS=true]   # generate a new app skeleton
docker-compose down

# Web (from web/)
npm run dev
npm test                    # jest
npm run lint                 # eslint + tsc --noEmit + prettier --check + knip + syncpack
npm run lint:fix
npm run build                # required before finishing web tasks, see below
npm run generate             # regenerate lib/gen/ ConnectRPC clients from proto

# Gateway (from gateway/, macOS only — cgo + AppKit)
make build / make dist / make test / make lint/fix

# Proto (when any .proto file changes, run BOTH generators)
cd api && make proto/generate   # regenerates api/gen/
cd web && npm run generate      # regenerates web/lib/gen/
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed; CI's proto-staleness check also runs `buf lint` (e.g. RPC response types must be named `<Method>Response`) — run `make lint/proto` locally first. **`make proto/generate` must be the last step touching `api/gen/` before committing** — `make lint/fix`'s `gci` pass runs across the whole repo (including generated files) and reorders their import groups, which CI's proto-staleness check flags as stale since it diffs a raw `buf generate` (never gci'd) against the committed files. If `make lint/fix` ran after (or in the same session as) `make proto/generate` for any reason, re-run `make proto/generate` afterward and `git diff api/gen web/lib/gen` should show nothing before committing.

Run a single Go test: `go test ./apps/books/internal/services/... -run TestName -v` (from `api/`). Single Jest test: `npx jest path/to/file.test.ts -t "test name"` (from `web/`).

## Adding a New Tool

```bash
cd api && make scaffold NAME=mytool [DB=true] [JOBS=true]
```

The scaffold does **not** auto-register the app — after scaffolding: register it in `api/cmd/api/apps.go`, implement handlers/routes in `api/apps/mytool/routes.go`, add domain logic under `api/apps/mytool/internal/`, edit the initial migration if `DB=true`, then `make build` to verify.

## Starting a Task

Before exploring or reading code for a task, always pull the latest `main` first (`git checkout main && git pull` from the repo root, or `git fetch origin main`) — don't explore against a stale checkout, since another session or the user may have merged changes since.

Before making any changes, always create a completely fresh worktree off up-to-date `main` — never edit in the main checkout or reuse an existing branch/worktree, even one from earlier in this same task; it may already be merged or based on a stale `main`. Prefer the `EnterWorktree` tool; if unavailable, fall back to:

```bash
git checkout main && git pull
git worktree add ../<descriptive-branch-name> -b <descriptive-branch-name> main
```

**After `EnterWorktree` (or the fallback) returns, every subsequent Read/Edit/Write absolute path must be rebased onto the new worktree directory it reports** — do not keep reusing an absolute path prefix from earlier in the session (e.g. the original checkout, or a prior worktree). Nothing rewrites old paths for you: a stale prefix silently edits the wrong checkout, and since a Bash `cd` doesn't persist between tool calls either, `pwd` alone won't catch it. If this happens, recover by diffing the wrongly-edited files (`git diff --cached`/`git diff`), restoring that checkout to clean, and applying the diff (`git apply`) in the correct worktree — don't just re-run the edits from memory, since that risks drift from what was actually tested.

Before editing, always create a tracking GitHub issue for the work via the `refine-issue` skill (not a bare `gh issue create`), so Priority/Status/labels get set — do this even for work that wasn't explicitly requested as an "issue", e.g. tooling/doc changes. If a finalized plan exists (from plan mode or otherwise), record it in the issue's `## Plan` section before the first edit, and move Status to "In progress" at that point.

## Finishing a Task — Required Final Steps

1. **Lint** — `cd api && make lint/fix` and/or `cd web && npm run lint` (whichever area changed); `cd gateway && make lint/fix` for gateway changes.
2. **Coverage** — target ≥80% on changed code. API: `cd api && docker-compose up -d && make test/cov/report && docker-compose down` (always stop the DB after). Web: `cd web && npm run test:cov`.
3. **Build** (web changes only) — `cd web && npm run build`. Next.js's server/client boundary check (a Server Component importing anything from a file that pulls in client-only hooks) is enforced **only** by `next build`, not `tsc --noEmit`, ESLint, or Jest — lint/coverage passing does not mean the build passes. Put constants shared across the boundary in a plain `lib/` module with no React imports.
4. **Open the PR yourself** — don't wait to be asked:
   ```bash
   git push -u origin HEAD
   gh pr view --json number >/dev/null 2>&1 || gh pr create --fill --base main
   ```
   Never push to `main` directly; never open as `--draft`. Reference the tracking issue from "Starting a Task" in the PR body using a closing keyword (e.g. `Fixes #123`, `Closes #123`) so the issue auto-closes on merge — a bare `#123` or "Related to #123" leaves the issue open even after merge (this happened with issue #727 / PR #728).
   - **Code-only changes** (no `CLAUDE.md`, `Makefile`/npm-script, lint config, CI workflow, or script edits): enable auto-merge right away, in the same breath as creating the PR — `gh pr merge --auto --squash` only merges once checks pass, so there's no reason to wait for green first: `gh pr create --fill --base main && gh pr merge --auto --squash`.
   - **Tooling/harness changes** (anything touching `CLAUDE.md`, Makefile targets, lint config, `.github/workflows/*`, scripts, or hooks): do **not** enable auto-merge — these need the user's own review.
5. **Monitor CI until green, fixing it yourself if it isn't**:
   ```bash
   gh pr checks --watch
   gh pr view --json mergeable,mergeStateStatus,statusCheckRollup
   ```
   A red PR or non-`MERGEABLE` state is not "done" — diagnose the actual failure (don't just re-run blindly) and repeat from step 1. Once green + mergeable, report the PR URL (auto-merge was already armed in step 4 for code-only changes; for tooling/harness changes, stop here and wait for review).
6. **Reflect on whether this change exposed a doc/tooling gap** — once CI is green, look back at the commit range for this task and ask: (a) would a CLAUDE.md addition/correction, a new Make/npm target, a lint rule, a CI workflow tweak, or a script have made this specific change faster or safer to implement; (b) did the PR need more than one push to go green (check `gh pr checks`/`gh run list` and the commit log for fixup commits), and if so, what local check would have caught the failure before pushing. Only flag something concrete tied to what actually happened — not speculative "would be nice" additions. If nothing is worth flagging, stop here. If something is: in a **separate** fresh worktree off `main`, open a tracking issue, edit only `CLAUDE.md`/tooling files, and open an independent, non-draft PR referencing it (never stacked on the original PR) — following this same checklist for that PR too.

## CI

`.github/workflows/main.yml` orchestrates reusable workflows (`proto-check`, `build-api`, `build-web`, `build-gateway`, `api-lint`, `api-test`, `web-lint`, `web-test`, `gateway-test`) gated by a `changes` path filter. `gateway-test` runs on a `macos-14` runner (needs to compile cgo/AppKit). The `api`/`web`/`gateway` filters exclude `**/*.md` (e.g. a `CLAUDE.md`-only change doesn't set that filter's output), so a docs-only PR triggers none of the build/lint/test jobs — `ci-pass` then has nothing to wait on and skips triggering Codecov entirely (see its "Trigger and wait for Codecov to report" step, guarded on at least one test job having actually run).

- **On PRs**: full suite runs; `ci-pass` aggregates and is the required check. It also gates on Codecov's `codecov/project`/`codecov/patch` statuses — `codecov.yml` sets `notify.manual_trigger: true` so Codecov posts nothing until `ci-pass` runs `codecovcli send-notifications` after every test job uploads (upload count varies 1-3 by path filter, so a fixed `after_n_builds` doesn't work).
- **On push to `main`**: lint/build/proto-check don't re-run (PR's green checks are trusted). `docker.yml` builds one merged `app` image from the root `Dockerfile` (multi-stage build is the build gate) and triggers DO deploy; test jobs still run to refresh Codecov's baseline but never gate deploy.

Because `main` deploys without re-testing, never push directly to `main` — only merge PRs whose CI passed. When editing any `.github/workflows/*.yml`, ensure its own `pull_request` trigger includes `.github/workflows/**` in its `paths` filter (docker-build workflows are the deliberate exception — push-to-main only).

The **kobo-gateway** app (`gateway/`) ships inside the merged `app` Docker image as a downloadable `.dmg`, built separately on `macos-14` by `build-gateway.yml` and handed to `docker.yml` as an artifact (no `gateway-builder` Docker stage). The `gateway` path filter in `main.yml` feeds `build-web`, `build-gateway`, and `docker.build_app` — keep it in sync if `gateway/` moves.

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update this file and `README.md` in the same change.
