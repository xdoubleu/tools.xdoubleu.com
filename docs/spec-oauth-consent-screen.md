# Spec: the OAuth consent screen

- Source of truth: `web/app/oauth/consent/`, `web/lib/oauth2as/consentClient.ts`, `api/internal/oauth2as/handlers.go`
- Issues: #1039

The authorization server behind this screen is ADR-0006.

## Shape

A server-rendered OAuth 2.1 consent screen for the apps MCP server (`/apps/mcp`
on the api), driving the api's own embedded fosite authorization server directly
— no external Auth provider.

## Behavior

1. The api's `AuthorizeHandler` (`api/internal/oauth2as/handlers.go`) redirects
   here with the pending authorization request's own query params (`client_id`,
   `scope`, `state`, `redirect_uri`, `code_challenge`, …) **echoed verbatim**.
2. The page reads the `accessToken` cookie server-side and calls
   `GET /oauth2/consent-info` (`lib/oauth2as/consentClient.ts`) for the client
   name and scope to display.
3. The `approveAuthorization`/`denyAuthorization` server actions
   (`app/oauth/consent/actions.ts`) POST the same query params plus
   `consent=allow|deny` back to `/oauth2/authorize`, forwarding the session
   cookie.
4. They then `redirect()` the browser to whatever `Location` fosite responds with
   — the OAuth client's own `redirect_uri`, carrying a code or an
   `access_denied` error.

## Invariants

- Query params are echoed verbatim in both directions; the consent screen is not
  a place to normalize or default them.
- No env config is needed beyond the existing `API_URL`.

## Known gaps

None recorded.
