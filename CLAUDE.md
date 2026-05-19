# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

```bash
# Development
docker-compose up -d        # Start local PostgreSQL 18

# Build
make build                  # Build ./bin/api (main server binary; run from api/)

# Testing
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)
make test/cov/per-pkg       # Per-package coverage with merged report

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues

# Code generation (TypeScript, run from web/)
yarn generate              # Regenerate web/lib/gen/ from proto definitions
```

> All `make` commands must be run from the `api/` directory. All `yarn` commands must be run from the `web/` directory.

To run a single test:
```bash
go test ./apps/backlog/... -run TestFunctionName
```

## Architecture

This is a Go monorepo (Go 1.26) that serves multiple web apps from a single binary. All apps are registered in `api/cmd/api/apps.go` and share a single HTTP mux routed by URL prefix. The apps expose ConnectRPC endpoints (for the Next.js 16 `web/` frontend).

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

Generated files: `api/gen/` Go proto stubs ARE committed. `web/lib/gen/` TypeScript clients ARE committed — only run `yarn generate` after editing `.proto` files (CI regenerates and commits them automatically via `build.yml`).

## Testing Notes

- Use mock injection for unit tests; place mocks in `internal/mocks/` or app-level `internal/<name>/mocks/`
- Integration tests hit a real database — start `docker-compose up -d` from `api/` before running tests locally
- Target ≥80% coverage on changed code; check with `make test/cov/report`
- Generated files (`api/gen/`, `_mock.go`) are excluded from coverage. `web/lib/gen/` TypeScript files are also excluded.
- When fixing bugs, write a failing test first before implementing the fix

## Web Frontend (web/)

The `web/` directory contains a Next.js 16 App Router application served as a static export (`output: 'export'`).

### Stack

| Concern | Library |
| --- | --- |
| Framework | Next.js 16, React 19, TypeScript strict |
| Styling | Tailwind CSS + shadcn/ui |
| API client | ConnectRPC (`@connectrpc/connect-web`) |
| Data fetching | SWR |
| Error tracking | Sentry (`@sentry/nextjs`) |
| Testing | Jest + React Testing Library |
| Linting | ESLint (eslint-config-next), Prettier, tsc --noEmit, knip |

### Key Paths

- `web/app/` — App Router pages and layouts
- `web/components/` — Reusable React components (shadcn/ui primitives in `components/ui/`)
- `web/lib/` — Utilities and ConnectRPC client setup
- `web/lib/gen/` — Generated TypeScript ConnectRPC clients from buf (committed; only regenerate after editing `.proto` files)
- `web/hooks/` — SWR data-fetching hooks

### Testing

Run `cd web && yarn test:cov` for coverage. Target ≥80% on `components/`, `lib/`, `hooks/` (excludes `lib/gen/`).

## CI

`.github/workflows/main.yml` fans out to five reusable workflows:

- `build.yml` — Go proto regeneration + commit (`api/gen/`) + Go build + TS client regeneration + commit (`web/lib/gen/`) + web build
- `api-lint.yml` — `golangci-lint` + SQL lint
- `api-test.yml` — PostgreSQL 18 service + `make test/cov/report` + Codecov upload (`flags: api`)
- `web-lint.yml` — ESLint + Prettier + `tsc --noEmit` + knip
- `web-test.yml` — `yarn test:cov` + Codecov upload (`flags: web`)

