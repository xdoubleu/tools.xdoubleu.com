# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 / React 19 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`. A separate macOS-only Go module, `kobo-gateway/`, ships as a downloadable menu-bar helper (own `CLAUDE.md`). `sentrytools/` is a tiny third Go module holding slog→Sentry glue `api` pulls in via a local `replace` directive → [`docs/adr-0009-sentrytools-extracted-module.md`](docs/adr-0009-sentrytools-extracted-module.md).

Apps: **games**, **books** (Go package `apps/books`, schema `books`, proto `books.v1`), **feeds**, **watchparty**, **recipes**, **mealplans**, **shoppinglist**, **dashboard** (no schema of its own), **trains** (SNCB/NMBS, schema `trains`, proto `trains.v1` — GTFS static ingest → [`docs/spec-trains-gtfs-ingest.md`](docs/spec-trains-gtfs-ingest.md) plus a CSA journey planner → [`docs/spec-trains-journey-search.md`](docs/spec-trains-journey-search.md); `web/app/trains` is its first user-visible page — station pickers and a route overview). All apps are registered in `api/cmd/api/apps.go` (implements the `App` interface: `Routes`, `ApplyMigrations`, `GetName`, `GetDisplayName`, `GetDomain`, `Start`) — **migrations run sequentially in registration order**, so schema dependencies between apps dictate the list order. **dashboard** owns the public Games and Reading dashboards and the share-token lifecycle, reaching other apps only through exported methods on their structs → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](docs/adr-0007-dashboard-app-owns-public-sharing.md).

Shared Go code lives in `api/internal/` (auth, config, encryption, family, observability, github, sentryapi, mailer, oauthconn, mcptools, repositories, safedial, testhelper). Any outbound fetch of a
**user-supplied URL** must go through `api/internal/safedial` — its dialer
refuses non-public IPs, which is what keeps books/feeds from being
turned into an SSRF pivot against the container's own network. Each app under `api/apps/<name>/` follows: `internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, and (where relevant) `pkg/`.

**Deploy shape:** `api` and `web` build their own images and deploy as two independent Kamal services behind one shared kamal-proxy instance and domain, routed by path prefix. `api` strips its own `/api` prefix in-process (`api/cmd/api/kamal_proxy_shim.go`) so `/.well-known/*` reaches it untouched; `web` is the catch-all. **Deploy order is web-then-api and is required, not preferred** → [`docs/adr-0001-two-service-kamal-deploy.md`](docs/adr-0001-two-service-kamal-deploy.md).

**Long-request handler deadlines:** a handler that outlives the edge proxy's response timeout gets its connection reset before it writes a byte — no server log, no Sentry event. `deployLogsCtxTimeout` (`api/cmd/api/routes.go`) = 20s and `liveLogDeadline` (`api/internal/digitalocean/logs_live.go`) = 8s. Raising either means also raising `proxy.response_timeout` in `config/deploy.api.yml`; nothing tests this → [`docs/adr-0017-long-request-handler-deadlines.md`](docs/adr-0017-long-request-handler-deadlines.md).

A largely read-only **MCP server** at `/apps/mcp` exposes each app's own read RPCs as `<app>_<rpc>` tools plus 20 unprefixed admin observability tools, so a local Claude CLI can pull production domain data and system health as context. App tools are gated by the caller's own per-app access and return only that user's data; observability tools require admin. Two are deliberate mutations (`resolve_sentry_issue`, `dismiss_security_alert`) → [`docs/spec-mcp-server.md`](docs/spec-mcp-server.md); auth flow in [`docs/adr-0006-embedded-oauth21-authorization-server.md`](docs/adr-0006-embedded-oauth21-authorization-server.md) and `README.md`.

**MCP coverage gaps:** if the user describes a production issue and there's no MCP tool that surfaces it, or an existing tool returns wrong/incomplete data, fix that gap first (add/correct the tool) before investigating the issue itself — otherwise the same blind spot just recurs next time. Record the case in [`docs/convention-mcp-gap-first.md`](docs/convention-mcp-gap-first.md), which also lists the known open gaps.

