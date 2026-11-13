import { useEffect } from 'react'
import * as Sentry from '@sentry/react'
import { isRouteErrorResponse, useRouteError } from 'react-router'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { reloadPage } from '@/lib/navigate'

export function NotFound() {
  return (
    <PageContainer size="centered">
      <PageHeader title="Page not found" description="This page does not exist." />
    </PageContainer>
  )
}

export function RouteError() {
  const error = useRouteError()

  useEffect(() => {
    if (!isRouteErrorResponse(error)) Sentry.captureException(error)
  }, [error])

  return (
    <PageContainer size="centered">
      <PageHeader title="Something went wrong" description="An unexpected error occurred." />
      {/* A reload also recovers from a stale deploy's missing route chunk. */}
      <Button onClick={reloadPage}>Try again</Button>
    </PageContainer>
  )
}
