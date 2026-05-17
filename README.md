# tools.xdoubleu.com

A Go monorepo serving multiple web tools from a single binary. Built with Go 1.25, PostgreSQL, and Supabase authentication.

## Tools

- **backlog** — Goals/backlog tracker with external sync (Steam, Hardcover/Goodreads), background jobs, and WebSocket live updates.
- **watchparty** — WebRTC screen sharing with draggable camera overlays for real-time collaboration.
- **icsproxy** — Calendar (ICS) feed filtering and proxying with event hiding and holiday management.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and contact-based sharing.
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, and search.

## Quick Start

```bash
# Start the database
docker-compose up -d

# Run development server
make run

# Run tests
make test

# Build the binary
make build

# Stop the database
docker-compose down
```

## Development Commands

| Command | Purpose |
| --- | --- |
| `make test` | Run all tests |
| `make test/cov/report` | Generate coverage report (HTML) |
| `make test/cov/per-pkg` | Per-package coverage with merged summary |
| `make lint` | Run all linters (Go + SQL) |
| `make lint/fix` | Auto-fix linting issues |
| `make scaffold NAME=myapp [DB=true] [JOBS=true]` | Generate new app |

## Architecture

All tools are registered in `cmd/publish/apps.go` and share a single HTTP mux routed by URL prefix. Each tool lives in `apps/<name>/` with a consistent structure:

- **HTTP**: `net/http` + `justinas/alice` middleware
- **Database**: `jackc/pgx/v5` + `pressly/goose/v3` migrations
- **Authentication**: Supabase GoTrue
- **Templates**: `templ` (source `.templ` files compiled to Go)
- **Job queue**: `xdoubleu/essentia/v4` for background work

Each tool uses its own PostgreSQL schema. Shared code lives in `internal/` (auth, config, encryption, templates, repositories).

## Adding a New Tool

```bash
# Minimal tool (no DB, no background jobs)
make scaffold NAME=mytool

# Tool with database
make scaffold NAME=mytool DB=true

# Tool with database and background jobs
make scaffold NAME=mytool DB=true JOBS=true
```

After scaffolding:

1. Implement handlers and register routes in `apps/mytool/routes.go`
2. Add domain logic to `apps/mytool/internal/`
3. If using DB, edit `apps/mytool/migrations/00001_init.sql`
4. Run `make build` to verify

## Contributing

Refer to [CLAUDE.md](CLAUDE.md) for detailed development guidelines, testing practices, and linting standards. Always run `make lint/fix` before committing.
