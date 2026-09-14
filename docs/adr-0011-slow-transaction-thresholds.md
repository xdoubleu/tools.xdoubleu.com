# ADR-0011: Classify slow transactions by name shape; don't exclude WebSocket routes

- Status: Superseded by #1597 (the classification module was removed — see "Superseded" below)
- Issues: #1310, #1320, #1528, #1597
- Affects (historical): `api/internal/observability/jobs/slow_transactions.go`, `api/internal/communication/wstools/websocket.go`

## Context

The weekly digest's slow-transaction section needed to know what kind of thing
a transaction is to apply the right threshold — but **Sentry project names are
admin-configured free text**, so there was no reliable metadata to key off.
(Originally this classified the three per-class rules of
`jobs.ThresholdAlertJob`, retired in #1528.)

## Decision (historical — the module this describes was removed in #1597)

`classifyTransaction` in `slow_transactions.go` inferred a transaction's class
**purely from its name shape**:

| Shape | Class | Threshold | Reasoning |
|---|---|---|---|
| HTTP-verb prefix | api handler | 5s | Well under the 10s `httpWriteTimeout` in `main.go` that would otherwise kill the request first |
| Leading `/` or an embedded `.` | frontend | 5s | Typical page loads run 1–2s |
| anything else | background job | 60s | Some, like the steam sync, legitimately run tens of seconds |

`slowTransactionExcluded` carves out `NextNodeServer.clientComponentLoading`, a
Next.js-internal transaction that legitimately runs far longer than any real page
load, from the frontend class.

### WebSocket routes are deliberately not excluded (#1320)

games' and books' `GET .../api/progress` transactions — the `progressws`
WebSocket-upgrade routes in `apps/games/routes.go`/`apps/books/routes.go` — are
**deliberately not** in that exclusion list, even though sentryhttp's transaction
span covers the whole handler call, which for an upgraded socket doesn't return
until the connection closes. They therefore permanently register as slow the
moment any client session outlives 5s.

That is **accepted as an inherent property of a long-lived WebSocket route**, not
hidden from the list.

What the list is actually missing without a carve-out is a *bounded* signal for
that route family, and `wstools.WebSocketHandler.Handler()`'s
`acceptWithHandshakeSpan` (`internal/communication/wstools/websocket.go`) supplies
that separately: it measures **just the handshake/upgrade itself**
(`websocket.Accept`) as its own, separately-named Sentry transaction
(`"<method> <path> [ws-handshake]"`), started from a fresh context rather than the
request's — since `sentry.StartTransaction` hands back an already-present
transaction unchanged instead of starting a new one. So a real upgrade-path
regression still classifies as an HTTP handler and alerts at the normal 5s
threshold.

## Alternatives considered

### Excluding the `progressws` routes from the rule

Rejected: it would hide the route family entirely, including genuine
upgrade-path regressions. The handshake span gives the bounded signal without
suppressing anything.

### Classifying from Sentry metadata rather than the name

Not available — project names are admin-configured free text, which is precisely
why classification is name-shape-based.

## Superseded in part (#1528)

The p95 *latency alert* no longer ran here. `jobs.ThresholdAlertJob`,
`global.alert_states`, and the `slow_transaction_{http,job,frontend}_high`
notification sources were gone. Grafana evaluated request/job/frontend p95
off real Prometheus histograms (`http_request_duration_seconds`,
`job_duration_seconds`, `web_vitals_seconds`) and routed breaches through its
own SMTP contact point (`infra/grafana/provisioning/alerting/`).

After #1528, this ADR still governed the name-shape classification and the
`NextNodeServer.clientComponentLoading` / WebSocket-route decisions, which by
that point gated only the weekly digest's slow-transaction section — Grafana's
own histograms never went through this classification, and `GetSlowTransactions`'
`/monitoring` trending list was always the raw, unclassified `Trends()` output.

## Superseded (#1597)

`WeeklyDigestJob`'s slow-transaction section — the classification's last
remaining reader — was dropped: Grafana already alerts on p95 regressions in
real time, so the weekly restatement was redundant (adr-0010). With no reader
left, `slow_transactions.go` (`classifyTransaction`, `thresholdMsForClass`,
`slowTransactionExcluded`, `currentlySlowTransactions`) and its test were
deleted outright rather than kept as dead code. `GetSlowTransactions` and its
`get_slow_transactions` MCP tool are unaffected — they never used this
classification.

## Consequences

- Nothing in this repo classifies a Sentry transaction by name shape anymore.
- The WebSocket-route decision recorded above (don't exclude `progressws`
  routes) has no remaining code path to apply to; it's kept here purely as a
  record of the reasoning, should a future consumer reintroduce
  classification.

## Revisit when

A future consumer needs to classify transactions by class again — at which
point re-derive from this ADR's reasoning rather than reintroducing
`slow_transactions.go` unchanged, since the removed WebSocket-route
consequences may no longer all apply.
