# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools. The API is built with Go 1.26, PostgreSQL, and Supabase authentication. The frontend is built with Next.js 16, React 19, and TypeScript.

## Tools

- **backlog** — Goals/backlog tracker with external sync (Steam, Hardcover/Goodreads), background jobs, and WebSocket live updates.
- **watchparty** — WebRTC screen sharing with draggable camera overlays for real-time collaboration.
- **icsproxy** — Calendar (ICS) feed filtering and proxying with event hiding and holiday management.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and whole-recipe-book sharing with contacts (view-only or edit).
- **shoppinglist** — Shopping list with meal-plan ingredient aggregation, item categories, store-ordered export (group items by the aisle order of the store you're visiting), and full-list sharing with contacts (switch between your own and shared lists).
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, and search.

## Quick Start

```bash
# Start the database
cd api && docker-compose up -d

# Run development server (API)
cd api && make run

# Run development server (Web)
cd web && yarn dev

# Run tests (API)
cd api && make test

# Run tests (Web)
cd web && yarn test

# Build the API binary
cd api && make build

# Build the web frontend
cd web && yarn build

# Stop the database
cd api && docker-compose down
```

## API Commands (`api/`)

| Command | Purpose |
| --- | --- |
| `make test` | Run all tests |
| `make test/cov/report` | Generate coverage report (HTML) |
| `make test/cov/per-pkg` | Per-package coverage with merged summary |
| `make lint` | Run all linters (Go + SQL) |
| `make lint/fix` | Auto-fix linting issues |
| `make scaffold NAME=myapp [DB=true] [JOBS=true]` | Generate new app |

## Web Commands (`web/`)

| Command | Purpose |
| --- | --- |
| `yarn dev` | Start development server |
| `yarn build` | Build static export |
| `yarn test` | Run tests |
| `yarn test:cov` | Run tests with coverage |
| `yarn lint` | Run ESLint + Prettier |
| `yarn generate` | Regenerate TypeScript ConnectRPC clients from proto definitions (output: `web/lib/gen/`, committed) |
| `yarn lint:fix` | Auto-fix ESLint issues and reformat with Prettier |

## Architecture

All tools are registered in `api/cmd/api/apps.go` and share a single HTTP mux routed by URL prefix. Each tool lives in `api/apps/<name>/` with a consistent structure:

- **HTTP**: `net/http` + `justinas/alice` middleware
- **RPC**: `connectrpc.com/connect` — proto definitions in `proto/<app>/v1/`; Go stubs committed to `api/gen/`; TypeScript clients generated to `web/lib/gen/` (rebuilt in CI)
- **Database**: `jackc/pgx/v5` + `pressly/goose/v3` migrations
- **Authentication**: Supabase GoTrue
- **Job queue**: `xdoubleu/essentia/v4` for background work
- **Frontend**: Next.js 16, React 19, TypeScript, Tailwind + shadcn/ui

Each tool uses its own PostgreSQL schema. Shared Go code lives in `api/internal/` (auth, config, encryption, templates, repositories).

## Adding a New Tool

```bash
# Minimal tool (no DB, no background jobs)
cd api && make scaffold NAME=mytool

# Tool with database
cd api && make scaffold NAME=mytool DB=true

# Tool with database and background jobs
cd api && make scaffold NAME=mytool DB=true JOBS=true
```

After scaffolding:

1. Register the new app in `api/cmd/api/apps.go` (the scaffold command does not auto-register it)
2. Implement handlers and register routes in `api/apps/mytool/routes.go`
3. Add domain logic to `api/apps/mytool/internal/`
4. If using DB, edit `api/apps/mytool/migrations/00001_init.sql`
5. Run `cd api && make build` to verify

## Deploy Notes

**R2 bucket CORS:** the in-browser EPUB/KEPUB book preview reads file bytes client-side, so
each R2 bucket must have a CORS rule allowing `GET`/`HEAD` from its environment's web origin
(`http://localhost:3000` dev, `https://tools.xdoubleu.com` prod). See
[api/CLAUDE.md](api/CLAUDE.md) for the exact rule and how to apply it. This must be
re-applied if a bucket is recreated.

## Contributing

Refer to [CLAUDE.md](CLAUDE.md) for detailed development guidelines, testing practices, and linting standards. Always run `make lint/fix` (from `api/`) before committing.
