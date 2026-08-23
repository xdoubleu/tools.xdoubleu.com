# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools. The API is built with Go 1.26, PostgreSQL, and first-party authentication (bcrypt + TOTP). The frontend is built with Next.js 16, React 19, and TypeScript.

## Tools

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress and distribution, favourite games, with background sync jobs and WebSocket live updates.
- **books** — Book library and e-reader companion. External metadata sync (UniCat, Hardcover) and EPUB/PDF uploads, converted to KEPUB and synced to Kobo devices per-item. Devices sync against `/books/kobo/<token>/…`; devices set up under an older prefix (`/reading/kobo/…` or `/backlog/kobo/…`) must re-run the setup flow. Setup is entirely driven by **kobo-gateway** (`kobo-gateway/`), a downloadable macOS menu-bar app the books page drives over a loopback-only HTTP API — built on a macOS CI runner (its menu bar needs cgo + AppKit) and served as a `.dmg` at `/downloads/kobo-gateway.dmg`, so kobo-gateway code changes rebuild the *web* image too (see the `kobo_gateway` path filter in `main.yml`). Unrelated to the separate `gateway/` module below, which routes requests and supervises the `api`/`web` processes inside the merged deploy container.
- **watchparty** — WebRTC screen sharing with draggable camera overlays for real-time collaboration.
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
- **Authentication**: first-party (bcrypt password hashing, TOTP MFA via `pquerna/otp`, self-issued JWT sessions) — `api/internal/auth`
- **Job queue**: `api/internal/threading` + `api/internal/jobqueue` for background work
- **Frontend**: Next.js 16, React 19, TypeScript, Tailwind + shadcn/ui

Each tool uses its own PostgreSQL schema. Shared Go code lives in `api/internal/` (auth, config, encryption, templates, repositories).

