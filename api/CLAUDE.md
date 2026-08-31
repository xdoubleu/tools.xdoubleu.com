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
make lint                          # golangci-lint + sqlfluff + buf lint + lint/migrations
make lint/migrations               # fail on two migrations sharing a version number (goose skips the duplicate silently), or a new migration numbered below the existing max in its directory
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
instead) and calls `NewApps`, which registers 8 apps in a fixed order —
`books` before `games`, because `games`' final migration drops the
leftover `backlog` schema only after `books` has adopted its tables out of
it. Apps expose ConnectRPC endpoints consumed by `web/`.

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

Auth is first-party as of issue #1039 — no external Auth provider. `Service`/
`LocalService` live in `internal/auth/service.go`, backed by `api`'s own
`auth` Postgres schema (`api/cmd/api/migrations/00017_auth_schema.sql`,
`00018_auth_oauth2.sql`), replacing the previous Supabase GoTrue-backed
implementation (`supabase-community/auth-go`, now removed). Key points:

- **Password auth**: bcrypt (`golang.org/x/crypto/bcrypt`) hashes stored in
  `auth.users`. `ForgotPassword`/`ResetPasswordWithToken` deliver a one-time
  reset link via `internal/mailer` (Resend), reusing its `ErrNotConfigured`
  degrade-gracefully semantics rather than adding a second email path.
- **Sessions**: self-issued HS256 JWT access tokens (`JWT_SECRET`, verified
  locally — no more network round trip) carrying `sub`/`aal`/`exp`, plus
  opaque refresh tokens stored SHA-256-hashed in `auth.refresh_tokens` and
  **rotated on every use** (old row deleted, new one inserted) so sign-out/
  password-change/MFA-unenroll can actually revoke a session — a stateless
  JWT alone can't be revoked without a blocklist.
- **2FA**: TOTP via `pquerna/otp` (`internal/auth/mfa.go`), secrets encrypted
  at rest via the existing `internal/crypto.Sealer`. `ChallengeMFA` is a thin
  stub returning a synthetic challenge ID purely to preserve the existing
  two-step `ChallengeMFA`→`VerifyMFA` call shape — pquerna/otp is stateless,
  unlike GoTrue's old server-side challenge object; `totp.Validate` is what
  actually verifies. **Recovery codes** (`auth.recovery_codes`, bcrypt-hashed,
  single-use) are net-new — GoTrue never had these — generated on first TOTP
  enrollment and via `RegenerateRecoveryCodes`.
- **MCP OAuth 2.1 authorization server**: see "Apps MCP Server" below —
  `ResolveToken` tries local session-JWT verification first, then falls back
  to an injected `OAuth2TokenResolver` for fosite-issued opaque tokens
  (avoids an import cycle between `internal/auth` and `internal/oauth2as`).
