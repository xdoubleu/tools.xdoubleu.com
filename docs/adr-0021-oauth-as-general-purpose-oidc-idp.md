# ADR-0021: The embedded authorization server also issues OIDC ID tokens and supports confidential clients

- Status: Accepted
- Issues: #1469 (follows #1468)
- Affects: `api/internal/oauth2as/`, `api/cmd/api/oauth2as.go`, `api/cmd/api/routes.go`, `api/cmd/api/migrations/00044_*`, `api/cmd/api/migrations/00045_*`

## Context

ADR-0006's AS served only MCP: public clients, `offline_access`, opaque tokens,
dynamic registration. Grafana SSO (`generic_oauth`) needs OIDC, a confidential
client with a fixed id/secret, and a role claim. Grafana's own `[auth.github]`
would be cheaper but Grafana-only; a general OIDC IdP is reusable.

## Decision

- **ID tokens**: fosite's `OpenIDConnectExplicitFactory`/`RefreshFactory`, RS256
  with `OAUTH_OIDC_PRIVATE_KEY` (ephemeral per boot if unset). Access tokens
  stay opaque HMAC. JWKS at `GET /oauth2/jwks`; discovery at
  `/.well-known/openid-configuration`.
- **Scopes**: `openid`, `profile`, `email` join `offline_access`. Clients get
  only scopes they're registered for, so MCP clients are unaffected.
- **Confidential clients**: `public = false` with a bcrypt `secret_hash`;
  `client_secret_basic` or `_post`. PKCE stays enforced for all clients.
- **Static `grafana` client**, seeded by `00045` with a NULL hash;
  `EnsureGrafanaClientSecret` reconciles it from `OAUTH_GRAFANA_CLIENT_SECRET`
  each boot. `/oauth2/register` stays public-client-only.
- **Role claim**: `role = Admin` only for admins, **omitted otherwise**. With
  Grafana's `role_attribute_strict: true`, non-admins can't complete SSO —
  Grafana is admin-only. `email_verified` is hardcoded `true` (accounts are
  admin-provisioned).

## Alternatives considered

- **Userinfo endpoint** — ID-token claims suffice; less attack surface.
- **Confidential registration via `/oauth2/register`** — needs admin-gated
  registration tokens for one client.
- **Grafana `[auth.github]`** — Grafana-only.
- **External IdP** — out of scope (ADR-0005).

## Consequences

- This is the most security-sensitive package; `oidc_test.go` covers the
  confidential/public and scope paths, and `TestObserve_NeverLogsCredentials`
  covers `client_secret`.
- Rotating `OAUTH_GRAFANA_CLIENT_SECRET` applies on next boot; rotating
  `OAUTH_OIDC_PRIVATE_KEY` re-logs-in Grafana users.

## Revisit when

A third OAuth client appears (static seed vs admin-gated registration), or
first-party auth gains email verification (stop hardcoding `email_verified`).
