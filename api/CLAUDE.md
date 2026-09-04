# api/ — Backend

Go 1.26 backend for tools.xdoubleu.com. Run all `make` commands from this directory.

## Commands

```bash
docker-compose up -d              # start local Postgres — always before running/testing
docker-compose down

make run                          # go run ./cmd/api
make build                        # go build ./cmd/api → ./bin/api
make test                         # go test -p 1 ./...
make test/v                       # verbose
make test/race                    # with race detector
make test/cov/report               # coverage report (HTML, excludes mocks/gen)
make test/cov/diff                  # coverage on changed lines only, vs origin/main
make test/cov/per-pkg              # per-package coverage, merged
make lint                          # golangci-lint + sqlfluff + buf lint + lint/migrations + lint/kamal-secrets
make lint/migrations               # fail on two migrations sharing a version number (goose skips the duplicate silently), or a new migration numbered below the existing max in its directory
make lint/kamal-secrets            # fail if a name in config/deploy.{api,web}.yml's env.secret: is missing from .kamal/secrets or from main.yml's deploy-kamal env: block (issue #1405 — caught only at deploy time on main otherwise)
make lint/fix                      # golines + golangci-lint --fix + gci + sqlfluff fix + buf lint
make lint/pkg PKG=apps/recipes     # lint a single package
make proto/generate                # regenerate api/gen/ from proto/ (pair with `npm run generate` in web/)
make proto/check                   # regenerate + fail if that changed anything uncommitted (what CI's proto-staleness check does)
make lint/proto                    # buf lint — also part of make lint / lint/fix

go test ./apps/books/... -run TestFunctionName   # single test
```

## Architecture

All apps are registered in `cmd/api/apps.go` and share one HTTP mux routed by
URL prefix. `main.go` wraps the shared pgx pool in `postgres.NewSpanDB` once
(so every app's queries emit tracing spans; migrations use the raw pool
instead) and calls `NewApps`, which registers 9 apps in a fixed order —
`books` before `games`, because `games`' final migration drops the
leftover `backlog` schema only after `books` has adopted its tables out of
it. `trains` depends on no other schema and appends last (after `dashboard`).
Apps expose ConnectRPC endpoints consumed by `web/`.

The `App` interface (`cmd/api/apps.go`):

```go
type App interface {
    Routes(prefix string, mux *http.ServeMux)
    ApplyMigrations(ctx context.Context, db *pgxpool.Pool) error
    GetName() string
    GetDisplayName() string
    GetDomain() string
    Start() error
}
```

`ApplyMigrations` (`main.go`) takes a Postgres advisory lock before running
any migration, so concurrently-starting replicas never race each other, then
runs global migrations (embedded `cmd/api/migrations/*.sql`) before handing
off to each app's own. `apps/backlog/` is dead — only stale `coverage.out`
artifacts remain there, no `.go` source and nothing imports it; the name
survives only in a migration-ordering comment.

### App Structure

Each app lives in `apps/<name>/`:

```
apps/<name>/
├── app.go              # struct embedding app.Base (logger/config/auth), implements App
│                       # games/books/watchparty export Services/Repositories
│                       # (not private) so integration tests can seed data through
│                       # the real service layer
├── routes.go           # registers the ConnectRPC handler, wrapped in the app's own
│                       # AppAccess gate (per-app, not global admin-only)
├── connect_*.go         # ConnectRPC service implementations (split by concern in larger apps)
├── mcp.go               # optional: implements MCPToolProvider, wraps the app's own read RPCs
├── internal/
│   ├── dtos/           # request/response serialization
│   ├── models/         # domain models
│   ├── repositories/   # DB access (pgx/v5)
│   ├── services/       # business logic
│   ├── jobs/           # background jobs, if any
│   └── mocks/          # mocks for the above
└── migrations/         # Goose SQL migrations (per-app schema)
```

### Auth (`internal/auth`)

First-party — no external Auth provider. `Service`/`LocalService` in
`internal/auth/service.go`, backed by `api`'s own `auth` Postgres schema:
self-issued HS256 JWT access tokens verified locally, opaque refresh tokens
rotated on every use, TOTP 2FA and single-use recovery codes → [`docs/adr-0005-first-party-auth-replacing-gotrue.md`](../docs/adr-0005-first-party-auth-replacing-gotrue.md).

Rules when touching a handler or anything that mutates a user:

