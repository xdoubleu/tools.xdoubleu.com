import { getApiUrl } from '@/lib/env'

// Server-only client for the OAuth 2.1 consent flow (issue #1039), talking
// directly to this api's own embedded fosite authorization server instead of
// Supabase's proprietary `supabase.auth.oauth` endpoints. Both calls are
// plain fetches — no ConnectRPC framing, since /oauth2/* is plain HTTP per
// RFC 6749/7591/9207, not a Connect procedure.

export interface ConsentInfo {
  clientId: string
  clientName: string
  scope: string
}

interface ConsentInfoResponse {
  client_id: string
  client_name: string
  scope: string
}

// GET /oauth2/consent-info: echoes back the pending authorization request's
// client name and requested scope, keyed by client_id. Unauthenticated on
// this leg (see api/internal/oauth2as/handlers.go's ConsentInfoHandler) — it
// only reveals public client-registration details, never anything scoped to
// the signed-in user.
export async function getConsentInfo(query: URLSearchParams): Promise<ConsentInfo | null> {
  const clientId = query.get('client_id')
  if (!clientId) return null

  const params = new URLSearchParams({ client_id: clientId, scope: query.get('scope') ?? '' })

  const res = await fetch(`${getApiUrl()}/oauth2/consent-info?${params}`, { cache: 'no-store' })
  if (!res.ok) return null

  const data: ConsentInfoResponse = await res.json()
  return { clientId: data.client_id, clientName: data.client_name, scope: data.scope }
}

// POST /oauth2/authorize with the original authorization-request query
// params (echoed by the redirect that sent the browser to the consent page
// in the first place — see AuthorizeHandler) plus consent=allow|deny. The
// session cookie is forwarded so the handler can re-verify it server-side as
// defense in depth before minting a code.
//
// fosite's WriteAuthorizeResponse/WriteAuthorizeError reply with a real
// HTTP redirect (302 + Location) to the OAuth client's own redirect_uri, so
// this follows manually (redirect: 'manual') rather than letting fetch chase
// it — the caller hands the Location straight to Next's redirect() so the
// *browser* ends up there, not this server-side fetch.
export async function decideAuthorization(
  query: URLSearchParams,
  decision: 'allow' | 'deny',
  cookieHeader: string
): Promise<string> {
  const params = new URLSearchParams(query)
  params.set('consent', decision)

  const res = await fetch(`${getApiUrl()}/oauth2/authorize?${params}`, {
    method: 'POST',
    redirect: 'manual',
    headers: cookieHeader ? { cookie: cookieHeader } : undefined,
    cache: 'no-store'
  })

  const location = res.headers.get('location')
  if (!location) {
    throw new Error(
      decision === 'allow' ? 'Failed to approve authorization' : 'Failed to deny authorization'
    )
  }
  return location
}
