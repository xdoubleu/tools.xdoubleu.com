'use server'

import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'

// Server actions backing the OAuth consent screen. Each forwards the
// signed-in user's session cookie, records the consent decision by POSTing
// back to /oauth2/authorize with the original authorization-request query
// params, and sends the browser to the URL fosite responds with (carrying
// the authorization code on approval, or an access_denied error on denial).

async function decide(requestQuery: string, decision: 'allow' | 'deny'): Promise<void> {
  const store = await cookies()
  const cookieHeader = store
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ')

  const location = await decideAuthorization(
    new URLSearchParams(requestQuery),
    decision,
    cookieHeader
  )
  redirect(location)
}

export async function approveAuthorization(requestQuery: string): Promise<void> {
  await decide(requestQuery, 'allow')
}

export async function denyAuthorization(requestQuery: string): Promise<void> {
  await decide(requestQuery, 'deny')
}
