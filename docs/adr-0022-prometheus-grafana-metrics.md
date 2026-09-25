# ADR-0022: Prometheus + Grafana replace the hand-rolled metrics pipeline

- Status: Accepted
- Issues: #1468, #1528, #1529, #1541, #1554, #1555, #1556, #1564, #1570, #1574, #1580, #1592, #1608, #1702, #1709, #1717, #1723
- Affects: `api/internal/observability/`, `api/internal/middleware/metrics.go`, `api/cmd/api/metrics.go`, `api/cmd/api/mcp_prom_query.go`, `api/cmd/api/mcp_grafana_alerts.go`, `config/deploy.grafana.yml`, `infra/prometheus-compose.yml`, `infra/prometheus.yml`, `infra/grafana/`, `web/app/metrics/`, `web/lib/server/metrics.ts`

## Context

`internal/observability/` hand-rolled a metrics pipeline: a node_exporter text
parser, four snapshot jobs writing samples to Postgres, and `ThresholdAlertJob`
re-implementing rule evaluation, sustain windows, and breach state — ~2,500 Go
lines plus ~700 lines of chart components, reinventing a TSDB and alerting.

## Decision

**Prometheus + Grafana own metrics, graphs, and alerting.** `/monitoring` keeps
only GitHub/Sentry workflow surfaces with actions Grafana can't perform (now
`/monitoring/connections`); the app's own GitHub/Sentry OAuth clients still back
those and the related MCP tools.

- **Prometheus over VictoriaMetrics**: VM saves ~200 MB RAM, not decisive with
  ~3 GB free; PromQL and the ecosystem win.
- **Topology**: Prometheus + postgres_exporter are Tofu-managed compose
  accessories with no published ports, reachable only on the `kamal` network.
  Grafana is a third Kamal service at `/grafana` on the shared kamal-proxy
  (`strip_path_prefix: false`, with `GF_SERVER_SERVE_FROM_SUB_PATH`/
  `GF_SERVER_ROOT_URL`), deployed as a wrapper image
  (`infra/grafana.Dockerfile`, `build-grafana.yml`).
- **Provisioned, not click-configured**: datasources, plugins, dashboards, and
  alerting live under `infra/grafana/` and are baked into the image. Dashboard
  JSON is the source of truth (`allowUiUpdates: false`), checked by
  `make lint/grafana` and `make grafana/verify`.
- **SSO** via the embedded AS (ADR-0021), admin-only through
  `role_attribute_strict`; local `admin` / `GRAFANA_ADMIN_PASSWORD` stays as
  break-glass.
- **Instrumentation**: `api` exposes unprefixed `/metrics` (`promhttp`) with
  `http_request_duration_seconds` and `job_duration_seconds` histograms. `web`
  exposes `/metrics` with `web_vitals_seconds` from a browser beacon.
  `IssueSignalCollectorJob` exports issue signals as gauges every 5 minutes:
  `github_failing_pull_requests`, `github_workflow_run_failed{branch}`,
  `github_open_security_alerts{severity}`,
  `github_workflow_run_duration_seconds{workflow}`, `sentry_unresolved_issues`,
  `r2_orphaned_objects`, `r2_storage_bytes`, `postgres_schema_size_bytes{schema}`,
  and `automated_action_seconds_since_last_open{routine}`.
- **Alerting is Grafana-managed** (`infra/grafana/provisioning/alerting/`):
  host rules (CPU/memory 80%/85% for 15m, disk 85% instant), p95 rules
  (`RequestP95High`/`JobP95High`/`FrontendP95High`), the `service-health` group
  (`IssueFailingPRs`, `IssueMainCIRed`, `IssueSecurityAlerts`,
  `IssueSentryUnresolved`, `IssueOrphanedStorage`, `R2UsageHigh` at 9 GiB,
  `AutomatedActionStalled`, `AutomatedRoutineMissed`), and `TargetDown`/
  `TargetMissing`. Only `notify: slack` rules (outages:
  targets, disk, stalled or missed routines) reach the Slack contact point
  (`GRAFANA_SLACK_WEBHOOK_URL`); the rest are muted by the `always` mute
  timing and wait for the nightly sweep's `get_grafana_alerts`.
