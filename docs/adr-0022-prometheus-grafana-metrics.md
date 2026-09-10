# ADR-0022: Prometheus + Grafana replace the hand-rolled metrics pipeline

- Status: Accepted; extended by #1528 (see "Phase 2" below)
- Issues: #1468 (follows #1469/ADR-0021), #1528
- Affects: `api/internal/observability/`, `api/internal/middleware/metrics.go`,
  `api/cmd/api/metrics.go`, `api/cmd/api/mcp_prom_query.go`,
  `config/deploy.grafana.yml`, `infra/prometheus-compose.yml`,
  `infra/prometheus.yml`, `infra/grafana/provisioning/`,
  `web/app/metrics/`, `web/lib/server/metrics.ts`, `web/components/monitoring/`

## Context

`internal/observability/` hand-rolled a complete metrics pipeline:
`hostmetrics.go` parsed node_exporter's Prometheus text-exposition format by
hand, four snapshot jobs (`host_metrics_snapshot`, `db_size_snapshot`,
`transaction_latency_snapshot`, `workflow_runs_snapshot`) wrote samples into
Postgres on a timer, and `threshold_alert.go` re-implemented rule evaluation,
sustain windows, hysteresis, and breach/recovery state in
`global.alert_states`. That was ~2,500 non-test Go lines plus ~700 lines of
chart components (`MultiSeriesChart`, `HostMetricsCard`, `AlertStatesCard`,
`TransactionLatencyHistoryCard`) re-deriving what a real TSDB and its
alerting layer give for free.

## Decision

