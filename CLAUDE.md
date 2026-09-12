# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Monorepo Overview

Go 1.26 backend (`api/`) serving multiple apps from a single binary, paired with a Next.js 16 / React 19 frontend (`web/`, standalone Node server). Apps share a single HTTP mux and expose ConnectRPC endpoints. Each app owns its own PostgreSQL schema; shared proto definitions live in `proto/`. A separate macOS-only Go module, `kobo-gateway/`, ships as a downloadable menu-bar helper (own `CLAUDE.md`). `sentrytools/` is a tiny third Go module holding slog→Sentry glue `api` pulls in via a local `replace` directive → [`docs/adr-0009-sentrytools-extracted-module.md`](docs/adr-0009-sentrytools-extracted-module.md).

Apps: **games**, **books** (Go package `apps/books`, schema `books`, proto `books.v1`), **feeds**, **watchparty**, **recipes**, **mealplans**, **shoppinglist**, **dashboard** (no schema of its own), **trains** (SNCB/NMBS, schema `trains`, proto `trains.v1` — daily GTFS static ingest, a CSA journey planner, and a GTFS-Realtime poll job overlaying delays/cancellations/alerts; `web/app/trains` and `web/app/trains/[journeyId]` are its user-visible pages). All apps are registered in `api/cmd/api/apps.go` (implements the `App` interface: `Routes`, `ApplyMigrations`, `GetName`, `GetDisplayName`, `GetDomain`, `Start`) — **migrations run sequentially in registration order**, so schema dependencies between apps dictate the list order. **dashboard** owns the public Games and Reading dashboards and the share-token lifecycle, reaching other apps only through exported methods on their structs → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](docs/adr-0007-dashboard-app-owns-public-sharing.md).

Shared Go code lives in `api/internal/` (auth, config, encryption, family, observability, github, sentryapi, mailer, oauthconn, mcptools, repositories, safedial, testhelper). Any outbound fetch of a
**user-supplied URL** must go through `api/internal/safedial` — its dialer
refuses non-public IPs, which is what keeps books/feeds from being
turned into an SSRF pivot against the container's own network. Each app under `api/apps/<name>/` follows: `internal/{models,repositories,services,jobs,helper,mocks}`, `migrations/`, and (where relevant) `pkg/`.

**Deploy shape:** `api` and `web` build their own images and deploy as two independent Kamal services behind one shared kamal-proxy instance and domain, routed by path prefix. `api` strips its own `/api` prefix in-process (`api/cmd/api/kamal_proxy_shim.go`) so `/.well-known/*` reaches it untouched; `web` is the catch-all. **Deploy order is web-then-api and is required, not preferred** → [`docs/adr-0001-two-service-kamal-deploy.md`](docs/adr-0001-two-service-kamal-deploy.md). A third Kamal service, `grafana` (`config/deploy.grafana.yml`), joined the same shared proxy/domain at `/grafana` in issue #1468 — order relative to it doesn't matter, only web-then-api is fixed → [`docs/adr-0022-prometheus-grafana-metrics.md`](docs/adr-0022-prometheus-grafana-metrics.md). Grafana deploys a thin wrapper image (`infra/grafana.Dockerfile`) that bakes in the Prometheus datasource + dashboards from `infra/grafana/` (issue #1527), plus the `grafana-github-datasource` / `grafana-sentry-datasource` backend plugins and their provisioned datasources (issue #1570 — Sentry unresolved-issues alerting queries the Sentry API directly through the plugin, GitHub signals stay Prometheus gauges for now) — the dashboard JSON is the source of truth (`allowUiUpdates:false`), checked by `make lint/grafana` (static JSON) and `make grafana/verify` (boots the image, asserts provisioning loaded), both also run by `build-grafana.yml`; a change under `infra/grafana/` rebuilds the image via `main.yml`'s `grafana_dockerfile` path filter. `make lint/infra` is the equivalent pre-merge gate for `infra/prometheus.yml` and the OpenTofu config (`main.yml`'s `infra-lint` job, issue #1561) — necessary because `infra-apply` runs only after merge and would not fail on a malformed `prometheus.yml` anyway. Grafana SSO is admin-only (the OIDC `role` claim in `api/internal/oauth2as/claims.go` is emitted only for admins) → [`docs/adr-0021-oauth-as-general-purpose-oidc-idp.md`](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md).

**Long-request handler deadlines:** a handler that outlives the edge proxy's response timeout gets its connection reset before it writes a byte — no server log, no Sentry event. `deployLogsCtxTimeout` (`api/cmd/api/routes.go`) = 20s and `liveLogDeadline` (`api/internal/digitalocean/logs_live.go`) = 8s. Raising either means also raising `proxy.response_timeout` in `config/deploy.api.yml`; nothing tests this → [`docs/adr-0017-long-request-handler-deadlines.md`](docs/adr-0017-long-request-handler-deadlines.md).

