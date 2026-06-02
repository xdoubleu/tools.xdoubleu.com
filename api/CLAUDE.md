# api/ — Backend

Go 1.26 backend for tools.xdoubleu.com. Run all `make` commands from this directory.

## Common Commands

```bash
# Local DB
docker-compose up -d        # Start local PostgreSQL 18 (from repo root)

# Build
make build                  # Build ./bin/api (main server binary)

# Testing
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)
make test/cov/per-pkg       # Per-package coverage with merged report

# Single test
go test ./apps/backlog/... -run TestFunctionName

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues
make lint/pkg PKG=apps/recipes  # Lint a single package

# Proto code generation (also run `yarn generate` from web/ — they're paired)
make proto/generate
```

## Architecture

A Go monorepo that serves multiple web apps from a single binary. All apps are registered in `cmd/api/apps.go` and share a single HTTP mux routed by URL prefix. Apps expose ConnectRPC endpoints consumed by the Next.js frontend in `web/`.

### App Structure

Each app lives in `apps/<name>/` and follows a consistent layout:

```
apps/<name>/
├── app.go              # App struct embedding app.Base, implements App interface
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

- **`app.Base`** — Embedded struct providing logger, config, templates, and auth service to every app
- **`app.HTTPError`** — Shared HTTP error type (`Status int`, `Message string`); import as `iapp "tools.xdoubleu.com/internal/app"` in handler files to avoid collision with the app struct
- **`auth/`** — Supabase GoTrue authentication (`gotrue-go`)
- **`config/`** — Centralized config loaded from `.env` via `xdoubleu/essentia/v4`
- **`constants/`** — Shared constants
- **`contacts/`** — Contact management service (used by recipes for sharing)
- **`crypto/`** — Encryption utilities
- **`models/`** — Shared domain models
- **`repositories/`** — Shared DB repositories
- **`templates/`** — Shared utility functions (date formatting, fraction parsing, etc.)
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

- **backlog** — Goals/backlog tracker with external sync (Steam, Hardcover/Goodreads). Has background jobs (2 workers) and WebSocket live updates. Uses `backlog` DB schema.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no background jobs.
- **icsproxy** — ICS calendar feed filtering and proxying. Uses `icsproxy` DB schema.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and contact-based sharing. Uses `recipes` DB schema.
- **shoppinglist** — Custom items plus meal-plan ingredient aggregation, with user-defined categories, a name→category catalog, and per-store category ordering that drives a store-ordered (Apple Notes) export. Uses `shoppinglist` DB schema.
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, search, and background archive jobs. Uses `todos` DB schema.

### Database Conventions

- Each app uses its own PostgreSQL schema (e.g., `backlog`, `icsproxy`)
- Migrations live in `apps/<name>/migrations/` and follow Goose SQL format
- `updated_at` columns are managed via PostgreSQL triggers
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking

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
