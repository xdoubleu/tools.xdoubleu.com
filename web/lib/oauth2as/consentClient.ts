import { getApiUrl } from '@/lib/env'

// Server-only client for the OAuth consent flow against the api's embedded
// fosite server; plain fetches, since /oauth2/* isn't a Connect procedure.

export interface ConsentInfo {
  clientId: string
  clientName: string
  scope: string
  // Binds an approval to this user and request; absent without a session.
  consentToken?: string
}

interface ConsentInfoResponse {
  client_id: string
  client_name: string
  scope: string
  consent_token?: string
}

// GET /oauth2/consent-info with the full authorize request and the session
// cookie: the client name, scope, and the consent token approval requires.
export async function getConsentInfo(
  query: URLSearchParams,
  accessToken?: string
): Promise<ConsentInfo | null> {
  if (!query.get('client_id')) return null

  const res = await fetch(`${getApiUrl()}/oauth2/consent-info?${query}`, {
    cache: 'no-store',
    headers: accessToken ? { cookie: `accessToken=${accessToken}` } : undefined
  })
  if (!res.ok) return null

  const data: ConsentInfoResponse = await res.json()
  return {
    clientId: data.client_id,
    clientName: data.client_name,
    scope: data.scope,
    consentToken: data.consent_token
  }
}

// POST /oauth2/authorize with the original params plus consent=allow|deny,
// forwarding the session cookie and, to approve, the consent token. fosite
// replies with a redirect, so redirect: 'manual' hands the Location to Next's
// redirect() for the browser to follow.
export async function decideAuthorization(
  query: URLSearchParams,
  decision: 'allow' | 'deny',
  cookieHeader: string,
  consentToken = ''
): Promise<string> {
  const params = new URLSearchParams(query)
  params.set('consent', decision)

  const headers: Record<string, string> = {}
  if (cookieHeader) headers.cookie = cookieHeader
  if (consentToken) headers['X-OAuth-Consent-Token'] = consentToken

  const res = await fetch(`${getApiUrl()}/oauth2/authorize?${params}`, {
    method: 'POST',
    redirect: 'manual',
    headers: Object.keys(headers).length > 0 ? headers : undefined,
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
