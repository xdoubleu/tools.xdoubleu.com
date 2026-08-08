# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools. The API is built with Go 1.26, PostgreSQL, and Supabase authentication. The frontend is built with Next.js 16, React 19, and TypeScript.

## Tools

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress and distribution, favourite games, with background sync jobs and WebSocket live updates.
- **books** — Book library and e-reader companion. External metadata sync (UniCat, Hardcover) and EPUB/PDF uploads, converted to KEPUB and synced to Kobo devices per-item. Devices sync against `/books/kobo/<token>/…`; devices set up under an older prefix (`/reading/kobo/…` or `/backlog/kobo/…`) must re-run the setup flow. Setup is entirely driven by **kobo-gateway** (`kobo-gateway/`), a downloadable macOS menu-bar app the books page drives over a loopback-only HTTP API — built on a macOS CI runner (its menu bar needs cgo + AppKit) and served as a `.dmg` at `/downloads/kobo-gateway.dmg`, so kobo-gateway code changes rebuild the *web* image too (see the `kobo_gateway` path filter in `main.yml`). Unrelated to the separate `gateway/` module below, which routes requests and supervises the `api`/`web` processes inside the merged deploy container.
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

## Kobo Gateway Commands (`kobo-gateway/`, macOS only)

| Command | Purpose |
| --- | --- |
| `make build` | Build the kobo-gateway binary (needs cgo + Xcode command line tools) |
| `make dist` | Package into `dist/kobo-gateway/`: `KoboGateway.app` → `.dmg`, plus the raw binary |
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

**Deploy shape:** `api`, `web`, and `gateway` build into one Docker image
(root `Dockerfile`) and run as a single DigitalOcean App Platform component
(issue #558 — App Platform bills per component, and api/web both used to
run on separate smallest-tier instances; split into 3 processes in #904).
`gateway` (`gateway/`, its own Go module) is PID 1; it spawns both `api` and
the Next.js standalone server as supervised children and reverse-proxies
every request between them, stripping `/api` for the api child and routing
everything else to the web child — the same split the two-component DO
ingress used to provide. `api` itself has no awareness of any of this.

## Apps MCP server

Every app's own **read-only** data — plus the admin observability signals — is
exposed to a locally-running Claude CLI over a single streamable-HTTP MCP
server at `/apps/mcp`, so production domain data and system health can be
pulled in as context for testing/verifying changes. Every app tool wraps an
existing **read** RPC of an app (games, books, recipes, mealplans,
shoppinglist, todos, icsproxy) — no per-app tool ever mutates. App tools are
named `<app>_<rpc>` (e.g. `games_get_steam`, `books_search_library`,
`todos_list_tasks`); the 9 observability tools are unprefixed
(`get_job_stats`, `get_usage_stats`, `get_storage_stats`,
`get_database_stats`, `get_failing_pull_requests`, `get_sentry_issues`,
`resolve_sentry_issue`, `get_deploy_status`, `get_deploy_logs`). One of those,
`resolve_sentry_issue`, is a deliberate exception to read-only: it marks a
Sentry issue resolved, so an admin-authenticated agent can close out an issue
it just filed a fix for.

Point a local Claude Code at it (OAuth is handled automatically — no header):

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

Auth is **MCP OAuth 2.1**: the api is the OAuth resource server (it verifies the
Bearer token and advertises protected-resource metadata), **Supabase Auth is the
authorization server**, and the `/oauth/consent` page (web) shows the approval
screen. On first use Claude Code discovers the metadata, dynamically registers,
runs the PKCE flow against Supabase (a browser consent screen opens), and then
calls the server with the issued token. Authorization differs per tool: the app
tools are gated by the **caller's own per-app access** (admin, or the app in
their app-access list) and return only that signed-in user's own data, exactly
what they can already see over HTTP; the observability tools require the
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

