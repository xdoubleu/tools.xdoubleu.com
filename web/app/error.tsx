'use client'

import * as Sentry from '@sentry/nextjs'
import { useEffect } from 'react'

import { Button } from '@/components/ui/button'

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
    <div className="mx-auto w-full max-w-sm py-12 text-center">
      <h1 className="text-lg font-semibold text-fg">Something went wrong</h1>
      <p className="mt-2 text-sm text-muted">
        {error.digest ? `Error reference: ${error.digest}` : 'An unexpected error occurred.'}
      </p>
      <Button onClick={reset} className="mt-6">
        Try again
      </Button>
    </div>
  )
}
