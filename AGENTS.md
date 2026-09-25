# AGENTS.md

The repository contract for any coding agent. Harness mechanics live in `CLAUDE.md` (Claude Code) and `opencode.json` / `.opencode/plugins/` (OpenCode).

## Monorepo Overview

- `api/` — Go 1.26, one binary serving every app over ConnectRPC on a shared mux. Each app owns a PostgreSQL schema; protos live in `proto/`.
- `web/` — Next.js 16 / React 19, standalone Node server.
- `kobo-gateway/` — separate macOS-only Go module, a downloadable menu-bar helper.
- `sentrytools/` — small Go module (slog→Sentry) pulled into `api` via a local `replace` → [adr-0009](docs/adr-0009-sentrytools-extracted-module.md).

Apps: games, books, feeds, watchparty, recipes, mealplans, shoppinglist, dashboard (no schema; owns public dashboards and share tokens, reaching other apps only through their exported methods → [adr-0007](docs/adr-0007-dashboard-app-owns-public-sharing.md)), trains (SNCB GTFS static + realtime, CSA planner), learningpaths (agent-authored curricula, per-user, no family sharing). All are registered in `api/cmd/api/apps.go`; **migrations run in registration order**, so schema dependencies dictate the order.

Shared Go code: `api/internal/`. Each app: `api/apps/<name>/internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, optional `pkg/`.

Read `api/AGENTS.md`, `web/AGENTS.md`, `kobo-gateway/AGENTS.md` before working in those subtrees.

**Deploy:** `api` and `web` are two Kamal services behind one kamal-proxy, routed by path; `api` strips `/api` in-process (`api/cmd/api/kamal_proxy_shim.go`). **Deploy order web-then-api is required** → [adr-0001](docs/adr-0001-two-service-kamal-deploy.md). `grafana` is a third service at `/grafana` → [adr-0022](docs/adr-0022-prometheus-grafana-metrics.md).

**Handler deadlines:** a handler outliving kamal-proxy's response timeout is reset with no log or Sentry event. Keep long-request deadlines below `proxy.response_timeout` (`config/deploy.api.yml`) → [adr-0017](docs/adr-0017-long-request-handler-deadlines.md).

## Production Safety Rules

- Unattended agents act only on human-approved work: only a human moves an issue to Ready, and routine PRs never auto-merge → [convention-unattended-agent-trust](docs/convention-unattended-agent-trust.md).
- **Never push to `main`** — it deploys on every push without re-testing. Merge only PRs with green CI.
- Every outbound fetch of a **user-supplied URL** goes through `api/internal/safedial` (refuses non-public IPs; SSRF guard), never a bare `http.Client`.
- Never skip, disable, or quarantine a test, or bypass hooks/signing, unless a human asks for that specific exception.
- Deploy secrets are listed in three places that must agree (`make lint/kamal-secrets`) → [convention-deploy-secrets](docs/convention-deploy-secrets.md). `infra/README.md` holds the full list.
- A migration that changes an existing column's meaning needs a backfill, or a comment saying why none is needed.

## Code Conventions

- **Prefer `ast-grep` over `grep`/`rg` for code** (e.g. `ast-grep run --pattern 'func Name($$$) $$$' --lang go`); use grep/rg for non-code files. Prefer go-to-definition/find-references once you have a symbol.
- **Concise docs and comments:** say each thing once, don't overexplain, don't restate the code, describe current behavior only, no issue numbers or history inline. `make lint/docs` enforces budgets → [convention-concise-docs-and-comments](docs/convention-concise-docs-and-comments.md).
- **Don't read generated code** (`api/gen/`, `*/mocks/`, `web/lib/gen/`) — read the `.proto` or source interface.
- Delegate noisy bulk exploration (grep sweeps, log/CI trawls) to a subagent where the harness supports one.

## Commands

```bash
# api/ (needs `docker-compose up -d` for Postgres)
make run | test | build | lint | lint/fix | test/cov/report
make test/mutation/diff     # gremlins on packages changed vs origin/main
make arch/diagram           # Mermaid package graph → api/package-graph.mmd (not committed)
go test ./apps/books/internal/services/... -run TestName -v

# web/
npm run dev | test | lint | lint:fix | build   # build is required before finishing web work
npm run test:mutation:diff  # StrykerJS on files changed vs origin/main
npx jest path/to/file.test.ts -t "test name"

# kobo-gateway/ (macOS only)
make build | dist | test | lint/fix

# repo root
make lint/docs | lint/skills | lint/workflows | lint/infra | lint/grafana | grafana/verify | hooks/test