- A per-token TTL cache (`AUTH_CACHE_TTL` seconds, default 60, `0` disables — tests use 0 via `testhelper.NewTestConfig`) sits in front of every resolution; a hit skips both enrichment queries, so **role/app-access changes can lag by up to the TTL**.
- Tokens are evicted on SignOut/UpdatePassword/VerifyMFA/UnenrollTOTP. **Anything mutating *another* session's role/app-access (admin `SetRole`/`SetAppAccess`) must call `InvalidateUserCache()` (clear-all) afterwards.**
- `enrichUser` overlays the DB-managed `Role`/`AppAccess` onto the raw auth user, which on its own always resolves to `RoleUser` with no app access. **A DB failure here is returned, not swallowed** — falling back to the unenriched user would be indistinguishable from "no access" to `AdminAccess`/`AppAccess`, and would be cached for the rest of the TTL instead of retrying next request.
- `GetCurrentUser` (`cmd/api/connect_auth_handlers.go`) is the two-layer pattern any Connect handler needing DB-enriched attributes should follow: resolve the session via `auth.GetUser`, then look up `appUsersRepo.GetByID` and prefer the DB role/app-access/display-name when that lookup succeeds, falling back to the bare auth-schema values otherwise.
- `ResolveToken` tries local session-JWT verification first, then falls back to an injected `OAuth2TokenResolver` for fosite-issued opaque tokens (avoids an import cycle with `internal/oauth2as`) → [`docs/adr-0006-embedded-oauth21-authorization-server.md`](../docs/adr-0006-embedded-oauth21-authorization-server.md).

### Shared Internal Packages (`internal/`)

