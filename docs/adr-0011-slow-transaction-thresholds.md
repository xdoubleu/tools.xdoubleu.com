# ADR-0011: Classify slow transactions by name shape; don't exclude WebSocket routes

- Status: Accepted
- Issues: #1310, #1320
- Affects: `api/internal/observability/jobs/threshold_alert.go`, `api/internal/communication/wstools/websocket.go`

## Context

`jobs.ThresholdAlertJob` has three slow-transaction rules (#1310):
`slow_transaction_http_high`, `_job_high`, `_frontend_high`. To apply the right
threshold it must know what kind of thing a transaction is — but **Sentry project
names are admin-configured free text**, so there is no reliable metadata to key
off.

## Decision

`classifyTransaction` in `threshold_alert.go` infers a transaction's class
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
until the connection closes. They therefore permanently breach
`slow_transaction_http_high` the moment any client session outlives 5s.

That is **accepted as an inherent property of a long-lived WebSocket route**, not
hidden from the rule.

What the rule is actually missing without a carve-out is a *bounded* signal for
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

## Consequences

- The `progressws` routes are expected to sit in permanent breach of
  `slow_transaction_http_high`. **That is not a bug**, and should not be
  "fixed" by adding an exclusion.
- Renaming a transaction can silently reclassify it and change its threshold.

## Revisit when

Sentry offers reliable structured transaction metadata, removing the need to
infer class from the name.
