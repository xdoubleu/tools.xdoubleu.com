# ADR-0048: An admin delegates an MCP client to an agent account at consent

- Status: Accepted
- Affects: `api/internal/platform/oauth2as/handlers.go` (`Delegator`),
  `api/internal/platform/agentaccounts/store.go`, `api/internal/platform/oauth2as/wiring.go`,
  `global.agent_account_clients` (migration 00064), `web/app/oauth/consent/`

## Context

Connecting an agent through [ADR-0039](adr-0039-agent-accounts.md) takes an
admin page, a one-time secret, a Keychain item and a `headersHelper`. Every MCP
client already runs the OAuth flow with a signed-in person, and that flow is the
natural place to choose who the client acts as.

## Decision

- The consent page asks an admin who the client signs in as: themselves, a new
  agent account, or an existing standing one. People see no choice.
- A new agent is created on the page through `CreateAgentAccount` (no step-up
  code), with the per-app none/read/write grants picked there. Its
  client_credentials secret is never shown; rotating it later gives a headless
  credential.
- The approving POST carries `agent_account_id`. `AuthorizeHandler` refuses it
  from a non-admin, and the `Delegator` checks that the account is active and
  standing and that the client uses authorization_code. The authorize session's
  subject is then the agent user, so the client's access and refresh tokens act
  as the agent with its grants.
- `global.agent_account_clients` records which client acts as which account; a
  later approval replaces it, and approving as oneself removes it.
- Revoking an account also deletes the codes and tokens whose subject is its
  user, so a delegated client stops at once.
- Admins only for now.

## Alternatives considered

- Hand a delegated client a client_credentials secret: keeps the Keychain step.
- Bind the token to the person and add an "acting as" claim: every check would
  have to read it, the flaw ADR-0047 avoids with a distinct `agent` role.
