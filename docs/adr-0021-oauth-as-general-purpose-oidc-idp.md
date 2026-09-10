# ADR-0021: The embedded authorization server also issues OIDC ID tokens and supports confidential clients

- Status: Accepted
- Issues: #1469 (follows #1468)
- Affects: `api/internal/oauth2as/`, `api/cmd/api/oauth2as.go`, `api/cmd/api/routes.go`, `api/cmd/api/migrations/00044_*`, `api/cmd/api/migrations/00045_*`

## Context

ADR-0006 built `internal/oauth2as` (ory/fosite) purely to serve the MCP OAuth
flow: PKCE-enforced authorization code + refresh, **public clients only**, one
supported scope (`offline_access`), opaque HMAC tokens, RFC 7591 dynamic
registration only. Its "Revisit when" named exactly this trigger — "a non-MCP
OAuth client becomes a requirement".

That client is **Grafana** (deployed by #1468, which replaces the hand-rolled
metrics/alerting stack with Prometheus + Grafana). Grafana's `generic_oauth`
provider cannot talk to the AS as ADR-0006 left it: it needs OIDC (an ID token
or a userinfo endpoint), a **confidential** client with a real `client_secret`,
a **fixed** client_id/secret it can be configured with ahead of time, and a
**role claim** to map onto its roles.

Grafana ships its own local login, so this is a convenience layer — one identity
for the app and Grafana — not a requirement to run Grafana. The cheaper
alternative (Grafana OSS's first-class `[auth.github]` with a pre-created user)
was considered and rejected: it only ever covers Grafana, whereas making the AS
a general OIDC IdP is a reusable capability.

## Decision

`internal/oauth2as` becomes a general-purpose OpenID Connect provider, without
disturbing the MCP flow.

- **OIDC ID tokens.** `server.go` composes fosite's
  `OpenIDConnectExplicitFactory` / `OpenIDConnectRefreshFactory` on top of the
  existing OAuth2 handlers, signing ID tokens **RS256** with an RSA key from
  `OAUTH_OIDC_PRIVATE_KEY` (PEM). When that env var is empty the api generates
  an ephemeral key at boot — fine for local dev and for a single relying party
  that just re-logs-in after a restart. **Access tokens stay opaque HMAC
  tokens** introspected via `ResolveAccessToken`; only the ID token is
  asymmetric. The public key is published at `GET /oauth2/jwks` and the
  document is also served at `/.well-known/openid-configuration`.

- **Scopes.** `openid`, `profile`, `email` join `offline_access` in
  `SupportedScopes`. A client may only request a scope it is registered for
  (fosite's default scope strategy), so the MCP clients — registered for
  `offline_access` alone — are unaffected. `grantOfflineAccess`'s server-side
  grant (ADR-0006) is unchanged.

- **Confidential clients.** A client row with `public = false` and a non-NULL
  bcrypt `secret_hash` is a confidential client; fosite then requires and
  verifies a `client_secret` via either `client_secret_basic` or
  `client_secret_post`. `GetClient` already surfaced both columns — no new
  client type was needed. PKCE stays enforced for every client, confidential
  ones included (Grafana sets `use_pkce = true`).

- **The Grafana client is static, not dynamic.** Migration `00045` seeds one
  fixed row (`id = 'grafana'`, redirect
  `https://tools.xdoubleu.com/grafana/login/generic_oauth`) with a NULL
  `secret_hash`. `oauth2as.EnsureGrafanaClientSecret` reconciles the hash from
  `OAUTH_GRAFANA_CLIENT_SECRET` on every boot, so the plaintext secret never
  enters version control. The dynamic `/oauth2/register` endpoint stays
  public-client-only.

- **Role claim.** The ID token carries `role` = `Admin` when the user is
  `models.RoleAdmin` (resolved through the same DB-overlay path as
  `GetCurrentUser`), and **omits the claim entirely otherwise**. Since #1468's
  successor work (#1527) Grafana is the only relying party and its dashboards
  expose host + Postgres internals, so access is admin-only: with
  `role_attribute_strict: true` a token with no `role` is refused, which is the
  intended outcome for a non-admin — they get a valid ID token but cannot
  complete Grafana SSO. `email`/`email_verified` (hardcoded `true` —
  first-party accounts are admin-provisioned, ADR-0005) and
  `name`/`preferred_username` follow the granted `email`/`profile` scopes.
  Grafana maps these with `role_attribute_path` / `login_attribute_path`.

## Alternatives considered

- **A userinfo endpoint.** Grafana can read claims from the ID token via
  `role_attribute_path` etc., so `/oauth2/userinfo` would be one more
  authenticated endpoint on the most security-sensitive package for no gain.
  Dropped.
- **A `DefaultOpenIDConnectClient` fosite client type.** Unnecessary: a plain
  `DefaultClient` with `Public = false` already triggers fosite's
  client-secret authentication for both basic and post methods.
- **Confidential clients through `/oauth2/register`.** Would mean gating that
  endpoint with an admin token and issuing registration access tokens. Not
  worth it for one static client; revisit if a second confidential client
  appears.
- **Grafana `[auth.github]`.** See Context — cheaper, but Grafana-only.
- **An external IdP replacing first-party auth.** Explicitly out of scope
  (ADR-0005): app-access lists, the family model (ADR-0008) and the admin role
  gating observability tools all stay in `internal/auth` regardless.

## Consequences

- This is the most security-sensitive package in the repo and its client model
  just widened — the exact place an authentication bypass would hide. The
  confidential-vs-public and scope-grant paths carry dedicated tests
  (`oidc_test.go`), and `TestObserve_NeverLogsCredentials` gained a
  `client_secret` case.
- **Rotating `OAUTH_GRAFANA_CLIENT_SECRET`** is picked up automatically on the
  next boot (the reconciler rewrites the hash when the configured secret stops
  matching). **Rotating `OAUTH_OIDC_PRIVATE_KEY`** invalidates every
  outstanding ID token — Grafana users re-log-in.
- Two new deploy secrets, in the three lists `make lint/kamal-secrets` checks.
- The Grafana redirect URI is committed before #1468 finalises the Grafana
  host; if it lands elsewhere a follow-up migration corrects the seeded row.
- End-to-end Grafana login cannot be verified until #1468 deploys Grafana; this
  issue delivers and tests the AS capability plus the seeded client.

## Revisit when

A third OAuth client appears (is a static seed still the right shape, or is it
time for an admin-gated confidential registration path?), or first-party auth
grows an actual email-verification state (then `email_verified` must stop being
hardcoded).
