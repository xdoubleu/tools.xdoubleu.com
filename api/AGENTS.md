# api/ — Backend

Run all `make` commands from this directory. Shared commands are in the root `AGENTS.md`; the extra ones:

```bash
make test/v | make test/race
make test/cov/diff                 # coverage on lines changed vs origin/main (heuristic, see Testing)
make test/cov/per-pkg              # per-package coverage, merged
make lint/pkg PKG=apps/recipes     # lint one package
make lint/fix/pkg PKG=apps/recipes # auto-fix one package
make lint/golines                  # fails if golines would rewrite anything (CI runs it)
make migration/new app=<app> name=<name>  # app=<name>|platform/<name>|global
make lint/migrations               # duplicate version, or a new migration that isn't timestamp-versioned
make lint/kamal-secrets            # deploy-secret lists agree; deploy.grafana.yml env names must be GF_-prefixed (Grafana ignores others)
make lint/cross-schema             # SQL schema direction, adr-0038
make lint/proto-local-versions     # proto/generate/local plugin versions match buf.gen.yaml's `remote:` pins
make db/migrate                    # migrate an empty DB (else 42P01)
make db/reset                      # recreate the Postgres volume — shared by every worktree, see Testing
```

## Architecture

- `NewApps` (`cmd/api/apps.go`) registration order: `books` before `games` (games' last migration drops `backlog` after books adopted its tables); `dashboard` after apps it reads; `trains`/`learningpaths`/`movies`, then `taste` (reads apps; `showsmigrations` fill `podcasts`).
- `main.go` wraps the pgx pool in `postgres.NewSpanDB` (tracing); migrations use the raw pool. Platform handlers: `internal/platform/obs`, `internal/platform/account` (each `Deps`/`Register`).
- `ApplyMigrations` takes a Postgres advisory lock (concurrent replicas never race), runs global migrations (`cmd/api/migrations/`), then each app's.
- `ApplyMigrationsFromFS` (`internal/app/base.go`) runs goose with `WithAllowMissing()`, so out-of-order merges self-heal.

Per-app layout (`apps/<name>/`):

```
app.go         # embeds app.Base; books/games route test seeding through a test-only TestSeeds handle
routes.go      # ConnectRPC handler wrapped in the app's own AppAccess gate
connect_*.go   # service implementations
mcp.go         # optional MCPToolProvider
internal/{dtos,models,repositories,services,jobs,mocks}/
migrations/    # Goose SQL, own schema only
```

### Auth (`internal/platform/auth`)

First-party: HS256 JWT access tokens, opaque rotating refresh tokens, TOTP 2FA + recovery codes → [`adr-0005`](../docs/adr-0005-first-party-auth-replacing-gotrue.md).

- A per-token TTL cache (`AUTH_CACHE_TTL`, default 60s, `0` disables; tests use 0) fronts every resolution — **role/app-access changes lag up to the TTL**.
- SignOut/UpdatePassword/VerifyMFA/UnenrollTOTP evict the token. **Admin `SetRole`/`SetAppAccess` must call `InvalidateUserCache()`.**
- `enrichUser` overlays DB `Role`/`AppAccess`; **a DB failure is returned, never swallowed** — the unenriched user looks like "no access" and would be cached for the TTL.
- `GetCurrentUser` (`internal/platform/account/auth_handlers.go`) is the pattern for handlers needing DB-enriched attributes: `auth.GetUser`, then `appUsersRepo.GetByID`, preferring DB values when found.
- `constants.AuthChannelContextKey` is `browser` (cookies) or `mcp` (bearer); both yield the same `models.User`. A missing channel is not a browser.
- `ResolveToken` verifies session JWTs locally, then falls back to an injected `OAuth2TokenResolver` for `oauth2as` tokens → [`adr-0006`](../docs/adr-0006-embedded-oauth21-authorization-server.md).

### Shared packages (`internal/`)

- **`app`** — `Base`, `HTTPError`, `ScrubInternalErrors` (every `New*ServiceHandler` call must pass it). Unawaited request work uses `Base.Background.Go`, never a bare `go`.
- **`connecttools`** — `MapError`: DB/`HTTPError` errors → Connect codes.
- **`sentrytools`** — Sentry hub middleware, `GoRoutineWrapper`, slog→Sentry `Init`/`LogHandler` → [`adr-0009`](../docs/adr-0009-sentrytools-extracted-module.md).
- **`platform/family`** — the one sharing model; a user with no row is an implicit family-of-one (`EnsureFamily`). `InviteByEmail` requires a registered invitee and emails off the request path. **Leaving cannot un-merge family-scoped data** → [`adr-0008`](../docs/adr-0008-family-as-single-sharing-concept.md).
- **`crypto`** — AES-256-GCM `Sealer` for OAuth tokens at rest.
- **`mailer`** — Resend HTTP client; `Send`/`SendTo` return `ErrNotConfigured` when unset.
- **`github`, `sentryapi`** — read-only observability clients; resolve config from `global.oauth_connections` per call; return `ErrNotConfigured`/`ErrNotConnected` instead of failing.
- **`platform/oauthconn`** — token refresh + single-use CSRF `StateStore`. A `NULL` `config` means "connected, not configured".
- **`observability`** — `TrackedJob` (→ `global.job_runs`, `job_duration_seconds`; cancels a run's ctx after `DefaultRunDeadline` unless the job implements `Deadlined`, so jobs must honour ctx), `ObserveJobPhase`, `UsageRecorder` (→ `global.usage_daily`; `countingResponseWriter` in `cmd/api/usage_middleware.go` **must forward `Flush`/`Hijack`** or WebSocket upgrades break), and the cross-app jobs: weekly digest (feed sections only, each gated by `global.notification_settings`), transaction-latency snapshot, issue-signal collector (Prometheus gauges; Sentry via `sentryapi` because Grafana's Sentry datasource can't be alerted on). Never imports `apps/*` — `main.go` adapters bridge. Alerting is Grafana's → [`adr-0022`](../docs/adr-0022-prometheus-grafana-metrics.md), [`adr-0010`](../docs/adr-0010-two-weekly-digest-emails.md), [`adr-0012`](../docs/adr-0012-ubuntu-release-check-on-vps.md).
- **`notifications`** — email-only: `EnqueueEmail` (fixed recipient), `EnqueueTo` (arbitrary).
- **`repositories`** — shared repos over the `global` schema.
- **`safedial`** — `Client(timeout, maxRedirects, allowPrivate)` blocks non-public IPs at dial time (survives redirects and DNS rebinding); `allowPrivate` is on outside prod. `webfetch` (user-supplied URLs) also dials through it.
- **`httpclient`** — retrying client behind github, sentryapi, hardcover, unicat, steam; `SetBackoffBase` speeds up tests.
- **`mcptools`** — `RequireAppAccess`, `AddReadTool`/`AddWriteTool`, `Unwrap`/`Result`, `KeepRequestDeadline`/`BoundToolCalls`/`LogSlowRequests` (tool calls keep their deadline, feed `mcp_tool_duration_seconds`).
- **`testhelper`** — `ConnectTestDB`, `NewTestConfig`, `BuildMux`, `CreateRequestTester`.
- Job queue: `internal/kit/threading` + `internal/platform/jobqueue`.
- **`apps/internal/`** — app-only shared packages: `consumption` (`ListConsumption`/`Find*`/`Want*`), `progressws`/`progresshistory` (job-progress topics, carry-forward reads).

### Apps — non-obvious rules

- **books** — any byte change to the KEPUB pipeline files (list in `converter_version_check_test.go`), comments included, requires bumping `currentKEPUBConverterVersion`, which re-converts every book. A trigger pins `read` books at 100%.
- **feeds** — queries exclude `expired_at` items.
- **mealplans/shoppinglist/recipes** — family-scoped by `family_id`; shoppinglist stores stay per-user.
- **dashboard** — public services (`apps/dashboard/connect_public.go`) run **without auth middleware**; they resolve an opaque share token (`global.profile_shares`) and **must never read the user-context key**, then delegate to exported methods on the live app structs. Token management is in `DashboardService` behind `Access`, not `dashboard`'s `AppAccess`. `GetToday` (and MCP `dashboard_get_today`) gives a block per usable app → [`adr-0007`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).
- **trains** → [`adr-0035`](../docs/adr-0035-trains-on-irail.md):
  - Everything goes through `pkg/irail` (rate-limited, 429 retried once, 20 s cache); handlers never call iRail directly.
  - **Keep a whole-train cancellation (`cancelled`) distinct from a skipped stop (`skipped`)** — conflating them fakes cancellations.
  - Station ids are 9-digit UIC codes; a `journey_id` carries the iRail vehicle id, never a stored row id.
- **learningpaths** → [`adr-0023`](../docs/adr-0023-learningpaths-mcp-write-tools.md):
  - Scoped by `user_id`; another user's path reads as not-found.
  - Proto nests `LearningPath{modules{items}}`; Create/Update replace the whole tree; `RecordItemProgress` is the single-item exception.
  - MCP write tools are gated by `RequireAppAccess` and scope via context, never an argument. Authoring guidance lives in tool descriptions and `mcp_authoring_guide.go`.
  - Todoist: per-user `learningpaths.oauth_connections` (PK `user_id, provider`), separate from admin `global.oauth_connections`; `learningpaths/internal/todoist` is create+delete, idempotent — only the active module's items are tasks.
  - Linked books/feed items resolve through exported `Books`/`Feeds` methods, never their internals.

### Database

- Migrations: `apps/<name>/migrations/`; `global` schema in `cmd/api/migrations/`. **New files are UTC-timestamp versioned, via `make migration/new`** (never `000NN`); run `make lint/sql lint/migrations` before the gate.
- **No wide TEXT column in list queries or `RETURNING`** — select `<col> IS NOT NULL AND <col> <> ''`; only single-row reads fetch it → [`convention-database-queries`](../docs/convention-database-queries.md).
- Cross-schema SQL: `recipes ← mealplans ← shoppinglist` only → [`adr-0038`](../docs/adr-0038-downstream-cross-schema-reads.md).

### MCP

Apps opt in via `MCPToolProvider` (`cmd/api/apps.go`) in `apps/<name>/mcp.go`; shared gating in `internal/platform/mcptools`. Write tools need an ADR.

## Linting

- Config: repo-root `.golangci.yml` (shared with `kobo-gateway/`). Limits: line length 88, `gci` order standard → default → `prefix(tools.xdoubleu.com)`, `funlen` 100 lines/50 statements, `cyclop` 30. `nolintlint` requires a reason except for `funlen`/`gocognit`/`lll`.
- `depguard`: `apps/<name>/internal/...` is app-private; services can't import Connect, repositories `net/http`.
- Run `make lint/fix` last; `GOLANGCI_LINT_CACHE` is per-checkout.

## Testing

- No DB mocking: integration tests hit real Postgres (17, as in production); other mocks: `internal/mocks/`.
- `make test/cov/diff` (`scripts/diff_coverage.sh`) prints a gate; CI runs it on PRs → [`adr-0013`](../docs/adr-0013-diff-scoped-coverage.md). Offline it uses local `main`. Delete wrappers left unused.
- **One Postgres container (`api-db-1`) is shared by every worktree.** `docker-compose down` and `make db/reset` hit all sessions; never stop it. `relation ... does not exist` may be leftover state, teardown, or (CI only) a `global.*` table a test assumed: `cmd/api` creates them and package order isn't fixed; fake the interface or create it (`feeds` does).
- **`relation "X" already exists` panic** = another worktree's DDL ran without its `goose_db_version` row (in `global`; check via `docker exec api-db-1 psql`). If the schema matches, insert the row.
- **Concurrent `go test` from two worktrees races on fixtures**, e.g. `TestAppsMCPCallAllToolsAsAdmin`. Re-run alone first.
- No Docker? Try `.claude/hooks/session-start.sh`, or in a sandbox `scripts/sandbox_postgres.sh`, then `make db/migrate`. Modules resolve offline; build before reporting a gap.
- Bug fixes: failing test first. Reproduce production state via the MCP read tools; commit external input's **real bytes** as gzipped `testdata/` from the app's own client (example: `apps/feeds/internal/services/scrape_live_internal_test.go`).
- A bug closes on production observation or user confirmation, not a green suite.
- `git stash` is repo-global: never blind-`pop`.
- Backfill migrations: test against a row built to predate the migration, not only fresh rows.
- Kobo firmware can't be verified server-side: say so in the PR; the user's retest is acceptance.

- `go: no such tool "covdata"`: sandbox artifact; rerun `.claude/hooks/session-start.sh`.

`ROLE=workers` runs only `StartWorkers()` jobs; `heapWatchThreshold` stays under deploy caps.

## File size

Go files projected over ~300 lines need a split: tests by feature/handler group, source by concern.
