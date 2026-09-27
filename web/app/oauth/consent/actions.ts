'use server'

import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'

// Consent server actions: POST the decision back to /oauth2/authorize with
// the session cookie and original params, then follow fosite's redirect.

async function decide(
  requestQuery: string,
  decision: 'allow' | 'deny',
  consentToken?: string
): Promise<void> {
  const store = await cookies()
  const cookieHeader = store
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join('; ')

  const location = await decideAuthorization(
    new URLSearchParams(requestQuery),
    decision,
    cookieHeader,
    consentToken
  )
  redirect(location)
}

// consentToken comes from the rendered consent page; a forged form can't
// supply it.
export async function approveAuthorization(
  requestQuery: string,
  consentToken: string
): Promise<void> {
  await decide(requestQuery, 'allow', consentToken)
}

export async function denyAuthorization(requestQuery: string): Promise<void> {
  await decide(requestQuery, 'deny')
}
