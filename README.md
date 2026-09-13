# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools. The API is built with Go 1.26, PostgreSQL, and first-party authentication (bcrypt + TOTP). The frontend is built with Next.js 16, React 19, and TypeScript.

## Tools

- **games** — Steam backlog tracker: library sync, achievements, completion-rate progress and distribution, favourite games, with background sync jobs and WebSocket live updates.
- **books** — Book library and e-reader companion. External metadata sync (UniCat, Hardcover) and EPUB/PDF uploads, converted to KEPUB and synced to Kobo devices per-item. Devices sync against `/books/kobo/<token>/…`; devices set up under an older prefix (`/reading/kobo/…` or `/backlog/kobo/…`) must re-run the setup flow. Setup is entirely driven by **kobo-gateway** (`kobo-gateway/`), a downloadable macOS menu-bar app the books page drives over a loopback-only HTTP API — built on a macOS CI runner (its menu bar needs cgo + AppKit) and served as a `.dmg` at `/downloads/kobo-gateway.dmg`, so kobo-gateway code changes rebuild the *web* image too (see the `kobo_gateway` path filter in `main.yml`). Unrelated to the separate `gateway/` module below, which routes requests and supervises the `api`/`web` processes inside the merged deploy container.
- **watchparty** — WebRTC screen sharing with draggable camera overlays for real-time collaboration.
- **recipes** — Recipe management with fraction parsing and iCal export. The recipe book is shared as a unit with everyone in your family (see **family** below).
- **shoppinglist** — Shopping list with meal-plan ingredient aggregation, item categories, and store-ordered export (group items by the aisle order of the store you're visiting). Shared as a unit with your family; stores themselves stay private per-user.
- **family** — Invite other users by email into a shared "family": one recipe book, one meal plan set and one shopping list that everyone in it sees and edits. At most one family per user; each member can set a display name. Replaces the previous per-app "share with a contact" model (issue #1349, #1403).
- **trains** — SNCB/NMBS journey planner that stays useful when a trip is delayed mid-journey: a route overview between two stations and a live journey view that re-plans an alternative when a delay breaks it (issue #1388). Routes are computed in-house from the Belgian Mobility Company's open GTFS timetable, refreshed daily. In progress — as of issue #1392 the timetable ingest, journey-search RPC and the `/trains` route overview page exist; the live, self-updating journey view is a later slice.
- **learningpaths** — Self-directed learning curricula: a path (title, goal, recurring-routine description) made of ordered modules, each with ordered items you check off as you progress, plus a freeform resources list. Scoped per-user, no family sharing. This is the foundational slice (issue #1472) of epic #1471 — MCP write tools and books/feeds resource linking land in later, independent PRs. A per-user Todoist connection (issue #1475) lets you send a path item to your own Todoist account as a one-way task.

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

Every app's own data — plus the admin observability signals — is exposed to a
locally-running Claude CLI over a single streamable-HTTP MCP server at
`/apps/mcp`, so production domain data and system health can be pulled in as
context for testing/verifying changes. Every app tool wraps an existing
**read** RPC of an app (games, books, feeds, recipes, mealplans,
shoppinglist, trains, learningpaths) — no per-app tool mutates, with one
deliberate exception: `learningpaths` also exposes three mutating tools
(`learningpaths_create_path`, `learningpaths_update_path`,
`learningpaths_record_progress`), since agent-authored curricula is that
app's core use case, not an add-on → [`docs/adr-0023-learningpaths-mcp-write-tools.md`](docs/adr-0023-learningpaths-mcp-write-tools.md).
App tools are
named `<app>_<rpc>` (e.g. `games_get_steam`, `books_search_library`,
`recipes_list_recipes`, `trains_search_journeys`); the observability tools are unprefixed
(`get_job_stats`, `get_usage_stats`, `get_storage_stats`,
`get_database_stats`, `get_failing_pull_requests`, `get_workflow_runs`,
`get_security_alerts`, `get_sentry_issues`, `resolve_sentry_issue`,
`dismiss_security_alert`, `get_slow_transactions`, `prom_query`,
`get_grafana_alerts`, among
others). `prom_query(promql)` (issue #1468) runs an arbitrary PromQL query
against Prometheus — host CPU/memory/disk, Postgres stats, api's and web's
own `/metrics` (request/job/Web-Vitals latency histograms, issue #1528) and
the `github_*`/`r2_*`/`postgres_schema_size_bytes` issue-signal gauges (issue
#1529; Sentry unresolved issues moved to a Grafana datasource plugin in
#1570) — now that Grafana + Prometheus own metrics/graphs/alerting (see
[`docs/adr-0022-prometheus-grafana-metrics.md`](docs/adr-0022-prometheus-grafana-metrics.md));
it replaced four narrower tools (`get_host_metrics`,
`get_database_size_history`, `get_transaction_latency_history`, and
`get_alert_states`). `get_grafana_alerts` (issue #1564) is the read path
for alert *state*: alerting is Grafana-managed now and Grafana-managed
alerts never appear in Prometheus `ALERTS{}`, so it queries Grafana's own
ruler API for each rule's current state and active instances.
Two tools are a deliberate
exception to read-only: `resolve_sentry_issue` marks a Sentry issue
resolved, and `dismiss_security_alert` dismisses/resolves an open GitHub
Dependabot, code-scanning, or secret-scanning alert — so an
admin-authenticated agent can close out an issue it just filed a fix for, or
an alert it's already triaged, without switching to github.com/sentry.io.

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
be an **admin**. The `trains_*` tools (`trains_search_stations`,
`trains_search_journeys`, `trains_get_journey_detail`, `trains_get_feed_info`)
are still gated by trains app-access, but the SNCB/NMBS timetable and its
realtime overlay are public data shared by every user, so they return the
same rows for any caller who holds that access. `trains_get_journey_detail`
reports only the *current* live state of a journey — the realtime feed is
replaced wholesale every 30s and nothing is retained.

No external setup is required — `JWT_SECRET` and `OAUTH_HMAC_SECRET` are the
only auth-related secrets for the MCP flow (see
[`config/deploy.api.yml`](config/deploy.api.yml) for the full env list).

The same embedded AS also acts as an **OpenID Connect provider** (issue #1469):
RS256 ID tokens signed with `OAUTH_OIDC_PRIVATE_KEY` and published at
`/oauth2/jwks`, the `openid`/`profile`/`email` scopes alongside
`offline_access`, and support for confidential clients (`client_secret_basic` /
`client_secret_post`). One static confidential client, `grafana`, is seeded for
Grafana SSO — its secret comes from `OAUTH_GRAFANA_CLIENT_SECRET` and its ID
token carries a `role` claim (`Admin`) only for admin users, so with Grafana's
`role_attribute_strict` a non-admin can't complete Grafana SSO — Grafana access
is admin-only. See
[`docs/adr-0021-oauth-as-general-purpose-oidc-idp.md`](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md).

## Deploy Notes

**Where deploys happen:** every push to `main` builds one merged `app` image
and `.github/workflows/main.yml`'s `deploy-kamal` job ships it to the Hetzner
VPS with Kamal. [`config/deploy.api.yml`](config/deploy.api.yml),
[`config/deploy.web.yml`](config/deploy.web.yml), and
[`config/deploy.grafana.yml`](config/deploy.grafana.yml) (issue #1468) are the
deploy configs (committed, read as-is — Kamal evaluates each as ERB), and
every app secret is
a **repo Secret**; see [`infra/README.md`](infra/README.md) for the full list,
the one-time host bootstrap, and how to deploy or roll back by hand. Each
secret name is declared in three places that must stay in sync — a deploy
config's `env.secret:`, `.kamal/secrets`, and `main.yml`'s `deploy-kamal`
`env:` block — and `api`'s `make lint/kamal-secrets` (CI job `API Kamal
Secrets Lint`) fails a PR that leaves them inconsistent (issue #1405).
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

**A third Kamal service, `grafana` (issue #1468):** Grafana joins `api`/`web`
on the same shared kamal-proxy instance and domain, at `/grafana`, replacing
the hand-rolled metrics dashboard that used to live at
`/monitoring/observability`. Prometheus + `postgres_exporter` back it as
Tofu-managed compose accessories (like Postgres/node_exporter), not Kamal
services — see [`infra/README.md`](infra/README.md) and
[`docs/adr-0022-prometheus-grafana-metrics.md`](docs/adr-0022-prometheus-grafana-metrics.md).
The Prometheus datasource and the host/Postgres/API/overview dashboards are
provisioned from `infra/grafana/` (baked into the wrapper image, issue #1527);
the dashboard JSON is the source of truth, checked by `make lint/grafana` and
`make grafana/verify`.

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

**Issue-signal alerting (issues #561, #1529):** the realtime "email me the first
time a new issue is seen" job (`IssueNotifierJob` / `global.notified_issues`) was
retired in #1541. GitHub failing-PR / red-main-CI / open-security-alert counts,
unresolved-Sentry count, and R2 orphaned-object / storage-byte totals are now
exported as Prometheus gauges by `IssueSignalCollectorJob` and alerted on by
Grafana's `service-health` alert group
([`docs/adr-0022-prometheus-grafana-metrics.md`](docs/adr-0022-prometheus-grafana-metrics.md)),
delivered through Grafana's own SMTP contact point. `RESEND_API_KEY` /
`EMAIL_FROM` / `NOTIFY_EMAIL_TO` (repo Secrets) are still used — by the two
weekly digest emails and the Ubuntu-release-check timer; any unset var makes
those a no-op rather than failing.

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
