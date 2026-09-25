# ADR-0006: Embed `ory/fosite` as the MCP authorization server, and grant `offline_access` server-side

- Status: Accepted — extended by ADR-0021 (OIDC + confidential clients)
- Issues: #1039, #1177, #1715
- Affects: `api/internal/oauth2as/`, `api/cmd/api/oauth2as.go`, `api/cmd/api/mcp.go`, `web/app/oauth/consent`

## Context

`/apps/mcp` needs OAuth 2.1. The authorization server was a hardcoded Supabase
Cloud URL, even though first-party auth (ADR-0005) already had every piece.

## Decision

The api is **both** resource server and authorization server:
`internal/oauth2as` embeds `ory/fosite` with PKCE, RFC 7591 dynamic client
registration, and RFC 8414 metadata. `mcpAuthServerIssuer` is `cfg.AuthIssuer`
(default `cfg.APIURL`). The web `/oauth/consent` page drives approval.

- **`offline_access` is granted server-side** (`grantOfflineAccess`,
  `scopes.go`), not just advertised. MCP clients routinely send no `scope`;
  without the grant they get no refresh token and must re-authenticate hourly
  (#1177).
- **Every `/oauth2/*` rejection is logged** (`observe.go`), because fosite
  writes errors only to the response, never slog. A failed `refresh_token`
  grant logs at **Error only on detected reuse** (a rotated-out token replayed
  past the 30s `refreshTokenReuseGracePeriod`, matched via
  `fosite.ErrInactiveToken`), since fosite then revokes the whole token family.
  Every other 4xx — expiry, unknown token, bad PKCE, denied consent — is
  **Warn** (#1715).

## Alternatives considered

- **External AS** — one needless external dependency in a self-contained flow.
- **Advertise-only `offline_access`** — what broke in #1177.

## Consequences

- **Never log credentials** — the form carries code, verifier, and refresh
  token. `TestObserve_NeverLogsCredentials` guards this.
- A new scope means revisiting `grantOfflineAccess` (ADR-0021 did).

## Revisit when

Partly superseded by ADR-0021 (non-MCP client: Grafana). This ADR still governs
the MCP flow, consent page, and rejection logging.
