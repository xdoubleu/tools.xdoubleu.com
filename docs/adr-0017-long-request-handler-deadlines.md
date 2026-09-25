# ADR-0017: Pin long-request handler deadlines under the edge proxy's ceiling

- Status: Accepted (the two constants below were since removed with their handlers; the rule applies to any long handler)
- Issues: #672, #1113
- Affects: `api/cmd/api/routes.go`, `api/internal/digitalocean/logs_live.go`, `config/deploy.api.yml`

## Context

A handler outliving the edge proxy's response timeout has its connection
**reset before it writes a byte** — no log, no Sentry event. This hit
`GetDeployLogs` twice (#672).

## Decision

Handler deadlines stay below the edge ceiling:

| Constant | Location | Value |
|---|---|---|
| `deployLogsCtxTimeout` | `api/cmd/api/routes.go` | 20s |
| `liveLogDeadline` | `api/internal/digitalocean/logs_live.go` | 8s |

The edge is kamal-proxy; its ceiling is `proxy.response_timeout` in
`config/deploy.api.yml` (unset, so Kamal's default). Raising a deadline
**requires raising that too** — a deadline above the ceiling never fires.

## Alternatives considered

- **Rely on the proxy timeout** — gives no server-side signal; the in-handler
  deadline returns a real error.

## Consequences

Enforced only by convention: **no test binds these constants to the ceiling.**

## Revisit when

A test binds them to `proxy.response_timeout`.
