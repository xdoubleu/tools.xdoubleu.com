import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { PageContainer } from '@/components/ui/page-container'
import { getConsentInfo } from '@/lib/oauth2as/consentClient'
import ConsentForm from './ConsentForm'

// OAuth 2.1 consent screen. The api's own embedded fosite authorization
// server (issue #1039, replacing Supabase) redirects the browser here with
// the pending authorization request's own query params (client_id, scope,
// state, redirect_uri, code_challenge, ...) — see AuthorizeHandler in
// api/internal/oauth2as/handlers.go. This page runs server-side so it can
// read the HttpOnly session cookie and, on approval, echo those same params
// back to /oauth2/authorize alongside the user's decision.

interface ConsentPageProps {
  searchParams: Promise<Record<string, string | string[] | undefined>>
}

function toQueryString(params: Record<string, string | string[] | undefined>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (typeof value === 'string') query.set(key, value)
    else if (Array.isArray(value) && value[0] !== undefined) query.set(key, value[0])
  }
  return query.toString()
}

export default async function ConsentPage({ searchParams }: ConsentPageProps) {
  const rawParams = await searchParams
  const query = toQueryString(rawParams)
  const params = new URLSearchParams(query)

  if (!params.get('client_id')) redirect('/')

  const store = await cookies()
  if (!store.get('accessToken')) {
    const next = `/oauth/consent?${query}`
    redirect(`/auth/sign-in?next=${encodeURIComponent(next)}`)
  }

  const info = await getConsentInfo(params)
  if (!info) {
    return (
      <PageContainer size="narrow" className="p-6">
        <p className="text-danger">This authorization request is invalid or has expired.</p>
      </PageContainer>
    )
  }

  return (
    <PageContainer size="narrow" className="p-6">
      <ConsentForm requestQuery={query} clientName={info.clientName} scope={info.scope} />
    </PageContainer>
  )
}
