# ADR-0011: Classify slow transactions by name shape; don't exclude WebSocket routes

- Status: Accepted; superseded in part by #1528 (the p95 *alert* moved to Grafana — see "Superseded in part" below)
- Issues: #1310, #1320, #1528
- Affects: `api/internal/observability/jobs/slow_transactions.go`, `api/internal/communication/wstools/websocket.go`, `web/lib/observability.ts`

## Context

The `/monitoring` trending "currently slow" list and the weekly digest's
slow-transaction section need to know what kind of thing a transaction is to
apply the right threshold — but **Sentry project names are admin-configured free
text**, so there is no reliable metadata to key off. (Originally this classified
the three per-class rules of `jobs.ThresholdAlertJob`, retired in #1528.)

## Decision

`classifyTransaction` in `slow_transactions.go` (mirrored in
`web/lib/observability.ts`) infers a transaction's class **purely from its name
shape**:

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

The p95 *latency alert* no longer runs here. `jobs.ThresholdAlertJob`,
`global.alert_states`, and the `slow_transaction_{http,job,frontend}_high`
notification sources are gone. Grafana now evaluates request/job/frontend p95
off real Prometheus histograms (`http_request_duration_seconds`,
`job_duration_seconds`, `web_vitals_seconds`) and routes breaches through its
own SMTP contact point (`infra/grafana/provisioning/alerting/`).

What this ADR still governs: the name-shape classification and the
`NextNodeServer.clientComponentLoading` / WebSocket-route decisions, which
still gate the `/monitoring` trending list (`currentlySlowTransactions`) and
the weekly digest's slow-transaction section — both Sentry-derived, and neither
an alert.

## Consequences

- The `progressws` routes are expected to sit permanently in the trending
  "slow" list. **That is not a bug**, and should not be "fixed" by adding an
  exclusion.
- Renaming a transaction can silently reclassify it and change its threshold.
- The classification lives in two places (`slow_transactions.go`,
  `web/lib/observability.ts`) that must be kept in sync.

## Revisit when

Sentry offers reliable structured transaction metadata, removing the need to
infer class from the name.