**Prometheus over VictoriaMetrics.** VictoriaMetrics' case was RAM
(~150 MB vs Prometheus' ~350 MB) — real on a single ~4 GB VPS, but not
decisive given the headroom measured at issue-filing time (~3 GB free).
Prometheus wins on adoption, documentation, and ecosystem, and avoids
MetricsQL's soft lock-in versus PromQL, which every exporter, Grafana
dashboard, and future contributor already knows.

**Grafana owns metrics, graphs, and alerting; `/monitoring` keeps owning
workflow over GitHub/Sentry.** The dashboard chart components listed above
are deleted outright — Grafana replaces them, not embeds alongside them.
`/monitoring/observability` (the whole page/route) is deleted; the
`/monitoring` Issues page's "Observability" link now points at `/grafana`
instead. `/monitoring` itself is untouched: `SentryCard`,
`FailingPullRequestsCard`, `SecurityAlertsCard`, `OrphanedStorageCard`,
`WorkflowRunsCard`, `IssuesClient`, `OAuthConnectionsCard`, and the settings
page are GitHub/Sentry workflow surfaces with action buttons
(`resolve_sentry_issue`, `dismiss_security_alert`) — Grafana has no equivalent
for that and isn't asked to grow one.

**Grafana is reachable at `/grafana` on the public domain, no VPN/tunnel.**
It is a third Kamal service (`config/deploy.grafana.yml`), sharing the
existing kamal-proxy instance and domain with `api`/`web`
(→ [ADR-0001](adr-0001-two-service-kamal-deploy.md)) via
`proxy.path_prefix: "/grafana"`, `strip_path_prefix: false` (Grafana must see
the prefix itself — `GF_SERVER_SERVE_FROM_SUB_PATH`/`GF_SERVER_ROOT_URL` are
set accordingly, mirroring how `api/cmd/api/kamal_proxy_shim.go` handles
`/api` in-process rather than relying on kamal-proxy stripping it). Grafana
deploys a thin wrapper image this repo builds and pushes to its own GHCR
namespace (`infra/grafana.Dockerfile` + `build-grafana.yml`, issue #1509) —
`FROM grafana/grafana:X` plus a `service` label for Kamal's `validate_image`,
and since #1527 the baked-in datasource/dashboard provisioning under
`infra/grafana/`.

**SSO reuses the embedded AS from ADR-0021**, which shipped ahead of this
issue specifically to unblock it: Grafana's `generic_oauth` provider points
at `https://tools.xdoubleu.com/oauth2/{authorize,token}`, maps
`role`/`preferred_username` claims with `role_attribute_strict: true`, and
authenticates as the static confidential `grafana` client
(`OAUTH_GRAFANA_CLIENT_SECRET`, migration `00045`). Local admin/password
login stays enabled as a break-glass path (`GRAFANA_ADMIN_PASSWORD`), not
disabled — SSO is the default, not the only way in. Follow-up #1527 made SSO
**admin-only**: the AS emits the `role` claim only for admins, so
`role_attribute_strict` turns a non-admin token into a refused login.

**Datasource and dashboards are provisioned, not click-configured** (#1527):
`infra/grafana/` holds the Prometheus datasource and the host / Postgres /
API-runtime / overview dashboards that replace the removed
`/monitoring/observability` charts. They are baked into the wrapper image
(`infra/grafana.Dockerfile`), the dashboard JSON is the single source of
truth (`allowUiUpdates: false`); `make lint/grafana` statically validates the
JSON and `make grafana/verify` boots the image to confirm the datasource and
dashboards actually provision (issue #1533) — a
change under `infra/grafana/` rebuilds the image via `main.yml`'s
`grafana_dockerfile` path filter.

**Prometheus + postgres_exporter are Tofu-managed compose accessories, not
Kamal services** — same shape as `node-exporter-compose.yml`
(`infra/prometheus-compose.yml`, provisioned by a new
`null_resource.prometheus` in `infra/main.tf`, mirroring
`null_resource.node_exporter`/`null_resource.postgres`). Neither publishes a
host port at all (stricter than "127.0.0.1 only") — reachable only from
containers already on the `kamal` Docker network: Grafana, and the new
`prom_query` MCP tool.

**`prom_query(promql)` replaces four narrow MCP tools with one general
one.** `get_host_metrics`, `get_database_size_history`,
`get_transaction_latency_history`, and `get_alert_states` are removed;
`prom_query` (`api/cmd/api/mcp_prom_query.go`) proxies Prometheus's own
`/api/v1/query` HTTP API directly, admin-gated the same way every other
observability tool is, and returns Prometheus's raw JSON response rather than
a proto message — there is no fixed shape to model when the whole point is
letting an agent ask an arbitrary PromQL question. `get_workflow_run_stats`
is also removed (see "What got removed" below).

**A `/metrics` endpoint on `api`, unprefixed like `/health`.** Uses
`github.com/prometheus/client_golang`'s `promhttp.Handler()` — the process/Go
runtime metrics (`go_*`, `process_*`) that library already tracks — rather
than api hand-rolling its own exposition format writer. Registered directly
in `cmd/api/routes.go`, never through the public `/api` `path_prefix`:
Prometheus scrapes it over the internal Docker network at the container's own
port, the same pattern `/health` already uses for Kamal's own healthcheck.

**Alert rules live in the repo as Prometheus's own YAML**
(`infra/prometheus/alert-rules.yml`), diffable and PR-reviewed — the same
spirit as the old typed Go `alertRule` slice, just in Prometheus's native
format instead of a bespoke one. Thresholds and sustain windows are carried
over exactly: `host_cpu_high`/`host_memory_high` at 80%/85% sustained 15
minutes (`for: 15m`), `host_disk_high` at 85% instant (no `for:`, since a full
disk doesn't need to sustain to matter).

> **Superseded by Phase 2 (#1528):** nothing ever wired those Prometheus
> rules to a delivery path — they fired into `ALERTS{}` and were mailed
> nowhere. #1528 moved every rule into Grafana alerting provisioning
> (`infra/grafana/provisioning/alerting/`), where Grafana evaluates and
> routes them through a real contact point. See "Phase 2" below.

**Contact point: Grafana's own SMTP, reusing the existing Resend account.**
`config/deploy.grafana.yml` sets `GF_SMTP_*` env vars pointing at Resend's
SMTP relay (`smtp.resend.com:587`), with `GF_SMTP_PASSWORD`/
`GF_SMTP_FROM_ADDRESS` fed from the *same* `RESEND_API_KEY`/`EMAIL_FROM` repo
secrets `api`'s own mailer already uses — no new mail credential to
provision. This is explicitly the least load-bearing part of this change;
Slack (ADR-0020) was considered but Grafana's contact-point model doesn't
share `internal/notifications`' delivery queue, so wiring it in would mean a
second, Grafana-native Slack integration for no clear benefit over SMTP.

## What got removed

- Snapshot jobs: `host_metrics_snapshot`, `db_size_snapshot`,
  `workflow_runs_snapshot` (all in `api/internal/observability/jobs/`), plus
  their repositories (`HostMetricsRepository`, `DBSizeSamplesRepository`,
  `WorkflowRunsRepository`), models, and proto RPCs (`GetHostMetrics`,
  `GetDatabaseSizeHistory`, `GetWorkflowRunStats`). `hostmetrics.go` (the
  hand-rolled node_exporter text-format parser) is deleted entirely — nothing
  in api parses that format any more, Prometheus does.
- `GetDatabaseStats` is now a live-only snapshot (`pg_database_size`/
  `pg_class`) — no `window_days`, no `TableGrowth`, no history; growth over
  time is a Grafana/Prometheus question now (`postgres_exporter`'s
  `pg_database_size_bytes` over time).
- Host/CI/storage rules in `threshold_alert.go`
  (`host_cpu_high`/`host_memory_high`/`host_disk_high`/`r2_usage_high`/
  `ci_duration_high`) and their `global.notification_settings`/
  `global.alert_states` rows — migration `00046` drops the rows and the
  now-empty snapshot tables (`host_metric_samples`, `db_size_samples`,
  `workflow_run_samples`, `workflow_job_samples`).
- The main-branch-CI-failure email alert (`WorkflowRunsSnapshotJob.
  notifyMainFailure`, `failing_main_ci` notification source) is removed along
  with its snapshot table. Phase 3 (#1539/#1540) restored a proactive
  replacement: the `IssueSignalCollectorJob` exports GitHub/Sentry/R2 signals
  as Prometheus gauges, and the Grafana `service-health` alert group fires
  `IssueMainCIRed` (and `IssueFailingPRs`, `IssueSecurityAlerts`,
  `IssueSentryUnresolved`) off them. `GetWorkflowRuns`/`WorkflowRunsCard` and
  `GetFailingPullRequests` (both live, GitHub-API-backed, kept — see below)
  still surface a red main branch on `/monitoring`.
- `r2_usage_high` is likewise restored in Phase 3: `r2_storage_bytes` /
  `r2_orphaned_objects` gauges from the same collector back the Grafana
  `R2UsageHigh` (9 GiB budget) and `IssueOrphanedStorage` rules.
  `GetStorageStats`/`OrphanedStorageCard` on `/monitoring` still surface it
  live. `ci_duration_high` remains the only signal with no Grafana/Prometheus
  replacement — Prometheus has no GitHub Actions workflow-duration exporter.
- Frontend chart components: `MultiSeriesChart`, `HostMetricsCard`,
  `AlertStatesCard`, `TransactionLatencyHistoryCard`, and the history half of
  `DatabaseCard` (the schema/total-size live snapshot half is kept). The
  `/monitoring/observability` page/route is deleted entirely.

## What was verified to stay, and why

- **The slow-transaction classification helpers** (`classifyTransaction`,
  `thresholdMsForClass`, `slowTransactionExcluded`) stay — they key off
  *live* Sentry data, not stored host-metric samples, and still gate the
  `/monitoring` trending list and the weekly digest. In #1468 they lived in
  `threshold_alert_slow_transactions.go` inside `ThresholdAlertJob`;
  Phase 2 (#1528) retired that job and moved the helpers into
  `slow_transactions.go`, and the p95 *alert* they used to drive is now a
  Grafana rule on real histograms. `global.alert_states` /
  `AlertStatesRepository` — kept here in #1468 solely for that job — are
  dropped in #1528 (migration `00047`).
- **`TransactionLatencySnapshotJob`/`TransactionLatencyRepository` are
  kept**, not removed despite reading like snapshot-job scope. Both
  `WeeklyDigestJob` (`currentlySlowTransactions`) and `GetSlowTransactions`'
  `trending` field depend on `Trends()` over the same
  `global.transaction_latency_daily` table this job populates — removing it
  would have silently broken the weekly digest's slow-transaction section
  and the `/monitoring` Issues page's trending list, neither of which this
  issue was meant to touch. Only the *raw per-point* `GetTransactionLatencyHistory`
  RPC (and its dedicated `TransactionLatencyHistoryCard` chart) is removed —
  that was pure history-graphing, the exact duplication Grafana is meant to
  absorb.
- **`GetWorkflowRuns`/`WorkflowRunsCard`/`GetFailingPullRequests` stay** —
  both are live GitHub-API calls with no DB dependency, unrelated to the
  removed `workflow_run_samples`/`workflow_job_samples` snapshot tables that
  only `GetWorkflowRunStats` (removed) and the main-failure alert (removed,
  see above) read from.

## Alternatives considered

- **VictoriaMetrics.** Rejected — see Decision; RAM wasn't decisive given
  measured headroom, and Prometheus's ecosystem/documentation/PromQL
  familiarity won on every other axis.
- **Grafana embedding old-style charts alongside Prometheus panels.**
  Rejected — half-migrating would leave both a Postgres-backed chart stack
  and a Prometheus-backed one to maintain. Grafana replaces the charts, it
  doesn't sit next to them.
- **Tailscale/Cloudflare Access + `cloudflared` for Grafana access**
  (raised as an open question in the original issue). Rejected in favor of
  the existing kamal-proxy path-routing pattern `api`/`web` already use — no
  new network topology, no firewall change, and SSO via ADR-0021 already
  gives every existing admin a login without a second network layer to
  configure.
- **A Grafana-native Slack contact point** (ADR-0020's webhook). Considered
  and dropped — see Decision's contact-point paragraph.

## Phase 2 (#1528): instrument latency, unify alerting in Grafana, retire ThresholdAlertJob

#1468 left two loose ends this issue closes:

- **The Prometheus alert rules delivered nowhere.** No Alertmanager, no
  Grafana alert provisioning — `infra/prometheus/alert-rules.yml`'s header
  *claimed* Grafana alerted off them, but nothing wired it. #1528 deletes
  that file (and the `rule_files:` entry, the compose mount, the `main.tf`
  upload) and recreates every rule under
  `infra/grafana/provisioning/alerting/` — `rules.yml` (the 5 migrated
  host/`up` rules verbatim, plus 3 new p95 rules), `contactpoints.yml` (one
  email receiver reusing `GF_SMTP_*`, no new secret), `policies.yml` (one
  flat route). `make grafana/verify` now asserts all of it provisioned.
- **`ThresholdAlertJob` was the last hand-rolled alerting.** It, its
  `slow_transaction_{http,job,frontend}_high` sources, and
  `global.alert_states` / `AlertStatesRepository` / `models.AlertState` are
  all deleted (migration `00047`). The p95 signal it approximated off Sentry
  transaction stats is now real: an `http_request_duration_seconds`
  histogram (`api/internal/middleware/metrics.go`), a `job_duration_seconds`
  histogram (`api/internal/observability/trackedjob.go`), and a
  `web_vitals_seconds` histogram fed by a browser Web-Vitals beacon to a new
  `/metrics` route on the `web` Node server (`web/app/metrics/route.ts`,
  scraped as a new `web` Prometheus job). Grafana's `RequestP95High` /
  `JobP95High` / `FrontendP95High` rules evaluate those.

Sentry keeps showing transaction data for manual review; it is no longer an
alert source.

## Consequences

- All alerting now lives in one place (Grafana). A contributor asking "why
  didn't I get an alert" has exactly one system to check.
  `docs/adr-0011-slow-transaction-thresholds.md`'s classification/threshold
  reasoning still governs the `/monitoring` trending list and the weekly
  digest, neither of which is an alert.
- The main-branch-CI-failure and R2-usage-threshold alerts have no
  replacement (see "What got removed") — an accepted, documented gap rather
  than a silent one.
- Kamal container naming isn't Tofu-managed, so the Prometheus scrape
  targets for `api` and (added in #1528) `web` (`infra/prometheus.yml`)
  assume a specific Docker network alias that could not be verified against
  real infra in this change — flagged in `infra/README.md` as the one thing
  worth confirming by hand after the first real deploy.
- This is the second of two Grafana-adjacent issues (after #1469/ADR-0021);
  end-to-end Grafana login and alert delivery could not be verified against
  real infra in this change either — the same caveat ADR-0021 recorded for
  its own half.

## Revisit when

R2 usage or CI duration need a real alert again — either write a small
custom exporter (a Prometheus metric api's own `/metrics` exposes,
sourced from `global.storage_snapshots`/GitHub's API) or accept that those
two specific signals stay workflow-surfaced-only on `/monitoring` rather than
alerted.
