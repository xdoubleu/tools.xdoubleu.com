# AGENTS.md

The repository contract for any coding agent working in this repo — Claude
Code, OpenCode, or anything else. Harness-specific orchestration lives
elsewhere: `CLAUDE.md` (Claude Code) and `opencode.json` /
`.opencode/plugins/` (OpenCode) both build on top of this file rather than
repeating it. If you're looking for skill/hook/command mechanics, this is
the wrong file.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 / React 19 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`. A separate macOS-only Go module, `kobo-gateway/`, ships as a downloadable menu-bar helper (own `CLAUDE.md`). `sentrytools/` is a tiny third Go module holding slog→Sentry glue `api` pulls in via a local `replace` directive → [`docs/adr-0009-sentrytools-extracted-module.md`](docs/adr-0009-sentrytools-extracted-module.md).

Apps: **games**, **books** (Go package `apps/books`, schema `books`, proto `books.v1`), **feeds**, **watchparty**, **recipes**, **mealplans**, **shoppinglist**, **dashboard** (no schema of its own), **trains** (SNCB/NMBS, schema `trains`, proto `trains.v1` — daily GTFS static ingest, a CSA journey planner, and a GTFS-Realtime poll job overlaying delays/cancellations/alerts; `web/app/trains` and `web/app/trains/[journeyId]` are its user-visible pages), **learningpaths** (agent-authorable learning curricula, schema `learningpaths`, proto `learningpaths.v1` — a `LearningPath` of ordered `Module`s of ordered `Item`s, scoped by `user_id` alone with no family sharing; `web/app/learningpaths` is its user-visible UI). All apps are registered in `api/cmd/api/apps.go` (implements the `App` interface: `Routes`, `ApplyMigrations`, `GetName`, `GetDisplayName`, `GetDomain`, `Start`) — **migrations run sequentially in registration order**, so schema dependencies between apps dictate the list order. **dashboard** owns the public Games and Reading dashboards and the share-token lifecycle, reaching other apps only through exported methods on their structs → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](docs/adr-0007-dashboard-app-owns-public-sharing.md).

Shared Go code lives in `api/internal/` (auth, config, encryption, family, observability, github, sentryapi, mailer, oauthconn, mcptools, repositories, safedial, testhelper). Any outbound fetch of a
**user-supplied URL** must go through `api/internal/safedial` — its dialer
refuses non-public IPs, which is what keeps books/feeds from being
turned into an SSRF pivot against the container's own network. Each app under `api/apps/<name>/` follows: `internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, and (where relevant) `pkg/`.

Read `api/AGENTS.md`, `web/AGENTS.md`, and `kobo-gateway/AGENTS.md` before
working in those subtrees — harness-neutral project knowledge (package
layout, per-module commands, testing quirks) in the standard nested location
both harnesses discover: Claude Code (≥2.1.277) loads a nested `AGENTS.md`
on demand when it opens a file in that subtree, and OpenCode is pointed at
the same files via `opencode.json`'s `instructions` array.

**Deploy shape:** `api` and `web` build their own images and deploy as two independent Kamal services behind one shared kamal-proxy instance and domain, routed by path prefix. `api` strips its own `/api` prefix in-process (`api/cmd/api/kamal_proxy_shim.go`) so `/.well-known/*` reaches it untouched; `web` is the catch-all. **Deploy order is web-then-api and is required, not preferred** → [`docs/adr-0001-two-service-kamal-deploy.md`](docs/adr-0001-two-service-kamal-deploy.md). A third Kamal service, `grafana` (`config/deploy.grafana.yml`), joined the same shared proxy/domain at `/grafana` — order relative to it doesn't matter, only web-then-api is fixed → [`docs/adr-0022-prometheus-grafana-metrics.md`](docs/adr-0022-prometheus-grafana-metrics.md).

**Long-request handler deadlines:** a handler that outlives the edge proxy's response timeout gets its connection reset before it writes a byte — no server log, no Sentry event. `deployLogsCtxTimeout` (`api/cmd/api/routes.go`) = 20s and `liveLogDeadline` (`api/internal/digitalocean/logs_live.go`) = 8s. Raising either means also raising `proxy.response_timeout` in `config/deploy.api.yml`; nothing tests this → [`docs/adr-0017-long-request-handler-deadlines.md`](docs/adr-0017-long-request-handler-deadlines.md).

## Production Safety Rules

