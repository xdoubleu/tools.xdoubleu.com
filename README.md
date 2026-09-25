# tools.xdoubleu.com

[![Main Workflow](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml/badge.svg)](https://github.com/xdoubleu/tools.xdoubleu.com/actions/workflows/main.yml)
[![codecov](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com/graph/badge.svg)](https://codecov.io/gh/xdoubleu/tools.xdoubleu.com)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A monorepo serving multiple web tools: a Go 1.26 + PostgreSQL API with
first-party auth (bcrypt + TOTP), and a Next.js 16 / React 19 / TypeScript
frontend. Architecture, commands, and conventions live in
[AGENTS.md](AGENTS.md).

## Tools

- **games** — Steam backlog tracker: library sync, achievements, completion progress and distribution, favourites, live updates over WebSocket.
- **books** — Book library and e-reader companion: metadata sync (UniCat, Hardcover), EPUB/PDF uploads converted to KEPUB and synced to Kobo devices at `/books/kobo/<token>/…` (devices on an older `/reading/kobo/…` or `/backlog/kobo/…` prefix must re-run setup). Setup is driven by **kobo-gateway** (`kobo-gateway/`), a macOS menu-bar app served as `/downloads/kobo-gateway.dmg`; it's built on a macOS runner and bundled into the *web* image, so its changes rebuild web.
- **watchparty** — WebRTC screen sharing with draggable camera overlays.
- **recipes** — Recipe management with fraction parsing and iCal export, shared with your family.
- **shoppinglist** — Shopping list with meal-plan ingredient aggregation, categories, and store-ordered export. Shared with your family; stores stay per-user.
- **family** — Invite users by email into one shared recipe book, meal plan set, and shopping list. One family per user.
- **trains** — SNCB/NMBS journey planner built on the open GTFS timetable (refreshed daily) with a realtime overlay, re-planning when a delay breaks a journey.
- **learningpaths** — Personal learning curricula: ordered modules of checkable items plus resources, with an optional one-way Todoist connection. Per-user, no family sharing.

Books and games can also be shared publicly via a revocable token link (Sharing page) at `/profile/<token>` — read-only, no account needed.

## Quick Start

```bash
cd api && docker-compose up -d   # database
cd api && make run               # API dev server
cd web && npm run dev            # web dev server
cd api && make test              # API tests
cd web && npm test               # web tests
```

Full command list: [AGENTS.md](AGENTS.md#commands). Run `make lint/fix` (from `api/`) before committing.

## Apps MCP server

`/apps/mcp` exposes every app's read RPCs as `<app>_<rpc>` tools (e.g.
`games_get_steam`, `books_search_library`, `trains_search_journeys`) plus
unprefixed admin observability tools (`prom_query`, `get_grafana_alerts`,
`get_sentry_issues`, `get_job_stats`, `get_automated_actions`, …), so a coding
agent can pull production data and system health in as context. Which tools
mutate is listed in [AGENTS.md](AGENTS.md#mcp).

- App tools are gated by the caller's per-app access and return only their own data (trains' timetable is public, so every caller with access sees the same rows). Observability tools require admin.
- `record_action` opens (`mode: "open"`) and closes (`mode: "close"`, passing back the id) a `global.automated_actions` row — the only record of a routine that runs outside the api process. `get_automated_actions` reads it; `get_job_stats` reads in-process `global.job_runs`.
- A Grafana alert in the "immediate" group POSTs to `/webhooks/grafana-alert` (bearer `ROUTINE_FIRE_TOKEN`), and `routines.Client.Fire` opens the action row before firing the routine.
- `get_grafana_alerts` reads alert state from Grafana's ruler API, since Grafana-managed alerts never appear in Prometheus `ALERTS{}`.

Connect a local Claude Code (OAuth is automatic):

```bash
claude mcp add --transport http tools-apps https://tools.xdoubleu.com/api/apps/mcp
```

OpenCode connects through `opencode.json`'s `mcp.servers` block. Auth is
first-party MCP OAuth 2.1: the api is both resource server and authorization
server (`api/internal/oauth2as`, dynamic client registration + PKCE; the web
`/oauth/consent` page shows approval) → [ADR-0006](docs/adr-0006-embedded-oauth21-authorization-server.md).
The same AS is an OIDC provider for Grafana SSO (admin-only) →
[ADR-0021](docs/adr-0021-oauth-as-general-purpose-oidc-idp.md).

## Deploy Notes

Every push to `main` builds the images and `deploy-kamal` ships them to the
Hetzner VPS with Kamal (`config/deploy.{api,web,grafana}.yml`, ERB, read
as-is). OpenTofu under `infra/` provisions the host only.
[`infra/README.md`](infra/README.md) has the full secret list, host bootstrap,
and manual deploy/rollback; secret names must agree across three places →
[convention-deploy-secrets](docs/convention-deploy-secrets.md).

- **Services:** `api` and `web` are independent Kamal services on one kamal-proxy (`/api/*`, `/.well-known/*` → `api`; the rest → `web`) → [ADR-0001](docs/adr-0001-two-service-kamal-deploy.md). `grafana` is a third at `/grafana`, backed by Tofu-managed Prometheus + postgres_exporter → [ADR-0022](docs/adr-0022-prometheus-grafana-metrics.md).
- **`GOMEMLIMIT=300MiB`** on `api` is carried over from when it shared a container with Node and hasn't been re-tuned; watch `docker stats` before changing it.
- **R2 bucket CORS:** each bucket needs a `GET`/`HEAD` CORS rule for its web origin (`http://localhost:3000` dev, `https://tools.xdoubleu.com` prod) for the in-browser book preview; re-apply if a bucket is recreated. Exact rule: [api/AGENTS.md](api/AGENTS.md).
- **PostHog:** product analytics + session replay on `web`, on for every family member with a passive disclosure in `/settings`. Needs a hand-created PostHog Cloud **EU** project and its key as `POSTHOG_KEY` → [ADR-0024](docs/adr-0024-posthog-product-analytics.md).
- **GitHub/Sentry OAuth** (monitoring integrations): register one OAuth App per provider with callback `https://tools.xdoubleu.com/api/admin/oauth/{provider}/callback`, then set the client id/secret secrets plus `ENCRYPTION_KEY` (`openssl rand -base64 32`). A missing pair logs a startup warning and that provider's Connect button fails. Flow details: [api/AGENTS.md](api/AGENTS.md).
- **Alerting:** issue signals (failing PRs, red main, security alerts, Sentry, R2 orphans) are Prometheus gauges alerted on by Grafana. `RESEND_API_KEY`/`EMAIL_FROM`/`NOTIFY_EMAIL_TO` drive the weekly digests and the Ubuntu release check; any unset makes those a no-op.
- **Email-relay newsletter feeds:** a per-feed inbound address for newsletters without RSS, via Resend inbound. One-time setup:
  1. Add a **receiving** domain (e.g. `mail.tools.xdoubleu.com`) in Resend, with its own MX/TXT records.
  2. Register the webhook `https://tools.xdoubleu.com/api/feeds/email/inbound` for `email.received`; put its signing secret in `EMAIL_INBOUND_SECRET`.
  3. Set `EMAIL_INBOUND_DOMAIN` to that domain.

  Either unset disables the feature: no addresses are minted and the webhook rejects everything.

## Contributing

[AGENTS.md](AGENTS.md) is the shared contract for any coding agent;
[CLAUDE.md](CLAUDE.md) adds Claude Code specifics.

## Agent Infrastructure

- **Claude Code** — `CLAUDE.md` + `.claude/` (skills, hooks, settings) + plugins from the `xdoubleu/skills` marketplace.
- **OpenCode** (OpenRouter models) — `opencode.json` + `.opencode/plugins/repo-guard`, reading `.claude/skills/` via compatibility discovery. Generic dependencies (`ship-pr`, `task-worktree`, `refine-issue`, `issue-triage`, `session-retro`, `grilling`, `code-review`) are installed under `.agents/skills/` via `npx skills add xdoubleu/skills …` / `npx skills add mattpocock/skills …` (tracked in `skills-lock.json`; refresh with `npx skills update`). No model is pinned — set `OPENROUTER_API_KEY` and pick one at the CLI.

Both follow [convention-task-lifecycle](docs/convention-task-lifecycle.md).
