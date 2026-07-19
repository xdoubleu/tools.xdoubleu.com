'use client'

import { useState, useTransition } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { approveAuthorization, denyAuthorization } from './actions'

interface ConsentFormProps {
  authorizationId: string
  clientName: string
  scope: string
}

// Human-readable labels for the standard OAuth scopes Supabase issues. Unknown
// scopes fall back to their raw value so nothing is silently hidden.
const scopeLabels: Record<string, string> = {
  openid: 'Verify your identity',
  email: 'Read your email address',
  profile: 'Read your basic profile'
}

export default function ConsentForm({ authorizationId, clientName, scope }: ConsentFormProps) {
  const [pending, startTransition] = useTransition()
  const [error, setError] = useState<string | null>(null)

  const scopes = scope.split(' ').filter(Boolean)

  function decide(action: (id: string) => Promise<void>) {
    setError(null)
    startTransition(async () => {
      try {
        await action(authorizationId)
      } catch {
        setError('Something went wrong. Please try again.')
      }
    })
  }

  return (
    <Card className="mx-auto max-w-md">
      <CardHeader>
        <CardTitle className="text-xl">Authorize {clientName}</CardTitle>
        <p className="text-sm text-muted">
          {clientName} wants read-only access to your tools.xdoubleu.com monitoring data.
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <p className="mb-2 text-sm font-medium">This will allow it to:</p>
          <ul className="list-disc space-y-1 pl-5 text-sm text-muted">
            {scopes.map((s) => (
              <li key={s}>{scopeLabels[s] ?? s}</li>
            ))}
          </ul>
        </div>

        {error && <p className="text-sm text-danger">{error}</p>}

        <div className="flex gap-3">
          <Button
            className="flex-1"
            disabled={pending}
            onClick={() => decide(approveAuthorization)}
          >
            Approve
          </Button>
          <Button
            variant="secondary"
            className="flex-1"
            disabled={pending}
            onClick={() => decide(denyAuthorization)}
          >
            Deny
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
