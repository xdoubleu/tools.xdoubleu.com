import { cookies } from 'next/headers'
import { createClient, type SupabaseClient } from '@supabase/supabase-js'

// Server-only Supabase client for the OAuth 2.1 consent flow.
//
// The app's session lives in HttpOnly, SameSite=Strict cookies that browser JS
// cannot read, so the consent page and its actions must run server-side. We
// read the access token from the request cookies and hand it to a short-lived
// Supabase client (no session persistence, no auto-refresh) purely to call the
// `supabase.auth.oauth` server endpoints on behalf of the signed-in user.
export async function createOAuthServerClient(): Promise<SupabaseClient | null> {
  const url = process.env.SUPABASE_URL
  const anonKey = process.env.SUPABASE_ANON_KEY
  if (!url || !anonKey) return null

  const store = await cookies()
  const accessToken = store.get('accessToken')?.value
  const refreshToken = store.get('refreshToken')?.value
  if (!accessToken) return null

  const supabase = createClient(url, anonKey, {
    auth: {
      persistSession: false,
      autoRefreshToken: false,
      detectSessionInUrl: false
    }
  })

  await supabase.auth.setSession({
    access_token: accessToken,
    refresh_token: refreshToken ?? ''
  })

  return supabase
}