- **MCP**: `prom_query` proxies Prometheus's `/api/v1/query` as raw JSON
  (replacing four narrow tools). `get_grafana_alerts` reads rule state from
  Grafana's ruler API (`/api/prometheus/grafana/api/v1/rules`) over the
  **public** `GRAFANA_URL` with Basic auth `admin`/`GRAFANA_ADMIN_PASSWORD`,
  because Grafana-managed alerts never appear in Prometheus `ALERTS{}`.
- **GitHub/Sentry Grafana datasource plugins** are provisioned for dashboards
  and exploration; every alert still runs on Prometheus gauges.

## Hard-won invariants

Each of these was violated once; keep them.

- **Discover `api`/`web`/`grafana` by Docker label, not DNS.** Kamal names
  containers `<service>-<role>-<version>` with no service-name alias, so
  `api`/`web` were never scraped for the entire retention window (#1554). The
  app jobs use `docker_sd_configs` filtered on the `service` label. Prometheus
  needs the Docker socket (`:ro`) and the host's docker gid (resolved at
  provision time into `group_add`; the image runs as `nobody`).
- **`up == 0` can't see a job that discovers nothing** — no series exists.
  `TargetMissing` uses `absent(up{job=...})` for api, web, and grafana;
  `TargetDown` is one multi-dimensional `up == 0` rule.
- **Verify data arrived, not just that code shipped.** After a metrics change,
  confirm with `prom_query` that each series is non-empty.
- **Reload Prometheus on config change.** `docker compose up -d` doesn't restart
  on a bind-mounted file's content change, so `null_resource.prometheus` sends
  `docker compose kill -s HUP prometheus` after `up -d` (#1717).
- **Avoid the `job` label collision.** A metric's own `job` label becomes
  `exported_job`; `metric_relabel_configs` renames `job_duration_seconds`' to
  `job_name`, and new gauges use other label names (`workflow`, `schema`).
- **Aggregate away `instance` in panels.** Each deploy mints a new `instance`
  (container name), so per-instance exprs are wrapped in `min`/`max by (job)`
  (#1574, #1580). The churn itself is deliberate.
- **Grafana self-scrape path is `/grafana/metrics`** (bare `/metrics`
  redirects). There is no `grafana_alerting_rule_evaluations_total` in this
  version.
- **`web`'s `GET /metrics` requires `Bearer OBSERVABILITY_INGEST_SECRET`**,
  delivered to Prometheus via `credentials_file`; unset skips the gate rather
  than closing it. `POST /metrics` (the beacon) is unauthenticated by design.
  `deploy-kamal` `needs` `infra-apply` so the token is sent before it's required.
- **`IssueSentryUnresolved` must use the Prometheus gauge, not the Sentry
  plugin.** Grafana's SSE layer can't convert the plugin's Issues response into
  any series (`input data must be a wide series but got type long`,
  grafana/sentry-datasource#266) before any expression runs, and
  `eventsStats` rejects `is:unresolved` (#1608, #1702, #1709).
- **`AutomatedRoutineMissed`** uses a hardcoded `knownRoutines` list and a
  shared 27h threshold (daily + 3h buffer); a never-opened routine reports a
  large sentinel. Update both by hand when routines or schedules change.

## Alternatives considered

- **VictoriaMetrics** — see Decision.
- **Grafana alongside the old charts** — two chart stacks to maintain.
- **Tailscale/Cloudflare Access for Grafana** — the existing kamal-proxy path
  routing plus SSO needs no new network layer.
- **SMTP contact point** — shipped first; replaced by Slack for centralized
  delivery (#1592).
- **Migrating GitHub signals onto the GitHub plugin** — its coverage is looser
  than `internal/github`'s; deferred until validated against live data.

## Consequences

- All alerting lives in Grafana; "why no alert?" has one place to look.
- `TransactionLatencySnapshotJob` remains because `GetSlowTransactions`'
  trending list reads its table.
- CI duration is a panel, not an alert.

## Revisit when

CI duration needs an alert (the metric now exists), or GitHub plugin queries are
validated well enough to retire the GitHub half of the collector.
