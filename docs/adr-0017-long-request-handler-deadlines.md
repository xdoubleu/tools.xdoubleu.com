# ADR-0017: One request budget under the server's WriteTimeout

- Status: Accepted
- Issues: #672, #1113, #2599, #2656
- Affects: `api/internal/communication/httptools/budget.go`, `api/cmd/api/routes.go`, `api/cmd/api/main.go`, `api/internal/mcptools/tool_calls.go`, `config/deploy.api.yml`

## Context

The api's `http.Server` `WriteTimeout` (10s) is set when a request's headers
are read. A response written after it never arrives: the connection closes,
the metrics record the handler's status, and nothing is logged or sent to
Sentry. kamal-proxy's `proxy.response_timeout` (Kamal's 30s default) sits
outside it and never fires first. Kobo library syncs were cut this way when
the upstream store call alone took its 10s timeout.

## Decision

`httptools.Budget` gives every request a context deadline of
`RequestBudget` (8s), `WriteSlack` below `WriteTimeout`, so a handler that
runs out still writes its error, and logs the overrun.

- Routes that legitimately take longer are listed in `isLongRequest`
  (`api/cmd/api/routes.go`): they get `LongRequestBudget` and a write
  deadline moved to match via `http.ResponseController`.
- WebSocket upgrades are exempt: hijacking clears the connection's deadlines.
- A request-path call that must leave the handler time to answer times out at
  `UpstreamTimeout`; work that can't fit the budget runs as a job.
- `request_budget_test.go` fails if the budgets stop fitting `WriteTimeout` or
  `proxy.response_timeout`.

## Alternatives considered

- **Raise `WriteTimeout`** — every handler would then hang on a stalled
  dependency for longer, and the cut would still be silent.
- **`http.TimeoutHandler`** — buffers the whole response and breaks
  `Flush`/`Hijack`.

## Consequences

A handler that drops the request context (`context.WithoutCancel`) escapes
the deadline; `httptools.Detach` keeps the deadline while dropping
cancellation. The MCP SDK drops it for tool calls, so `mcptools` restores it
less a second, leaving the SDK time to write the result.
