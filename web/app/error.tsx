'use client'

import * as Sentry from '@sentry/nextjs'
import { useEffect } from 'react'

import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function ErrorBoundary({
  error,
  reset
}: {
  error: Error & { digest?: string }
  reset: () => void
}) {
  useEffect(() => {
    Sentry.captureException(error)
  }, [error])

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Something went wrong"
        description={
          error.digest ? `Error reference: ${error.digest}` : 'An unexpected error occurred.'
        }
      />
      <Button onClick={reset}>Try again</Button>
    </PageContainer>
  )
}
