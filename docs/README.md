# docs/

The "why" behind this codebase. **Past tense lives here; imperative guidance
lives in the `CLAUDE.md` files.** If a sentence explains what something *used to
be*, what was *tried and rejected*, or *which incident* produced a rule, it
belongs in this directory — not in a `CLAUDE.md` and not in a code comment (see
`convention-comments-describe-current-behavior.md`).

Three kinds, distinguished by what they answer:

- **ADR** — a decision with alternatives and consequences. If it has no
  "alternatives considered", it's a spec.
- **Convention** — a rule contributors and agents must follow.
- **Spec** — how an existing subsystem actually works.

Start from `TEMPLATE-adr.md`, `TEMPLATE-convention.md`, or `TEMPLATE-spec.md`.
**When you add a document here, add it to the table below and to the
"Documented Decisions" index in the root `CLAUDE.md`** — that index is what makes
these files discoverable to a Claude session, since only `CLAUDE.md` files are
loaded automatically.

## Decisions (ADRs)

| Document | Covers | Issues |
|---|---|---|
| [adr-0001-two-service-kamal-deploy.md](adr-0001-two-service-kamal-deploy.md) | Two independent Kamal services, one proxy; web-then-api deploy order | #558, #904, #1038, #1029, #1034, #1113, #1132, #1106, #1111 |
| [adr-0002-kobo-gateway-ci-cache-split.md](adr-0002-kobo-gateway-ci-cache-split.md) | Cache written only from a `workflow_run` job; artifact handoff to web | #1322, #1192, #1347 |
| [adr-0003-buildkit-gha-cache-over-hashfiles.md](adr-0003-buildkit-gha-cache-over-hashfiles.md) | BuildKit `type=gha` cache instead of hand-computed keys | #948, #900 |
| [adr-0004-runtime-release-env-vs-compile-stamp.md](adr-0004-runtime-release-env-vs-compile-stamp.md) | `RELEASE` at runtime for api/web, compile-stamped for kobo-gateway | #1038 |
| [adr-0005-first-party-auth-replacing-gotrue.md](adr-0005-first-party-auth-replacing-gotrue.md) | First-party auth replaces Supabase GoTrue | #1039 |
| [adr-0006-embedded-oauth21-authorization-server.md](adr-0006-embedded-oauth21-authorization-server.md) | Embedded fosite AS; `offline_access` granted server-side | #1039, #1177 |
| [adr-0007-dashboard-app-owns-public-sharing.md](adr-0007-dashboard-app-owns-public-sharing.md) | Schema-less `dashboard` app owns public dashboards and share tokens | #737 |
| [adr-0008-family-as-single-sharing-concept.md](adr-0008-family-as-single-sharing-concept.md) | One `family` concept replaces per-app sharing and contacts | #1349, #1403 |
| [adr-0009-sentrytools-extracted-module.md](adr-0009-sentrytools-extracted-module.md) | slog→Sentry glue as its own module via a local `replace` | #926, #1038 |
| [adr-0010-two-weekly-digest-emails.md](adr-0010-two-weekly-digest-emails.md) | Weekly digest sends two emails, not one | #1014, #1253, #1355, #1214 |
| [adr-0011-slow-transaction-thresholds.md](adr-0011-slow-transaction-thresholds.md) | Name-shape classification; WebSocket routes not excluded | #1310, #1320 |
| [adr-0012-ubuntu-release-check-on-vps.md](adr-0012-ubuntu-release-check-on-vps.md) | Local systemd timer replaces the Ubuntu release job | #1134 |
| [adr-0013-diff-scoped-coverage.md](adr-0013-diff-scoped-coverage.md) | Gate on changed-line coverage; the signature-coverage fixup | #1301, #1364, #1376 |
| [adr-0014-start-finish-task-enforcement.md](adr-0014-start-finish-task-enforcement.md) | `ExitPlanMode` and `Stop` hooks enforce the task pairing | #1236, #1238, #1400 |
| [adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md](adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) | Own Go module; `GOTOOLCHAIN=go1.24.13` pin | darwinkit#286 |
| [adr-0016-kobo-gateway-loopback-tls-and-login-item.md](adr-0016-kobo-gateway-loopback-tls-and-login-item.md) | Loopback HTTPS for Safari; LaunchAgents over `SMAppService` | — |
| [adr-0017-long-request-handler-deadlines.md](adr-0017-long-request-handler-deadlines.md) | Handler deadlines pinned under the edge proxy ceiling | #672, #1113 |

## Conventions

| Document | Covers | Issues |
|---|---|---|
| [convention-comments-describe-current-behavior.md](convention-comments-describe-current-behavior.md) | No historical or stale claims in code comments | — |
| [convention-database-queries.md](convention-database-queries.md) | Never select a wide TEXT column in a list query; read direction | #1027 |
| [convention-deploy-secrets.md](convention-deploy-secrets.md) | A deploy secret is declared in three places that must agree | #1390, #1404, #1405 |
| [convention-mcp-gap-first.md](convention-mcp-gap-first.md) | Fix the missing MCP tool before investigating the incident | #1027, #1195, #1214, #1357, #1374, #1377 |
| [convention-ui-standards.md](convention-ui-standards.md) | Web UI rules, theming, and the server/client import trap | #1412 |

## Specs

| Document | Covers | Issues |
|---|---|---|
| [spec-ci-pipeline.md](spec-ci-pipeline.md) | How `main.yml` and its reusable workflows run | #863, #1405 |
| [spec-kobo-gateway-runtime.md](spec-kobo-gateway-runtime.md) | Server, TLS, menu bar, self-update, crash recovery | — |
| [spec-mcp-server.md](spec-mcp-server.md) | The `/apps/mcp` tool surface and its two deliberate mutations | #1039 |
| [spec-oauth-consent-screen.md](spec-oauth-consent-screen.md) | The server-rendered `/oauth/consent` flow | #1039 |
| [spec-observability-subsystem.md](spec-observability-subsystem.md) | `TrackedJob`, `UsageRecorder`, host metrics, log tee, snapshots | #1027, #915, #1040, #848, #1217 |
| [spec-trains-gtfs-ingest.md](spec-trains-gtfs-ingest.md) | GTFS static import, feed traps, `trip_id` churn | #1388, #1389, #1390 |
| [spec-ui-primitives.md](spec-ui-primitives.md) | Generated inventory of `components/ui/` — check before building a component | #1412 |
| [spec-web-data-flow.md](spec-web-data-flow.md) | Two ConnectRPC transports; RSC prefetch → SWR hydration | #1318 |

## Infrastructure

Host-layer decisions are **not** duplicated here. `../infra/README.md` is the
runbook and decision log for the VPS itself, organized by issue number — it is
also the single source of truth for the full deploy-secret list. Sections other
documents link into:

- *Automate Kamal deploys in CI* (#1036) — the full secrets list
- *GoTrue is gone* (#1039) — see also `adr-0005`
- *Getting notified of a new Ubuntu LTS release* — see also `adr-0012`