## Code Navigation (ast-grep, LSP)

**Prefer `ast-grep` over `grep`/`rg` for code searches** — it understands syntax trees, so results are exact (no false positives from comments/strings). Reserve `grep`/`rg` for non-code files (logs, configs, docs).

```bash
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript
ast-grep run --pattern '...' --lang go api/apps/recipes/   # scope to a subtree
```

`$NAME` matches one node, `$$$` matches zero or more, `$$` matches one complex expression.

**Once you have a concrete symbol, prefer the `LSP` tool's go-to-definition/find-references over `ast-grep`** — LSP resolves interfaces, generics, and shadowing correctly (gopls/typescript-language-server understand bindings), which ast-grep's pure syntax matching can't guarantee: a structural pattern can over- or under-match without semantic resolution. The two aren't interchangeable, though — `ast-grep` is for structural pattern search with no starting symbol ("find every call shaped like X across the tree," "find every struct matching this field pattern"), which LSP has no equivalent for. Use LSP when navigating from a known symbol; use `ast-grep` when searching by shape.

**Comments must describe current behavior, not history.** Never write a comment that references removed code, superseded architecture, or frames a landed change as still-pending — a stale claim actively misleads the next reader (human or Claude). If historical context genuinely explains *why* the current code looks the way it does, phrase it so it stays true regardless of when it's read ("replicating what X used to provide", never "X hasn't happened yet") → [`docs/convention-comments-describe-current-behavior.md`](docs/convention-comments-describe-current-behavior.md).

**Do not read `api/gen/`, `api/internal/mocks/`, `api/apps/*/internal/mocks/`, or `web/lib/gen/`** to discover field names, RPC signatures, or mock signatures — read the corresponding `.proto` file in `proto/` or the source interface instead; it's smaller and is the source of truth.

## Delegating to Subagents

Prefer the `Agent` tool for noisy, multi-step, or bulk data-gathering — a grep sweep across many files, a log/CI trawl, an MCP call whose raw output is large (e.g. `mcp__tools-apps__get_logs`, `get_sentry_issues`), or open-ended codebase exploration for a research question — rather than doing it inline in the main session; have the subagent return only the distilled findings. This is the same principle plan mode already applies via the `Explore` agent type, extended to non-plan-mode work: keep raw, mostly-discarded tool output out of the main context, not just the final answer.

## Commands

```bash
# API (from api/)
docker-compose up -d       # start Postgres (needed before running/testing)
make run                   # go run ./cmd/api
make test                  # go test -p 1 ./...
make lint                  # golangci-lint + sqlfluff + buf lint + migration-version check (duplicate or out-of-order) + Kamal deploy-secret consistency check
make lint/fix               # auto-fix (golines, golangci-lint --fix, gci, sqlfluff, buf lint)
make test/cov/report        # coverage report
make build                  # go build ./cmd/api
docker-compose down

# Web (from web/)
npm run dev
npm test                    # jest
npm run lint                 # eslint + tsc --noEmit + prettier --check + knip + syncpack
npm run lint:fix
npm run build                # required before finishing web tasks, see below
npm run generate             # regenerate lib/gen/ ConnectRPC clients from proto

# Kobo Gateway (from kobo-gateway/, macOS only — cgo + AppKit)
make build / make dist / make test / make lint/fix

# Proto (when any .proto file changes, run BOTH generators)
cd api && make proto/generate   # regenerates api/gen/
cd web && npm run generate      # regenerates web/lib/gen/
cd api && make proto/check      # regenerate + fail if that changed anything uncommitted (mirrors CI)
cd web && npm run generate:check
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed. Both directories are fully excluded from every lint/fix tool in this repo (gci's `--skip-generated`, golangci-lint's `formatters.exclusions.paths: (^|/)gen/`, web's `.prettierignore` and eslint `ignores`), so there's no ordering dependency between regenerating and running lint/fix — run them in either order. `make proto/check` / `npm run generate:check` run the exact same regenerate-then-diff CI's proto-staleness check (`proto-check.yml`) does, so use those to verify locally rather than reasoning about the exclusion config by hand. Run `make lint/proto` to also catch `buf lint` issues (e.g. RPC response types must be named `<Method>Response`) before pushing.

**Prefer the commands above over ad-hoc equivalents.** If a check/build/verification isn't covered by an existing `make`/`npm run` target, add one to the relevant `Makefile`/`package.json` rather than improvising it with raw tool invocations, and if an existing target doesn't do quite what's needed, fix the target itself. This file documents *what* a command does and *why*, never *how* — re-deriving a command's mechanics in prose here is exactly what let a stale claim (a false ordering requirement between `make proto/generate` and `make lint/fix`) drift out of sync between this file and `api/CLAUDE.md` until `make proto/check`/`npm run generate:check` replaced both explanations with one command.

Run a single Go test: `go test ./apps/books/internal/services/... -run TestName -v` (from `api/`). Single Jest test: `npx jest path/to/file.test.ts -t "test name"` (from `web/`).

## Starting a Task

Before exploring, reading code, or making any change, use the `start-task` skill — it pulls latest `main`, creates a completely fresh worktree (never edit in the main checkout or reuse an existing branch/worktree), and creates/refines the GitHub tracking issue via `refine-issue` before the first edit.

**Exiting plan mode does not count as having started the task.** `permissions.defaultMode` is `plan`, so nearly every session here begins by planning — and an approved plan is not a substitute for `start-task`, which still runs before the first edit, with the approved plan recorded in the tracking issue's `## Plan` section.

