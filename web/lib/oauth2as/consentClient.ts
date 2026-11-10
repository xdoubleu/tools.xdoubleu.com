import { getApiUrl } from '@/lib/env'

// Browser client for the OAuth consent flow against the api's embedded fosite
// server; plain fetches, since /oauth2/* isn't a Connect procedure. The
// HttpOnly session cookie rides along via credentials.

export interface ConsentInfo {
  clientId: string
  clientName: string
  scope: string
  // Binds an approval to this user and request; absent without a session.
  consentToken?: string
  // An admin may authorize the client as an agent account instead.
  canDelegate: boolean
}

interface ConsentInfoResponse {
  client_id: string
  client_name: string
  scope: string
  consent_token?: string
  can_delegate?: boolean
}

// GET /oauth2/consent-info with the full authorize request: the client name,
// scope, and the consent token approval requires.
export async function getConsentInfo(query: URLSearchParams): Promise<ConsentInfo | null> {
  if (!query.get('client_id')) return null

  const res = await fetch(`${getApiUrl()}/oauth2/consent-info?${query}`, {
    credentials: 'include',
    cache: 'no-store'
  })
  if (!res.ok) return null

  const data: ConsentInfoResponse = await res.json()
  return {
    clientId: data.client_id,
    clientName: data.client_name,
    scope: data.scope,
    consentToken: data.consent_token,
    canDelegate: data.can_delegate ?? false
  }
}

// POST /oauth2/authorize with the original params plus consent=allow|deny and,
// to approve, the consent token and the agent account it delegates to, if any. fosite answers with a redirect, which fetch
// can't expose, so Accept: application/json makes the api return it as
// {"location": ...} for the caller to navigate to.
export async function decideAuthorization(
  query: URLSearchParams,
  decision: 'allow' | 'deny',
  consentToken = '',
  agentAccountId = ''
): Promise<string> {
  const params = new URLSearchParams(query)
  params.set('consent', decision)
  if (agentAccountId) params.set('agent_account_id', agentAccountId)

  const headers: Record<string, string> = { Accept: 'application/json' }
  if (consentToken) headers['X-OAuth-Consent-Token'] = consentToken

  const res = await fetch(`${getApiUrl()}/oauth2/authorize?${params}`, {
    method: 'POST',
    credentials: 'include',
    headers,
    cache: 'no-store'
  })

  const body: { location?: string } = await res.json().catch(() => ({}))
  if (!body.location) {
    throw new Error(
      decision === 'allow' ? 'Failed to approve authorization' : 'Failed to deny authorization'
    )
  }
  return body.location
}
