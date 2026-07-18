# api/ — Backend

Go 1.26 backend for tools.xdoubleu.com. Run all `make` commands from this directory.

## Common Commands

```bash
# Local DB — always start before running tests, stop after
docker-compose up -d        # Start local PostgreSQL 18
docker-compose down         # Stop after tests are done

# Build
make build                  # Build ./bin/api (main server binary)
# The kobo-gateway macOS menu-bar app is a separate Go module — see gateway/CLAUDE.md

# Testing (requires DB running — docker-compose up -d first)
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)
make test/cov/per-pkg       # Per-package coverage with merged report

# Single test
go test ./apps/reading/... -run TestFunctionName

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues
make lint/pkg PKG=apps/recipes  # Lint a single package

# Proto code generation (also run `npm run generate` from web/ — they're paired)
make proto/generate
```

## Docker Image

The api image uses `debian:12-slim` (not distroless) as the final stage because the
**reading conversion features** shell out to Calibre's `ebook-convert` binary — to
convert PDFs to EPUB before kepubification, and to build article EPUBs from
extracted web-page HTML. Calibre requires Qt and Python shared libraries that
distroless cannot provide.

This makes the image significantly larger than a distroless build (~700 MB vs ~20 MB).
The Calibre layer is cached via `type=gha` GitHub Actions layer caching, so CI rebuild
times are only affected when `apt-get install calibre` would pull a new version.

## R2 Bucket CORS

The in-browser EPUB/KEPUB preview reads bytes client-side via epub.js (XHR), so the R2
bucket must allow cross-origin GET requests from the web frontend's origin. Apply this CORS
policy to **each** bucket (adjust `AllowedOrigins` per environment):

```json
[{
  "AllowedOrigins": ["http://localhost:3000"],
  "AllowedMethods": ["GET", "HEAD"],
  "AllowedHeaders": ["*"],
  "ExposeHeaders": ["Content-Length", "Content-Range", "Accept-Ranges", "ETag"],
  "MaxAgeSeconds": 3600
}]
```

Set it via the Cloudflare R2 dashboard (bucket → Settings → CORS Policy) or `aws s3api
put-bucket-cors` against the R2 endpoint. Production origin: `https://tools.xdoubleu.com`.
PDF preview (iframe navigation) is unaffected by this rule. Recreating a bucket requires
re-applying the rule — it is not stored in this repo.

## Architecture

A Go monorepo that serves multiple web apps from a single binary. All apps are registered in `cmd/api/apps.go` and share a single HTTP mux routed by URL prefix; `main.go` wraps the shared pgx pool in `postgres.NewSpanDB` once so every app's queries emit tracing spans (migrations use the raw pool). Registration order matters for migrations: `reading` registers before `games` because games' final migration drops the leftover `backlog` schema after both apps have adopted their tables. Apps expose ConnectRPC endpoints consumed by the Next.js frontend in `web/`.

### App Structure

Each app lives in `apps/<name>/` and follows a consistent layout:

```
apps/<name>/
├── app.go              # App struct embedding app.Base, implements App interface
│                       # Apps whose integration tests seed data through the
│                       # service layer (games, reading, watchparty) export a
│                       # Services field; the rest keep services private
├── routes.go           # HTTP route registration
├── handlers.go         # HTTP handlers (shared middleware/error helpers)
│                       # Large apps split handler code across focused files,
│                       # e.g. tasks_crud.go, tasks_list.go, tasks_subtasks.go
├── connect*.go         # ConnectRPC service implementations
├── internal/
│   ├── dtos/           # Request/response serialization
│   ├── models/         # Domain models
│   ├── repositories/   # DB access layer (pgx/v5)
│   ├── services/       # Business logic
│   ├── jobs/           # Background jobs (if any)
│   ├── helper/         # App-specific utilities
│   └── mocks/          # Mock implementations for testing
├── pkg/                # Reusable packages (external client integrations)
└── migrations/         # Goose SQL migrations (per-app schema)
```

### ConnectRPC Auth Handler Conventions

The `GetCurrentUser` handler (in `cmd/api/connect_auth_handlers.go`) uses a two-layer role resolution pattern:

1. Call `h.app.auth.GetUser(ctx, token)` to validate the session and get the GoTrue user (including its `Role` field).
2. Call `h.app.appUsersRepo.GetByID(ctx, user.ID)` to retrieve the DB-enriched user record. If found, prefer the DB role over the GoTrue role. If not found, fall back to the GoTrue role.