- **`app`** — `Base` (logger/config/auth embedded into every app), `HTTPError`, `ScrubInternalErrors` (Connect interceptor logging CodeInternal/CodeUnknown and replacing the client-facing message — every `New*ServiceHandler` call must pass it).
- **`auth`** — see above.
- **`config`** — centralized config, loaded from `.env`/environment variables.
- **`connecttools`** — `MapError`, shared by any app's ConnectRPC handlers to translate `database.ErrResourceNotFound`/`ErrResourceConflict` and `iapp.HTTPError` into the matching Connect error code, so recipes/mealplans/shoppinglist (and any future app with the same DB/HTTPError-to-Connect mapping needs) don't each reimplement it.
- **`sentrytools`** — `Middleware` (Connect/HTTP request-scoped Sentry hub) and `GoRoutineWrapper` (background-goroutine Sentry tracing). The slog→Sentry `LogHandler` and startup `Init` live in the repo-root `sentrytools/` module instead → [`docs/adr-0009-sentrytools-extracted-module.md`](../docs/adr-0009-sentrytools-extracted-module.md).
- **`family`** — the single sharing concept: `global.families`/`global.family_members` (at most one family per user; a user with no row is an implicit family-of-one, lazily materialized by `FamilyRepository.EnsureFamily`; each row carries the member's own `display_name`) and `global.family_invites` (pending-only). `family.v1.FamilyService`: `GetFamily`/`InviteToFamily`/`AcceptFamilyInvite`/`DeclineFamilyInvite`/`SetFamilyDisplayName`/`LeaveFamily` (`web/app/family`). `InviteByEmail` requires the invitee to already be a registered user and emails them off the request path — a send failure is logged, never fails the request. recipes/mealplans/shoppinglist key their data by `family_id` via `repositories.FamilyRepository`. **Leaving a family cannot un-merge already-family-scoped data** → [`docs/adr-0008-family-as-single-sharing-concept.md`](../docs/adr-0008-family-as-single-sharing-concept.md).
- **`crypto`** — AES-256-GCM `Sealer`, used to encrypt OAuth tokens at rest.
- **`mailer`** — thin Resend HTTP client (no SDK). `Send` (fixed recipient) and `SendTo` (arbitrary recipient) share `ErrNotConfigured` degrade-gracefully semantics when the API key/from/to is unset.
- **`github`, `sentryapi`** — read-only external observability clients (failing PRs, open Dependabot/code-scanning/secret-scanning security alerts, unresolved Sentry issues). Each resolves its admin-picked identifier fresh on every call from `global.oauth_connections.config`, exposes a discovery method for the admin picker, and returns `ErrNotConfigured`/`ErrNotConnected` rather than failing when the provider isn't set up yet.
- **`oauthconn`** — shared plumbing for the admin-configurable OAuth connections (GitHub/Sentry): token refresh via `oauth2.Config.TokenSource`, single-use CSRF `StateStore` for the browser redirect leg. Tokens are stored encrypted in `global.oauth_connections`; a `NULL` `config` column means "connected but not yet configured" — a distinct degraded state each provider's fetch path checks.
- **`observability`** — `TrackedJob` (job timing/panic recovery → `global.job_runs`), `UsageRecorder` (per-endpoint request counts **and response bytes** → `global.usage_daily`; its `countingResponseWriter` in `cmd/api/usage_middleware.go` **must keep forwarding `Flush`/`Hijack`** or `progressws` WebSocket upgrades break), and the cross-app jobs registered on `main.go`'s own queue: issue notifier, weekly digest, threshold alerts, host-metrics, transaction-latency and workflow-run snapshots. **Every notification source checks `global.notification_settings` before notifying**, and skips without writing to `notified_issues` when disabled, so re-enabling picks the item back up. This package never imports an `apps/*` package — `main.go`'s adapters bridge them → [`docs/spec-observability-subsystem.md`](../docs/spec-observability-subsystem.md), [`adr-0010`](../docs/adr-0010-two-weekly-digest-emails.md), [`adr-0011`](../docs/adr-0011-slow-transaction-thresholds.md), [`adr-0012`](../docs/adr-0012-ubuntu-release-check-on-vps.md).
- **`progressws`** — WebSocket service broadcasting background-job progress ("X of N") keyed by job-ID topics.
- **`progresshistory`** — generic cumulative-progress storage with carry-forward reads (games/books progress graphs).
- **`repositories`** — shared DB repos over the `global` schema (users, families/family invites, the observability tables, `oauth_connections`, `profile_shares`).
- **`safedial`** — `Client(timeout, maxRedirects, allowPrivate)`, an `http.Client` whose `net.Dialer.Control` refuses to connect to non-public IPs (loopback, RFC1918, link-local incl. `169.254.169.254`, CGNAT, multicast). Any code fetching a **user-supplied URL** must build its client here: both apps' `pkg/webfetch` do. Blocking at dial time rather than validating the URL is what makes it survive redirects and DNS rebinding. `allowPrivate` (wired to `cfg.Env != config.ProdEnv`) keeps httptest-based tests and local development working.
- **`mcptools`** — `RequireAppAccess` (per-app MCP gate, mirrors `auth.AppAccess`), `AddReadTool`, `Unwrap`/`Result`.
- **`testhelper`** — `ConnectTestDB` for integration tests, `NewTestConfig` (auth cache TTL 0), `BuildMux` for a test handler from any `Routes`/`GetName` app, `CreateRequestTester` for exercising a handler over real HTTP.

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress/distribution, favourites, per-user Steam settings. Background sync job + WebSocket live updates. Schema `games` (adopted from the former `backlog` schema).
- **books** — book library and Kobo e-reader companion. Pure-Go PDF/HTML→EPUB conversion (no Calibre), dual metadata enrichment (UniCat + Hardcover). Serves the raw Kobo sync protocol. Background jobs + WebSocket live updates. Schema `books`.
- **feeds** — RSS/Atom and email-relay newsletter subscriptions, standalone from `books` since #734. Poll job + Resend inbound-email webhook. Schema `feeds`.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no jobs, own custom domain (`watchparty.xdoubleu.com`).
- **recipes** — recipe management: fraction parsing, iCal export. The recipe book is family-scoped (see `internal/family`). Schema `recipes`.
- **mealplans** — weekly meal planning with per-plan iCal feeds, family-scoped. Schema `mealplans` (its `plans` tables were adopted from `recipes` via `ALTER TABLE ... SET SCHEMA`).
- **shoppinglist** — custom items plus meal-plan ingredient aggregation, categories, store-ordered export, family-scoped. Stores themselves stay private per-user even when the rest of the list is shared. Schema `shoppinglist`.
- **dashboard** — centralizes the public Games and Reading (books+feeds) dashboards, both private/owner and public/shared views, plus the share-token lifecycle → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](../docs/adr-0007-dashboard-app-owns-public-sharing.md). No DB, no jobs, like `watchparty`; registers last in `apps.go` since it holds live references to the already-constructed `games`/`books`/`feeds` apps. See "Public Dashboard Sharing" below.
- **trains** — SNCB/NMBS timetable + CSA journey planning; realtime delay overlay is a later slice. Schema `trains`. `jobs.StaticImportJob` is a daily GTFS static import via `pkg/bmc`, swapped in atomically → [`docs/spec-trains-gtfs-ingest.md`](../docs/spec-trains-gtfs-ingest.md). `trains.v1.TrainService.SearchJourneys` routes over an in-memory Connection Scan Algorithm index (`pkg/csa`) built from a rolling window of the ingested timetable and rebuilt by `jobs.RouterRefreshJob` → [`docs/spec-trains-journey-search.md`](../docs/spec-trains-journey-search.md). Still no user-visible page of its own (that's #1388's slice 4). **`trip_id` is a daily-churning stopping-pattern variant — never persist it as a long-lived FK from user data or return it from `SearchJourneys`; group user-facing output by `trips.trip_short_name`.**