# proto changes: run BOTH generators (…/local variants avoid buf.build)
cd api && make proto/generate    # check: make proto/check
cd web && npm run generate       # check: npm run generate:check
```

Generated stubs are committed and excluded from all linters. If a check isn't covered by a `make`/`npm run` target, add or fix the target rather than improvising.

## Task Lifecycle

Fresh branch off up-to-date `main`, tracking issue before the first edit, lint/coverage/build gates, PR, CI green → [convention-task-lifecycle](docs/convention-task-lifecycle.md). Both harnesses run it through the `start-task`/`finish-task` skills.

## MCP

`/apps/mcp` (`api/cmd/api/mcp_apps.go`) exposes each app's read RPCs as `<app>_<rpc>` tools (caller's own data only) plus admin-only observability tools (`prom_query`, `get_grafana_alerts`, `get_sentry_issues`, …). Mutating tools: learningpaths' write tools ([adr-0023](docs/adr-0023-learningpaths-mcp-write-tools.md)) and `resolve_sentry_issue`, `dismiss_security_alert`, `record_action`, `notify_slack` (admin). Auth is first-party OAuth 2.1 with dynamic client registration → [adr-0006](docs/adr-0006-embedded-oauth21-authorization-server.md). Headless routines use client_credentials as the `service` role: observability tools only, no user data → [adr-0025](docs/adr-0025-machine-client-credentials-service-role.md).

**Gap first:** if no MCP tool surfaces a production issue, or one returns wrong data, fix the tool before the issue → [convention-mcp-gap-first](docs/convention-mcp-gap-first.md).

## CI

`.github/workflows/main.yml` runs reusable build/lint/test workflows behind a `changes` path filter (`**/*.md` excluded); kobo-gateway jobs run on macOS. **`ci-pass` is the required check.** Pushes to `main` rebuild and deploy but skip lint. If `ci-pass` times out waiting for Codecov, push a new commit — re-running never helps.

## Docs

When a change touches structure, packages, targets, shared services, or conventions, update this file in the same change. `docs/` holds ADRs, specs, and conventions only — the code is the spec for current behavior. Register new docs in `docs/README.md` and below.

- **ADRs:** [0001](docs/adr-0001-two-service-kamal-deploy.md) two-service deploy · [0002](docs/adr-0002-kobo-gateway-ci-cache-split.md) kobo CI cache · [0003](docs/adr-0003-buildkit-gha-cache-over-hashfiles.md) image cache · [0004](docs/adr-0004-runtime-release-env-vs-compile-stamp.md) `RELEASE` · [0005](docs/adr-0005-first-party-auth-replacing-gotrue.md) auth · [0006](docs/adr-0006-embedded-oauth21-authorization-server.md) OAuth AS · [0007](docs/adr-0007-dashboard-app-owns-public-sharing.md) dashboard · [0008](docs/adr-0008-family-as-single-sharing-concept.md) family sharing · [0009](docs/adr-0009-sentrytools-extracted-module.md) sentrytools · [0010](docs/adr-0010-two-weekly-digest-emails.md) digests · [0011](docs/adr-0011-slow-transaction-thresholds.md) slow transactions · [0012](docs/adr-0012-ubuntu-release-check-on-vps.md) Ubuntu check · [0013](docs/adr-0013-diff-scoped-coverage.md) diff coverage · [0014](docs/adr-0014-start-finish-task-enforcement.md) lifecycle hooks · [0015](docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) kobo toolchain pin · [0016](docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md) kobo TLS · [0017](docs/adr-0017-long-request-handler-deadlines.md) deadlines · [0018](docs/adr-0018-completion-average-population.md) completion average · [0019](docs/adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md) trains router · [0020](docs/adr-0020-ui-configured-alert-delivery-channel.md) alert channel (superseded) · [0021](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md) OIDC IdP · [0022](docs/adr-0022-prometheus-grafana-metrics.md) Prometheus/Grafana · [0023](docs/adr-0023-learningpaths-mcp-write-tools.md) learningpaths writes · [0024](docs/adr-0024-posthog-product-analytics.md) PostHog
- **Specs:** [agent routines on Actions](docs/spec-agent-routines-on-actions.md) · routine prompts — [red-pr-repair](docs/spec-routine-red-pr-repair.md) · [ready-issues-executor](docs/spec-routine-ready-issues-executor.md) · [nightly-maintenance-sweep](docs/spec-routine-nightly-maintenance-sweep.md) · [posthog-ux-discovery](docs/spec-routine-posthog-ux-discovery.md)
- **Conventions:** [concise-docs-and-comments](docs/convention-concise-docs-and-comments.md) · [database-queries](docs/convention-database-queries.md) · [task-lifecycle](docs/convention-task-lifecycle.md) · [deploy-secrets](docs/convention-deploy-secrets.md) · [feature-review-policy](docs/convention-feature-review-policy.md) · [mcp-gap-first](docs/convention-mcp-gap-first.md) · [ui-standards](docs/convention-ui-standards.md) · [unattended-agent-trust](docs/convention-unattended-agent-trust.md)
- Host-layer decisions: [`infra/README.md`](infra/README.md).
