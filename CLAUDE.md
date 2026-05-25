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
make lint/pkg PKG=apps/recipes  # Lint a single package

# Code generation (TypeScript, run from web/)
yarn generate              # Regenerate web/lib/gen/ from proto definitions
```

> All `make` commands must be run from the `api/` directory. All `yarn` commands must be run from the `web/` directory.

## Agent Delegation

**For any task that writes, modifies, or refactors code, always spawn the `orchestrate-analyze` subagent — never implement inline.**

`orchestrate-analyze` will analyze the codebase, identify what needs to change, then delegate to `orchestrate-backend` and/or `orchestrate-frontend` for execution.

Inline tool calls (Read, Edit, Write, Bash) are only permitted for read-only exploration and lookups, never for implementation.

## Code Navigation (ast-grep)

**Prefer `ast-grep` over `grep` for any code search.** It understands syntax trees so results are exact — no false positives from comments or strings.

```bash
# Find all call sites of a function (Go)
ast-grep run --pattern '$$.FunctionName($$$)' --lang go

# Find a function definition (Go)
ast-grep run --pattern 'func FunctionName($$$) $$$' --lang go

# Find all call sites (TypeScript)
ast-grep run --pattern 'functionName($$$)' --lang typescript

# Find interface/type usage (TypeScript)
ast-grep run --pattern 'const $VAR: TypeName = $$$' --lang typescript

# Scope to a subtree
ast-grep run --pattern '...' --lang go api/apps/recipes/
```

Key patterns:

- `$NAME` — matches any single node (identifier, expression)
- `$$$` — matches zero or more nodes (argument lists, body)
- `$$` — matches a single node that can be a complex expression

Use ast-grep **instead of reading `web/lib/gen/` or `api/gen/`** to find field names and RPC signatures — search the `.proto` files with ast-grep or read them directly.

To run a single test:
```bash
# Go
go test ./apps/backlog/... -run TestFunctionName

# Web (Jest accepts a path/name pattern as a positional arg — no flag needed)
cd web && yarn test:single MealPlanCalendar        # matches by filename
cd web && yarn test:single -t "renders correctly"  # matches by test name
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

### ConnectRPC Auth Handler Conventions

The `GetCurrentUser` handler (in `api/cmd/api/connect_auth_handlers.go`) uses a two-layer role resolution pattern:

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

Generated files: `api/gen/` Go proto stubs ARE committed. `web/lib/gen/` TypeScript clients ARE committed — only run the generators after editing `.proto` files (CI regenerates and commits them automatically via `build.yml`).

**Do not read `web/lib/gen/` or `api/gen/` files to discover field names, message types, or RPC signatures.** Read the corresponding `.proto` file in `proto/` instead — it is much smaller and is the source of truth. Generated files are 5–10× larger and contain the same information.

### Proto code generation (both must run when a `.proto` file changes)

```bash
# From api/  — regenerates Go stubs into api/gen/
make proto/generate

# From web/  — regenerates TypeScript clients into web/lib/gen/
yarn generate
```

These two commands are always paired. A proto change without running both will leave one side stale.

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

See `.github/workflows/` for the full pipeline. Five workflows fan out from `main.yml`: `build`, `api-lint`, `api-test`, `web-lint`, `web-test`.

