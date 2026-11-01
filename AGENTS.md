# AGENTS.md

The repository contract for any coding agent. Harness mechanics live in `CLAUDE.md` (Claude Code) and `opencode.json` / `.opencode/plugins/` (OpenCode).

## Monorepo Overview

- `api/` — Go 1.27, one binary serving every app over ConnectRPC on a shared mux, plus the web build (`internal/spa`). Each app owns a PostgreSQL schema; protos live in `proto/`.
- `web/` — Vite 8 / React 19 SPA served by the api.
- `kobo-gateway/` — separate macOS-only Go module, a downloadable menu-bar helper.

Apps: games, books, feeds, watchparty, recipes, mealplans, shoppinglist, dashboard (no schema; owns public dashboards and share tokens, using other apps only through exported methods → [adr-0007](docs/adr-0007-dashboard-app-owns-public-sharing.md)), trains (iRail journeys and live delays), learningpaths (agent-authored curricula, per-user, no family sharing), movies (TMDB-backed movies & series backlog, per-user plus a family-shared list), podcasts (per-user favourite iTunes shows), taste (per-user stats, LLM profile and picks over apps' `ListConsumption`/`Find*`/`Want*`), factory (admin-only task board, imports no other app). All are registered in `api/cmd/api/apps.go`; **migrations run in registration order**, so schema dependencies dictate the order.

Shared Go code: `api/internal/`. Each app: `api/apps/<name>/internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, optional `pkg/`.

Read the `AGENTS.md` of `api/`, `web/` or `kobo-gateway/` before working there.

**Deploy:** `api` is the root Kamal service and strips `/api` in-process (`api/cmd/api/kamal_proxy_shim.go`). `grafana` is a second service at `/grafana` → [adr-0034](docs/adr-0034-vite-spa-served-by-the-api.md), [adr-0022](docs/adr-0022-prometheus-grafana-metrics.md).

**Handler deadlines:** a response written after the server's `WriteTimeout` is cut silently. Requests get `httptools.RequestBudget`; longer routes go in `isLongRequest` → [adr-0017](docs/adr-0017-long-request-handler-deadlines.md).

## Production Safety Rules

- Unattended agents act only on human-approved work: only a human moves an issue to Ready, and no PR auto-merges except Renovate's → [convention-unattended-agent-trust](docs/convention-unattended-agent-trust.md).
- **Never push to `main`** — it deploys on every push without re-testing. Merge only PRs with green CI.
- Every outbound fetch of a **user-supplied URL** goes through `api/internal/safedial` (refuses non-public IPs; SSRF guard), never a bare `http.Client`.
- Never skip, disable, or quarantine a test, or bypass hooks/signing, unless a human asks for that specific exception.
- Deploy secrets are listed in three places that must agree (`make lint/kamal-secrets`) → [convention-deploy-secrets](docs/convention-deploy-secrets.md). `infra/README.md` has the full list.
- A migration that changes an existing column's meaning needs a backfill, or a comment saying why none is needed.

## Code Conventions

- **Prefer `ast-grep` over `grep`/`rg` for code** (e.g. `ast-grep run --pattern 'func Name($$$) $$$' --lang go`); use grep/rg for non-code files.
- **Concise docs and comments:** say each thing once, don't overexplain, don't restate the code, describe current behavior only, no issue numbers or history inline. `make lint/docs` enforces budgets → [convention-concise-docs-and-comments](docs/convention-concise-docs-and-comments.md).
- **Don't read generated code** (`api/gen/`, `*/mocks/`, `web/lib/gen/`) — read the `.proto` or source interface.
- Delegate noisy bulk exploration to subagents.

## Commands

```bash
# api/ (needs `docker-compose up -d` for Postgres)
make run | test | build | lint | lint/fix | lint/vuln | test/cov/report
make test/mutation/diff     # gremlins on packages changed vs origin/main
make arch/diagram           # Mermaid package graph → api/package-graph.mmd (not committed)

# web/
npm run dev | test | lint | lint:fix | build   # build is required before finishing web work
npm run test:mutation:diff  # StrykerJS on files changed vs origin/main
npx vitest run path/to/file.test.ts -t "test name"

# kobo-gateway/ (macOS only)
make build | dist | test | lint/fix

# repo root
make lint/docs | lint/skills | lint/workflows | lint/infra | lint/grafana | grafana/verify | hooks/test | routines/test | lint/deploy-version | lint/agent-headers

# proto changes: run BOTH generators (…/local variants avoid buf.build)
cd api && make proto/generate    # check: make proto/check
cd web && npm run generate       # check: npm run generate:check
```

Generated stubs are committed and never linted. If a check isn't covered by a `make`/`npm run` target, add or fix the target.

## Task Lifecycle

Fresh branch off up-to-date `main`, tracking issue before the first edit, lint/coverage/build gates, PR, CI green → [convention-task-lifecycle](docs/convention-task-lifecycle.md). Both harnesses run it through the `start-task`/`finish-task` skills.

## MCP

`/apps/mcp` (`api/cmd/api/mcp_apps.go`) exposes each app's read RPCs as `<app>_<rpc>` tools (caller's own data only) plus admin-only observability tools (`prom_query`, `get_sentry_issues`, …). Mutating tools: learningpaths' write tools ([adr-0023](docs/adr-0023-learningpaths-mcp-write-tools.md)), the draft-only `recipes_create_recipe` ([adr-0026](docs/adr-0026-recipes-mcp-write-tool.md)), and `resolve_sentry_issue`, `dismiss_security_alert`, `record_action` (admin). Auth is first-party OAuth 2.1 with dynamic registration → [adr-0006](docs/adr-0006-embedded-oauth21-authorization-server.md). Headless routines use client_credentials as the `service` role: observability tools only → [adr-0025](docs/adr-0025-machine-client-credentials-service-role.md). Agents use `agent` accounts: only `factory_*` board tools, never Ready → [adr-0039](docs/adr-0039-agent-accounts.md), [adr-0040](docs/adr-0040-factory-mcp-tools.md).

**Gap first:** if no MCP tool surfaces a production issue, or one returns wrong data, fix the tool before the issue → [convention-mcp-gap-first](docs/convention-mcp-gap-first.md).

## CI

`.github/workflows/main.yml` runs reusable build/lint/test workflows behind a `changes` path filter (`**/*.md` excluded); kobo-gateway jobs run on macOS. **`ci-pass` is the required check.** Pushes to `main` rebuild and deploy but skip lint, except the docs word budgets. If `ci-pass` times out waiting for Codecov, push a new commit — re-running never helps.

## Docs

When a change touches structure, packages, targets, shared services, or conventions, update this file. `docs/` holds ADRs, specs, and conventions only — the code is the spec for current behavior. Register new docs below.

- **ADRs:** [0001](docs/adr-0001-two-service-kamal-deploy.md) two-service deploy · [0002](docs/adr-0002-kobo-gateway-ci-cache-split.md) kobo CI cache · [0003](docs/adr-0003-buildkit-gha-cache-over-hashfiles.md) image cache · [0004](docs/adr-0004-runtime-release-env-vs-compile-stamp.md) `RELEASE` · [0005](docs/adr-0005-first-party-auth-replacing-gotrue.md) auth · [0006](docs/adr-0006-embedded-oauth21-authorization-server.md) OAuth AS · [0007](docs/adr-0007-dashboard-app-owns-public-sharing.md) dashboard · [0008](docs/adr-0008-family-as-single-sharing-concept.md) family sharing · [0009](docs/adr-0009-sentrytools-extracted-module.md) sentrytools (superseded) · [0010](docs/adr-0010-two-weekly-digest-emails.md) digests · [0011](docs/adr-0011-slow-transaction-thresholds.md) slow transactions · [0012](docs/adr-0012-ubuntu-release-check-on-vps.md) Ubuntu check · [0013](docs/adr-0013-diff-scoped-coverage.md) diff coverage · [0014](docs/adr-0014-start-finish-task-enforcement.md) lifecycle hooks · [0015](docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) kobo module · [0016](docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md) kobo TLS · [0017](docs/adr-0017-long-request-handler-deadlines.md) deadlines · [0018](docs/adr-0018-completion-average-population.md) completion average · [0019](docs/adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md) trains router · [0020](docs/adr-0020-ui-configured-alert-delivery-channel.md) alert channel (superseded) · [0021](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md) OIDC IdP · [0022](docs/adr-0022-prometheus-grafana-metrics.md) Prometheus/Grafana · [0023](docs/adr-0023-learningpaths-mcp-write-tools.md) learningpaths writes · [0024](docs/adr-0024-posthog-product-analytics.md) PostHog · [0025](docs/adr-0025-machine-client-credentials-service-role.md) routines machine identity · [0026](docs/adr-0026-recipes-mcp-write-tool.md) recipes draft write · [0027](docs/adr-0027-offline-service-worker-and-persisted-swr.md) offline reads · [0028](docs/adr-0028-offline-write-outbox.md) offline writes · [0029](docs/adr-0029-cross-device-reading-positions.md) reading positions · [0030](docs/adr-0030-readium-web-reader.md) Readium · [0031](docs/adr-0031-physical-copy-page-sync.md) physical pages · [0032](docs/adr-0032-openrouter-llm-for-taste.md) OpenRouter · [0033](docs/adr-0033-shared-family-movies-list-and-taste.md) sharing · [0034](docs/adr-0034-vite-spa-served-by-the-api.md) SPA · [0035](docs/adr-0035-trains-on-irail.md) iRail · [0036](docs/adr-0036-kobo-gateway-native-appkit-menu-bar.md) kobo AppKit · [0037](docs/adr-0037-fosite-successor.md) fosite · [0038](docs/adr-0038-downstream-cross-schema-reads.md) cross-schema · [0039](docs/adr-0039-agent-accounts.md) agent accounts · [0040](docs/adr-0040-factory-mcp-tools.md) board MCP tools
- **Specs:** [agent routines on Actions](docs/spec-agent-routines-on-actions.md)
- **Conventions:** [concise-docs-and-comments](docs/convention-concise-docs-and-comments.md) · [database-queries](docs/convention-database-queries.md) · [task-lifecycle](docs/convention-task-lifecycle.md) · [deploy-secrets](docs/convention-deploy-secrets.md) · [feature-review-policy](docs/convention-feature-review-policy.md) · [mcp-gap-first](docs/convention-mcp-gap-first.md) · [ui-standards](docs/convention-ui-standards.md) · [unattended-agent-trust](docs/convention-unattended-agent-trust.md)
- Host-layer decisions: [`infra/README.md`](infra/README.md).
