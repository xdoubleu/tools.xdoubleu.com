# ADR-0006: Embed `ory/fosite` as the MCP authorization server, and grant `offline_access` server-side

- Status: Accepted — extended by ADR-0021 (OIDC + confidential clients)
- Issues: #1039, #1177
- Affects: `api/internal/oauth2as/`, `api/cmd/api/oauth2as.go`, `api/cmd/api/mcp.go`, `web/app/oauth/consent`

## Context

The MCP server at `/apps/mcp` needs OAuth 2.1. Before #1039 the authorization
server was a hardcoded Supabase Cloud URL — an external dependency for a flow the
api already had every ingredient for, once ADR-0005 made auth first-party.

## Decision

The api is **both** the resource server (`ResolveToken` verifies the bearer
token) **and** the authorization server. `internal/oauth2as` embeds `ory/fosite`:
PKCE-enforced, RFC 7591 dynamic client registration, RFC 8414 metadata at
`/.well-known/oauth-authorization-server`. Wired up in `cmd/api/oauth2as.go` and
`cmd/api/mcp.go`; `mcpAuthServerIssuer` points at `cfg.AuthIssuer`, which
defaults to `cfg.APIURL`.

The web `/oauth/consent` page drives the approval, calling the api's own
`/oauth2/*` endpoints directly — no external Auth provider involved.

### `offline_access` is granted server-side, not just requested

`offline_access` is the only scope this server supports and the sole gate on
whether fosite issues a refresh token. It is advertised in both metadata
documents and echoed in the RFC 7591 registration response, *and*
`AuthorizeHandler` **grants it on top of whatever the client actually
requested** (`grantOfflineAccess`, `internal/oauth2as/scopes.go`).

The server-side grant is the part that matters: MCP clients routinely send **no
`scope` parameter**, and without it they get an access token with no refresh
token and a forced interactive re-authentication every hour (#1177).

### Every `/oauth2/*` rejection is logged

`internal/oauth2as/observe.go` logs every rejection from `/oauth2/authorize`,
`/oauth2/token` and `/oauth2/register`. fosite writes its errors straight into
the HTTP response and **never through slog**, and `usage_daily` counts
`oauth2/token` hits regardless of outcome — so before this, nothing server-side
distinguished a failed token exchange from a successful one, and #1177 stayed
invisible until a user reported it.

Log levels are deliberately split:

- A **failed `refresh_token` grant logs at Error only when fosite detected
  genuine reuse** — a rotated-out token replayed past
  `storage.go`'s 30s `refreshTokenReuseGracePeriod`
  (`refreshTokenReuseDetected` in `observe.go`, `errors.Is`-matched against
  `fosite.ErrInactiveToken`) — even though it's a 400. That's the one
  refresh-token failure that's a genuinely new signal: fosite responds to it
  by revoking the *entire* token family, not just rejecting one request, so it
  always means a working client just lost its whole session.
- **Every other refresh_token failure** — plain expiry, or a token fosite never
  found (malformed, tampered, already revoked, or a scanner probing
  `/oauth2/token` with a self-registered `client_id`, since dynamic client
  registration is public) — is routine, same as **every other 4xx**: expired
  authorization code, bad PKCE verifier, denied consent. All of these stay at
  **Warn**, so they can't bury the one signal above. This was tightened in
  issue #1715 after a single expired/not-found refresh-token rejection reached
  Sentry as an Error — the original "every refresh_token failure is Error"
  policy over-fired on exactly the routine cases this split now excludes.

## Alternatives considered

### Keeping an external authorization server (Supabase Cloud)

Rejected once #1039 made auth first-party: it left one hardcoded external
dependency in the middle of an otherwise self-contained flow.

### Only advertising `offline_access` and requiring clients to request it

That was the original behavior and it is exactly what broke (#1177). Real MCP
clients omit `scope`, so an advertise-only contract silently degrades to
hourly re-authentication.

### Logging fosite errors via its own response path

Not possible — fosite writes errors into the HTTP response and never through
slog, which is why `observe.go` exists as a separate layer.

## Consequences

- **Credentials must never be logged.** The request form carries the code, the
  PKCE verifier and the refresh token. `TestObserve_NeverLogsCredentials` is the
  guard — keep it passing.
- Adding a second scope means revisiting `grantOfflineAccess`. ADR-0021 did
  this — it now grants `offline_access` on top of whatever was requested only
  when the client is registered for it, and `openid`/`profile`/`email` were
  added alongside.
- See root `README.md` for the client-setup command.

## Revisit when

Superseded in part by ADR-0021: the "non-MCP OAuth client" trigger has fired
(Grafana). This ADR still governs the MCP flow, the consent page and the
rejection-logging policy; the OIDC/confidential-client extension lives in
ADR-0021.
