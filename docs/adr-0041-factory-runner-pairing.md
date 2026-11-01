# ADR-0041: A runner pairs with a single-use code and proves itself with a device token

- Status: Accepted
- Issues: #2705, #2701
- Affects: `api/apps/factory/` (`connect_runners.go`, `internal/services/runners.go`,
  migration `00003`), `api/cmd/api/routes.go` (`isLongRequest`)

## Context

The runner is a daemon on the owner's Mac that will run agent jobs. It talks to
the factory over an outbound connection, so the server must be able to tell it
from anyone else, and the runner must not trust what the server sends
([#2701](https://github.com/xdoubleu/tools.xdoubleu.com/issues/2701) decision
16). It is not a user: no cookie, no OAuth token, no MCP session.

## Decision

- **Two services.** `RunnerAdminService` (list, create, revoke) is for an admin in
  a browser session. `RunnerService` (`Pair`, `Heartbeat`) is the runner's own and
  is mounted with no cookie gate; each call authenticates itself. The runner can't
  reach `TaskService`: a device token is no session, so the task API sees no admin.
- **Pairing code.** Creating a runner needs a TOTP or recovery code
  (`VerifyCurrentFactor`) and returns a single-use code: ten characters of
  Crockford base32 (50 bits), valid ten minutes, shown once. Only its SHA-256 is
  stored. `Pair` trades it for a device token in one UPDATE, so a code works once.
- **Device token.** 32 random bytes with a `frt_` prefix, returned once, stored as
  a SHA-256 hash (the token is random, so no slow hash is needed). The runner sends
  it as a bearer token. Revoking sets `revoked_at`, and a revoked token stops
  authenticating on the next call.
- **One runner at a time.** A partial unique index allows one row with no
  `revoked_at`. A second create is a conflict until the first is revoked; a runner
  that never paired and whose code ran out is revoked automatically.
- **Heartbeat.** An outbound long-poll: the server records `last_seen`, holds the
  request for 20 seconds, records again and answers, and the runner calls again.
  The route is a long route (25-second budget, adr-0017). A runner is online if
  heard from in the last 45 seconds; the board polls every 15.

## Alternatives considered

- A long-lived shared secret in the environment: no way to revoke or rotate it
  from the phone, and nothing ties it to a person's step-up.
- The runner as an OAuth client like an agent: it would carry a user identity and
  tools it must not have.
- A WebSocket: an inbound upgrade through the proxy for no gain over a poll.

## Consequences

- The pairing code is brute-forceable only within ten minutes against the global
  rate limit; 50 bits makes that impractical, and a used or expired code is gone.
- Losing a pairing code or a device token means revoking and pairing again.
- The runner side (`factory-runner pair`, the Keychain, its own repo allowlist and
  job validator) lands in the next slice.

## Revisit when

More than one runner is needed, or the runner should be reachable without polling.
