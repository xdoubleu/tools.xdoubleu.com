# docs/

The "why" behind this codebase — long-term decisions and the rules that came out
of them. Nothing else.

**The code is the spec.** How a subsystem works belongs in the code and its
comments, not here; a prose description of current behavior only rots. Only two
kinds of document live in this directory:

- **ADR** — a decision with alternatives and consequences. No "alternatives
  considered" means it isn't one.
- **Convention** — a rule contributors and agents must follow.

Imperative guidance lives in the `CLAUDE.md` files; past tense — what something
used to be, what was tried and rejected, which incident produced a rule — lives
here. Start from `TEMPLATE-adr.md` or `TEMPLATE-convention.md`, keep it short,
and register a new document in the root `CLAUDE.md` index (that index is what
makes it discoverable, since only `CLAUDE.md` files load automatically).

## Decisions (ADRs)

| Document | Covers | Issues |
|---|---|---|
| [adr-0001](adr-0001-two-service-kamal-deploy.md) | Two independent Kamal services, one proxy; web-then-api deploy order | #558, #904, #1038, #1029, #1034, #1113, #1132, #1106, #1111 |
| [adr-0002](adr-0002-kobo-gateway-ci-cache-split.md) | Cache written only from a `workflow_run` job; artifact handoff to web | #1322, #1192, #1347 |
| [adr-0003](adr-0003-buildkit-gha-cache-over-hashfiles.md) | BuildKit `type=gha` cache instead of hand-computed keys | #948, #900 |
| [adr-0004](adr-0004-runtime-release-env-vs-compile-stamp.md) | `RELEASE` at runtime for api/web, compile-stamped for kobo-gateway | #1038 |
| [adr-0005](adr-0005-first-party-auth-replacing-gotrue.md) | First-party auth replaces Supabase GoTrue | #1039 |
| [adr-0006](adr-0006-embedded-oauth21-authorization-server.md) | Embedded fosite AS; `offline_access` granted server-side | #1039, #1177 |
| [adr-0007](adr-0007-dashboard-app-owns-public-sharing.md) | Schema-less `dashboard` app owns public dashboards and share tokens | #737 |
| [adr-0008](adr-0008-family-as-single-sharing-concept.md) | One `family` concept replaces per-app sharing and contacts | #1349, #1403 |
| [adr-0009](adr-0009-sentrytools-extracted-module.md) | slog→Sentry glue as its own module via a local `replace` | #926, #1038 |
| [adr-0010](adr-0010-two-weekly-digest-emails.md) | Weekly digest sends two emails, not one | #1014, #1253, #1355, #1214 |
| [adr-0011](adr-0011-slow-transaction-thresholds.md) | Name-shape classification; WebSocket routes not excluded (p95 alert moved to Grafana in #1528) | #1310, #1320, #1528 |
| [adr-0012](adr-0012-ubuntu-release-check-on-vps.md) | Local systemd timer replaces the Ubuntu release job | #1134 |
| [adr-0013](adr-0013-diff-scoped-coverage.md) | Gate on changed-line coverage; the signature-coverage fixup | #1301, #1364, #1376 |
| [adr-0014](adr-0014-start-finish-task-enforcement.md) | `ExitPlanMode` and `Stop` hooks enforce the task pairing | #1236, #1238, #1400 |
| [adr-0015](adr-0015-kobo-gateway-separate-module-and-toolchain-pin.md) | Own Go module; `GOTOOLCHAIN=go1.24.13` pin | darwinkit#286 |
| [adr-0016](adr-0016-kobo-gateway-loopback-tls-and-login-item.md) | Loopback HTTPS for Safari; LaunchAgents over `SMAppService` | — |
| [adr-0017](adr-0017-long-request-handler-deadlines.md) | Handler deadlines pinned under the edge proxy ceiling | #672, #1113 |
| [adr-0018](adr-0018-completion-average-population.md) | Which games the Steam completion averages count | #1375, #1424 |
| [adr-0019](adr-0019-trains-in-memory-router-and-dual-gtfs-feeds.md) | Trains router warmed off the request path; the two GTFS feeds correlated by `(trip_short_name, service date)` | #1388, #1390, #1391, #1484 |
| [adr-0020](adr-0020-ui-configured-alert-delivery-channel.md) | One global email/Slack alert-delivery switch; webhook URL stored DB-encrypted and set in the UI; digests stay email-only. **Superseded by #1530** — Grafana took over alerting, no producer left, so notifications collapsed to email-only (`slack` client, `notification_channel_config` table, RPC and UI removed) | #1482, #1530 |
| [adr-0021](adr-0021-oauth-as-general-purpose-oidc-idp.md) | The embedded AS also issues OIDC ID tokens and supports confidential clients (Grafana SSO) | #1469 |
| [adr-0022](adr-0022-prometheus-grafana-metrics.md) | Prometheus + Grafana replace the hand-rolled host/CI/storage metrics pipeline; `prom_query` MCP tool; what got removed; Phase 2 (#1528) — latency histograms + Grafana alerting, `ThresholdAlertJob` retired; Phase 3 (#1529) — issue signals as Prometheus gauges + Grafana `service-health` alerts, `IssueNotifierJob`/`notified_issues` retired; Phase 4 (#1530) — notifications collapsed to email-only, `internal/slack` + `notification_channel_config` removed; Phase 5 (#1554) — `api`/`web` were never scraped at all, so every Phase 2/3 metric was empty; label-based `docker_sd_configs` discovery, `TargetDown`/`TargetMissing` alerts, `app-performance` dashboard | #1468, #1528, #1529, #1530, #1554 |

## Conventions

| Document | Covers | Issues |
|---|---|---|
| [convention-comments-describe-current-behavior](convention-comments-describe-current-behavior.md) | No historical or stale claims in code comments | — |
| [convention-database-queries](convention-database-queries.md) | Never select a wide TEXT column in a list query; read direction | #1027 |
| [convention-deploy-secrets](convention-deploy-secrets.md) | A deploy secret is declared in three places that must agree | #1390, #1404, #1405 |
| [convention-mcp-gap-first](convention-mcp-gap-first.md) | Fix the missing MCP tool before investigating the incident | #1027, #1195, #1214, #1357, #1374, #1377, #1424 |
| [convention-ui-standards](convention-ui-standards.md) | Web UI rules, theming, the server/client import trap | #1412 |

## Infrastructure

Host-layer decisions live in [`../infra/README.md`](../infra/README.md), the
runbook for the VPS itself and the single source of truth for the deploy-secret
list.
