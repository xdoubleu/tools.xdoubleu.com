# api/ — Backend

Run all `make` commands from this directory. Shared commands are in the root `AGENTS.md`; the extra ones:

```bash
make test/v | make test/race
make test/cov/diff                 # coverage on lines changed vs origin/main (heuristic, see Testing)
make test/cov/per-pkg              # per-package coverage, merged
make lint/pkg PKG=apps/recipes     # lint one package
make lint/fix/pkg PKG=apps/recipes # auto-fix one package — prefer over repo-wide lint/fix, whose golines pass reformats unrelated files
make lint/migrations               # duplicate version (goose silently skips it) or a new one numbered below the dir's max
make lint/kamal-secrets            # deploy-secret lists agree; deploy.grafana.yml env names must be GF_-prefixed (Grafana ignores others)
make lint/proto-local-versions     # proto/generate/local plugin versions match buf.gen.yaml's `remote:` pins
make db/reset                      # recreate the Postgres volume — shared by every worktree, see Testing
```

## Architecture

- `NewApps` (`cmd/api/apps.go`) registers apps in a fixed order: `books` before `games` (games' last migration drops the leftover `backlog` schema after books adopted its tables); `dashboard` after `games`/`books`/`feeds` (holds live references to them); `trains`/`learningpaths` last.
- `main.go` wraps the pgx pool in `postgres.NewSpanDB` for tracing spans; migrations use the raw pool.
- `ApplyMigrations` takes a Postgres advisory lock (concurrent replicas never race), runs global migrations (`cmd/api/migrations/`), then each app's.
- `ApplyMigrationsFromFS` (`internal/app/base.go`) runs goose with `WithAllowMissing()`: stacked sibling PRs can merge migrations out of numeric order, and this makes that self-healing instead of a startup panic.

Per-app layout (`apps/<name>/`):

```
app.go         # embeds app.Base; games/books/watchparty export Services/Repositories for integration-test seeding
routes.go      # ConnectRPC handler wrapped in the app's own AppAccess gate
connect_*.go   # service implementations
mcp.go         # optional MCPToolProvider
internal/{dtos,models,repositories,services,jobs,mocks}/
migrations/    # Goose SQL, own schema only
```

### Auth (`internal/auth`)

First-party: HS256 JWT access tokens, opaque rotating refresh tokens, TOTP 2FA + recovery codes → [`adr-0005`](../docs/adr-0005-first-party-auth-replacing-gotrue.md).

- A per-token TTL cache (`AUTH_CACHE_TTL`, default 60s, `0` disables; tests use 0) fronts every resolution — **role/app-access changes lag up to the TTL**.
- SignOut/UpdatePassword/VerifyMFA/UnenrollTOTP evict the token. **Admin `SetRole`/`SetAppAccess` must call `InvalidateUserCache()`.**
- `enrichUser` overlays DB `Role`/`AppAccess`; **a DB failure is returned, never swallowed** — the unenriched user looks like "no access" and would be cached for the TTL.
- `GetCurrentUser` (`cmd/api/connect_auth_handlers.go`) is the pattern for handlers needing DB-enriched attributes: `auth.GetUser`, then `appUsersRepo.GetByID`, preferring DB values when found.
- `ResolveToken` verifies session JWTs locally, then falls back to an injected `OAuth2TokenResolver` for fosite tokens → [`adr-0006`](../docs/adr-0006-embedded-oauth21-authorization-server.md).

### Shared packages (`internal/`)

- **`app`** — `Base`, `HTTPError`, `ScrubInternalErrors` (every `New*ServiceHandler` call must pass it).
- **`connecttools`** — `MapError`: DB/`HTTPError` errors → Connect codes.
- **`sentrytools`** — request-scoped Sentry hub middleware, `GoRoutineWrapper`; slog handler/`Init` live in the root `sentrytools/` module → [`adr-0009`](../docs/adr-0009-sentrytools-extracted-module.md).
- **`family`** — the one sharing model; a user with no row is an implicit family-of-one (`EnsureFamily`). `InviteByEmail` requires a registered invitee and emails off the request path. **Leaving cannot un-merge family-scoped data** → [`adr-0008`](../docs/adr-0008-family-as-single-sharing-concept.md).
- **`crypto`** — AES-256-GCM `Sealer` for OAuth tokens at rest.
- **`mailer`** — Resend HTTP client; `Send`/`SendTo` return `ErrNotConfigured` when unset.
- **`github`, `sentryapi`** — read-only observability clients; resolve config from `global.oauth_connections` per call; return `ErrNotConfigured`/`ErrNotConnected` instead of failing.
- **`oauthconn`** — token refresh + single-use CSRF `StateStore`. A `NULL` `config` means "connected, not configured".
- **`observability`** — `TrackedJob` (→ `global.job_runs`, `job_duration_seconds`), `ObserveJobPhase`, `UsageRecorder` (→ `global.usage_daily`; `countingResponseWriter` in `cmd/api/usage_middleware.go` **must forward `Flush`/`Hijack`** or WebSocket upgrades break), and the cross-app jobs: weekly digest (feed sections only, each gated by `global.notification_settings`), transaction-latency snapshot, issue-signal collector (Prometheus gauges; Sentry via `sentryapi` because Grafana's Sentry datasource can't be alerted on). Never imports `apps/*` — `main.go` adapters bridge. Alerting is Grafana's → [`adr-0022`](../docs/adr-0022-prometheus-grafana-metrics.md), [`adr-0010`](../docs/adr-0010-two-weekly-digest-emails.md), [`adr-0012`](../docs/adr-0012-ubuntu-release-check-on-vps.md).
- **`notifications`** — email-only: `EnqueueEmail` (fixed recipient), `EnqueueTo` (arbitrary).
- **`progressws`** / **`progresshistory`** — job-progress WebSocket topics; cumulative progress with carry-forward reads.
- **`repositories`** — shared repos over the `global` schema.
- **`safedial`** — `Client(timeout, maxRedirects, allowPrivate)` blocks non-public IPs at dial time (survives redirects and DNS rebinding); `allowPrivate` is on outside prod.
- **`mcptools`** — `RequireAppAccess`, `AddReadTool`, `Unwrap`/`Result`.
- **`testhelper`** — `ConnectTestDB`, `NewTestConfig`, `BuildMux`, `CreateRequestTester`.
- Job queue: `internal/threading` + `internal/jobqueue`.

### Apps — non-obvious rules

- **books** — any byte change to the KEPUB pipeline files (list in `converter_version_check_test.go`), comments included, requires bumping `currentKEPUBConverterVersion`, which re-converts every book. Avoid cosmetic edits there.
- **watchparty** — no DB, own domain `watchparty.xdoubleu.com`.
- **mealplans/shoppinglist/recipes** — family-scoped by `family_id`; shoppinglist stores stay per-user.
- **dashboard** — public services (`apps/dashboard/connect_public.go`) run **without auth middleware**; they resolve an opaque share token (`global.profile_shares`) and **must never read the user-context key**, then delegate to exported methods on the live app structs. Token management is in `DashboardService` behind `Access`, not `dashboard`'s `AppAccess` → [`adr-0007`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).
- **trains** → [`adr-0019`](../docs/adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md):
  - `SearchJourneys` reads an in-memory CSA index built off the request path; while nil it returns `CodeUnavailable`, never a lazy build.
  - **An import that starts writing something new must bump `services.ImportParserVersion`** — conditional GETs otherwise pin old rows.
  - **Keep GTFS-RT trip `CANCELED` distinct from stop `SKIPPED`/`NO_DATA`** — conflating them fakes cancellations.
  - **Never persist, return, or join on `trip_id`** (churns daily); correlate by `(trip_short_name, service date)`.
- **learningpaths** → [`adr-0023`](../docs/adr-0023-learningpaths-mcp-write-tools.md):
  - Scoped by `user_id` only (no family); another user's path reads as not-found.
  - Proto uses nested `LearningPath{modules{items}}`; Create/Update round-trip the whole tree; `RecordItemProgress` is the single-item exception.
  - MCP write tools are gated by `RequireAppAccess` and scope via context, never an argument. Authoring guidance lives in tool descriptions and `mcp_authoring_guide.go`.
  - Todoist: per-user `learningpaths.oauth_connections` (PK `user_id, provider`), separate from admin `global.oauth_connections`; `internal/todoist` is create-only.
  - Linked books/feed items resolve through exported `Books`/`Feeds` methods, never their internals.

### Database

- Migrations: `apps/<name>/migrations/`; `global` schema in `cmd/api/migrations/`.
- **No wide TEXT column in list queries or `RETURNING`** — select `<col> IS NOT NULL AND <col> <> ''`; only single-row reads fetch it. Large trains tables (`stop_times`, `calendar_dates`) select only used columns → [`convention-database-queries`](../docs/convention-database-queries.md).
- Cross-schema reads only downstream: `recipes ← mealplans ← shoppinglist`. Grep downstream repos before changing an upstream schema.

### MCP

Apps opt in via `MCPToolProvider` (`cmd/api/apps.go`), wrapping only read handlers in `apps/<name>/mcp.go`; shared gating in `internal/mcptools`.

## Linting

- Config is the repo-root `.golangci.yml` (shared with `kobo-gateway/`). Limits: line length 88, `gci` order standard → default → `prefix(tools.xdoubleu.com)`, `funlen` 100 lines/50 statements, `cyclop` 30. `nolintlint` requires a reason except for `funlen`/`gocognit`/`lll`.
- `depguard`: nothing outside `apps/<name>/**` may import `apps/<name>/internal/...`.
- Run `make lint/fix` last. `GOLANGCI_LINT_CACHE` is per-checkout so worktrees don't bleed paths.

## Testing

- No DB mocking: integration tests hit real Postgres (18 in CI). Mocks live in `internal/mocks/`.
- `make test/cov/diff` (`tools/diff_coverage_go.py`) prints a primary (gate) and a conservative number per file; Codecov's patch figure usually lands between them — treat it as a heuristic.
- `make test/cov/report` runs `tools/extend_signature_coverage.py` so wrapped signatures aren't reported as missed → [`adr-0013`](../docs/adr-0013-diff-scoped-coverage.md).
- **One Postgres container (`api-db-1`) is shared by every worktree.** `docker-compose down` and `make db/reset` hit all sessions — never stop it when finishing. Failures like `relation ... does not exist` may be leftover state (`db/reset`) or a concurrent teardown (check `docker ps` uptime).
- **`relation "X" already exists` panic** = another worktree's DDL applied without its `goose_db_version` row. Check with `docker exec api-db-1 psql -U postgres -d postgres -c "SET search_path=global; SELECT version_id, is_applied FROM goose_db_version ORDER BY version_id DESC LIMIT 8;"`; if the schema already matches, insert the missing row (`INSERT INTO goose_db_version (version_id, is_applied) VALUES (<n>, true);`).
- **Concurrent `go test` runs from two worktrees race on shared fixtures** (`-p 1` is per-invocation), e.g. `TestAppsMCPCallAllToolsAsAdmin`, `TestListBooksInExactSources_Admin_ReturnsOverlapBook`. Re-run in isolation before calling it a regression.
- `git stash` is repo-global: never blind-`pop`; confirm your push created an entry and pop only that ref.
- No Docker? `.claude/hooks/session-start.sh` has a `pg_ctlcluster` fallback — try it before reporting tests as blocked.
- Bug fixes: failing test first. Reproduce production state via the MCP read tools; for external input, commit the **real bytes** as gzipped `testdata/` fetched with the app's own client (example: `apps/feeds/internal/services/scrape_live_internal_test.go`).
- A bug closes on a production observation or user confirmation, not a green suite.
- Backfill migrations: test against a row built to predate the migration, not only fresh rows.
- Kobo-firmware behavior can't be verified server-side — say so in the PR and treat the user's on-device retest as acceptance.

- `make test/cov/report` exiting 1 with `go: no such tool "covdata"` is a sandbox toolchain-download artifact (golang/go#75031), not a coverage failure; re-run `.claude/hooks/session-start.sh`, which builds it.

## File size

Go files projected over ~300 lines need a split: tests by feature/handler group, source by concern.