### Database Conventions

- Each app owns its own Postgres schema, migrated via Goose SQL files in `apps/<name>/migrations/`.
- Cross-cutting tables live in schema `global`, migrations embedded in `cmd/api/migrations/`.
- **Never put a wide TEXT column in a list query's column list.** The deployed database is reached over a transaction-mode pooler and billed per byte returned. Multi-row reads and `RETURNING` clauses select `<col> IS NOT NULL AND <col> <> ''` as a boolean; a dedicated single-row read is the only query selecting the column itself (`apps/feeds/internal/repositories/items.go`, `apps/books/internal/repositories/books_scan.go`). The same applies to any query whose result the caller throws away → [`docs/convention-database-queries.md`](../docs/convention-database-queries.md).
- The same rule bounds `trains`: `stop_times` (~0.8M rows) and `calendar_dates` (~1.07M rows) are large, so any list query over them must select only the columns it uses.
- Downstream apps may **read** an upstream app's schema directly in SQL. The allowed dependency direction is acyclic: `recipes ← mealplans ← shoppinglist`. **Reads only, never the reverse**, and each app's migrations touch only its own schema — grep downstream repositories before changing an upstream schema.
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking.

### Apps MCP Server

Every app's own read RPCs, plus 20 admin-gated observability tools, are exposed
to a local Claude CLI over a largely read-only MCP server at `/apps/mcp`
(`cmd/api/mcp_apps.go`). Apps opt in via `MCPToolProvider`
(`RegisterMCPTools(srv *mcp.Server)`, `cmd/api/apps.go`), each wrapping only its
**read** handlers in `apps/<name>/mcp.go` — **no per-app tool is ever mutating**.
Shared gating lives in `internal/mcptools`. Auth is MCP OAuth 2.1 with the api as
both resource server and authorization server → [`docs/spec-mcp-server.md`](../docs/spec-mcp-server.md), [`docs/adr-0006-embedded-oauth21-authorization-server.md`](../docs/adr-0006-embedded-oauth21-authorization-server.md).

### Public Dashboard Sharing

Owned entirely by the `dashboard` app. `dashboard.v1.PublicGamesDashboardService`
and `dashboard.v1.PublicReadingDashboardService`
(`apps/dashboard/connect_public.go`) are registered **without any auth
middleware** — every request carries an opaque share token
(`global.profile_shares`) that resolves to the owning user, so **public handlers
must never read the user-context key**. Each handler resolves the token, then
delegates to an exported method on the live `*games.Games`/`*books.Books`/
`*feeds.Feeds` reference rather than duplicating business logic. Token
management lives in `dashboard.v1.DashboardService` behind normal `Access`,
deliberately not gated by `dashboard`'s own `AppAccess` → [`docs/adr-0007-dashboard-app-owns-public-sharing.md`](../docs/adr-0007-dashboard-app-owns-public-sharing.md).

### Key Libraries