A largely read-only **MCP server** at `/apps/mcp` exposes each app's own read RPCs as `<app>_<rpc>` tools plus 17 unprefixed admin observability tools (including `prom_query(promql)` against Prometheus, issue #1468, and `get_grafana_alerts` for Grafana-managed alert-rule state, which never reaches Prometheus `ALERTS{}`, issue #1564), so a local Claude CLI can pull production domain data and system health as context. App tools are gated by the caller's own per-app access and return only that user's data; observability tools require admin. The `trains_*` tools are the one exception to "only that user's data" — still access-gated, but the SNCB/NMBS timetable and its realtime overlay are public, so every holder sees the same rows. `trains_get_journey_detail` reports only the *current* live state; the realtime feed is replaced wholesale every 30s and nothing is retained, so "why didn't it offer an alternative this morning?" stays unanswerable without a persisted-history feature (raised on epic #1388, out of scope for #1397). Two are deliberate mutations (`resolve_sentry_issue`, `dismiss_security_alert`); no per-app tool is ever mutating. Auth flow in [`docs/adr-0006-embedded-oauth21-authorization-server.md`](docs/adr-0006-embedded-oauth21-authorization-server.md) and `README.md`. The same embedded AS (`api/internal/oauth2as/`) also acts as an OIDC IdP — RS256 ID tokens, `openid`/`profile`/`email` scopes, confidential clients, a static Grafana SSO client with a role claim → [`docs/adr-0021-oauth-as-general-purpose-oidc-idp.md`](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md).

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

# Infra (from the repo root; needs Docker + tofu)
make lint/infra             # validate infra/prometheus.yml + the OpenTofu config
make lint/grafana           # static check of the provisioned dashboard JSON
make grafana/verify         # boot the Grafana image, assert provisioning loads

# Proto (when any .proto file changes, run BOTH generators)
cd api && make proto/generate   # regenerates api/gen/
cd web && npm run generate      # regenerates web/lib/gen/
cd api && make proto/check      # regenerate + fail if that changed anything uncommitted (mirrors CI)
cd web && npm run generate:check
cd api && make proto/generate/local   # same output, without reaching buf.build — for environments that can't (e.g. Claude Code on the web)
cd web && npm run generate:local
```

Generated stubs (`api/gen/`, `web/lib/gen/`) ARE committed. Both directories are fully excluded from every lint/fix tool in this repo (gci's `--skip-generated`, golangci-lint's `formatters.exclusions.paths: (^|/)gen/`, web's `.prettierignore` and eslint `ignores`), so there's no ordering dependency between regenerating and running lint/fix — run them in either order. `make proto/check` / `npm run generate:check` run the exact same regenerate-then-diff CI's proto-staleness check (`proto-check.yml`) does, so use those to verify locally rather than reasoning about the exclusion config by hand. Run `make lint/proto` to also catch `buf lint` issues (e.g. RPC response types must be named `<Method>Response`) before pushing.

**Prefer the commands above over ad-hoc equivalents.** If a check/build/verification isn't covered by an existing `make`/`npm run` target, add one to the relevant `Makefile`/`package.json` rather than improvising it with raw tool invocations, and if an existing target doesn't do quite what's needed, fix the target itself. This file documents *what* a command does and *why*, never *how* — re-deriving a command's mechanics in prose here is exactly what let a stale claim (a false ordering requirement between `make proto/generate` and `make lint/fix`) drift out of sync between this file and `api/CLAUDE.md` until `make proto/check`/`npm run generate:check` replaced both explanations with one command.

Run a single Go test: `go test ./apps/books/internal/services/... -run TestName -v` (from `api/`). Single Jest test: `npx jest path/to/file.test.ts -t "test name"` (from `web/`).

## Starting a Task

Before exploring, reading code, or making any change, use the `start-task` skill — it pulls latest `main`, creates a completely fresh worktree (never edit in the main checkout or reuse an existing branch/worktree), and creates/refines the GitHub tracking issue via `refine-issue` before the first edit.

**Exiting plan mode does not count as having started the task.** `permissions.defaultMode` is `plan`, so nearly every session here begins by planning — and an approved plan is not a substitute for `start-task`, which still runs before the first edit, with the (already-grilled, see below) plan recorded in the tracking issue's `## Plan` section.