- A per-token TTL cache (`AUTH_CACHE_TTL` seconds, default 60, `0` disables — tests use 0 via `testhelper.NewTestConfig`) still sits in front of every resolution; a hit skips both enrichment queries, so role/app-access changes can lag by up to the TTL.
- `enrichUser` overlays the DB-managed `Role`/`AppAccess` onto the raw auth user, which on its own always resolves to `RoleUser` with no app access. A DB failure here is **returned, not swallowed** — silently falling back to the unenriched user would be indistinguishable from "no access" to `AdminAccess`/`AppAccess`, and would get cached for the rest of the TTL instead of retrying next request.
- Tokens are evicted on SignOut/UpdatePassword/VerifyMFA/UnenrollTOTP; anything mutating *another* session's role/app-access (admin `SetRole`/`SetAppAccess`) must call `InvalidateUserCache()` (clear-all) afterwards.
- `GetCurrentUser` (`cmd/api/connect_auth_handlers.go`) is the two-layer pattern any Connect handler needing DB-enriched attributes should follow: resolve the session via `auth.GetUser`, then look up `appUsersRepo.GetByID` and prefer the DB role/app-access/display-name when that lookup succeeds, falling back to the bare auth-schema values otherwise.
- The one-time GoTrue → first-party cutover (issue #1039) was fully automatic, not a manual runbook: migration `00017_auth_schema.sql` detected a GoTrue-shaped legacy `auth` schema (via `auth.instances`, a table name only GoTrue/Supabase ever created) and renamed it to `auth_gotrue_legacy` before creating the new tables; a since-removed `internal/legacyauth.Migrate` then copied existing users' bcrypt password hashes and verified TOTP factors across, idempotently, on every `api` boot until the schema was dropped. Once stable in production, `auth_gotrue_legacy` was dropped by `00019_drop_auth_gotrue_legacy.sql`. The `gotrue` container has been removed from `infra/` entirely — `api` never talks to it.

### Shared Internal Packages (`internal/`)

- **`app`** — `Base` (logger/config/auth embedded into every app), `HTTPError`, `ScrubInternalErrors` (Connect interceptor logging CodeInternal/CodeUnknown and replacing the client-facing message — every `New*ServiceHandler` call must pass it).
- **`auth`** — see above.
- **`config`** — centralized config, loaded from `.env`/environment variables.
- **`connecttools`** — `MapError`, shared by any app's ConnectRPC handlers to translate `database.ErrResourceNotFound`/`ErrResourceConflict` and `iapp.HTTPError` into the matching Connect error code, so recipes/mealplans/shoppinglist (and any future app with the same DB/HTTPError-to-Connect mapping needs) don't each reimplement it.
- **`sentrytools`** — `Middleware` (Connect/HTTP request-scoped Sentry hub) and `GoRoutineWrapper` (background-goroutine Sentry tracing). The slog→Sentry `LogHandler` and startup `Init` used to live here too but moved to the repo-root `sentrytools/` module (its own `go.mod`) once a second consumer needed the exact same byte-for-byte logic (issue #926) — see root `CLAUDE.md`'s CI section.
- **`contacts`** — contact management (editable display names) shared by recipes/mealplans/shoppinglist sharing. `AddByEmail` emails the recipient after the request is persisted; a send failure is logged, never fails the request.
- **`crypto`** — AES-256-GCM `Sealer`, used to encrypt OAuth tokens at rest.
- **`mailer`** — thin Resend HTTP client (no SDK). `Send` (fixed recipient) and `SendTo` (arbitrary recipient) share `ErrNotConfigured` degrade-gracefully semantics when the API key/from/to is unset.
- **`github`, `sentryapi`** — read-only external observability clients (failing PRs, open Dependabot/code-scanning/secret-scanning security alerts, unresolved Sentry issues). Each resolves its admin-picked identifier fresh on every call from `global.oauth_connections.config`, exposes a discovery method for the admin picker, and returns `ErrNotConfigured`/`ErrNotConnected` rather than failing when the provider isn't set up yet.
- **`oauthconn`** — shared plumbing for the admin-configurable OAuth connections (GitHub/Sentry): token refresh via `oauth2.Config.TokenSource`, single-use CSRF `StateStore` for the browser redirect leg. Tokens are stored encrypted in `global.oauth_connections`; a `NULL` `config` column means "connected but not yet configured" — a distinct degraded state each provider's fetch path checks.
- **`observability`** — `TrackedJob` (times/records every job run in `global.job_runs`, recovers panics, logs failures at Error so they reach Sentry), `UsageRecorder` (per-endpoint request counts **and response bytes**, flushed to `global.usage_daily`; the byte counter is fed by `cmd/api/usage_middleware.go`'s `countingResponseWriter`, which must keep forwarding `Flush`/`Hijack` or `progressws` WebSocket upgrades break. Bytes measure what left the api — a proxy for what it read out of Postgres, not a direct measure of database egress, but close enough on passthrough list endpoints to identify the culprit, which is what issue #1027 needed and didn't have), `jobs.IssueNotifierJob` (cross-app, registered directly on `main.go`'s own job queue since it isn't scoped to one app — polls Sentry/GitHub every 5 minutes and emails an admin the first time an issue/failing PR is seen, deduped via `global.notified_issues`; the GitHub half only alerts on failing PRs carrying the `dependencies` label, issue #915 — Renovate opens those, see root `renovate.json5`, and nobody is otherwise watching them, unlike a PR a user or a Claude Code session opened, which already has someone driving it to green; each source additionally checks `global.notification_settings` before notifying and skips without writing to `notified_issues` when disabled, so re-enabling picks the item back up, issue #1214), `jobs.TransactionLatencySnapshotJob` (also cross-app on `main.go`'s job queue — snapshots each transaction's p95 duration/request count from `sentryapi.Client.ListTransactionStats` into `global.transaction_latency_daily` once a day, issue #848; `repositories.TransactionLatencyRepository.Trends` compares a recent window against the prior one to flag endpoints/pages regressing, surfaced by `get_slow_transactions`), `jobs.ThresholdAlertJob`'s three slow-transaction rules (`slow_transaction_http_high`/`_job_high`/`_frontend_high`, issue #1310 — `classifyTransaction` in `threshold_alert.go` infers a transaction's class purely from its name shape, since Sentry project names are admin-configured free text: an HTTP-verb prefix is an api handler (threshold 5s, well under the 10s `httpWriteTimeout` in `main.go` that would otherwise kill the request first), a leading `/` or an embedded `.` is a frontend transaction (5s, typical page loads run 1-2s) — `slowTransactionExcluded` carves out `NextNodeServer.clientComponentLoading`, a Next.js-internal transaction that legitimately runs far longer than any real page load, from that class, and — since issue #1320 — also excludes any `GET .../api/progress` transaction (games' and books' `progressws` WebSocket-upgrade routes, `apps/games/routes.go`/`apps/books/routes.go`): sentryhttp's transaction span covers the whole handler call, which for an upgraded socket doesn't return until the connection closes, so its "duration" measures how long a client kept the tab open rather than request latency and would otherwise breach `slow_transaction_http_high` permanently — and everything else is a background job (60s, since some like the steam sync legitimately run tens of seconds), and `jobs.WeeklyDigestJob` (also cross-app on `main.go`'s job queue, `RunEvery = 7 days`, issue #1014 — unlike `IssueNotifierJob` it has no per-item dedup, so every run emails an admin a summary of everything still open: unresolved Sentry issues, failing `dependencies`-labeled PRs, and feeds currently failing to poll (`apps/feeds.Feeds.ListUnhealthy`, exposed the same way `BuildSummary` is), sending a short "no open issues" email when a source is enabled but has nothing to report, so a missing weekly email is itself a sign the job stopped running; each section is also omitted when its source is disabled in `global.notification_settings`, issue #1214, and the email is suppressed entirely — not sent as an empty digest — when every one of the three sources is disabled, issue #1253. `main.go`'s `feedsHealthAdapter` bridges `*feeds.Feeds` into the job's own `unhealthyFeedLister` interface so this `internal/` package never imports an `apps/*` package). A prior `jobs.UbuntuReleaseJob` (issue #1134) polled Canonical's meta-release feed and compared it against a hardcoded baseline constant that had to be bumped by hand after every real `do-release-upgrade` — nobody did, so it fired a stale/wrong alert. It was removed in favor of a systemd timer running locally on the VPS that checks `do-release-upgrade -c` directly and emails via Resend, so the check never depends on a hand-maintained constant and no external system needs to SSH into the box — see `infra/README.md`'s "Getting notified of a new Ubuntu LTS release" section. Also (issue #1040): `hostmetrics.go` (a hand-rolled Prometheus text-exposition parser, no client library — scrapes `node-exporter:9100/metrics` for CPU/memory/disk usage), `jobs.HostMetricsSnapshotJob` (also cross-app on `main.go`'s job queue, `RunEvery = 60s` — scrapes and inserts one `global.host_metric_samples` row, then prunes both that table and `global.log_entries` past a 30-day retention window), `LogRepoHandler` (a `slog.Handler` composed into `main.go`'s handler chain alongside `sentrytools.NewLogHandler`, teeing every one of api's own log records into `global.log_entries` in-process — `web`'s logs instead reach the same table over `POST /api/observability/logs`, a plain HTTP endpoint gated by the `OBSERVABILITY_INGEST_SECRET` shared-secret header since web holds no admin session to authenticate a Connect call with). Also (issue #1217): `jobs.WorkflowRunsSnapshotJob` (also cross-app on `main.go`'s job queue, `RunEvery = 5m`, matching `IssueNotifierJob`'s cadence — polls `github.Client.ListWorkflowRuns`, persists each newly-completed run plus its per-job breakdown into `global.workflow_run_samples`/`global.workflow_job_samples` so duration/failure history survives past `github.Client`'s own 45s in-memory cache, prunes both past a 90-day retention window, and — reusing the same `global.notified_issues` dedup `IssueNotifierJob` uses, and gated by `NotificationSourceFailingMainCI` in `global.notification_settings` like every other admin notification, issue #1253 — emails an admin the first time a run on `main` fails, since `main` deploys straight off a passing push with no re-test, so a failure there is always a genuine incident. `get_workflow_run_stats` (`cmd/api/connect_observability_external.go`) reports this history as aggregates only — main-branch failures (expected empty), and avg/p95 duration per workflow and per job — deliberately not another raw run list like `get_workflow_runs` already is).
- **`progressws`** — WebSocket service broadcasting background-job progress ("X of N") keyed by job-ID topics.
- **`progresshistory`** — generic cumulative-progress storage with carry-forward reads (games/books progress graphs).
- **`repositories`** — shared DB repos over the `global` schema (users, contacts, the observability tables, `oauth_connections`, `profile_shares`).
- **`safedial`** — `Client(timeout, maxRedirects, allowPrivate)`, an `http.Client` whose `net.Dialer.Control` refuses to connect to non-public IPs (loopback, RFC1918, link-local incl. `169.254.169.254`, CGNAT, multicast). Any code fetching a **user-supplied URL** must build its client here: both apps' `pkg/webfetch` do. Blocking at dial time rather than validating the URL is what makes it survive redirects and DNS rebinding. `allowPrivate` (wired to `cfg.Env != config.ProdEnv`) keeps httptest-based tests and local development working.
- **`mcptools`** — `RequireAppAccess` (per-app MCP gate, mirrors `auth.AppAccess`), `AddReadTool`, `Unwrap`/`Result`.
- **`testhelper`** — `ConnectTestDB` for integration tests, `NewTestConfig` (auth cache TTL 0), `BuildMux` for a test handler from any `Routes`/`GetName` app, `CreateRequestTester` for exercising a handler over real HTTP.

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress/distribution, favourites, per-user Steam settings. Background sync job + WebSocket live updates. Schema `games` (adopted from the former `backlog` schema).
- **books** — book library and Kobo e-reader companion. Pure-Go PDF/HTML→EPUB conversion (no Calibre), dual metadata enrichment (UniCat + Hardcover). Serves the raw Kobo sync protocol. Background jobs + WebSocket live updates. Schema `books`.
- **feeds** — RSS/Atom and email-relay newsletter subscriptions, standalone from `books` since issue #734. Poll job + Resend inbound-email webhook. Schema `feeds`.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no jobs, own custom domain (`watchparty.xdoubleu.com`).
- **recipes** — recipe management: fraction parsing, iCal export, whole-recipe-book sharing. Schema `recipes`.
- **mealplans** — weekly meal planning with per-plan iCal feeds and sharing. Schema `mealplans` (its `plans` tables were adopted from `recipes` via `ALTER TABLE ... SET SCHEMA`).
- **shoppinglist** — custom items plus meal-plan ingredient aggregation, categories, store-ordered export, sharing. Stores themselves stay private per-user even when the rest of the list is shared. Schema `shoppinglist`.
- **dashboard** — centralizes the public Games and Reading (books+feeds) dashboards, both private/owner and public/shared views, plus the share-token lifecycle (issue #737). No DB, no jobs, like `watchparty`; registers last in `apps.go` since it holds live references to the already-constructed `games`/`books`/`feeds` apps. See "Public Dashboard Sharing" below.

### Database Conventions

- Each app owns its own Postgres schema, migrated via Goose SQL files in `apps/<name>/migrations/`.
- Cross-cutting tables live in schema `global`, migrations embedded in `cmd/api/migrations/`.
- **Never put a wide TEXT column in a list query's column list.** The deployed database is reached over a transaction-mode pooler and billed per byte returned, so a page of rows carrying a large column is billed egress on every request. `feeds`' `itemColumns`/`itemListColumns` split (`apps/feeds/internal/repositories/items.go`) and `books`' `bookColumns` (`apps/books/internal/repositories/books_scan.go`) show the pattern: multi-row reads and `RETURNING` clauses select `<col> IS NOT NULL AND <col> <> ''` as a boolean, and a dedicated single-row read is the only query selecting the column itself. Getting this wrong on `feeds.items.content_html` exhausted the whole monthly egress quota and took the site down (issue #1027). The same applies to any query whose result the caller then throws away — a job that only needs to know which rows exist should select ids only, not the columns it's about to discard.
- Downstream apps may **read** an upstream app's schema directly in SQL instead of going through an internal API — the allowed dependency direction is acyclic: `recipes ← mealplans ← shoppinglist`. `mealplans` joins `recipes.recipes`; `shoppinglist`'s export/item-name-catalog features join both `mealplans.*` and `recipes.*`. Reads only, never the reverse direction, and each app's migrations touch only its own schema — grep downstream repositories before changing an upstream schema.
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking.

### Apps MCP Server

Every app's own read RPCs, plus admin observability, are exposed to a local
Claude CLI over a largely read-only MCP server at `/apps/mcp`
(`cmd/api/mcp_apps.go`). Apps opt in via the `MCPToolProvider` interface
(`RegisterMCPTools(srv *mcp.Server)`, `cmd/api/apps.go`) — each implementing
app has its own `apps/<name>/mcp.go` wrapping only its **read** Connect
handlers, so no per-app tool is ever mutating. `registerObservabilityMCPTools`
adds the 18 unprefixed admin-gated observability tools on top, sharing the
exact same internal methods the Connect handlers use — 17 are read, plus
`resolve_sentry_issue`, the one deliberate mutation (marks a Sentry issue
resolved via `sentryapi.Client.ResolveIssue`), letting an admin-authenticated
agent close out an issue it just filed a fix for. Auth is MCP OAuth 2.1,
entirely first-party as of issue #1039: the api is both the resource server
(`ResolveToken` verifies the bearer token) **and** the authorization server —
`internal/oauth2as` embeds `ory/fosite` (PKCE-enforced, RFC 7591 dynamic
client registration, RFC 8414 metadata at
`/.well-known/oauth-authorization-server`), wired up in `cmd/api/oauth2as.go`
and `cmd/api/mcp.go` (`mcpAuthServerIssuer` now points at `cfg.AuthIssuer`,
which defaults to `cfg.APIURL`, instead of a hardcoded Supabase Cloud URL).
`offline_access` is the only scope this server supports and the sole gate on
whether fosite issues a refresh token — so it's advertised in both metadata
documents and echoed in the RFC 7591 registration response, *and*
`AuthorizeHandler` grants it on top of whatever the client actually requested
(`grantOfflineAccess`, `internal/oauth2as/scopes.go`). The server-side grant is
what matters: MCP clients routinely send no `scope` parameter, and without it
they get an access token with no refresh token and a forced interactive
re-authentication every hour (issue #1177). Every rejection from
`/oauth2/authorize`, `/oauth2/token` and `/oauth2/register` is logged by
`internal/oauth2as/observe.go` — fosite writes its errors straight into the
HTTP response and never through slog, and `usage_daily` counts `oauth2/token`
hits regardless of outcome, so before this nothing server-side distinguished a
failed token exchange from a successful one and #1177 stayed invisible until a
user reported it. A **failed `refresh_token` grant logs at Error** (so the root
`sentrytools` `LogHandler` reports it) even though it's a 400: unlike any other
4xx there it takes a refresh token this server itself issued, so it always
means a working client just lost its session. Every other 4xx — expired code,
bad PKCE verifier, denied consent, scanners probing `/oauth2/*` — stays at Warn
so it can't bury that signal. Credentials must never be logged; the request
form carries the code, the PKCE verifier and the refresh token, and
`TestObserve_NeverLogsCredentials` is the guard. The web `/oauth/consent` page
drives the approval, calling the api's own `/oauth2/*` endpoints directly — no
external Auth provider involved. See root
`README.md` for the client-setup command.

### Public Dashboard Sharing

Public dashboard serving is owned entirely by the `dashboard` app (issue
#737) — `games`/`books` no longer have any public-sharing code of their
own. `dashboard.v1.PublicGamesDashboardService` and
`dashboard.v1.PublicReadingDashboardService` (`apps/dashboard/connect_public.go`)
are registered **without** any auth middleware — every request carries an
opaque share token (`global.profile_shares`, keyed by `(user_id, app)` where
`app` is now `'games'`/`'reading'`) that resolves to the owning user. Public
handlers must never read the user-context key, since no middleware sets it.
Each handler resolves the token, then delegates to an exported method on the
live `*games.Games`/`*books.Books`/`*feeds.Feeds` reference `dashboard` was
constructed with (`BuildSharedSteam`, `BuildSharedLibrary`, `BuildSharedFeeds`,
...) rather than duplicating any business logic — see the Apps list above
for why this is exported methods, not direct schema reads.

The owner manages both dashboards' tokens (plus their public display name)
through `dashboard.v1.DashboardService` (`cmd/api/connect_dashboard.go`,
behind normal `Access` — deliberately **not** gated by `dashboard`'s own
`AppAccess`, so a user's ability to share their games/reading dashboard
doesn't depend on a separate `dashboard` app-access grant; see that file's
doc comment). `dashboard`'s own `Routes()` therefore only ever registers the
two public services above.

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

`golangci-lint` (40+ linters), configured by the repo-root `.golangci.yml` — not `api/.golangci.yml`, which moved there so `gateway/` and `kobo-gateway/` share the exact same config (golangci-lint's config search walks up from the working directory to find it). Key constraints: max line length 88 (`golines`), import order standard → default → `prefix(tools.xdoubleu.com)` (`gci`), max function length 100 lines/50 statements (`funlen`), max cyclomatic complexity 30 (`cyclop`). `nolintlint` requires an explanation on every `//nolint` except `funlen`/`gocognit`/`lll`. Always run `make lint/fix` as the final step; fix anything the auto-fixer can't resolve manually. If a `.proto` file changed this session, order relative to `lint/fix` doesn't matter — run `make proto/check` (or `make proto/generate` in `api/` and `npm run generate` in `web/`) whenever it's convenient and commit the result; see root `CLAUDE.md`'s Commands section for why there's no ordering dependency. `GOLANGCI_LINT_CACHE` is set in the Makefile to a `.golangci-cache/` directory local to the checkout, so concurrent worktrees (which otherwise share the same Go module path and, by default, a single global cache) don't bleed each other's file paths into lint output.

## Testing Notes

- Mock injection for unit tests; mocks live in `internal/mocks/` or an app's own `internal/mocks/`.
- Integration tests hit a real database — `docker-compose up -d` before running locally.
- Target ≥80% coverage on changed code (`make test/cov/report`); generated files and `_mock.go` are excluded. `make test/cov/diff` (`tools/diff_coverage_go.py`) reports coverage on just the lines changed vs `origin/main`, approximating what CI's `codecov/patch` check gates on — run it before pushing to catch an uncovered branch locally instead of after a CI round trip. `make test/cov/per-pkg` merges per-package Go coverage profiles via the repo-root `tools/merge_coverage.py` — its web-side sibling, `tools/diff_coverage_ts.py`, does the equivalent lcov-based diff scoping for `web/` (`npm run test:cov:diff`).
- Repeated local test runs against the same Postgres volume can leave state that breaks a later run with failures unrelated to what's actually being changed (e.g. `relation ... does not exist`, `resource conflicts with existing resource`) — CI never hits this since every job gets a fresh container. `make db/reset` recreates the volume; run it if a failure looks like leftover state rather than a real regression.
- **One Postgres container is shared by every worktree.** `docker-compose.yml` sets no `name:`, so Compose derives the project name from the `api/` directory — identical in every checkout — and binds a fixed host port, so all concurrent sessions resolve to the same `api-db-1`. `docker-compose down` (and `make db/reset`, which is `down -v` and destroys the shared *volume*) therefore hits every other session too, and the victim sees exactly the symptoms above in whatever suite it was running — indistinguishable from leftover state. Before reaching for `db/reset`, check `docker ps` for a container that's only been up seconds: a database that vanished mid-run is a concurrent teardown, not pollution. Don't stop the container when finishing a task (issue #1205).
- **A concurrent worktree applying migrations can also desync `goose_db_version` itself**, distinct from the stale-test-data symptom above: `make test`/`make run` panics with `relation "X" already exists` (or a missing column another worktree's session already added) instead of a normal test failure, because one worktree's migration DDL landed on the shared schema while its `goose_db_version` row never committed (e.g. the other session panicked mid-run). Diagnose with `docker exec api-db-1 psql -U postgres -d postgres -c "SET search_path=global; SELECT version_id, is_applied FROM goose_db_version ORDER BY version_id DESC LIMIT 8;"` (`postgres` is the default `DB_DSN` database) and compare against the migration file the panic names. If `\d global.<table>` shows the schema already matches that migration, the DDL already applied — insert the missing row rather than dropping/recreating anything: `docker exec api-db-1 psql -U postgres -d postgres -c "SET search_path=global; INSERT INTO goose_db_version (version_id, is_applied) VALUES (<n>, true);"`.
- Write a failing test first when fixing a bug.

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code — split `_test.go` by feature/handler group, source by concern (extract large string constants to a companion file).
