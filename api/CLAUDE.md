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
- **`config`** — centralized config loaded from `.env` via `xdoubleu/essentia/v4`.
- **`contacts`** — contact management (editable display names) shared by recipes/mealplans/shoppinglist sharing. `AddByEmail` emails the recipient after the request is persisted; a send failure is logged, never fails the request.
- **`crypto`** — AES-256-GCM `Sealer`, used to encrypt OAuth tokens at rest.
- **`mailer`** — thin Resend HTTP client (no SDK). `Send` (fixed recipient) and `SendTo` (arbitrary recipient) share `ErrNotConfigured` degrade-gracefully semantics when the API key/from/to is unset.
- **`github`, `sentryapi`, `digitalocean`** — read-only external observability clients (failing PRs, unresolved Sentry issues, DO deployments and their build/deploy/runtime logs). Each resolves its admin-picked identifier fresh on every call from `global.oauth_connections.config`, exposes a discovery method for the admin picker, and returns `ErrNotConfigured`/`ErrNotConnected` rather than failing when the provider isn't set up yet. `digitalocean.Client.DeploymentLogsStream` (backing the one server-streaming RPC in the codebase, `GetDeployLogs`) yields each component/type pair to the caller as soon as it resolves instead of collecting everything first, so the first byte reaches the client well inside DigitalOcean's own edge timeout regardless of how long the slowest component takes.
- **`oauthconn`** — shared plumbing for the admin-configurable OAuth connections (GitHub/Sentry/DigitalOcean): token refresh via `oauth2.Config.TokenSource`, single-use CSRF `StateStore` for the browser redirect leg. Tokens are stored encrypted in `global.oauth_connections`; a `NULL` `config` column means "connected but not yet configured" — a distinct degraded state each provider's fetch path checks.
- **`observability`** — `TrackedJob` (times/records every job run in `global.job_runs`, recovers panics, logs failures at Error so they reach Sentry), `UsageRecorder` (per-endpoint request counts, flushed to `global.usage_daily`), and `jobs.IssueNotifierJob` (cross-app, registered directly on `main.go`'s own job queue since it isn't scoped to one app — polls Sentry/DO every 5 minutes and emails an admin the first time an issue/failed deploy is seen, deduped via `global.notified_issues`).
- **`progressws`** — WebSocket service broadcasting background-job progress ("X of N") keyed by job-ID topics.
- **`progresshistory`** — generic cumulative-progress storage with carry-forward reads (games/books progress graphs).
- **`repositories`** — shared DB repos over the `global` schema (users, contacts, the observability tables, `oauth_connections`, `profile_shares`).
- **`mcptools`** — `RequireAppAccess` (per-app MCP gate, mirrors `auth.AppAccess`), `AddReadTool`, `Unwrap`/`Result`.
- **`testhelper`** — `ConnectTestDB` for integration tests, `NewTestConfig` (auth cache TTL 0), `BuildMux` for a test handler from any `Routes`/`GetName` app.

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress/distribution, favourites, per-user Steam settings. Background sync job + WebSocket live updates. Schema `games` (adopted from the former `backlog` schema).
- **books** — book library and Kobo e-reader companion. Pure-Go PDF/HTML→EPUB conversion (no Calibre), dual metadata enrichment (UniCat + Hardcover). Serves the raw Kobo sync protocol. Background jobs + WebSocket live updates. Schema `books`.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no jobs, own custom domain (`watchparty.xdoubleu.com`).
- **icsproxy** — ICS calendar feed filtering/proxying. Schema `icsproxy`.
- **recipes** — recipe management: fraction parsing, iCal export, whole-recipe-book sharing. Schema `recipes`.
- **mealplans** — weekly meal planning with per-plan iCal feeds and sharing. Schema `mealplans` (its `plans` tables were adopted from `recipes` via `ALTER TABLE ... SET SCHEMA`).
- **shoppinglist** — custom items plus meal-plan ingredient aggregation, categories, store-ordered export, sharing. Stores themselves stay private per-user even when the rest of the list is shared. Schema `shoppinglist`.
- **todos** — task management: sections, workspaces, subtasks, policies, archive, search. Background archive job.