**Merged single-component deploy (issue #558, split into 3 processes in
#904):** the api/web/gateway env vars all live on one `app` component in
[`do-app.yaml`](do-app.yaml). `gateway` (`gateway/`) is PID 1 and spawns
both `api` and the Next.js child; env vars only needed for local debugging
(not set in production, defaults match): `WEB_PORT`/`WEB_NODE_BIN`/
`WEB_SERVER_JS` (`3000`/`node`/`/app/web/server.js`) for the web child, and
`API_PORT`/`API_BIN_PATH` (`8001`/`/app/bin/api`) for the api child.
`GOMEMLIMIT=300MiB` is a soft ceiling so the Go GC(s) don't crowd out the
Node child's `NODE_OPTIONS=--max-old-space-size=192` inside the shared
512 MB instance — not yet re-tuned for the extra Go runtime #904 added, so
watch `docker stats` after that deploy specifically. The merge is only
cost-neutral-or-better if peak memory (steady-state plus a PDF→EPUB
conversion) stays under the 512 MB instance; if `docker stats` on the
deployed image shows it running close to that ceiling, move `do-app.yaml`'s
`app` component to the 1 GB tier rather than let it OOM — at that point the
~$5/mo saving from merging is gone and the two-component shape (revert this
change) is no worse.

**R2 bucket CORS:** the in-browser EPUB/KEPUB book preview reads file bytes client-side, so
each R2 bucket must have a CORS rule allowing `GET`/`HEAD` from its environment's web origin
(`http://localhost:3000` dev, `https://tools.xdoubleu.com` prod). See
[api/CLAUDE.md](api/CLAUDE.md) for the exact rule and how to apply it. This must be
re-applied if a bucket is recreated.

**GitHub/Sentry/DigitalOcean OAuth (observability integrations, issue #440):** each
provider needs its own OAuth App registered once, with callback URL
`https://tools.xdoubleu.com/api/admin/oauth/{provider}/callback` (`github`, `sentry`,
`digitalocean`). The resulting client id/secret pairs, plus a generated
`ENCRYPTION_KEY` (`openssl rand -base64 32`), are declared as `SECRET`
placeholders in [`do-app.yaml`](do-app.yaml) but must be pushed to the *live* DO App
explicitly — editing `do-app.yaml` alone doesn't update a running app:

```bash
doctl apps update <DO_APP_ID> --spec do-app.yaml
# then set each secret's value via `doctl apps update --env` or the DO dashboard
```

Until all six vars are set, the api logs a startup warning per missing provider and
its "Connect" button on `/monitoring` fails with a provider-side error instead of
one from this app. See [api/CLAUDE.md](api/CLAUDE.md) for the full connect-flow
mechanics.

**New-issue notification emails (issue #561):** a background job (`notify-new-issues`,
runs every 5 minutes) emails an admin the first time a new unresolved Sentry issue or a
failed DigitalOcean deployment is seen, via the [Resend](https://resend.com) API (free
tier). Set `RESEND_API_KEY`, `EMAIL_FROM`, and `NOTIFY_EMAIL_TO` (also `SECRET`
placeholders in [`do-app.yaml`](do-app.yaml), pushed the same way as above). Any unset
var makes the job a no-op — it still runs and records nothing, rather than failing.

**Email-relay newsletter feeds (issue #595):** lets a user subscribe a newsletter that
has no public RSS feed (or is paywalled, e.g. Substack) by giving it a per-feed inbound
email address instead of a feed URL; mail sent to that address is ingested into the
library the same way RSS items are. Reuses the Resend account above (inbound webhooks,
not a new provider) — one-time setup:

1. Add a **receiving** domain (e.g. `mail.tools.xdoubleu.com`) in the Resend dashboard —
   separate from the existing *sending* domain, with its own DNS MX/TXT records.
2. Register an inbound webhook endpoint (`https://tools.xdoubleu.com/api/feeds/email/inbound`)
   subscribed to the `email.received` event; copy its signing secret into
   `EMAIL_INBOUND_SECRET`.
3. Set `EMAIL_INBOUND_DOMAIN` to the receiving domain from step 1 (also `SECRET`
   placeholders in [`do-app.yaml`](do-app.yaml), pushed the same way as above).

Either var unset disables the feature: `CreateFeed(kind=EMAIL)` refuses to mint an
address, and the webhook endpoint rejects every request rather than accepting unsigned
mail.

## Contributing

Refer to [CLAUDE.md](CLAUDE.md) for detailed development guidelines, testing practices, and linting standards. Always run `make lint/fix` (from `api/`) before committing.