**Grill the scope during planning, before `ExitPlanMode` — not after.** When a task goes through plan mode, invoke the `grilling` skill (the round-by-round design-tree interview) against the plan during Phase 3 (Review), and settle its whole frontier before calling `ExitPlanMode`. Record that grilling happened, and its settled decisions, in the Final Plan file written in Phase 4 — a fresh session or subagent resuming the work has no memory of the planning conversation and needs this to avoid re-grilling from scratch. `start-task`'s `refine-issue` step then transcribes that already-settled plan into the issue's `## Plan` section and states in one line that it was pre-grilled during planning — it does not re-run the round-by-round interview. `refine-issue`'s (and `issue-triage`'s/`refine-feature`'s) own full grilling step still applies in full whenever a task skips plan mode entirely, or when the plan changes materially after approval (a reopened issue, a new `## Plan` revision, scope discovered mid-implementation) — in those cases nothing was grilled yet, or what was grilled is now stale. This is unconditional in both directions: a plan-mode task cannot exit plan mode without a completed grilling round, and a non-plan-mode (or materially changed) task cannot skip `refine-issue`'s own grilling; there is no "looks trivial" escape hatch either way.

## Finishing a Task

Once a task's changes are complete, use the `finish-task` skill — it covers lint, coverage, the web build, opening the PR (including the auto-merge decision), watching CI to green, and the mandatory `session-retro` that follows.

**This is unconditional, and opening the PR is pre-authorized** — the user does not need to ask for one, and a branch that is merely committed and pushed is not a finished task. Should `finish-task` never load for any reason, the floor is still: lint the areas that changed, ≥80% coverage on changed code, `npm run build` for web changes, then a non-draft PR whose body closes the tracking issue with a keyword (`Fixes #123`), then watch CI to green.

An `ExitPlanMode` hook and a `Stop` hook in `.claude/settings.json` enforce both halves, and `make hooks/test` exercises them. **In a Claude Code on the web session nothing external enforces this** — opening the PR unprompted is the only guardrail left there → [`docs/adr-0014-start-finish-task-enforcement.md`](docs/adr-0014-start-finish-task-enforcement.md).

When a change adds or alters a page or component under `web/`, run the `mobile-review` skill before `finish-task` — `docs/convention-ui-standards.md`'s mobile-first rule is review-only, so nothing else in the pipeline looks at a 375px viewport.

`start-task`/`finish-task` are thin, project-specific wrappers around generic skills (`task-worktree`, `ship-pr`, `session-retro`, `refine-issue`, `issue-triage`) published from the `xdoubleu/xdoubleu-claude-plugins` marketplace repo — declared in `.claude/settings.json`'s `extraKnownMarketplaces`/`enabledPlugins`. `refine-issue`/`issue-triage`'s repo/project-board/label config lives in `.claude/github-triage.config.json`, not in the skill files — edit that file, not the plugin, when this repo's board/labels change.

## CI

`.github/workflows/main.yml` orchestrates reusable workflows (`proto-check`, `build-api`, `build-web`, `build-kobo-gateway`, `api-lint`, `web-lint`, `kobo-gateway-lint`, `api-test`, `web-test`, `kobo-gateway-test`) gated by a `changes` path filter. `kobo-gateway-*` jobs run on `macos-14` (cgo/AppKit). On PRs the full suite runs and **`ci-pass` is the required check**; on push to `main` the lint jobs don't re-run but the build jobs do, since they produce the deploy artifacts, and `deploy-kamal` then deploys.

The `api`/`web`/`kobo_gateway` filters exclude `**/*.md`, so a docs-only PR triggers none of the build/lint/test jobs and `ci-pass` skips Codecov entirely.

**Because `main` deploys without re-testing, never push directly to `main`** — only merge PRs whose CI passed. When editing any `.github/workflows/*.yml`, ensure its own `pull_request` trigger includes `.github/workflows/**` in its `paths` filter (docker-build workflows are the deliberate exception — push-to-main only).

**Never add a cache or artifact write to a workflow reachable from `pull_request` or `workflow_dispatch`.** kobo-gateway builds on `macos-14` and reaches `build-web.yml` as a workflow-run artifact; only `save-kobo-gateway-cache.yml` ever writes its cache → [`docs/adr-0002-kobo-gateway-ci-cache-split.md`](docs/adr-0002-kobo-gateway-ci-cache-split.md). The `kobo_gateway` path filter feeds `build-kobo-gateway` and `build-web`'s own gate — keep it in sync if `kobo-gateway/` moves. `build-web.yml` **cannot** be dispatched standalone; use `main.yml`'s own `workflow_dispatch`.