Any Connect handler that needs DB-enriched user attributes must follow this same fallback pattern rather than relying solely on the GoTrue response.

### Auth (`internal/auth`)

The `Service` interface and its `GoTrueService` implementation (Supabase, via `supabase-community/auth-go`) live together in `internal/auth`. Conventions:

- Every auth method doing I/O takes a `context.Context` first. auth-go v1.5.0 has no context support, so propagation stops at the GoTrue boundary; the DB enrichment queries and the cache do consume it.
- The middleware (`Access`/`TemplateAccess`/…) resolves users through a **per-token TTL cache** (`AUTH_CACHE_TTL` seconds, default 60, `0` disables — tests use 0 via `testhelper.NewTestConfig`). A cache hit skips the GoTrue round-trip and both enrichment queries, so role/app-access changes and the `last_seen` upsert can lag by up to the TTL.
- Tokens are evicted on SignOut, UpdatePassword, VerifyMFA, and UnenrollTOTP. Anything that mutates roles or app access for *other* sessions (admin `SetRole`/`SetAppAccess`) must call `InvalidateUserCache()` (clear-all) afterwards.
- `SignInRenderer` is injected post-construction from `cmd/api` (the templ sign-in page lives there).

### Shared Internal Packages (`internal/`)

- **`app.Base`** — Embedded struct providing logger, config, and auth service to every app
- **`app.HTTPError`** — Shared HTTP error type (`Status int`, `Message string`); import as `iapp "tools.xdoubleu.com/internal/app"` in handler files to avoid collision with the app struct
- **`app.ScrubInternalErrors(logger)`** — Connect handler option that logs CodeInternal/CodeUnknown errors and replaces the client-facing message with a generic one; every `New*ServiceHandler` call must pass it
- **`auth/`** — Auth interface + `GoTrueService` implementation, middleware, and per-token user cache (see "Auth" above)
- **`config/`** — Centralized config loaded from `.env` via `xdoubleu/essentia/v4`
- **`constants/`** — Shared constants
- **`contacts/`** — Contact management service with editable display names (used by recipes, shopping list, and meal-plan sharing)
- **`crypto/`** — Encryption utilities
- **`models/`** — Shared domain models
- **`repositories/`** — Shared DB repositories over the `global` schema (users, contacts, the observability tables: `JobRunsRepository`, `UsageRepository`, `StorageSnapshotsRepository`, `DBStatsRepository`, and `ProfileSharesRepository` for the public-profile share tokens)
- **`observability/`** — Cross-cutting instrumentation. `TrackedJob` decorates any `threading.Job` so every run is timed and recorded in `global.job_runs`, panics are recovered, and failures log at Error level (so they reach Sentry); wrap jobs at registration (see `apps/{todos,games,reading}/app.go`). `UsageRecorder` counts requests per `(day, app, endpoint)` in memory and flushes to `global.usage_daily`; the counting `usageMiddleware` sits in the `cmd/api` alice chain after `domainMiddleware`.
- **`progressws/`** — WebSocket service broadcasting background-job progress (start/stop state, live "X of N" counts) keyed by job-ID topics
- **`progresshistory/`** — Generic cumulative-progress storage with carry-forward reads (used by games and reading progress graphs)
- **`mocks/`** — Shared mock implementations
- **`testhelper/`** — Test utilities: `ConnectTestDB(dsn)` wraps `postgres.Connect` for integration tests; `BuildMux(Routable)` constructs a test `http.Handler` from any app that implements `Routes`/`GetName`

### Key Libraries