- **Never push directly to `main`** — only merge PRs whose CI passed; `main` deploys on every push without re-testing.
- Any outbound fetch of a **user-supplied URL** must go through `api/internal/safedial`, never a bare `http.Client` — see above.
- Never skip, disable, or quarantine a test, and never bypass hooks/signing (`--no-verify`, `--no-gpg-sign`) to force a change through, unless a human explicitly asks for that specific exception.
- The deploy-secret list is declared in three places that must agree — `config/deploy.{api,web}.yml`'s `env.secret:`, `.kamal/secrets`, and each `Deploy <svc> via Kamal` step's `env:` block in `main.yml`. `make lint/kamal-secrets` checks this → [`docs/convention-deploy-secrets.md`](docs/convention-deploy-secrets.md). `infra/README.md` is the single source of truth for the full secrets list.
- A migration that changes the meaning of an existing column for already-existing rows needs a backfill, or an explicit comment saying why one isn't needed (`api/AGENTS.md`'s Testing Notes has the concrete case).

## Code Navigation and Conventions

**Prefer `ast-grep` over `grep`/`rg` for code searches** — it understands syntax trees, so results are exact (no false positives from comments/strings). Reserve `grep`/`rg` for non-code files (logs, configs, docs).

```bash
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript
ast-grep run --pattern '...' --lang go api/apps/recipes/   # scope to a subtree
```

`$NAME` matches one node, `$$$` matches zero or more, `$$` matches one complex expression. Once you have a concrete symbol, prefer your editor/tooling's go-to-definition and find-references over `ast-grep` when it's available — semantic resolution (gopls/typescript-language-server) handles interfaces, generics, and shadowing correctly, which pure syntax matching can't guarantee.

**Comments must describe current behavior, not history.** Never write a comment that references removed code, superseded architecture, or frames a landed change as still-pending — a stale claim actively misleads the next reader. If historical context genuinely explains *why* the current code looks the way it does, phrase it so it stays true regardless of when it's read ("replicating what X used to provide", never "X hasn't happened yet") → [`docs/convention-comments-describe-current-behavior.md`](docs/convention-comments-describe-current-behavior.md).

**Do not read `api/gen/`, `api/internal/mocks/`, `api/apps/*/internal/mocks/`, or `web/lib/gen/`** to discover field names, RPC signatures, or mock signatures — read the corresponding `.proto` file in `proto/` or the source interface instead; it's smaller and is the source of truth.

For noisy, multi-step, or bulk data-gathering (a grep sweep across many files, a log/CI trawl, open-ended codebase exploration for a research question) prefer delegating to a subagent/background task where your harness supports one, rather than pulling raw, mostly-discarded tool output into your own main context.

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
make arch/diagram           # Mermaid package-dependency diagram (godepgraph, auto-installed) → api/package-graph.mmd, not committed
make test/mutation/diff      # gremlins mutation testing scoped to Go packages changed vs origin/main
make test/mutation           # gremlins mutation testing, full repo (slow one-time baseline, not routine)
docker-compose down

# Web (from web/)
npm run dev
npm test                    # jest
npm run lint                 # eslint + tsc --noEmit + prettier --check + knip + syncpack
npm run lint:fix
npm run build                # required before finishing web tasks, see below
npm run test:mutation:diff    # StrykerJS mutation testing scoped to TS/TSX files changed vs origin/main
npm run test:mutation        # StrykerJS mutation testing, full repo (slow one-time baseline, not routine)
npm run generate             # regenerate lib/gen/ ConnectRPC clients from proto

# Kobo Gateway (from kobo-gateway/, macOS only — cgo + AppKit)
make build / make dist / make test / make lint/fix

# Infra (from the repo root; needs Docker + tofu)
make lint/infra             # validate infra/prometheus.yml + the OpenTofu config
make lint/grafana           # static check of the provisioned dashboard JSON
make grafana/verify         # boot the Grafana image, assert provisioning loads
make lint/workflows         # validate every .github/workflows/*.yml parses as YAML

# Proto (when any .proto file changes, run BOTH generators)
cd api && make proto/generate   # regenerates api/gen/
cd web && npm run generate      # regenerates web/lib/gen/
cd api && make proto/check      # regenerate + fail if that changed anything uncommitted (mirrors CI)
cd web && npm run generate:check
cd api && make proto/generate/local   # same output, without reaching buf.build — for environments that can't (e.g. Claude Code on the web)
cd web && npm run generate:local
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed. Both directories are fully excluded from every lint/fix tool in this repo, so there's no ordering dependency between regenerating and running lint/fix — run them in either order. `make proto/check` / `npm run generate:check` run the exact same regenerate-then-diff CI's proto-staleness check does, so use those to verify locally rather than reasoning about the exclusion config by hand. Run `make lint/proto` to also catch `buf lint` issues (e.g. RPC response types must be named `<Method>Response`) before pushing.

