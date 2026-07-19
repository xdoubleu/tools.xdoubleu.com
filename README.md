# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools. The API is built with Go 1.26, PostgreSQL, and Supabase authentication. The frontend is built with Next.js 16, React 19, and TypeScript.

## Tools

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress and distribution, favourite games, with background sync jobs and WebSocket live updates.
- **reading** (formerly **books**) — Reading library and e-reader companion for books, arXiv papers, web articles, and RSS posts. Each item carries a fixed category (`book`/`paper`/`article`/`rss`). Books get external metadata sync (UniCat, Hardcover) and EPUB/PDF uploads; papers are ingested from pasted arXiv URLs (metadata + PDF); articles are pasted URLs readability-extracted into EPUBs; RSS subscriptions are polled hourly and new posts land in the library automatically, with a per-feed toggle that auto-syncs every new post to a Kobo. Everything with a stored file converts to KEPUB and syncs to Kobo devices per-item. Devices sync against `/reading/kobo/<token>/…`; devices set up under an older prefix (`/books/kobo/…` or `/backlog/kobo/…`) must re-run the setup flow. Setup is entirely driven by **kobo-gateway** (`gateway/`), a downloadable macOS menu-bar app the reading page drives over a loopback-only HTTP API — built on a macOS CI runner (its menu bar needs cgo + AppKit) and served as a `.dmg` at `/downloads/kobo-gateway.dmg`, so gateway code changes rebuild the *web* image too (see the `gateway` path filter in `main.yml`).
- **watchparty** — WebRTC screen sharing with draggable camera overlays for real-time collaboration.
- **icsproxy** — Calendar (ICS) feed filtering and proxying with event hiding and holiday management.
- **recipes** — Recipe management with fraction parsing, iCal export, shopping lists, and whole-recipe-book sharing with contacts (view-only or edit).
- **shoppinglist** — Shopping list with meal-plan ingredient aggregation, item categories, store-ordered export (group items by the aisle order of the store you're visiting), and full-list sharing with contacts (switch between your own and shared lists).
- **todos** — Task management with sections, workspaces, subtasks, policies, archive, and search.

Books and games can also be shared publicly: a revocable token link (managed from the Sharing page) exposes read-only profile pages at `/profile/<token>` with the same dashboards, libraries, and backlogs — no account needed.

## Quick Start

```bash
# Start the database
cd api && docker-compose up -d

# Run development server (API)
cd api && make run

# Run development server (Web)
cd web && npm run dev

# Run tests (API)
cd api && make test

# Run tests (Web)
cd web && npm test

# Build the API binary
cd api && make build

# Build the web frontend
cd web && npm run build

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

## Gateway Commands (`gateway/`, macOS only)

| Command | Purpose |
| --- | --- |
| `make build` | Build the kobo-gateway binary (needs cgo + Xcode command line tools) |
| `make dist` | Package into `dist/gateway/`: `KoboGateway.app` → `.dmg`, plus the raw binary |
| `make test` | `go test ./...` |
| `make lint` / `make lint/fix` | `go vet` + `gofmt` |

## Web Commands (`web/`)

| Command | Purpose |
| --- | --- |
| `npm run dev` | Start development server |
| `npm run build` | Build the standalone production server |
| `npm test` | Run tests |
| `npm run test:cov` | Run tests with coverage |
| `npm run lint` | Run ESLint + Prettier |
| `npm run generate` | Regenerate TypeScript ConnectRPC clients from proto definitions (output: `web/lib/gen/`, committed) |
| `npm run lint:fix` | Auto-fix ESLint issues and reformat with Prettier |

## Architecture

All tools are registered in `api/cmd/api/apps.go` and share a single HTTP mux routed by URL prefix. Each tool lives in `api/apps/<name>/` with a consistent structure:

- **HTTP**: `net/http` + `justinas/alice` middleware
- **RPC**: `connectrpc.com/connect` — proto definitions in `proto/<app>/v1/`; Go stubs committed to `api/gen/`; TypeScript clients generated to `web/lib/gen/` (rebuilt in CI)
- **Database**: `jackc/pgx/v5` + `pressly/goose/v3` migrations
- **Authentication**: Supabase GoTrue
- **Job queue**: `xdoubleu/essentia/v4` for background work
- **Frontend**: Next.js 16, React 19, TypeScript, Tailwind + shadcn/ui

Each tool uses its own PostgreSQL schema. Shared Go code lives in `api/internal/` (auth, config, encryption, templates, repositories).

## Monitoring MCP server

The admin observability signals are exposed to a locally-running Claude CLI over
a **read-only** MCP server (streamable-HTTP) at `/monitoring/mcp`. The tools wrap
only the `ObservabilityService` read methods — no write RPC is reachable, so the
server is read-only by construction. Tools: `get_job_stats`, `get_usage_stats`,
`get_storage_stats`, `get_database_stats`, `get_github_issues`,
`get_sentry_issues`, `get_deploy_status`.

Point a local Claude Code at it (OAuth is handled automatically — no header):

```bash
claude mcp add --transport http tools-obs https://tools.xdoubleu.com/api/monitoring/mcp
```

Auth is **MCP OAuth 2.1**: the api is the OAuth resource server (it verifies the
Bearer token and advertises protected-resource metadata), **Supabase Auth is the
authorization server**, and the `/oauth/consent` page (web) shows the approval
screen. On first use Claude Code discovers the metadata, dynamically registers,
runs the PKCE flow against Supabase (a browser consent screen opens), and then
calls the server with the issued token. Every tool additionally requires the
signed-in user to be an **admin**.

**One-time Supabase setup** (dashboard → **Authentication → OAuth Server**):
enable the OAuth 2.1 server, set the **Authorization Path** to `/oauth/consent`,
enable **dynamic client registration**, and confirm the **Site URL** is
`https://tools.xdoubleu.com`. Set the web component's `SUPABASE_URL`
(`https://<project-ref>.supabase.co`) and `SUPABASE_ANON_KEY` (see
[`do-app.yaml`](do-app.yaml)). Until this is configured the endpoint returns a
401 challenge but the flow cannot complete.

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
