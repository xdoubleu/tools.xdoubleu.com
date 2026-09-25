'use server'

import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'

// Consent server actions: POST the decision back to /oauth2/authorize with
// the session cookie and original params, then follow fosite's redirect.

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
