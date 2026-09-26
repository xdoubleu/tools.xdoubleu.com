# ADR-0025: Headless routines authenticate by client_credentials as an observability-only service role

- Status: Accepted
- Issues: #1901, #1851
- Affects: `api/internal/oauth2as/`, `api/cmd/api/connect_access.go`
  (`requireObservability`), `api/cmd/api/migrations/00053_*`

## Context

The scheduled agent routines move to GitHub Actions (`opencode run`), which
can't do the interactive OAuth 2.1 + PKCE flow `/apps/mcp` requires. Refresh
tokens rotate ([ADR-0005](adr-0005-first-party-auth-replacing-gotrue.md)), so
a stored refresh token isn't an option either. The routines need the
observability tools, and they must not see user data.

## Decision

- The embedded AS enables the **client_credentials** grant. fosite only honours
  it for a confidential client whose `grant_types` includes it. The one such
  client is the static `routines` client, seeded like Grafana's
  ([ADR-0021](adr-0021-oauth-as-general-purpose-oidc-idp.md)): the migration
  leaves `secret_hash` NULL, and boot reconciles it from
  `OAUTH_ROUTINES_CLIENT_SECRET`. Tokens last an hour; there is no refresh
  token.
- `TokenHandler` sets the token's subject to the seeded service user
  (`machineSubject`). A client_credentials client without a mapping is refused
  with `unauthorized_client`.
- The service user has the new **`service`** role and no app access. It has no
  usable password hash, so it can't sign in, and `GetAllUsers` leaves it out.
- Observability MCP tools use `requireObservability` (admin or service).
  Everything else is unchanged and denies `service`: `requireAdmin` (access
  RPCs, admin pages), `RequireAppAccess` (app tools, learningpaths writes).
  Fosite tokens are only accepted on `/apps/mcp`.

## Alternatives considered

- Static bearer token mapped to the user: bypasses the AS, never expires, and
  can't be revoked without a deploy.
- The service user as `admin`: admin also means `ListUsers`, `SetRole`, every
  app's data and admin pages, so one injected prompt could escalate.
- One client per routine with a tool allowlist: tighter, but the owner chose a
  single identity limited to observability.

## Consequences

- One secret opens every observability tool, including the mutating ones
  (`resolve_sentry_issue`, `dismiss_security_alert`, `record_action`). Rotate
  it by updating the Actions and deploy secrets together.
- Observability output that embeds user data (`get_logs` lines,
  `get_oauth_connections`' connected-by email) remains visible to the service
  role.
- A routine running longer than the one-hour token lifetime loses MCP access
  mid-run.

## Revisit when

A routine needs a narrower tool set than "all observability", or a second
machine client appears.