### Database Conventions

- Each app owns its own Postgres schema, migrated via Goose SQL files in `apps/<name>/migrations/`.
- Cross-cutting tables live in schema `global`, migrations embedded in `cmd/api/migrations/`.
- Downstream apps may **read** an upstream app's schema directly in SQL instead of going through an internal API — the allowed dependency direction is acyclic: `recipes ← mealplans ← shoppinglist`. `mealplans` joins `recipes.recipes`; `shoppinglist`'s export/item-name-catalog features join both `mealplans.*` and `recipes.*`. Reads only, never the reverse direction, and each app's migrations touch only its own schema — grep downstream repositories before changing an upstream schema.
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking.

### Apps MCP Server

Every app's own read RPCs, plus admin observability, are exposed to a local
Claude CLI over a largely read-only MCP server at `/apps/mcp`
(`cmd/api/mcp_apps.go`). Apps opt in via the `MCPToolProvider` interface
(`RegisterMCPTools(srv *mcp.Server)`, `cmd/api/apps.go`) — each implementing
app has its own `apps/<name>/mcp.go` wrapping only its **read** Connect
handlers, so no per-app tool is ever mutating. `registerObservabilityMCPTools`
adds the 9 unprefixed admin-gated observability tools on top, sharing the
exact same internal methods the Connect handlers use — 8 are read, plus
`resolve_sentry_issue`, the one deliberate mutation (marks a Sentry issue
resolved via `sentryapi.Client.ResolveIssue`), letting an admin-authenticated
agent close out an issue it just filed a fix for. Auth is MCP OAuth
2.1: the api is the resource server (`auth.RequireBearerToken` verifies a
Supabase access token), Supabase is the authorization server, and the web
`/oauth/consent` page drives the approval. See root `README.md` for setup.

### Public Profile Sharing

`books.v1.PublicLibraryService` and `games.v1.PublicGamesService` (each
app's `connect_public.go`) are registered **without** any auth middleware —
every request carries an opaque share token (`global.profile_shares`, keyed
by `(user_id, app)`) that resolves to the owning user. Public handlers must
never read the user-context key, since no middleware sets it. The owner
manages both apps' tokens independently through `profile.v1.ProfileService`
(`cmd/api/connect_profile.go`, behind normal `Access`).

### Key Libraries

| Concern | Library |
| --- | --- |
| HTTP | `net/http` + `justinas/alice` |
| RPC | `connectrpc.com/connect` |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` |
| Auth | `supabase-community/auth-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `xdoubleu/essentia/v4` threading.JobQueue |
| MCP | `modelcontextprotocol/go-sdk` |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

## Linting

`golangci-lint` (40+ linters). Key constraints: max line length 88 (`golines`), import order standard → default → `prefix(tools.xdoubleu.com)` (`gci`), max function length 100 lines/50 statements (`funlen`), max cyclomatic complexity 30 (`cyclop`). `nolintlint` requires an explanation on every `//nolint` except `funlen`/`gocognit`/`lll`. Always run `make lint/fix` as the final step; fix anything the auto-fixer can't resolve manually. Exception: if a `.proto` file changed this session, `make proto/generate` must run *after* `make lint/fix` — `gci` reformats generated files' import grouping too, which then fails CI's proto-staleness check (see root `CLAUDE.md`). `GOLANGCI_LINT_CACHE` is set in the Makefile to a `.golangci-cache/` directory local to the checkout, so concurrent worktrees (which otherwise share the same Go module path and, by default, a single global cache) don't bleed each other's file paths into lint output.

## Testing Notes

- Mock injection for unit tests; mocks live in `internal/mocks/` or an app's own `internal/mocks/`.
- Integration tests hit a real database — `docker-compose up -d` before running locally.
- Target ≥80% coverage on changed code (`make test/cov/report`); generated files and `_mock.go` are excluded.
- Write a failing test first when fixing a bug.

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code — split `_test.go` by feature/handler group, source by concern (extract large string constants to a companion file).
