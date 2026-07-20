import { cookies } from 'next/headers'
import { redirect } from 'next/navigation'
import { PageContainer } from '@/components/ui/page-container'
import { createOAuthServerClient } from '@/lib/supabase/oauthServer'
import ConsentForm from './ConsentForm'

// OAuth 2.1 consent screen. Supabase (the authorization server) redirects the
// browser here with an `authorization_id` after a client — e.g. a local Claude
// CLI — begins the authorization-code flow against the observability MCP server.
// This page runs server-side so it can read the HttpOnly session cookie and
// drive the Supabase consent endpoints on the user's behalf.

interface ConsentPageProps {
  searchParams: Promise<{ authorization_id?: string }>
}

export default async function ConsentPage({ searchParams }: ConsentPageProps) {
  const { authorization_id: authorizationId } = await searchParams
  if (!authorizationId) redirect('/')

  const store = await cookies()
  if (!store.get('accessToken')) {
    const next = `/oauth/consent?authorization_id=${encodeURIComponent(authorizationId)}`
    redirect(`/auth/sign-in?next=${encodeURIComponent(next)}`)
  }

  const supabase = await createOAuthServerClient()
  if (!supabase) {
    return (
      <PageContainer size="narrow" className="p-6">
        <p className="text-danger">OAuth server is not configured.</p>
      </PageContainer>
    )
  }

  const { data, error } = await supabase.auth.oauth.getAuthorizationDetails(authorizationId)
  if (error || !data) {
    return (
      <PageContainer size="narrow" className="p-6">
        <p className="text-danger">This authorization request is invalid or has expired.</p>
      </PageContainer>
    )
  }

  // Already consented for these scopes — Supabase returns the redirect directly.
  if (!('authorization_id' in data)) {
    redirect(data.redirect_url)
  }

  return (
    <PageContainer size="narrow" className="p-6">
      <ConsentForm
        authorizationId={authorizationId}
        clientName={data.client.name}
        scope={data.scope}
      />
    </PageContainer>
  )
}
