# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Commands

```bash
# Development
docker-compose up -d        # Start local PostgreSQL 18

# Build
make build                  # Build ./bin/publish (main server binary)

# Testing
make test                   # Run all tests
make test/v                 # Verbose output
make test/race              # With race detector
make test/cov/report        # Coverage report (HTML, excludes mocks)

# Linting
make lint                   # Run all linters (Go + SQL)
make lint/fix               # Auto-fix issues

# Scaffolding
make scaffold NAME=myapp [DB=true] [JOBS=true]   # Generate a new app
```

To run a single test:
```bash
go test ./apps/backlog/... -run TestFunctionName
```

## Architecture

This is a Go monorepo (Go 1.25) that serves multiple web apps from a single binary. All apps are registered in `cmd/publish/apps.go` and share a single HTTP mux routed by URL prefix.

### App Structure

Each app lives in `apps/<name>/` and follows a consistent layout:

```
apps/<name>/
├── app.go              # App struct embedding app.Base, implements App interface
├── routes.go           # HTTP route registration
├── handler.go          # HTTP handlers
├── internal/
│   ├── dtos/           # Request/response serialization
│   ├── models/         # Domain models
│   ├── repositories/   # DB access layer (pgx/v5)
│   ├── services/       # Business logic
│   ├── jobs/           # Background jobs (if any)
│   ├── helper/         # App-specific utilities
│   └── mocks/          # Mock implementations for testing
├── pkg/                # Reusable packages (external client integrations)
├── migrations/         # Goose SQL migrations (per-app schema)
└── templates/html/     # Embedded HTML templates
```

### Shared Internal Packages (`internal/`)

- **`app.Base`** — Embedded struct providing logger, config, templates, and auth service to every app
- **`auth/`** — Supabase GoTrue authentication (`gotrue-go`)
- **`config/`** — Centralized config loaded from `.env` via `xdoubleu/essentia/v4`
- **`crypto/`** — Encryption utilities
- **`repositories/`** — Shared DB repositories
- **`templates/`** — Shared HTML templates
- **`mocks/`** — Shared mock implementations

### Key Libraries

| Concern | Library |
|---|---|
| HTTP | `net/http` + `justinas/alice` (middleware chaining) |
| Database | `jackc/pgx/v5` + `pressly/goose/v3` (migrations) |
| Auth | `supabase-community/gotrue-go` |
| WebSocket | `coder/websocket` |
| Error tracking | `getsentry/sentry-go` |
| Job queue | `xdoubleu/essentia/v4` threading.JobQueue |
| Testing | `stretchr/testify` |

### Apps

- **backlog** — Goals/backlog tracker with external sync (Steam, Hardcover/Goodreads). Has background jobs (2 workers) and WebSocket live updates. Uses `backlog` DB schema.
- **watchparty** — WebRTC screen sharing with draggable camera overlays. No DB, no background jobs.
- **icsproxy** — ICS calendar feed filtering and proxying. Uses `icsproxy` DB schema.

### Database Conventions

- Each app uses its own PostgreSQL schema (e.g., `backlog`, `icsproxy`)
- Migrations live in `apps/<name>/migrations/` and follow Goose SQL format
- `updated_at` columns are managed via PostgreSQL triggers
- CI runs tests against a real PostgreSQL 18 instance — no DB mocking

### Adding a New App

1. Run `make scaffold NAME=myapp [DB=true] [JOBS=true]`
2. Register the app in `cmd/publish/apps.go`

## Linting

Strict linting is enforced via `golangci-lint` (40+ linters). Key constraints:
- Max line length: 88 characters (`golines`)
- Import order: standard → default → custom (`gci`)
- Max function length: 100 lines / 50 statements
- Max cyclomatic complexity: 30

Always run `make lint/fix` as the final step before committing. Manually fix anything the auto-fixer cannot resolve.

## Testing Notes

- Use mock injection for unit tests; place mocks in `internal/mocks/` or app-level `internal/<name>/mocks/`
- Integration tests hit a real database — start `docker-compose up -d` before running tests locally
- Target ≥80% coverage on changed code; check with `make test/cov/report`
- When fixing bugs, write a failing test first before implementing the fix