Images are cached via BuildKit's `type=gha,scope=<component>` — **no hand-computed `hashFiles()` cache key anywhere in this path** → [`docs/adr-0003-buildkit-gha-cache-over-hashfiles.md`](docs/adr-0003-buildkit-gha-cache-over-hashfiles.md). `RELEASE` is a runtime container `ENV` for `api`/`web`; only `kobo-gateway` is compile-stamped, so its bundled release can legitimately lag the deploy → [`docs/adr-0004-runtime-release-env-vs-compile-stamp.md`](docs/adr-0004-runtime-release-env-vs-compile-stamp.md).

**The deploy-secret list is declared in three places that must agree** — `config/deploy.{api,web}.yml`'s `env.secret:`, `.kamal/secrets`, and each `Deploy <svc> via Kamal` step's `env:` block in `main.yml`. `make lint/kamal-secrets` fails the PR when they disagree; a mismatch otherwise only surfaces at deploy time on `main` → [`docs/convention-deploy-secrets.md`](docs/convention-deploy-secrets.md). `infra/README.md` is the single source of truth for the full secrets list.

`golangci-lint` runs across all three Go modules (`api`, `kobo-gateway`, `sentrytools`) off one shared **root** `.golangci.yml` — its config search walks up from the working directory. A change to `sentrytools/` can break `api` without touching `api/`'s subtree, so that path filter is OR'd into `api`'s own gate → [`docs/adr-0009-sentrytools-extracted-module.md`](docs/adr-0009-sentrytools-extracted-module.md).

If `ci-pass` fails with "Timed out waiting for Codecov to report", Codecov's check-suite is stuck — push a new (even empty) commit; rerunning the job never helps (there is no API to rerequest another app's check-suite).

## Docs Impact

When a change touches project structure, packages, Make/npm targets, shared services, or architecture conventions, update this file and `README.md` in the same change.

