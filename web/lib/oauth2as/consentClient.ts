import { getApiUrl } from '@/lib/env'

// Server-only client for the OAuth consent flow against the api's embedded
// fosite server; plain fetches, since /oauth2/* isn't a Connect procedure.

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

// GET /oauth2/consent-info: the pending request's client name and scope.
// Unauthenticated; reveals only public client-registration details.
export async function getConsentInfo(query: URLSearchParams): Promise<ConsentInfo | null> {
  const clientId = query.get('client_id')
  if (!clientId) return null

  const params = new URLSearchParams({ client_id: clientId, scope: query.get('scope') ?? '' })

  const res = await fetch(`${getApiUrl()}/oauth2/consent-info?${params}`, { cache: 'no-store' })
  if (!res.ok) return null

  const data: ConsentInfoResponse = await res.json()
  return { clientId: data.client_id, clientName: data.client_name, scope: data.scope }
}

// POST /oauth2/authorize with the original params plus consent=allow|deny,
// forwarding the session cookie for re-verification. fosite replies with a
// 302, so redirect: 'manual' hands the Location to Next's redirect() for the
// browser to follow.
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