| Concern | Library |
| --- | --- |
| HTTP | `net/http` + `justinas/alice` |
| RPC | `connectrpc.com/connect` |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` |
| Auth | `golang.org/x/crypto/bcrypt` + `pquerna/otp` (TOTP) + `golang-jwt/jwt/v5` (sessions) + `ory/fosite` (embedded MCP OAuth 2.1 AS) |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `internal/threading` (WorkerPool) + `internal/jobqueue` (scheduling) |
| MCP | `modelcontextprotocol/go-sdk` |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

## Linting

`make lint` also runs two repo-consistency shell checks that aren't golangci-lint: `lint/migrations` (see Commands) and `lint/kamal-secrets` (`scripts/check_kamal_secrets.sh` — keeps the Kamal deploy-secret list in sync across `config/deploy.{api,web}.yml`, `.kamal/secrets`, and `main.yml`'s `deploy-kamal` env blocks; issue #1405).

`golangci-lint` (40+ linters), configured by the repo-root `.golangci.yml` — not `api/.golangci.yml`, which moved there so `gateway/` and `kobo-gateway/` share the exact same config (golangci-lint's config search walks up from the working directory to find it). Key constraints: max line length 88 (`golines`), import order standard → default → `prefix(tools.xdoubleu.com)` (`gci`), max function length 100 lines/50 statements (`funlen`), max cyclomatic complexity 30 (`cyclop`). `nolintlint` requires an explanation on every `//nolint` except `funlen`/`gocognit`/`lll`. Always run `make lint/fix` as the final step; fix anything the auto-fixer can't resolve manually. If a `.proto` file changed this session, order relative to `lint/fix` doesn't matter — run `make proto/check` (or `make proto/generate` in `api/` and `npm run generate` in `web/`) whenever it's convenient and commit the result; see root `CLAUDE.md`'s Commands section for why there's no ordering dependency. `GOLANGCI_LINT_CACHE` is set in the Makefile to a `.golangci-cache/` directory local to the checkout, so concurrent worktrees (which otherwise share the same Go module path and, by default, a single global cache) don't bleed each other's file paths into lint output.

## Testing Notes

- Mock injection for unit tests; mocks live in `internal/mocks/` or an app's own `internal/mocks/`.
- Integration tests hit a real database — `docker-compose up -d` before running locally.
- Target ≥80% coverage on changed code (`make test/cov/report`); generated files and `_mock.go` are excluded. `make test/cov/diff` (`tools/diff_coverage_go.py`) reports coverage on just the lines changed vs `origin/main`, approximating what CI's `codecov/patch` check gates on — run it before pushing to catch an uncovered branch locally instead of after a CI round trip. `make test/cov/per-pkg` merges per-package Go coverage profiles via the repo-root `tools/merge_coverage.py` — its web-side sibling, `tools/diff_coverage_ts.py`, does the equivalent lcov-based diff scoping for `web/` (`npm run test:cov:diff`).
- **`make test/cov/report` post-processes the profile before upload**, via `tools/extend_signature_coverage.py` — Go opens a function's first coverage block at its body's opening brace, so a `golines`-wrapped signature's parameter lines belong to no block and Codecov reports them as missed → [`docs/adr-0013-diff-scoped-coverage.md`](../docs/adr-0013-diff-scoped-coverage.md).
- Repeated local test runs against the same Postgres volume can leave state that breaks a later run with failures unrelated to what's actually being changed (e.g. `relation ... does not exist`, `resource conflicts with existing resource`) — CI never hits this since every job gets a fresh container. `make db/reset` recreates the volume; run it if a failure looks like leftover state rather than a real regression.
- **One Postgres container is shared by every worktree.** `docker-compose.yml` sets no `name:`, so Compose derives the project name from the `api/` directory — identical in every checkout — and binds a fixed host port, so all concurrent sessions resolve to the same `api-db-1`. `docker-compose down` (and `make db/reset`, which is `down -v` and destroys the shared *volume*) therefore hits every other session too, and the victim sees exactly the symptoms above in whatever suite it was running — indistinguishable from leftover state. Before reaching for `db/reset`, check `docker ps` for a container that's only been up seconds: a database that vanished mid-run is a concurrent teardown, not pollution. Don't stop the container when finishing a task (issue #1205).
- **A concurrent worktree applying migrations can also desync `goose_db_version` itself**, distinct from the stale-test-data symptom above: `make test`/`make run` panics with `relation "X" already exists` (or a missing column another worktree's session already added) instead of a normal test failure, because one worktree's migration DDL landed on the shared schema while its `goose_db_version` row never committed (e.g. the other session panicked mid-run). Diagnose with `docker exec api-db-1 psql -U postgres -d postgres -c "SET search_path=global; SELECT version_id, is_applied FROM goose_db_version ORDER BY version_id DESC LIMIT 8;"` (`postgres` is the default `DB_DSN` database) and compare against the migration file the panic names. If `\d global.<table>` shows the schema already matches that migration, the DDL already applied — insert the missing row rather than dropping/recreating anything: `docker exec api-db-1 psql -U postgres -d postgres -c "SET search_path=global; INSERT INTO goose_db_version (version_id, is_applied) VALUES (<n>, true);"`.
- Write a failing test first when fixing a bug.

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code — split `_test.go` by feature/handler group, source by concern (extract large string constants to a companion file).