**Keep documentation lean.** `docs/` holds long-term decisions (ADRs) and
conventions only — never a prose description of how a subsystem currently works.
**The code is the spec**: behavior belongs in the code and its comments, where it
cannot drift. The rule of thumb: *imperative mood stays in a `CLAUDE.md`; past
tense — what something used to be, what was tried and rejected, which issue
produced a rule — moves to `docs/`.* Register a new document in both
`docs/README.md` and the index below; that index is what makes it discoverable,
since only `CLAUDE.md` files load automatically.

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
- [`adr-0011-slow-transaction-thresholds`](docs/adr-0011-slow-transaction-thresholds.md) — name-shape classification for the `/monitoring` trending list + weekly digest; why WebSocket routes stay listed on purpose (the p95 *alert* moved to Grafana, #1528)
- [`adr-0012-ubuntu-release-check-on-vps`](docs/adr-0012-ubuntu-release-check-on-vps.md) — the job that became a systemd timer
- [`adr-0013-diff-scoped-coverage`](docs/adr-0013-diff-scoped-coverage.md) — changed-line coverage and the signature fixup
- [`adr-0014-start-finish-task-enforcement`](docs/adr-0014-start-finish-task-enforcement.md) — the two hooks and the web-session gap
- [`adr-0015-kobo-gateway-separate-module-and-toolchain-pin`](docs/adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) — never bump past Go 1.24.x alone
- [`adr-0016-kobo-gateway-loopback-tls-and-login-item`](docs/adr-0016-kobo-gateway-loopback-tls-and-login-item.md) — loopback HTTPS and LaunchAgents
- [`adr-0017-long-request-handler-deadlines`](docs/adr-0017-long-request-handler-deadlines.md) — deadlines vs the proxy ceiling
- [`adr-0018-completion-average-population`](docs/adr-0018-completion-average-population.md) — a delisted game counts unless a listed game took its achievements
- [`adr-0019-trains-in-memory-router-and-dual-gtfs-feeds`](docs/adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md) — router warmed off the request path; static and realtime feeds correlated by `(trip_short_name, service date)`, never `trip_id`
- [`adr-0020-ui-configured-alert-delivery-channel`](docs/adr-0020-ui-configured-alert-delivery-channel.md) — one global email/Slack switch for alerts; webhook URL DB-encrypted and UI-set, not a deploy secret; digests stay email-only. **Superseded by #1530** — alerting moved to Grafana, the fan-out lost its last producer, so `notifications` is email-only again (`slack` client, `notification_channel_config`, its RPC and UI all removed)
- [`adr-0021-oauth-as-general-purpose-oidc-idp`](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md) — the embedded AS also issues OIDC ID tokens (RS256, `/oauth2/jwks`) and supports confidential clients; the static Grafana SSO client and its admin-only role claim
- [`adr-0022-prometheus-grafana-metrics`](docs/adr-0022-prometheus-grafana-metrics.md) — Prometheus + Grafana replace the hand-rolled host/CI/storage metrics pipeline; the `prom_query` MCP tool; what got removed and what was verified to stay; datasource + dashboards provisioned from `infra/grafana/` (#1527); Phase 2 (#1528) — api/web latency histograms, all alerting unified in Grafana provisioning, `ThresholdAlertJob`/`alert_states` retired; Phase 3 (#1529) — six GitHub/Sentry/R2 issue signals exported as Prometheus gauges by `IssueSignalCollectorJob` + a Grafana `service-health` alert group, `IssueNotifierJob`/`global.notified_issues` retired; Phase 4 (#1530) — with no fan-out producer left, notification delivery collapsed to email-only: `internal/slack`, `global.notification_channel_config`, the `UpdateNotificationChannel` RPC and the channel UI all removed, superseding adr-0020; Phase 5 (#1554) — `api`/`web` had **never** been scraped (the assumed Kamal network alias does not exist), so every Phase 2/3 metric was empty from the day it shipped; discovery is now label-based `docker_sd_configs`, `TargetDown`/`TargetMissing` replace the per-job down rules (`absent()` catches the missing-target case `up == 0` cannot), and `app-performance` restores per-route/per-job/Web-Vitals panels; Phase 6 (#1556) — `IssueSignalCollectorJob` now also exports `github_workflow_run_duration_seconds{workflow}` and `postgres_schema_size_bytes{schema}` (the two pre-Grafana overviews that had no metric), each with an `app-performance` panel and no alert; Phase 7 (#1564) — `get_grafana_alerts` MCP tool reads Grafana-managed alert-rule state (invisible to `prom_query` since those alerts never land in `ALERTS{}`) over Grafana's public URL with admin basic-auth, the same "no internal alias" constraint Phase 5 hit; Phase 8 (#1574) — `overview.json`/`api-runtime.json` panels wrap every raw per-`instance` expr in `min`/`max by (job)`, since Phase 5's docker_sd `instance` label is the Kamal container name and churns every deploy (a new Overview row per deploy otherwise); Phase 9 (#1570) — `grafana-github-datasource` + `grafana-sentry-datasource` plugins registered as provisioned datasources; only `sentry_unresolved_issues` migrated (its alert + panel query the Sentry API through the plugin, gauge + `collectSentryIssues` removed), every GitHub signal stays a gauge pending live-query validation (follow-up on #1570); two new Grafana deploy secrets `GRAFANA_{GITHUB,SENTRY}_DATASOURCE_TOKEN`; Phase 10 (#1580) — `service-health.json` had the same per-redeploy-churn bug Phase 8 fixed elsewhere, just missed (fixed the same way); the four broader dashboards split into nine single-domain ones (`overview`, `host`, `postgres`, `api`, `web`, `github`, `sentry`, `r2`, `grafana-prometheus`), `overview.json` rebuilt as a one-tile-per-rule mirror of `rules.yml` (dropping the dead `ALERTS{}`-based panel), and a new `grafana` Prometheus scrape job (`metrics_path: /grafana/metrics`, confirmed live) feeds Grafana's own health into the new `grafana-prometheus` dashboard; also confirmed the app's own GitHub/Sentry OAuth connections remain required — the datasource plugins are a separate read-only path that doesn't replace `/monitoring` or the MCP tools built on those clients; Phase 11 (#1592) — the alert contact point moved from email to a Grafana-native Slack Incoming Webhook per user preference (`contactpoints.yml`/`policies.yml`'s `email` receiver → `slack`, `GRAFANA_SLACK_WEBHOOK_URL` replacing `NOTIFY_EMAIL_TO` in `deploy.grafana.yml`'s secret list only — `NOTIFY_EMAIL_TO` itself stays required for `api`'s own mailer), reversing the Slack rejection Phase 2's Decision text recorded

**Conventions**

- [`convention-comments-describe-current-behavior`](docs/convention-comments-describe-current-behavior.md)
- [`convention-database-queries`](docs/convention-database-queries.md) — wide TEXT columns; allowed cross-schema read direction
- [`convention-deploy-secrets`](docs/convention-deploy-secrets.md) — the three lists that must agree
- [`convention-mcp-gap-first`](docs/convention-mcp-gap-first.md) — fix the tool before the incident; open gaps
- [`convention-ui-standards`](docs/convention-ui-standards.md) — UI rules, theming, the server/client import trap

Host-layer decisions stay in [`infra/README.md`](infra/README.md), the single
source of truth for the deploy-secret list.