**Deploy shape:** `api` and `web` each build their own Docker image
(`api/Dockerfile`, `web/Dockerfile`) and deploy as two independent Kamal
services (`config/deploy.api.yml`, `config/deploy.web.yml`) to a Hetzner VPS,
sharing one kamal-proxy instance and domain. kamal-proxy routes `/api/*` and
`/.well-known/*` to `api`, everything else to `web` — replicating the
ingress split a hand-rolled `gateway/` Go module used to provide as PID 1 in
a single merged container (issue #558 merged them back when the target was
DigitalOcean App Platform, which bills per component; DO was decommissioned
in #1113, and `gateway/` was retired along with the merge in #1038).
`api` pulls its slog→Sentry logging glue from a second, tiny Go module,
`sentrytools/` (own `go.mod`, no deployable artifact of its own), via a
local `replace` directive rather than duplicating it.

## Apps MCP server

Every app's own **read-only** data — plus the admin observability signals — is
exposed to a locally-running Claude CLI over a single streamable-HTTP MCP
server at `/apps/mcp`, so production domain data and system health can be
pulled in as context for testing/verifying changes. Every app tool wraps an
existing **read** RPC of an app (games, books, recipes, mealplans,
shoppinglist, todos) — no per-app tool ever mutates. App tools are
named `<app>_<rpc>` (e.g. `games_get_steam`, `books_search_library`,
`todos_list_tasks`); the 12 observability tools are unprefixed
(`get_job_stats`, `get_usage_stats`, `get_storage_stats`,
`get_database_stats`, `get_failing_pull_requests`, `get_security_alerts`,
`get_failing_main_runs`, `get_sentry_issues`, `resolve_sentry_issue`,
`get_deploy_status`, `get_deploy_logs`, `get_slow_transactions`). One of those,
`resolve_sentry_issue`, is a deliberate exception to read-only: it marks a
Sentry issue resolved, so an admin-authenticated agent can close out an issue
it just filed a fix for.

Point a local Claude Code at it (OAuth is handled automatically — no header):

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

Auth is **MCP OAuth 2.1**, entirely first-party as of issue #1039: the api is
both the OAuth resource server (it verifies the Bearer token and advertises
protected-resource metadata) **and** the authorization server — an embedded
`ory/fosite`-backed AS (`api/internal/oauth2as`) serving RFC 7591 dynamic
client registration and RFC 8414 metadata directly, with no external Auth
provider involved. The `/oauth/consent` page (web) shows the approval screen,
calling the api's own `/oauth2/*` endpoints. On first use Claude Code
discovers the metadata, dynamically registers, runs the PKCE flow against
the api (a browser consent screen opens), and then calls the server with the
issued token. Authorization differs per tool: the app tools are gated by the
**caller's own per-app access** (admin, or the app in their app-access list)
and return only that signed-in user's own data, exactly what they can
already see over HTTP; the observability tools require the signed-in user to
be an **admin**.

No external setup is required — `JWT_SECRET` and `OAUTH_HMAC_SECRET` are the
only auth-related secrets (see
[`config/deploy.api.yml`](config/deploy.api.yml) for the full env list).

## Deploy Notes

**Where deploys happen:** every push to `main` builds one merged `app` image
and `.github/workflows/main.yml`'s `deploy-kamal` job ships it to the Hetzner
VPS with Kamal. [`config/deploy.api.yml`](config/deploy.api.yml) and
[`config/deploy.web.yml`](config/deploy.web.yml) are the deploy configs
(committed, read as-is — Kamal evaluates each as ERB), and every app secret is
a **repo Secret**; see [`infra/README.md`](infra/README.md) for the full list,
the one-time host bootstrap, and how to deploy or roll back by hand.
OpenTofu under `infra/` provisions the host only — it does not deploy the app.
DigitalOcean App Platform, which hosted this before #1029/#1034, was
decommissioned in #1113.

**Two independent services (issue #558, split into 3 processes in #904,
split back into 2 independent services in #1038):** `api` and `web` deploy
as two independent Kamal services on the VPS, sharing one kamal-proxy
instance and domain. kamal-proxy routes `/api/*` and `/.well-known/*` to
`api` (unstripped — `api/cmd/api/kamal_proxy_shim.go` strips `/api` itself),
everything else to `web`. This replaces a hand-rolled `gateway/` Go module
that used to be PID 1 in a single merged container and reverse-proxy
between `api` and the Next.js child; `GOMEMLIMIT=300MiB` on `api`'s own
config is a leftover soft ceiling from when it shared a container's memory
with the Node child — worth re-tuning now that `api` runs in its own
container, but not re-sized as part of #1038. Watch `docker stats` on the
VPS before changing it.

**R2 bucket CORS:** the in-browser EPUB/KEPUB book preview reads file bytes client-side, so
each R2 bucket must have a CORS rule allowing `GET`/`HEAD` from its environment's web origin
(`http://localhost:3000` dev, `https://tools.xdoubleu.com` prod). See
[api/CLAUDE.md](api/CLAUDE.md) for the exact rule and how to apply it. This must be
re-applied if a bucket is recreated.

**GitHub/Sentry/DigitalOcean OAuth (observability integrations, issue #440):** each
provider needs its own OAuth App registered once, with callback URL
`https://tools.xdoubleu.com/api/admin/oauth/{provider}/callback` (`github`, `sentry`,
`digitalocean`). The resulting client id/secret pairs, plus a generated
`ENCRYPTION_KEY` (`openssl rand -base64 32`), are set as repo Secrets and
listed in [`config/deploy.api.yml`](config/deploy.api.yml)'s `env.secret`:

```bash
gh secret set SENTRY_OAUTH_CLIENT_ID   # etc. — see infra/README.md for all names
```

They take effect on the next deploy to `main`. Note the `digitalocean`
provider is a *monitoring integration* and is unrelated to where this app is
hosted — it survived DO's decommissioning as a host (#1113).

Until all six vars are set, the api logs a startup warning per missing provider and
its "Connect" button on `/monitoring` fails with a provider-side error instead of
one from this app. See [api/CLAUDE.md](api/CLAUDE.md) for the full connect-flow
mechanics.

**New-issue notification emails (issue #561):** a background job (`notify-new-issues`,
runs every 5 minutes) emails an admin the first time a new unresolved Sentry issue or a
failed DigitalOcean deployment is seen, via the [Resend](https://resend.com) API (free
tier). Set `RESEND_API_KEY`, `EMAIL_FROM`, and `NOTIFY_EMAIL_TO` (repo Secrets,
same as above). Any unset
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
3. Set `EMAIL_INBOUND_DOMAIN` to the receiving domain from step 1 (repo Secrets,
   same as above).

Either var unset disables the feature: `CreateFeed(kind=EMAIL)` refuses to mint an
address, and the webhook endpoint rejects every request rather than accepting unsigned
mail.

## Contributing

Refer to [CLAUDE.md](CLAUDE.md) for detailed development guidelines, testing practices, and linting standards. Always run `make lint/fix` (from `api/`) before committing.