**Prefer the commands above over ad-hoc equivalents.** If a check/build/verification isn't covered by an existing `make`/`npm run` target, add one to the relevant `Makefile`/`package.json` rather than improvising it with raw tool invocations, and if an existing target doesn't do quite what's needed, fix the target itself.

Run a single Go test: `go test ./apps/books/internal/services/... -run TestName -v` (from `api/`). Single Jest test: `npx jest path/to/file.test.ts -t "test name"` (from `web/`).

## Task Lifecycle (git/worktree, issue tracking, shipping)

The full harness-neutral workflow contract — fresh branch off up-to-date
`main`, a tracking issue before the first edit, lint/coverage/build gates,
PR requirements, CI-green requirement — is written up once in
[`docs/convention-task-lifecycle.md`](docs/convention-task-lifecycle.md).
Read it before starting or shipping a change. Claude Code and OpenCode each
enforce and execute it through their own mechanism — `start-task`/
`finish-task` skills plus the enforcement hooks in `.claude/settings.json`
for Claude Code; the same skills (OpenCode reads `.claude/skills/` via
compatibility discovery, with their generic dependencies installed under
`.agents/skills/`) plus the `.opencode/plugins/repo-guard` plugin for
OpenCode. Those mechanisms are documented in `CLAUDE.md` and `README.md`'s
"Agent Infrastructure" section respectively, not here.

## MCP

`/apps/mcp` (`api/cmd/api/mcp_apps.go`) is a streamable-HTTP MCP server exposing each app's own read RPCs as `<app>_<rpc>` tools plus ~20 unprefixed admin observability tools (`prom_query`, `get_grafana_alerts`, `get_sentry_issues`, `get_failing_pull_requests`, etc.), so any MCP-capable coding agent can pull production domain data and system health in as context. App tools are gated by the caller's own per-app access and return only that user's data; observability tools require admin access. No per-app tool mutates except `learningpaths`' three write tools (agent-authored curricula is that app's core use case) → [`docs/adr-0023-learningpaths-mcp-write-tools.md`](docs/adr-0023-learningpaths-mcp-write-tools.md); four unprefixed tools are deliberate mutations (`resolve_sentry_issue`, `dismiss_security_alert`, `record_action`, `notify_slack`), all admin-gated.

Auth is first-party **MCP OAuth 2.1** — the api is both the resource server and, via its embedded `ory/fosite`-backed authorization server, the authorization server itself (RFC 7591 dynamic client registration, RFC 8414 metadata). Any MCP client that speaks OAuth 2.1 discovery + PKCE against a streamable-HTTP server can connect the same way, with no server-side change or per-client configuration needed — this is what makes the server usable from more than one harness. See `README.md`'s "Apps MCP server" section and [`docs/adr-0006-embedded-oauth21-authorization-server.md`](docs/adr-0006-embedded-oauth21-authorization-server.md) for the full flow; harness-specific connection instructions (`claude mcp add ...` vs. `opencode.json`'s `mcp.servers` block) live in `CLAUDE.md` / `opencode.json` respectively.

**MCP coverage gaps:** if a production issue has no MCP tool that surfaces it, or an existing tool returns wrong/incomplete data, fix that gap first (add/correct the tool) before investigating the issue itself — otherwise the same blind spot just recurs next time. Record the case in [`docs/convention-mcp-gap-first.md`](docs/convention-mcp-gap-first.md), which also lists the known open gaps.

## CI

`.github/workflows/main.yml` orchestrates reusable workflows (`proto-check`, `build-api`, `build-web`, `build-kobo-gateway`, `api-lint`, `web-lint`, `kobo-gateway-lint`, `api-test`, `web-test`, `kobo-gateway-test`) gated by a `changes` path filter. `kobo-gateway-*` jobs run on `macos-14` (cgo/AppKit). On PRs the full suite runs and **`ci-pass` is the required check**; on push to `main` the lint jobs don't re-run but the build jobs do, since they produce the deploy artifacts, and `deploy-kamal` then deploys.

The `api`/`web`/`kobo_gateway` filters exclude `**/*.md`, so a docs-only PR triggers none of the build/lint/test jobs and `ci-pass` skips Codecov entirely.

If `ci-pass` fails with "Timed out waiting for Codecov to report", Codecov's check-suite is stuck — push a new (even empty) commit; rerunning the job never helps (there is no API to rerequest another app's check-suite).

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update this file (and `CLAUDE.md`/`README.md` if the change is harness-specific or user-facing) in the same change.