## Finishing a Task

Once a task's changes are complete, use the `finish-task` skill — it covers lint, coverage, the web build, opening the PR (including the auto-merge decision), watching CI to green, and the mandatory `session-retro` that follows.

**This is unconditional, and opening the PR is pre-authorized** — the user does not need to ask for one, and a branch that is merely committed and pushed is not a finished task. Should `finish-task` never load for any reason, the floor is still: lint the areas that changed, ≥80% coverage on changed code, `npm run build` for web changes, then a non-draft PR whose body closes the tracking issue with a keyword (`Fixes #123`), then watch CI to green.

An `ExitPlanMode` hook and a `Stop` hook in `.claude/settings.json` enforce both halves, and `make hooks/test` exercises them. **In a Claude Code on the web session nothing external enforces this** — opening the PR unprompted is the only guardrail left there → [`docs/adr-0014-start-finish-task-enforcement.md`](docs/adr-0014-start-finish-task-enforcement.md).

`start-task`/`finish-task` are thin, project-specific wrappers around generic skills (`task-worktree`, `ship-pr`, `session-retro`, `refine-issue`, `issue-triage`) published from the `xdoubleu/xdoubleu-claude-plugins` marketplace repo — declared in `.claude/settings.json`'s `extraKnownMarketplaces`/`enabledPlugins`. `refine-issue`/`issue-triage`'s repo/project-board/label config lives in `.claude/github-triage.config.json`, not in the skill files — edit that file, not the plugin, when this repo's board/labels change.

## CI

`.github/workflows/main.yml` orchestrates reusable workflows (`proto-check`, `build-api`, `build-web`, `build-kobo-gateway`, `api-lint`, `web-lint`, `kobo-gateway-lint`, `api-test`, `web-test`, `kobo-gateway-test`) gated by a `changes` path filter. `kobo-gateway-*` jobs run on `macos-14` (cgo/AppKit). On PRs the full suite runs and **`ci-pass` is the required check**; on push to `main` the lint jobs don't re-run but the build jobs do, since they produce the deploy artifacts, and `deploy-kamal` then deploys → [`docs/spec-ci-pipeline.md`](docs/spec-ci-pipeline.md).

The `api`/`web`/`kobo_gateway` filters exclude `**/*.md`, so a docs-only PR triggers none of the build/lint/test jobs and `ci-pass` skips Codecov entirely.

