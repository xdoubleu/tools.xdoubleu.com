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
          {/* Not reset(): it doesn't re-run the root layout's failed fetch; reload does. */}
          <Button onClick={() => window.location.reload()}>Try again</Button>
        </div>
      </body>
    </html>
  )
}