**Keep documentation lean.** `docs/` holds long-term decisions (ADRs), specs, and
conventions only — never a prose description of how a subsystem currently works.
**The code is the spec**: behavior belongs in the code and its comments, where it
cannot drift. The rule of thumb: *imperative mood stays in this file (or a
harness adapter); past tense — what something used to be, what was tried and
rejected, which issue produced a rule — moves to `docs/`.* Register a new
document in both `docs/README.md` and the index below; that index is what makes
it discoverable, since only this file and `CLAUDE.md` load automatically for a
coding agent.

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
- [`adr-0011-slow-transaction-thresholds`](docs/adr-0011-slow-transaction-thresholds.md) — name-shape classification that used to back the weekly digest's slow-transaction section
- [`adr-0012-ubuntu-release-check-on-vps`](docs/adr-0012-ubuntu-release-check-on-vps.md) — the job that became a systemd timer
- [`adr-0013-diff-scoped-coverage`](docs/adr-0013-diff-scoped-coverage.md) — changed-line coverage and the signature fixup
- [`adr-0014-start-finish-task-enforcement`](docs/adr-0014-start-finish-task-enforcement.md) — Claude Code's three hooks enforcing the task lifecycle below
- [`adr-0015-kobo-gateway-separate-module-and-toolchain-pin`](docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) — never bump past Go 1.24.x alone
- [`adr-0016-kobo-gateway-loopback-tls-and-login-item`](docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md) — loopback HTTPS and LaunchAgents
- [`adr-0017-long-request-handler-deadlines`](docs/adr-0017-long-request-handler-deadlines.md) — deadlines vs the proxy ceiling
- [`adr-0018-completion-average-population`](docs/adr-0018-completion-average-population.md) — a delisted game counts unless a listed game took its achievements
- [`adr-0019-trains-in-memory-router-and-dual-gtfs-feeds`](docs/adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md) — router warmed off the request path; static and realtime feeds correlated by `(trip_short_name, service date)`, never `trip_id`
- [`adr-0020-ui-configured-alert-delivery-channel`](docs/adr-0020-ui-configured-alert-delivery-channel.md) — superseded; alerting moved to Grafana
- [`adr-0021-oauth-as-general-purpose-oidc-idp`](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md) — the embedded AS also issues OIDC ID tokens; the static Grafana SSO client and its admin-only role claim
- [`adr-0022-prometheus-grafana-metrics`](docs/adr-0022-prometheus-grafana-metrics.md) — Prometheus + Grafana metrics/alerting pipeline; the `prom_query`/`get_grafana_alerts` MCP tools
- [`adr-0023-learningpaths-mcp-write-tools`](docs/adr-0023-learningpaths-mcp-write-tools.md) — learningpaths' MCP tools are allowed to mutate, the one deliberate exception to "no per-app MCP tool is ever mutating," with an explicit non-precedent statement
- [`adr-0024-posthog-product-analytics`](docs/adr-0024-posthog-product-analytics.md) — PostHog Cloud (EU) product analytics + session replay, on by default for every family member with no consent gate

**Specs**

- [`spec-routine-red-pr-repair`](docs/spec-routine-red-pr-repair.md) — the morning claude.ai routine's exact prompt text and required connectors, for manual creation in the routines UI
- [`spec-routine-ready-issues-executor`](docs/spec-routine-ready-issues-executor.md) — the nightly claude.ai routine's exact prompt text and required connectors
- [`spec-routine-nightly-maintenance-sweep`](docs/spec-routine-nightly-maintenance-sweep.md) — the nightly claude.ai routine's exact prompt text and required connectors
- [`spec-routine-posthog-ux-discovery`](docs/spec-routine-posthog-ux-discovery.md) — the weekly claude.ai routine's exact prompt text and required connectors

**Conventions**

- [`convention-comments-describe-current-behavior`](docs/convention-comments-describe-current-behavior.md)
- [`convention-database-queries`](docs/convention-database-queries.md) — wide TEXT columns; allowed cross-schema read direction
- [`convention-task-lifecycle`](docs/convention-task-lifecycle.md) — the harness-neutral start-task/finish-task workflow contract, shared by Claude Code and OpenCode
- [`convention-deploy-secrets`](docs/convention-deploy-secrets.md) — the three lists that must agree
- [`convention-feature-review-policy`](docs/convention-feature-review-policy.md) — `feature`-labeled work skips human PR review behind an automated quality gate; `ci-pass` stays required either way
- [`convention-mcp-gap-first`](docs/convention-mcp-gap-first.md) — fix the tool before the incident; open gaps
- [`convention-ui-standards`](docs/convention-ui-standards.md) — UI rules, theming, the server/client import trap

Host-layer decisions stay in [`infra/README.md`](infra/README.md), the single
source of truth for the deploy-secret list.