**Because `main` deploys without re-testing, never push directly to `main`** — only merge PRs whose CI passed. When editing any `.github/workflows/*.yml`, ensure its own `pull_request` trigger includes `.github/workflows/**` in its `paths` filter (docker-build workflows are the deliberate exception — push-to-main only).

**Never add a cache or artifact write to a workflow reachable from `pull_request` or `workflow_dispatch`.** kobo-gateway builds on `macos-14` and reaches `build-web.yml` as a workflow-run artifact; only `save-kobo-gateway-cache.yml` ever writes its cache → [`docs/adr-0002-kobo-gateway-ci-cache-split.md`](docs/adr-0002-kobo-gateway-ci-cache-split.md). The `kobo_gateway` path filter feeds `build-kobo-gateway` and `build-web`'s own gate — keep it in sync if `kobo-gateway/` moves. `build-web.yml` **cannot** be dispatched standalone; use `main.yml`'s own `workflow_dispatch`.

Images are cached via BuildKit's `type=gha,scope=<component>` — **no hand-computed `hashFiles()` cache key anywhere in this path** → [`docs/adr-0003-buildkit-gha-cache-over-hashfiles.md`](docs/adr-0003-buildkit-gha-cache-over-hashfiles.md). `RELEASE` is a runtime container `ENV` for `api`/`web`; only `kobo-gateway` is compile-stamped, so its bundled release can legitimately lag the deploy → [`docs/adr-0004-runtime-release-env-vs-compile-stamp.md`](docs/adr-0004-runtime-release-env-vs-compile-stamp.md).

**The deploy-secret list is declared in three places that must agree** — `config/deploy.{api,web}.yml`'s `env.secret:`, `.kamal/secrets`, and each `Deploy <svc> via Kamal` step's `env:` block in `main.yml`. `make lint/kamal-secrets` fails the PR when they disagree; a mismatch otherwise only surfaces at deploy time on `main` → [`docs/convention-deploy-secrets.md`](docs/convention-deploy-secrets.md). `infra/README.md` is the single source of truth for the full secrets list.

`golangci-lint` runs across all three Go modules (`api`, `kobo-gateway`, `sentrytools`) off one shared **root** `.golangci.yml` — its config search walks up from the working directory. A change to `sentrytools/` can break `api` without touching `api/`'s subtree, so that path filter is OR'd into `api`'s own gate → [`docs/adr-0009-sentrytools-extracted-module.md`](docs/adr-0009-sentrytools-extracted-module.md).

If `ci-pass` fails with "Timed out waiting for Codecov to report", Codecov's check-suite is stuck — push a new (even empty) commit; rerunning the job never helps. See [`docs/spec-ci-pipeline.md`](docs/spec-ci-pipeline.md).

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update this file and `README.md` in the same change.

**Decisions, rationale and incident history go in `docs/`, not here.** The rule of thumb: *imperative mood stays in a `CLAUDE.md`; past tense moves to `docs/`.* If a sentence you are about to add explains what something used to be, what was tried and rejected, or which issue produced a rule, write it as a `docs/` document instead and leave one pointer line here. When you add a document, register it in both `docs/README.md` and the index below — that index is what makes these files discoverable, since only `CLAUDE.md` files load automatically.

## Documented Decisions (`docs/`)

Read the relevant file before changing the area it covers. Full index with issue
numbers: [`docs/README.md`](docs/README.md).

**Decisions (ADRs)**