| Concern | Library |
| --- | --- |
| HTTP | `net/http` + `justinas/alice` (middleware chaining) |
| RPC | `connectrpc.com/connect` — HTTP/1.1 RPC framework |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` (migrations) |
| Auth | `supabase-community/auth-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `xdoubleu/essentia/v4` threading.JobQueue |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion rate progress/distribution, user-set favourites, and the user's Steam integration settings. External client package lives in `pkg/steam/`. Has a background sync job (1 worker) and WebSocket live updates. The `steam_games.favourite` flag is user-set state: `UpsertGames` deliberately never writes it, so it survives every sync. Uses `games` DB schema (adopted from the former `backlog` schema).
- **reading** (formerly **books** — Go package `apps/reading/`, URL prefix `/reading`, schema `reading`, proto package `reading.v1`; entity types like `Book`/`BookService` keep their names) — Reading library and e-reader companion for books, arXiv papers, web articles, and RSS posts. Every catalog row has a fixed `category` (`book`/`paper`/`article`/`rss`) and non-book items carry a canonical `source_url` (dedup key, partial unique index). Ingestion paths: `LibraryService.AddBookByURL` routes arXiv URLs (`pkg/arxiv/`, Atom API) to paper ingestion (metadata + PDF download) and everything else to readability extraction (`go-shiori/go-readability`) + article-EPUB building via the shared Calibre subprocess slot (`conversion_calibre.go` — HTML and PDF conversions share one semaphore; article images are downloaded and localized first, `ingest_images.go`); `FeedService` manages RSS/Atom subscriptions (`reading.feeds` + per-feed seen-set `reading.feed_items`, parsed with `mmcdole/gofeed`), imported on subscribe and polled hourly by the `poll-feeds` job with conditional GETs; a per-feed `kobo_sync` flag auto-opts every new item into Kobo sync (tag + eager KEPUB); feed items whose link/GUID is an arXiv id are ingested as `paper`s (PDF) via `IngestService.IngestArxivByID`, not `rss` articles. Deleting a feed (`FeedService.Delete`) also removes the library items it ingested **except** any the user read or favourited (`FeedsRepository.ListRemovableBookIDs` → `BookService.RemoveFromLibrary`). RSS items are treated as an auto-pulled firehose distinct from deliberately-added reading: `buildLibraryData` keeps `category='rss'` items out of the reading-state shelves (returning them in `LibraryResponse.rss`) and the read-progress graph (`GetFinishedDates`) excludes them, so books/papers/articles count separately from RSS. All external web content goes through the size-capped `pkg/webfetch/` client. Book metadata enrichment queries two independent providers, concurrently per book (`fetchByISBN`/`searchProviders` in `book_resync.go`, via `errgroup`): UniCat (Belgian SRU/MARC catalog, no key) and Hardcover (GraphQL; set `HARDCOVER_API_KEY` — a free Bearer JWT from the account settings page that expires ~yearly and must be refreshed; left disabled/nil when unset. No daily quota, its 1 req/s limiter is the resync throughput floor). ISBN-less books are matched by title+author; resync/duplicate scans skip non-book categories. External client packages live in `pkg/unicat/`, `pkg/hardcover/`, `pkg/arxiv/`, `pkg/webfetch/`; the resync orchestration and per-source scan-status cache (`*_found` columns) live in `internal/services/book_resync.go`. Serves the raw Kobo sync protocol under `/reading/kobo/{token}/…` and a public cover proxy (`routes.go`); devices set up under the pre-rename `/books/kobo/…` prefix must re-run the gateway setup flow. Per-device debug logging (endpoint + request/response bodies) can be toggled from the Reading settings page; captured requests live in an in-memory `KoboLogStore` (`apps/reading/internal/services/kobo_log.go`), not the DB, and reset on restart. Has background jobs (2 workers) and WebSocket live updates, including a daily R2 bucket scan (`books-storage-scan`) that writes a `global.storage_snapshots` row for the admin dashboard. The object-store `Client` (`pkg/objectstore/`) exposes a paginated `List` used by that scan. Uses the `reading` DB schema — renamed in place from `books` by a pre-migration bootstrap in Go (`renameLegacyBooksSchema` in `app.go`: goose's version table lives inside the schema, so `ALTER SCHEMA … RENAME` carries the migration history along; historical migration files were rewritten to `reading.` for fresh installs, and R2 storage keys keep their `books/` prefix). The `books`→`reading` app identifier is also rewritten in `global` data by `cmd/api/migrations/00008`.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no background jobs.
- **icsproxy** — ICS calendar feed filtering and proxying. Uses `icsproxy` DB schema.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and whole-recipe-book sharing with contacts (`recipebook_access`, view-only or edit). Uses `recipes` DB schema.
- **shoppinglist** — Custom items plus meal-plan ingredient aggregation, with user-defined categories, a name→category catalog, and per-store category ordering that drives a store-ordered (Apple Notes) export. The whole list is shareable with contacts (`shoppinglist_access`, view-only or edit); most data RPCs accept an `owner_user_id` so a recipient can act on a shared owner's list. **Stores are the exception — they are private to each user:** the store RPCs take no `owner_user_id` and always act on the caller's own stores, so a share recipient orders an export by their own stores and never gains access to the owner's. Uses `shoppinglist` DB schema.
- **mealplans** — Weekly meal planning with per-plan iCal feeds and plan sharing with contacts. Uses `mealplans` DB schema (its `plans` tables were adopted from the `recipes` schema — the same `ALTER TABLE … SET SCHEMA` pattern later used for the games/books split).
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, search, and background archive jobs. Uses `todos` DB schema.

### Database Conventions

- Each app uses its own PostgreSQL schema (e.g., `reading`, `icsproxy`)
- Cross-cutting tables live in the `global` schema with migrations in `cmd/api/migrations/` (users, contacts, `profile_shares`, and observability: `job_runs`, `usage_daily`, `storage_snapshots`). The admin observability RPCs (`GetJobStats`/`GetUsageStats`/`GetStorageStats`/`GetDatabaseStats` in `cmd/api/connect_admin_stats.go`) read these plus live `pg_*` size queries.
- Migrations live in `apps/<name>/migrations/` and follow Goose SQL format
- `updated_at` columns are managed via PostgreSQL triggers
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking

### Cross-Schema Reads

Apps share one binary and one database, so downstream apps may **read** an
upstream app's schema directly in SQL instead of going through an internal API.
The allowed dependency direction is acyclic:

```
recipes ← mealplans ← shoppinglist
```

- `mealplans` joins `recipes.recipes` (meals reference recipes); its proto
  embeds `recipes.v1.Recipe`.
- `shoppinglist` is by design a read-side aggregator: its export and item-name
  catalog features join `mealplans.plan_meals`/`plans`/`plan_access` and
  `recipes.recipes`/`ingredients`.

Rules: reads only (never write another app's schema), never add a dependency
in the reverse direction, and each app's migrations touch only its own schema.
Upstream schema changes (recipes, mealplans) must grep downstream repositories
for affected columns.

### Public Profile Sharing

Reading and games expose read-only shareable-profile RPCs
(`reading.v1.PublicLibraryService`, `games.v1.PublicGamesService`, in each app's
`connect_public.go`). These are registered in `routes.go` **without** any
auth middleware: every request carries an opaque share token that resolves to
the owning user via the shared `ProfileSharesRepository`
(`global.profile_shares`, plaintext token, keyed by `(user_id, app)` — read-only
data, so the owner can copy the link anytime; unknown tokens, and tokens
resolved against the wrong app, return `CodeNotFound`). Reading and games each
have their own independent share link — disabling one never touches the
other. The owner manages both tokens through `profile.v1.ProfileService`,
handled in `cmd/api/connect_profile.go` behind `Access`; every RPC takes a
`ProfileApp` argument, and regenerating replaces that app's row, instantly
invalidating its old link. Public handlers must never read
`constants.UserContextKey` — no auth middleware runs, so it is never set.

`global.app_users` carries a nullable `display_name` column (the user's public
profile name, set via `ProfileService.SetDisplayName`). `CreateProfileShare`
requires it to be non-empty first (`CodeFailedPrecondition` otherwise) — a
share link is worthless without a name to attribute it to. The public RPCs
resolve it alongside the owning user ID (`ProfileSharesRepository.ResolveToken`,
a `LEFT JOIN` against `app_users`) and return it on `GetSharedLibraryResponse`/
`GetSharedSteamResponse` for the frontend to display.

## Linting

Strict linting is enforced via `golangci-lint` (40+ linters). Key constraints:

- Max line length: 88 characters (`golines`)
- Import order: standard → default → custom (`gci`)
- Max function length: 100 lines / 50 statements
- Max cyclomatic complexity: 30

Always run `make lint/fix` as the final step before committing. Manually fix anything the auto-fixer cannot resolve.

## Testing Notes

- Use mock injection for unit tests; place mocks in `internal/mocks/` or app-level `internal/<name>/mocks/`
- Integration tests hit a real database — start `docker-compose up -d` from the repo root before running tests locally
- Target ≥80% coverage on changed code; check with `make test/cov/report`
- Generated files (`gen/`, `_mock.go`) are excluded from coverage
- When fixing bugs, write a failing test first before implementing the fix

## File Size & Splits

Go files projected over ~300 lines need a split plan before adding more code:

- `*_test.go` — split by feature or handler group (e.g. `tasks_crud_test.go`, `tasks_search_test.go`)
- `.go` source — split by concern; extract large JS/TS string constants to a companion `.go` file
- `.templ` — split by UI concern (e.g. `views_list.templ`, `views_form.templ`)
