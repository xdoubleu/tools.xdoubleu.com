# ADR-0017: Pin long-request handler deadlines under the edge proxy's ceiling

- Status: Accepted
- Issues: #672, #1113
- Affects: `api/cmd/api/routes.go`, `api/internal/digitalocean/logs_live.go`, `config/deploy.api.yml`

## Context

A handler that outlives the edge proxy's response timeout gets its connection
**reset before it writes a byte** — no server log, no Sentry event. The failure
is completely silent server-side.

This bit `GetDeployLogs` twice (#672) under the old DigitalOcean edge (Envoy, a
hard ~25s that wasn't configurable).

## Decision

Handler deadlines are set below whatever the edge ceiling is. The ones added for
#672 are still in force:

| Constant | Location | Value |
|---|---|---|
| `deployLogsCtxTimeout` | `api/cmd/api/routes.go` | 20s |
| `liveLogDeadline` | `api/internal/digitalocean/logs_live.go` | 8s |

Since #1113 the edge is kamal-proxy, whose ceiling **is** configurable
(`proxy.response_timeout` in `config/deploy.api.yml`, currently unset so Kamal's
default applies).

So raising a handler deadline is now possible — but **must be paired with raising
that**. Any handler must still finish, or start streaming, under whichever
ceiling is in effect; a deadline set above it never fires.

## Alternatives considered

### Relying on the proxy timeout alone

Rejected: the proxy's reset produces no server-side signal at all. The in-handler
deadline is what makes the timeout observable and lets the handler return a real
error.

### Asserting the constants in a test

Not done. See Consequences.

## Consequences

- **Enforced only by convention and comments — no test asserts these constants
  stay under the ceiling.** Changing `proxy.response_timeout` and these constants
  independently will silently reintroduce #672's failure mode.

## Revisit when

Someone adds a test binding these constants to the configured
`proxy.response_timeout`, which would close the enforcement gap.
