import { useEffect, useState } from 'react'
import { Navigate } from 'react-router'
import { PageContainer } from '@/components/ui/page-container'
import { LoadingState } from '@/components/ui/states'
import { useSearchParams } from '@/hooks/navigation'
import { useCurrentUser } from '@/hooks/useAuth'
import { getConsentInfo, type ConsentInfo } from '@/lib/oauth2as/consentClient'
import ConsentForm from './ConsentForm'
import { Alert } from '@/components/ui/alert'

// OAuth 2.1 consent screen. The api's authorization server redirects here
// with the pending request's params; on approval they're echoed back to
// /oauth2/authorize unchanged.

export default function ConsentPage() {
  const params = useSearchParams()
  const query = params.toString()
  const hasClient = Boolean(params.get('client_id'))

  const { data: user, isLoading: userLoading } = useCurrentUser()
  const [info, setInfo] = useState<ConsentInfo | null | undefined>(undefined)

  useEffect(() => {
    if (!hasClient || !user) return
    let cancelled = false
    void getConsentInfo(new URLSearchParams(query))
      .catch(() => null)
      .then((result) => {
        if (!cancelled) setInfo(result)
      })
    return () => {
      cancelled = true
    }
  }, [hasClient, user, query])

  if (!hasClient) return <Navigate to="/" replace />
  if (userLoading) return <LoadingState className="p-6" />
  if (!user) {
    const next = `/oauth/consent?${query}`
    return <Navigate to={`/auth/sign-in?next=${encodeURIComponent(next)}`} replace />
  }
  if (info === undefined) return <LoadingState className="p-6" />
  if (!info) {
    return (
      <PageContainer size="narrow">
        <Alert tone="danger">This authorization request is invalid or has expired.</Alert>
      </PageContainer>
    )
  }

  return (
    <PageContainer size="narrow">
      <ConsentForm
        requestQuery={query}
        consentToken={info.consentToken ?? ''}
        clientName={info.clientName}
        scope={info.scope}
      />
    </PageContainer>
  )
}
