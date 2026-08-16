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
make test/cov/per-pkg              # per-package coverage, merged
make lint                          # golangci-lint + sqlfluff + buf lint
make lint/fix                      # golines + golangci-lint --fix + gci + sqlfluff fix + buf lint
make lint/pkg PKG=apps/recipes     # lint a single package
make proto/generate                # regenerate api/gen/ from proto/ (pair with `npm run generate` in web/)
make lint/proto                    # buf lint — also part of make lint / lint/fix
make scaffold NAME=x [DB=true] [JOBS=true]   # generate a new app skeleton

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
│                       # games/books/todos/watchparty export Services/Repositories
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

`Service`/`GoTrueService` (Supabase, via `supabase-community/auth-go`) live in
`internal/auth/service.go`. Key points:

- auth-go has no context support, so request-context propagation stops at the GoTrue boundary; the DB enrichment queries and the cache do consume it.
- A per-token TTL cache (`AUTH_CACHE_TTL` seconds, default 60, `0` disables — tests use 0 via `testhelper.NewTestConfig`) sits in front of every resolution; a hit skips the GoTrue round-trip and both enrichment queries, so role/app-access changes can lag by up to the TTL.
- `enrichUser` overlays the DB-managed `Role`/`AppAccess` onto the raw GoTrue user, which on its own always resolves to `RoleUser` with no app access. A DB failure here is **returned, not swallowed** — silently falling back to the unenriched user would be indistinguishable from "no access" to `AdminAccess`/`AppAccess`, and would get cached for the rest of the TTL instead of retrying next request.
- Tokens are evicted on SignOut/UpdatePassword/VerifyMFA/UnenrollTOTP; anything mutating *another* session's role/app-access (admin `SetRole`/`SetAppAccess`) must call `InvalidateUserCache()` (clear-all) afterwards.
- `GetCurrentUser` (`cmd/api/connect_auth_handlers.go`) is the two-layer pattern any Connect handler needing DB-enriched attributes should follow: resolve the session via `auth.GetUser` (GoTrue role), then look up `appUsersRepo.GetByID` and prefer the DB role/app-access/display-name when that lookup succeeds, falling back to the bare GoTrue values otherwise.

### Shared Internal Packages (`internal/`)

