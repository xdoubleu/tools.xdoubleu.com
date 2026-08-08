'use client'

import { useEffect } from 'react'
import * as Sentry from '@sentry/nextjs'
import { Button } from '@/components/ui/button'

interface GlobalErrorProps {
  error: Error & { digest?: string }
  reset: () => void
}

export default function GlobalError({ error }: GlobalErrorProps) {
  useEffect(() => {
    Sentry.captureException(error)
  }, [error])

  return (
    <html lang="en">
      <body className="bg-bg text-fg">
        <div className="flex min-h-screen flex-col items-center justify-center gap-4">
          <h1 className="text-2xl font-bold">Something went wrong</h1>
          <p className="text-sm text-muted">{error?.message || 'An unexpected error occurred'}</p>
          {/* Not reset(): a root-layout error means the layout's own data
              fetch threw, and reset() only clears client error-boundary
              state without re-running that fetch (see issue #852) — a full
              reload is what actually retries it. */}
          <Button onClick={() => window.location.reload()}>Try again</Button>
        </div>
      </body>
    </html>
  )
}
