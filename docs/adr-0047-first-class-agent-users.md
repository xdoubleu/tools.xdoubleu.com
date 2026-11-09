# ADR-0047: Agents are platform users with granted app access

- Status: Accepted
- Issues: epic `first-class-agents`, spike 453c1c64
- Affects: `api/internal/agentaccounts/`, `api/internal/mcptools/`,
  `api/internal/account/access.go`, `api/internal/oauth2as/seed.go`,
  `global.app_access`. Supersedes the "only `factory_*` tools" rule of
  [ADR-0039](adr-0039-agent-accounts.md) and [ADR-0040](adr-0040-factory-mcp-tools.md).

## Context

An agent account can use the board and nothing else: `AgentToolsOnly` refuses
every other tool, resource and prompt, and `SetAppAccess` refuses agent users. A
connected agent can't read the library, author learning paths or pick for the
taste profile, yet sees the whole board. Access should work for agents the way
it does for people: granted per app, revoked the same way.

## Decision

- **Identity.** An agent stays a user row with role `agent` (one user and one
  client_credentials client). The distinct role is the invariant that keeps
  person-only actions closed to it; a `user` row with an "is agent" flag would
  leave each check to remember the flag. Agents are shown in an Agents section,
  not the people list.
- **App access.** Agents use `global.app_access` like people, with a new
  `level` column (`read` | `write`; existing rows `write`, since a person's
  access has always been both). A new agent has no rows. A dedicated admin RPC
  grants and revokes agent access; `SetAppAccess` keeps refusing agents, so the
  person path can't be used by accident.
  - `RequireAppAccess` admits an agent with any grant; mutating tools use a new
    `RequireAppWrite`, which needs `write`. A write tool still needs its own ADR.
  - `factory` is an app like the others. A standing agent account is created
    with a `factory` `write` grant, which keeps today's behavior.
- **Tool list.** `AgentToolsOnly` becomes grant filtering: an agent lists the
  tools of the apps it holds. Resources and prompts follow the same grants.
  `factory_*` tools stay hidden from non-agents.
- **Never, whatever the grants.** An agent can't: call `SetRole`, `SetAppAccess`
  or any agent-management RPC; use admin or observability tools
  (`RequireAdmin` and `RequireObservability` exclude it); move a task into
  Ready; or read another person's data. Handlers already scope queries to the
  caller's user id, so an agent sees its own data plus whatever a family grant
  adds.
- **Job accounts.** Unchanged and never granted apps: the store refuses app
  access for a row with `job_id`. A job's account holds only the task-scoped
  `factory` grant.
- **Family.** An agent has a family-of-one like any user. It joins a person's
  family only when that person adds it (no email invite; an agent can't
  accept). Family data then needs both membership and the app grant.
- **Connection.** Keep client_credentials with no refresh token (RFC 6749
  §4.4.3): the client secret is the long-lived credential and the
  `headersHelper` already mints a token on a 401. What ends the manual dance is
  **secret rotation with overlap**: an admin rotates in the UI, the old secret
  keeps working for 24 hours, and the Keychain item is replaced. Server: a
  second `secret_hash` with an expiry. Client: the Keychain update only.
- **Routines.** The `service` role stays what it is (observability, no user
  data). Folding it into agents is out of scope.

## Slices

| # | Slice | Size |
|---|-------|------|
| 1 | `level` migration, agent grant RPC, Agents section in the admin UI | M |
| 2 | Gate swap: `RequireAppWrite`, grant-based tool list, drop `AgentToolsOnly`; a test that walks every registered tool and RPC as an agent with no grants | M |
| 3 | Read tools per app (books, feeds, games, movies, podcasts, taste, trains, recipes, mealplans, shoppinglist, learningpaths, dashboard), one task each | S each |
| 4 | Per-app write grants, each with its own ADR | S each |
| 5 | Agent in a family | S |
| 6 | Secret rotation with overlap, and the connection UX (3c1dd5ec) | M |
| 7 | Mark ADR-0039/0040 partly superseded, update `AGENTS.md` | S |

Slices 1 and 2 land first; 3–5 need slice 2; 6 is independent.

## Alternatives considered

- **Flag on a `user` row:** every person-only check must remember the flag.
- **A separate agent permission table:** two access systems to keep in sync.
- **Refresh tokens:** against the grant type's spec, and a stored refresh token
  is a second secret to revoke.

## Consequences

- Granting an app lets an agent read everything that app shows its own user.
- Existing accounts keep working: a migration gives them the `factory` grant.

## Revisit when

Agents need per-resource rather than per-app grants, or the `service` role
should become an agent.