- **`app`** — `Base` (logger/config/auth embedded into every app), `HTTPError`, `ScrubInternalErrors` (Connect interceptor logging CodeInternal/CodeUnknown and replacing the client-facing message — every `New*ServiceHandler` call must pass it).
- **`auth`** — see above.
- **`config`** — centralized config, loaded from `.env`/environment variables.
- **`connecttools`** — `MapError`, shared by any app's ConnectRPC handlers to translate `database.ErrResourceNotFound`/`ErrResourceConflict` and `iapp.HTTPError` into the matching Connect error code, so recipes/mealplans/shoppinglist (and any future app with the same DB/HTTPError-to-Connect mapping needs) don't each reimplement it.
- **`sentrytools`** — `Middleware` (Connect/HTTP request-scoped Sentry hub) and `GoRoutineWrapper` (background-goroutine Sentry tracing). The slog→Sentry `LogHandler` and startup `Init` used to live here too but moved to the repo-root `sentrytools/` module (its own `go.mod`) once `gateway` turned out to need a byte-for-byte copy — see root `CLAUDE.md`'s CI section.
- **`contacts`** — contact management (editable display names) shared by recipes/mealplans/shoppinglist sharing. `AddByEmail` emails the recipient after the request is persisted; a send failure is logged, never fails the request.
- **`crypto`** — AES-256-GCM `Sealer`, used to encrypt OAuth tokens at rest.
- **`mailer`** — thin Resend HTTP client (no SDK). `Send` (fixed recipient) and `SendTo` (arbitrary recipient) share `ErrNotConfigured` degrade-gracefully semantics when the API key/from/to is unset.
- **`github`, `sentryapi`, `digitalocean`** — read-only external observability clients (failing PRs, unresolved Sentry issues, DO deployments and their build/deploy/runtime logs). Each resolves its admin-picked identifier fresh on every call from `global.oauth_connections.config`, exposes a discovery method for the admin picker, and returns `ErrNotConfigured`/`ErrNotConnected` rather than failing when the provider isn't set up yet. `digitalocean.Client.DeploymentLogsStream` (backing the one server-streaming RPC in the codebase, `GetDeployLogs`) yields each component/type pair to the caller as soon as it resolves instead of collecting everything first, so the first byte reaches the client well inside DigitalOcean's own edge timeout regardless of how long the slowest component takes.
- **`oauthconn`** — shared plumbing for the admin-configurable OAuth connections (GitHub/Sentry/DigitalOcean): token refresh via `oauth2.Config.TokenSource`, single-use CSRF `StateStore` for the browser redirect leg. Tokens are stored encrypted in `global.oauth_connections`; a `NULL` `config` column means "connected but not yet configured" — a distinct degraded state each provider's fetch path checks.
- **`observability`** — `TrackedJob` (times/records every job run in `global.job_runs`, recovers panics, logs failures at Error so they reach Sentry), `UsageRecorder` (per-endpoint request counts **and response bytes**, flushed to `global.usage_daily`; the byte counter is fed by `cmd/api/usage_middleware.go`'s `countingResponseWriter`, which must keep forwarding `Flush`/`Hijack` or the streaming `GetDeployLogs` RPC and `progressws` WebSocket upgrades break. Bytes measure what left the api — a proxy for what it read out of Postgres, not a direct measure of database egress, but close enough on passthrough list endpoints to identify the culprit, which is what issue #1027 needed and didn't have), `jobs.IssueNotifierJob` (cross-app, registered directly on `main.go`'s own job queue since it isn't scoped to one app — polls Sentry/DO/GitHub every 5 minutes and emails an admin the first time an issue/failed deploy/failing PR is seen, deduped via `global.notified_issues`; the GitHub half only alerts on failing PRs carrying the `dependencies` label, issue #915 — Renovate opens those, see root `renovate.json5`, and nobody is otherwise watching them, unlike a PR a user or a Claude Code session opened, which already has someone driving it to green), `jobs.TransactionLatencySnapshotJob` (also cross-app on `main.go`'s job queue — snapshots each transaction's p95 duration/request count from `sentryapi.Client.ListTransactionStats` into `global.transaction_latency_daily` once a day, issue #848; `repositories.TransactionLatencyRepository.Trends` compares a recent window against the prior one to flag endpoints/pages regressing, surfaced by `get_slow_transactions`), and `jobs.WeeklyDigestJob` (also cross-app on `main.go`'s job queue, `RunEvery = 7 days`, issue #1014 — unlike `IssueNotifierJob` it has no dedup, so every run emails an admin a summary of everything still open: unresolved Sentry issues, a still-failing latest DO deployment, failing `dependencies`-labeled PRs, and feeds currently failing to poll (`apps/feeds.Feeds.ListUnhealthy`, exposed the same way `BuildSummary` is), sending a short "no open issues" email when there's nothing to report so a missing weekly email is itself a sign the job stopped running. `main.go`'s `feedsHealthAdapter` bridges `*feeds.Feeds` into the job's own `unhealthyFeedLister` interface so this `internal/` package never imports an `apps/*` package).
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
- **todos** — task management: sections, workspaces, subtasks, policies, archive, search. Background archive job.
- **dashboard** — centralizes the public Games and Reading (books+feeds) dashboards, both private/owner and public/shared views, plus the share-token lifecycle (issue #737). No DB, no jobs, like `watchparty`; registers last in `apps.go` since it holds live references to the already-constructed `games`/`books`/`feeds` apps. See "Public Dashboard Sharing" below.

### Database Conventions

- Each app owns its own Postgres schema, migrated via Goose SQL files in `apps/<name>/migrations/`.
- Cross-cutting tables live in schema `global`, migrations embedded in `cmd/api/migrations/`.
- **Never put a wide TEXT column in a list query's column list.** The deployed database is reached over a transaction-mode pooler and billed per byte returned, so a page of rows carrying a large column is billed egress on every request. `feeds`' `itemColumns`/`itemListColumns` split (`apps/feeds/internal/repositories/items.go`) and `books`' `bookColumns` (`apps/books/internal/repositories/books_scan.go`) show the pattern: multi-row reads and `RETURNING` clauses select `<col> IS NOT NULL AND <col> <> ''` as a boolean, and a dedicated single-row read is the only query selecting the column itself. Getting this wrong on `feeds.items.content_html` exhausted the whole monthly egress quota and took the site down (issue #1027). The same applies to any query whose result the caller then throws away — the hourly `todos-archive` job selects only ids for exactly this reason.
- Downstream apps may **read** an upstream app's schema directly in SQL instead of going through an internal API — the allowed dependency direction is acyclic: `recipes ← mealplans ← shoppinglist`. `mealplans` joins `recipes.recipes`; `shoppinglist`'s export/item-name-catalog features join both `mealplans.*` and `recipes.*`. Reads only, never the reverse direction, and each app's migrations touch only its own schema — grep downstream repositories before changing an upstream schema.
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking.

### Apps MCP Server

Every app's own read RPCs, plus admin observability, are exposed to a local
Claude CLI over a largely read-only MCP server at `/apps/mcp`
(`cmd/api/mcp_apps.go`). Apps opt in via the `MCPToolProvider` interface
(`RegisterMCPTools(srv *mcp.Server)`, `cmd/api/apps.go`) — each implementing
app has its own `apps/<name>/mcp.go` wrapping only its **read** Connect
handlers, so no per-app tool is ever mutating. `registerObservabilityMCPTools`
adds the 10 unprefixed admin-gated observability tools on top, sharing the
exact same internal methods the Connect handlers use — 9 are read, plus
`resolve_sentry_issue`, the one deliberate mutation (marks a Sentry issue
resolved via `sentryapi.Client.ResolveIssue`), letting an admin-authenticated
agent close out an issue it just filed a fix for. Auth is MCP OAuth
2.1: the api is the resource server (`auth.RequireBearerToken` verifies a
Supabase access token), Supabase is the authorization server, and the web
`/oauth/consent` page drives the approval. See root `README.md` for setup.

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
| Auth | `supabase-community/auth-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `internal/threading` (WorkerPool) + `internal/jobqueue` (scheduling) |
| MCP | `modelcontextprotocol/go-sdk` |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

## Linting

`golangci-lint` (40+ linters), configured by the repo-root `.golangci.yml` — not `api/.golangci.yml`, which moved there so `gateway/` and `kobo-gateway/` share the exact same config (golangci-lint's config search walks up from the working directory to find it). Key constraints: max line length 88 (`golines`), import order standard → default → `prefix(tools.xdoubleu.com)` (`gci`), max function length 100 lines/50 statements (`funlen`), max cyclomatic complexity 30 (`cyclop`). `nolintlint` requires an explanation on every `//nolint` except `funlen`/`gocognit`/`lll`. Always run `make lint/fix` as the final step; fix anything the auto-fixer can't resolve manually. Exception: if a `.proto` file changed this session, `make proto/generate` must run *after* `make lint/fix` — `gci` reformats generated files' import grouping too, which then fails CI's proto-staleness check (see root `CLAUDE.md`). `GOLANGCI_LINT_CACHE` is set in the Makefile to a `.golangci-cache/` directory local to the checkout, so concurrent worktrees (which otherwise share the same Go module path and, by default, a single global cache) don't bleed each other's file paths into lint output.

## Testing Notes

- Mock injection for unit tests; mocks live in `internal/mocks/` or an app's own `internal/mocks/`.
- Integration tests hit a real database — `docker-compose up -d` before running locally.
- Target ≥80% coverage on changed code (`make test/cov/report`); generated files and `_mock.go` are excluded.
- Write a failing test first when fixing a bug.

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code — split `_test.go` by feature/handler group, source by concern (extract large string constants to a companion file).