- [`adr-0001-two-service-kamal-deploy`](docs/adr-0001-two-service-kamal-deploy.md) — two Kamal services, one proxy; the required web-then-api deploy order
- [`adr-0002-kobo-gateway-ci-cache-split`](docs/adr-0002-kobo-gateway-ci-cache-split.md) — why only a `workflow_run` job may write the cache
- [`adr-0003-buildkit-gha-cache-over-hashfiles`](docs/adr-0003-buildkit-gha-cache-over-hashfiles.md) — image layer caching
- [`adr-0004-runtime-release-env-vs-compile-stamp`](docs/adr-0004-runtime-release-env-vs-compile-stamp.md) — how `RELEASE` is set per artifact
- [`adr-0005-first-party-auth-replacing-gotrue`](docs/adr-0005-first-party-auth-replacing-gotrue.md) — sessions, refresh-token rotation, 2FA
- [`adr-0006-embedded-oauth21-authorization-server`](docs/adr-0006-embedded-oauth21-authorization-server.md) — fosite AS; `offline_access` granted server-side
- [`adr-0007-dashboard-app-owns-public-sharing`](docs/adr-0007-dashboard-app-owns-public-sharing.md) — the schema-less `dashboard` app
- [`adr-0008-family-as-single-sharing-concept`](docs/adr-0008-family-as-single-sharing-concept.md) — the one sharing model
- [`adr-0009-sentrytools-extracted-module`](docs/adr-0009-sentrytools-extracted-module.md) — the local `replace` and its build-context consequence
- [`adr-0010-two-weekly-digest-emails`](docs/adr-0010-two-weekly-digest-emails.md) — digest split and suppression rules
- [`adr-0011-slow-transaction-thresholds`](docs/adr-0011-slow-transaction-thresholds.md) — why WebSocket routes stay in breach on purpose
- [`adr-0012-ubuntu-release-check-on-vps`](docs/adr-0012-ubuntu-release-check-on-vps.md) — the job that became a systemd timer
- [`adr-0013-diff-scoped-coverage`](docs/adr-0013-diff-scoped-coverage.md) — changed-line coverage and the signature fixup
- [`adr-0014-start-finish-task-enforcement`](docs/adr-0014-start-finish-task-enforcement.md) — the two hooks and the web-session gap
- [`adr-0015-kobo-gateway-separate-module-and-toolchain-pin`](docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) — never bump past Go 1.24.x alone
- [`adr-0016-kobo-gateway-loopback-tls-and-login-item`](docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md) — loopback HTTPS and LaunchAgents
- [`adr-0017-long-request-handler-deadlines`](docs/adr-0017-long-request-handler-deadlines.md) — deadlines vs the proxy ceiling
- [`adr-0018-completion-average-population`](docs/adr-0018-completion-average-population.md) — a delisted game counts unless a listed game took its achievements

**Conventions**

- [`convention-comments-describe-current-behavior`](docs/convention-comments-describe-current-behavior.md)
- [`convention-database-queries`](docs/convention-database-queries.md) — wide TEXT columns; allowed cross-schema read direction
- [`convention-deploy-secrets`](docs/convention-deploy-secrets.md) — the three lists that must agree
- [`convention-mcp-gap-first`](docs/convention-mcp-gap-first.md) — fix the tool before the incident; open gaps
- [`convention-ui-standards`](docs/convention-ui-standards.md) — UI rules, theming, the server/client import trap

**Specs**

- [`spec-ci-pipeline`](docs/spec-ci-pipeline.md) — workflow behavior, the stuck-Codecov gap
- [`spec-kobo-gateway-runtime`](docs/spec-kobo-gateway-runtime.md) — server, TLS, menu bar, self-update, crash recovery
- [`spec-mcp-server`](docs/spec-mcp-server.md) — the `/apps/mcp` tool surface
- [`spec-oauth-consent-screen`](docs/spec-oauth-consent-screen.md) — the `/oauth/consent` flow
- [`spec-observability-subsystem`](docs/spec-observability-subsystem.md) — jobs, usage recording, host metrics, log tee
- [`spec-trains-gtfs-ingest`](docs/spec-trains-gtfs-ingest.md) — GTFS import and its feed traps
- [`spec-trains-journey-search`](docs/spec-trains-journey-search.md) — CSA journey planner, rolling window, Pareto search
- [`spec-ui-primitives`](docs/spec-ui-primitives.md) — generated `components/ui/` inventory; check before building a component
- [`spec-web-data-flow`](docs/spec-web-data-flow.md) — RSC/SWR transports and hydration

Host-layer decisions stay in [`infra/README.md`](infra/README.md), the single
source of truth for the deploy-secret list.
