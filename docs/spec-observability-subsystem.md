# Spec: the `observability` subsystem

- Source of truth: `api/internal/observability/`, `api/cmd/api/usage_middleware.go`, `api/cmd/api/main.go`
- Issues: #1027, #915, #1040, #848, #1217, #1214, #1253

Cross-cutting decisions extracted from this subsystem live in ADR-0010 (two
digest emails), ADR-0011 (slow-transaction thresholds) and ADR-0012 (Ubuntu
release check).

## Shape

`internal/observability` holds the job-tracking and usage-recording primitives
plus the cross-app jobs registered directly on `main.go`'s **own** job queue
(rather than any one app's), because they aren't scoped to a single app.

| Piece | Role |
|---|---|
| `TrackedJob` | Times/records every job run in `global.job_runs`, recovers panics, logs failures at Error so they reach Sentry |
| `UsageRecorder` | Per-endpoint request counts **and response bytes**, flushed to `global.usage_daily` |
| `jobs.IssueNotifierJob` | Polls Sentry/GitHub every 5 min, emails an admin the first time an issue/failing PR is seen |
| `jobs.TransactionLatencySnapshotJob` | Daily p95 duration/request-count snapshot per transaction |
| `jobs.ThresholdAlertJob` | Threshold rules, incl. the three slow-transaction ones (ADR-0011) |
| `jobs.WeeklyDigestJob` | `RunEvery = 7 days`, sends two emails (ADR-0010) |
| `jobs.HostMetricsSnapshotJob` | `RunEvery = 60s`, host metrics + retention pruning (#1040) |
| `jobs.WorkflowRunsSnapshotJob` | `RunEvery = 5m`, CI run history + `main`-failure alerts (#1217) |
| `hostmetrics.go` | Hand-rolled Prometheus text-exposition parser (no client library) |
| `LogRepoHandler` | `slog.Handler` teeing api's own log records into `global.log_entries` |

## Behavior

### Usage recording and response bytes (#1027)

The byte counter is fed by `cmd/api/usage_middleware.go`'s
`countingResponseWriter`, which **must keep forwarding `Flush`/`Hijack`** or
`progressws` WebSocket upgrades break.

Bytes measure what left the api — a proxy for what it read out of Postgres, not
a direct measure of database egress, but close enough on passthrough list
endpoints to identify the culprit. That is what #1027 needed and didn't have.

### Issue notification (#915)

`IssueNotifierJob` polls Sentry/GitHub every 5 minutes and emails an admin the
first time an issue or failing PR is seen, deduped via `global.notified_issues`.

The GitHub half **only alerts on failing PRs carrying the `dependencies`
label** (#915): Renovate opens those (see root `renovate.json5`) and nobody is
otherwise watching them — unlike a PR a user or a Claude Code session opened,
which already has someone driving it to green.

### Transaction latency (#848)

`TransactionLatencySnapshotJob` snapshots each transaction's p95
duration/request count from `sentryapi.Client.ListTransactionStats` into
`global.transaction_latency_daily` once a day.
`repositories.TransactionLatencyRepository.Trends` compares a recent window
against the prior one to flag regressing endpoints/pages, surfaced by
`get_slow_transactions`.

### Host metrics and log tee (#1040)

- `hostmetrics.go` scrapes `node-exporter:9100/metrics` for CPU/memory/disk
  usage with a hand-rolled parser — no client library.
- `HostMetricsSnapshotJob` scrapes and inserts one `global.host_metric_samples`
  row every 60s, then prunes both that table and `global.log_entries` past a
  **30-day** retention window.
- `LogRepoHandler` is composed into `main.go`'s handler chain alongside
  `sentrytools.NewLogHandler`, teeing every one of api's own log records into
  `global.log_entries` in-process. **`web`'s logs instead reach the same table
  over `POST /api/observability/logs`** — a plain HTTP endpoint gated by the
  `OBSERVABILITY_INGEST_SECRET` shared-secret header, since web holds no admin
  session to authenticate a Connect call with.

### Workflow run history (#1217)

`WorkflowRunsSnapshotJob` (`RunEvery = 5m`, matching `IssueNotifierJob`'s
cadence) polls `github.Client.ListWorkflowRuns` and persists each
newly-completed run plus its per-job breakdown into
`global.workflow_run_samples`/`global.workflow_job_samples`, so duration/failure
history survives past `github.Client`'s own 45s in-memory cache. It prunes both
past a **90-day** retention window.

Reusing the same `global.notified_issues` dedup `IssueNotifierJob` uses, and
gated by `NotificationSourceFailingMainCI`, it **emails an admin the first time a
run on `main` fails** — `main` deploys straight off a passing push with no
re-test, so a failure there is always a genuine incident.

`get_workflow_run_stats` (`cmd/api/connect_observability_external.go`) reports
this history as **aggregates only** — main-branch failures (expected empty), and
avg/p95 duration per workflow and per job — deliberately not another raw run list
like `get_workflow_runs` already is.

## Invariants

- **`countingResponseWriter` must forward `Flush`/`Hijack`.** Dropping either
  breaks WebSocket upgrades.
- **Every notification source checks `global.notification_settings` before
  notifying** (#1214), and skips **without writing to `notified_issues`** when
  disabled — so re-enabling picks the item back up rather than silently
  swallowing it.
- Cross-app jobs register on `main.go`'s own job queue, never an app's.
- This `internal/` package never imports an `apps/*` package. `main.go`'s
  `feedsHealthAdapter`/`feedsOpenItemsAdapter` bridge `*feeds.Feeds` into the
  job's own narrow `unhealthyFeedLister`/`openFeedItemsLister` interfaces.

## Known gaps

- Response bytes are an api-egress proxy, not true database egress.
- `get_workflow_run_stats` intentionally exposes no raw run list.
