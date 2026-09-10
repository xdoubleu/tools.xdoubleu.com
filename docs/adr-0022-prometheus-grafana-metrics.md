# ADR-0022: Prometheus + Grafana replace the hand-rolled metrics pipeline

- Status: Accepted; extended by #1528 ("Phase 2") and #1529 ("Phase 3") below
- Issues: #1468 (follows #1469/ADR-0021), #1528, #1529
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
provision. Alert emails are sent *from* `GF_SMTP_FROM_ADDRESS` (the
`EMAIL_FROM` secret) but delivered *to* `$__env{NOTIFY_EMAIL_TO}` — the same
admin recipient `api`'s own notification emails use (`cfg.NotifyEmailTo`);
the contact point (`infra/grafana/provisioning/alerting/contactpoints.yml`)
and `config/deploy.grafana.yml` both carry that secret (issue #1528
follow-up — originally the contact point wrongly used the from-address as the
recipient). This is explicitly the least load-bearing part of this change;
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
  live. `ci_duration_high` was at this point the only signal with no
  Grafana/Prometheus replacement — Phase 6 (#1556) added the
  `github_workflow_run_duration_seconds` gauge (panel-only, no alert).
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

## Phase 3 (#1529): issue signals as Prometheus gauges, retire IssueNotifierJob

#1528's "Consequences" flagged two gaps: the main-branch-CI-failure alert and
the R2-usage-threshold alert had no replacement. Phase 3 closes both, and
retires the last hand-rolled notifier.

- **Six issue signals are now Prometheus gauges.**
  `IssueSignalCollectorJob` (`api/internal/observability/jobs/issue_signal_collector.go`,
  #1539) polls the same live GitHub/Sentry/R2 sources `/monitoring` already
  uses and exports `github_failing_pull_requests`,
  `github_workflow_run_failed{branch}`, `github_open_security_alerts{severity}`,
  `sentry_unresolved_issues`, `r2_orphaned_objects`, and `r2_storage_bytes` on
  api's `/metrics`.
- **A Grafana `service-health` alert group evaluates them** (#1540,
  `infra/grafana/provisioning/alerting/rules.yml`): `IssueFailingPRs`,
  `IssueMainCIRed` (`max(github_workflow_run_failed{branch="main"}) > 0`),
  `IssueSecurityAlerts`, `IssueSentryUnresolved`, `IssueOrphanedStorage`, and
  `R2UsageHigh` (`r2_storage_bytes > 9 GiB`). `failing_main_ci` and
  `r2_usage_high` are no longer open gaps — `ci_duration_high` was at this
  point the only signal with no Prometheus metric behind it (Phase 6 (#1556)
  later added `github_workflow_run_duration_seconds`, panel-only, no alert). A
  `service-health` Grafana dashboard graphs the
  gauges alongside the host/Postgres/api-runtime/overview dashboards.
- **`IssueNotifierJob` and `global.notified_issues` are retired** (#1541,
  migration `00048`). It was the realtime "email an admin the first time a
  new unresolved Sentry issue / failing PR / red main / security alert is
  seen" job with per-item dedup in `global.notified_issues`; Grafana's
  `service-health` group now delivers that first-seen alert through its own
  SMTP contact point. `WeeklyDigestJob` still checks
  `global.notification_settings` per section and is unaffected;
  `NOTIFY_EMAIL_TO` stays in use for the two weekly digests and the
  Ubuntu-release-check timer.
- **`prom_query` gains the new gauges** as read paths, and the
  `monitoring-sweep` skill is rewritten MCP-only around them (its stale
  `get_alert_states` reference — that tool went in #1528 — is removed).
  (`ALERTS{}` was listed here too, but Grafana-managed alerts never populate
  it — Phase 6 / #1564 adds `get_grafana_alerts` for actual alert state.)
- **The `web` `/monitoring` surface collapses to `/monitoring/connections`**
  (#1542, its own slice): with alerting and the issue digest owned by
  Grafana, the standalone Issues page is redundant and the OAuth-connection
  management is what remains worth a page.

## Phase 5 (#1554): the scrape targets that never worked

Phases 1–4 added metrics, alerts and dashboards on top of a pipeline that was
not delivering. `up{job="api"}` and `up{job="web"}` were `0` for the entire
90-day retention window — not flapping, never once up. Everything downstream
of those two targets was therefore empty from the day it shipped:
`http_request_duration_seconds`, `job_duration_seconds` and
`web_vitals_seconds` (#1528) had no samples, so `RequestP95High`,
`JobP95High` and `FrontendP95High` evaluated against nothing; all six
`IssueSignalCollectorJob` gauges (#1529) were absent, so the entire
`service-health` dashboard rendered "No data". The dashboards that looked
right — `host`, `postgres` — were exactly the ones fed by the two targets
that did work.

The cause is the alias assumption recorded above: Kamal names containers
`<service>-<role>-<version>` and creates no alias equal to the `service:`
name, so Docker's embedded DNS never resolved `tools-xdoubleu-com-api:8000`
or `tools-xdoubleu-com-web:3000`.

Three things follow, and the third is the one that matters:

1. **Discovery is now label-based.** The two app jobs use
   `docker_sd_configs` and keep only containers carrying the `service` Docker
   label Kamal's own `validate_image` step requires on every deployed image.
   Nothing about container naming or the version tag can break it again. The
   `node`/`postgres`/`prometheus` jobs stay `static_configs` — plain compose
   accessories with stable names, where discovery would buy nothing.
2. **Prometheus needs the Docker socket** (`:ro`) and, because the image runs
   as `nobody`, membership of the host's `docker` group. The gid is
   host-specific, so `null_resource.prometheus` resolves it at provision time
   rather than hardcoding it, and fails loudly if there is no such group.
3. **`up == 0` could never have caught this, and still can't.** A job whose
   discovery yields nothing produces no `up` series at all, so a
   down-detector matches nothing and stays silent. The old `static_configs`
   at least manufactured a failing target to alert on; label-based discovery
   removes even that consolation prize. `TargetMissing` closes it with
   `absent(up{job="api"}) or absent(up{job="web"})` — a rule that fires on the
   *absence* of a series. `APIDown`/`PostgresDown` are replaced by one
   multi-dimensional `TargetDown` (`up == 0`), so a newly added target is
   covered without anyone remembering to write a rule.

Two incidental findings, both verified rather than assumed:

- `job_duration_seconds` carries its own `job` label, which collides with the
  `job` label Prometheus attaches from the scrape config; Prometheus renames
  the exposed one to `exported_job`. A panel written the obvious way,
  `sum by (job) (...)`, would have silently collapsed every background job
  into one "api" series. `metric_relabel_configs` renames it to `job_name`.
- `GET https://tools.xdoubleu.com/metrics` answers 200 to the public internet
  (`web` is the kamal-proxy catch-all) and `POST /metrics` is unauthenticated
  ingest for the Web Vitals beacon. Tracked separately (#1555) so that an auth
  gate did not land in the same change as the fix to the thing being scraped —
  see Phase 6 below, where `GET` was gated once Phase 5's scrape path was
  confirmed working.

The new `app-performance` dashboard restores what Grafana had no equivalent
for: per-`route` p95 (the aggregate latency alerts can say something is slow
but never which endpoint), per-job duration and failure rate, and Core Web
Vitals at p75.

## Phase 6 (#1555): gating the `web` /metrics endpoint

Once Phase 5 made Prometheus actually reach `web` directly over the Docker
network, the public `GET https://tools.xdoubleu.com/metrics` route
(`web/app/metrics/route.ts` — `web` is the kamal-proxy catch-all) had no
reason to stay open. It now requires an `Authorization: Bearer` token equal
to `OBSERVABILITY_INGEST_SECRET` — the same shared secret `web/app/logs/route.ts`
already uses, now also a `web` Kamal deploy secret and, as
`TF_VAR_observability_ingest_secret`, written to a file on the VPS that
`infra/prometheus.yml`'s `web` scrape job reads via `credentials_file`. When
the secret is unset the gate is skipped rather than closed, matching
`app/logs/route.ts` and keeping a missing-secret misconfiguration from
re-creating the Phase 5 "every `web_*` metric silently zero" failure. `POST
/metrics` (the browser Web Vitals beacon) stays unauthenticated by design.
`deploy-kamal` `needs` `infra-apply`, so Prometheus starts sending the token
before `web` starts checking it — no scrape gap.

**What this says about the pipeline as a whole:** every phase verified its own
code and none verified that the data arrived. A metric that is registered,
exported and alerted on still tells you nothing if nobody ever confirmed a
sample landed. The post-deploy `prom_query` check in #1554's verification
section exists to make that a step rather than an assumption.

## Phase 6 (#1556): the two metrics that had nothing to graph

#1554 restored every panel that could be built from metrics that already
existed; two pre-Grafana overviews could not be, because nothing exported the
data. Phase 6 adds the missing Go instrumentation, both as gauges refreshed on
`IssueSignalCollectorJob`'s 5-minute timer (not scraped directly):

- `github_workflow_run_duration_seconds{workflow}` — the latest completed
  default-branch run's duration per workflow, computed in the loop
  `collectWorkflowRuns` already runs over the GitHub API response. Closes the
  long-standing `ci_duration_high` "revisit when" item: the data now exists.
  No alert was added — failing-run signal already exists and duration trends
  are read on the `app-performance` panel, not alerted.
- `postgres_schema_size_bytes{schema}` — per-schema on-disk size from
  `DBStatsRepository.SchemaSizes`, threaded into the job as a fourth
  interface-typed dep (`dbStatsRepo` moved above `newCrossAppJobs` in
  `cmd/api/main.go`). `postgres_exporter` only exposes per-*database* size.

Both got an `app-performance` timeseries panel. Label names are `workflow` /
`schema`, not `job` — the api scrape job's own `job` label would otherwise
collide the way `job_duration_seconds` does (see `infra/prometheus.yml`'s
`metric_relabel_configs`). Unverifiable until deployed by design — the
post-deploy `prom_query` check confirms each series is non-empty.

## Phase 7 (#1564): reading Grafana-managed alert state

Once alerting became Grafana-managed (Phase 2), `prom_query` stopped being able
to answer "is the alert for X firing?" — Grafana-managed alert rules are
evaluated inside Grafana and never populate Prometheus `ALERTS{}`, so the only
way to check state was to re-run each rule's PromQL by hand and reason about its
`noDataState`. #1563's false `PostgresDown` alert made that concrete.

`get_grafana_alerts` (`api/cmd/api/mcp_grafana_alerts.go`, admin-gated, modelled
on `prom_query` — raw-JSON passthrough) closes the gap: it calls Grafana's
Prometheus-compatible ruler endpoint
(`/api/prometheus/grafana/api/v1/rules`), which carries each rule's current
`state` and its `alerts[]` active instances.

It reaches Grafana over the **public URL** (`GRAFANA_URL`, default
`https://tools.xdoubleu.com/grafana`) through kamal-proxy, with HTTP Basic auth
as `admin` / `GRAFANA_ADMIN_PASSWORD` — the same value and repo Secret the
grafana service already gets as `GF_SECURITY_ADMIN_PASSWORD`, now added to the
`api` deploy-secret three-list. **Not** an internal `grafana:3000` hostname:
that is exactly the "assumed Kamal network alias" Phase 5 proved does not exist.
`get_grafana_alerts` also removes the last reason to reference `ALERTS{}` for
Grafana alert state (README/CLAUDE.md updated).

## Phase 8 (#1574): dashboard panels aggregate away the churning `instance` label

Phase 5's label-based discovery sets each `api`/`web` target's `instance` to the
Kamal container name, which embeds the deploy version — so every deploy mints a
new `instance` value. Panels that render a raw per-`instance` selector (`up`,
`scrape_duration_seconds`, the `api-runtime` Go/process gauges) therefore grew a
new row per deploy on the Overview dashboard and showed a doubled line during the
deploy overlap window elsewhere. The churn is deliberate — overlapping old/new
containers must not collide into one Prometheus series and `TargetDown` wants one
alert instance per container — so the fix is dashboard-side: `overview.json` and
`api-runtime.json` now wrap every such expr in `min`/`max by (job) (...)`. No
change to `infra/prometheus.yml` or the alert rules.

## Phase 9 (#1570): GitHub + Sentry as Grafana datasource plugins

The original question: can GitHub and Sentry be registered as Grafana
datasources that fetch their own data, with no app-side code? Yes — Grafana
ships official signed backend plugins for both (`grafana-github-datasource`,
`grafana-sentry-datasource`, both `alerting: true`). Phase 7 registers them
and moves the one signal that maps cleanly onto a plugin query off the
collector.

- **Both plugins are baked into the wrapper image**
  (`GF_INSTALL_PLUGINS` in `infra/grafana.Dockerfile`) and provisioned as
  datasources `github` / `sentry`
  (`infra/grafana/provisioning/datasources/issue-signals.yml`), tokens fed by
  `$__env{GRAFANA_GITHUB_DATASOURCE_TOKEN}` /
  `$__env{GRAFANA_SENTRY_DATASOURCE_TOKEN}` — the same non-`GF_`
  provisioning-interpolation mechanism `NOTIFY_EMAIL_TO` uses, added to the
  three deploy-secret lists and the Grafana allowlist in
  `api/scripts/check_kamal_secrets.sh`. The GitHub secret name is prefixed
  `GRAFANA_` because GitHub Actions forbids a repo secret named `GITHUB_*`.
- **Only `sentry_unresolved_issues` moved.** The `IssueSentryUnresolved`
  alert rule and the "Unresolved Sentry issues" `service-health` panel now
  run a `sentry` Issues query (`is:unresolved`, count reduce, `> 0`); the
  gauge, `collectSentryIssues`, and the `sentryapi` dependency are gone from
  `IssueSignalCollectorJob` (`cmd/api/main.go` drops the `sentryClient` arg —
  the client stays, `WeeklyDigestJob` and the `get_sentry_issues` MCP tool
  still use it). The job keeps its name — it still collects the GitHub CI,
  storage and schema-size signals.
- **Every GitHub signal stays a Prometheus gauge.** The GitHub plugin's
  per-PR-check, workflow-conclusion and Dependabot/secret-scanning coverage
  is looser than `internal/github`'s, so `github_failing_pull_requests`,
  `github_workflow_run_failed`, `github_workflow_run_duration_seconds` and
  `github_open_security_alerts` are unchanged. The `github` datasource is
  provisioned for ad-hoc exploration; migrating those signals (and dropping
  the GitHub half of the collector) waits on validating the plugin queries
  against live data — tracked as a follow-up on #1570.
- `scripts/validate_grafana_dashboards.py` now allows the `github`/`sentry`
  datasource uids and only requires `expr` on Prometheus targets;
  `scripts/verify_grafana_image.sh` exports dummy tokens and asserts all
  three datasource uids provisioned.
>>>>>>> 28fd6350 (Register GitHub + Sentry as Grafana datasources; move the Sentry issue signal off the collector)

## Consequences

- All alerting now lives in one place (Grafana). A contributor asking "why
  didn't I get an alert" has exactly one system to check.
  `docs/adr-0011-slow-transaction-thresholds.md`'s classification/threshold
  reasoning still governs the `/monitoring` trending list and the weekly
  digest, neither of which is an alert.
- The main-branch-CI-failure and R2-usage-threshold alerts were an accepted,
  documented gap for #1468/#1528; Phase 3 (#1529) closed both via the
  `service-health` gauge alerts. `ci_duration_high` was the one still-open
  gap; Phase 6 (#1556) added the `github_workflow_run_duration_seconds`
  gauge and an `app-performance` panel but deliberately no alert.
- Kamal container naming isn't Tofu-managed, so the Prometheus scrape
  targets for `api` and (added in #1528) `web` (`infra/prometheus.yml`)
  assumed a specific Docker network alias that could not be verified against
  real infra in this change — flagged in `infra/README.md` as the one thing
  worth confirming by hand after the first real deploy. **That assumption was
  wrong and the check was never performed**; Phase 5 (#1554) below records
  what it cost and replaces the mechanism.
- This is the second of two Grafana-adjacent issues (after #1469/ADR-0021);
  end-to-end Grafana login and alert delivery could not be verified against
  real infra in this change either — the same caveat ADR-0021 recorded for
  its own half.

## Revisit when

CI duration needs a real *alert*, not just a panel — Phase 6 (#1556) added
the `github_workflow_run_duration_seconds` gauge and an `app-performance`
panel, but no alert rule, on the reasoning that failing-run signal already
exists and duration is a trend to read rather than be paged on. If that
changes, a `ci_duration_high` rule in `infra/grafana/provisioning/alerting/`
now has a metric to evaluate.
