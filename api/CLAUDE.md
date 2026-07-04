# api/ — Backend

Go 1.26 backend for tools.xdoubleu.com. Run all `make` commands from this directory.

## Common Commands

```bash
# Local DB — always start before running tests, stop after
docker-compose up -d        # Start local PostgreSQL 18
docker-compose down         # Stop after tests are done

# Build
make build                  # Build ./bin/api (main server binary)

# Testing (requires DB running — docker-compose up -d first)
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)
make test/cov/per-pkg       # Per-package coverage with merged report

# Single test
go test ./apps/books/... -run TestFunctionName

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues
make lint/pkg PKG=apps/recipes  # Lint a single package

# Proto code generation (also run `npm run generate` from web/ — they're paired)
make proto/generate
```

## Docker Image

The api image uses `debian:12-slim` (not distroless) as the final stage because the
**books conversion feature** shells out to Calibre's `ebook-convert` binary to
convert PDFs to EPUB before kepubification. Calibre requires Qt and Python shared
libraries that distroless cannot provide.

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

A Go monorepo that serves multiple web apps from a single binary. All apps are registered in `cmd/api/apps.go` and share a single HTTP mux routed by URL prefix; `main.go` wraps the shared pgx pool in `postgres.NewSpanDB` once so every app's queries emit tracing spans (migrations use the raw pool). Registration order matters for migrations: `books` registers before `games` because games' final migration drops the leftover `backlog` schema after both apps have adopted their tables. Apps expose ConnectRPC endpoints consumed by the Next.js frontend in `web/`.

### App Structure

Each app lives in `apps/<name>/` and follows a consistent layout:

```
apps/<name>/
├── app.go              # App struct embedding app.Base, implements App interface
│                       # Apps whose integration tests seed data through the
│                       # service layer (games, books, watchparty) export a
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

1. Call `h.app.services.Auth.GetUser(token)` to validate the session and get the GoTrue user (including its `Role` field).
2. Call `h.app.appUsersRepo.GetByID(ctx, user.ID)` to retrieve the DB-enriched user record. If found, prefer the DB role over the GoTrue role. If not found, fall back to the GoTrue role.

Any Connect handler that needs DB-enriched user attributes must follow this same fallback pattern rather than relying solely on the GoTrue response.

### Shared Internal Packages (`internal/`)

- **`app.Base`** — Embedded struct providing logger, config, and auth service to every app
- **`app.HTTPError`** — Shared HTTP error type (`Status int`, `Message string`); import as `iapp "tools.xdoubleu.com/internal/app"` in handler files to avoid collision with the app struct
- **`app.ScrubInternalErrors(logger)`** — Connect handler option that logs CodeInternal/CodeUnknown errors and replaces the client-facing message with a generic one; every `New*ServiceHandler` call must pass it
- **`auth/`** — Supabase GoTrue authentication (`gotrue-go`)
- **`config/`** — Centralized config loaded from `.env` via `xdoubleu/essentia/v4`
- **`constants/`** — Shared constants
- **`contacts/`** — Contact management service with editable display names (used by recipes, shopping list, and meal-plan sharing)
- **`crypto/`** — Encryption utilities
- **`models/`** — Shared domain models
- **`repositories/`** — Shared DB repositories
- **`progressws/`** — WebSocket service broadcasting background-job progress (start/stop state, live "X of N" counts) keyed by job-ID topics
- **`progresshistory/`** — Generic cumulative-progress storage with carry-forward reads (used by games and books progress graphs)
- **`mocks/`** — Shared mock implementations
- **`testhelper/`** — Test utilities: `ConnectTestDB(dsn)` wraps `postgres.Connect` for integration tests; `BuildMux(Routable)` constructs a test `http.Handler` from any app that implements `Routes`/`GetName`

### Key Libraries

| Concern | Library |
| --- | --- |
| HTTP | `net/http` + `justinas/alice` (middleware chaining) |
| RPC | `connectrpc.com/connect` — HTTP/1.1 RPC framework |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` (migrations) |
| Auth | `supabase-community/gotrue-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `xdoubleu/essentia/v4` threading.JobQueue |
| Code generation | `buf` / `protoc-gen-go` / `protoc-gen-connect-go` |
| Testing | `stretchr/testify` |

### Apps

- **games** — Steam backlog tracker: library sync, achievements, completion rate progress/distribution, and the user's Steam integration settings. External client package lives in `pkg/steam/`. Has a background sync job (1 worker) and WebSocket live updates. Uses `games` DB schema (adopted from the former `backlog` schema).
- **books** — Book library and e-reader companion. Book metadata enrichment uses Open Library then Google Books as fallback (set `GOOGLE_BOOKS_API_KEY` for higher rate limits); ISBN-less books are matched by title+author. External client packages live in `pkg/openlibrary/`, `pkg/googlebooks/`, `pkg/unicat/`. Serves the raw Kobo sync protocol under `/books/kobo/{token}/…` and a public cover proxy. Has background jobs (2 workers) and WebSocket live updates. Uses `books` DB schema (adopted from the former `backlog` schema).
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no background jobs.
- **icsproxy** — ICS calendar feed filtering and proxying. Uses `icsproxy` DB schema.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and whole-recipe-book sharing with contacts (`recipebook_access`, view-only or edit). Uses `recipes` DB schema.
- **shoppinglist** — Custom items plus meal-plan ingredient aggregation, with user-defined categories, a name→category catalog, and per-store category ordering that drives a store-ordered (Apple Notes) export. The whole list is shareable with contacts (`shoppinglist_access`, view-only or edit); data RPCs accept an `owner_user_id` so a recipient can act on a shared owner's list. Uses `shoppinglist` DB schema.
- **mealplans** — Weekly meal planning with per-plan iCal feeds and plan sharing with contacts. Uses `mealplans` DB schema (its `plans` tables were adopted from the `recipes` schema — the same `ALTER TABLE … SET SCHEMA` pattern later used for the games/books split).
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, search, and background archive jobs. Uses `todos` DB schema.

### Database Conventions

- Each app uses its own PostgreSQL schema (e.g., `books`, `icsproxy`)
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
