# ADR-0011: Classify slow transactions by name shape; don't exclude WebSocket routes

- Status: Superseded by #1597 (classification removed)
- Issues: #1310, #1320, #1528, #1597
- Affects (historical): `api/internal/observability/jobs/slow_transactions.go`, `api/internal/communication/wstools/websocket.go`

## Context

Slow-transaction alerting needed per-class thresholds, but Sentry project names
are admin-configured free text — no reliable metadata to classify by.

## Decision (historical)

`classifyTransaction` inferred class from the transaction name:

| Shape | Class | Threshold | Why |
|---|---|---|---|
| HTTP-verb prefix | api handler | 5s | Under the 10s `httpWriteTimeout` |
| Leading `/` or embedded `.` | frontend | 5s | Page loads run 1–2s |
| anything else | background job | 60s | e.g. steam sync runs tens of seconds |

`NextNodeServer.clientComponentLoading` was excluded. The `progressws`
WebSocket routes were **not** excluded (#1320): their span lasts the whole
connection, so they always read slow. Instead `acceptWithHandshakeSpan`
(`wstools/websocket.go`) records the upgrade alone as a separate
`"<method> <path> [ws-handshake]"` transaction from a fresh context, giving a
bounded signal. That span still exists.

Grafana took over p95 alerting from Prometheus histograms (#1528), and #1597
removed the last reader (the weekly digest) and deleted `slow_transactions.go`.
`get_slow_transactions` never used the classification.

## Alternatives considered

- **Exclude `progressws` routes** — would hide real upgrade regressions.
- **Classify from Sentry metadata** — not available.

## Consequences

Nothing classifies transactions by name shape anymore.

## Revisit when

A consumer needs per-class transaction thresholds again — re-derive from this
reasoning rather than restoring the old file.
