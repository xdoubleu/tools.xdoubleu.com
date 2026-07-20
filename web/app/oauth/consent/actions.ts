'use server'

import { redirect } from 'next/navigation'
import { createOAuthServerClient } from '@/lib/supabase/oauthServer'

// Server actions backing the OAuth consent screen. Each resolves the requesting
// user's Supabase session from cookies, records the consent decision, and sends
// the browser back to the OAuth client's redirect URL (carrying the
// authorization code on approval, or an access_denied error on denial).

export async function approveAuthorization(authorizationId: string): Promise<void> {
  const supabase = await createOAuthServerClient()
  if (!supabase) throw new Error('OAuth server is not configured')

  const { data, error } = await supabase.auth.oauth.approveAuthorization(authorizationId, {
    skipBrowserRedirect: true
  })
  if (error || !data) {
    throw new Error(error?.message ?? 'Failed to approve authorization')
  }

  redirect(data.redirect_url)
}

export async function denyAuthorization(authorizationId: string): Promise<void> {
  const supabase = await createOAuthServerClient()
  if (!supabase) throw new Error('OAuth server is not configured')

  const { data, error } = await supabase.auth.oauth.denyAuthorization(authorizationId, {
    skipBrowserRedirect: true
  })
  if (error || !data) {
    throw new Error(error?.message ?? 'Failed to deny authorization')
  }

  redirect(data.redirect_url)
}
