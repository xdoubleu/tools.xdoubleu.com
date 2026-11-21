# ADR-0056: Agents read and write the data of the person they act for

- Status: Accepted
- Affects: `api/internal/platform/mcptools/agent_grants.go`,
  `api/internal/platform/agentaccounts/store.go`, `global.agent_accounts`.
  Supersedes the "or read another person's data" rule of
  [ADR-0047](adr-0047-first-class-agent-users.md).

## Context

An agent is its own user and handlers scope queries to the caller's user id, so
it saw only its own data plus family-shared data. Learning paths have no family
sharing: an agent saw an empty list, and the owner had to sign in to the MCP as
themselves.

## Decision

- **Principal.** A standing agent records the person it acts for in
  `agent_accounts.principal_user_id`: the admin who created it, or the one who
  delegates a client to an account that has none. Job accounts and accounts the
  factory made have none.
- **Resolution.** For a tool or resource of a per-user app (any held app but
  `factory`), `AgentGrants` runs the call as the principal's user id. The role
  stays `agent` and the grants stay the agent's, so `RequireAppAccess` and
  `RequireAppWrite` still bound it per app and level.
- **Unchanged.** `factory_*` tools keep the agent's own identity, so the board
  still attributes work to the agent. Admin, observability and agent-management
  tools stay closed to the agent role, and an agent can't use the HTTP app
  routes.
- **Family.** Data then follows the principal's own family membership, so an
  agent's own family join adds nothing for these apps.

## Consequences

An agent granted `write` on an app can change everything its principal owns
there; the grant level is the limit. Revoking the account or its grant ends it.
