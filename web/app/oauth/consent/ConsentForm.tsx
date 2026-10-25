import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { leaveTo } from '@/lib/navigate'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'
import { Alert } from '@/components/ui/alert'

interface ConsentFormProps {
  requestQuery: string
  consentToken: string
  clientName: string
  scope: string
}

// Unknown scopes show their raw value.
const scopeLabels: Record<string, string> = {
  openid: 'Verify your identity',
  email: 'Read your email address',
  profile: 'Read your basic profile'
}

export default function ConsentForm({
  requestQuery,
  consentToken,
  clientName,
  scope
}: ConsentFormProps) {
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const scopes = scope.split(' ').filter(Boolean)

  async function decide(decision: 'allow' | 'deny') {
    setError(null)
    setPending(true)
    try {
      const location = await decideAuthorization(
        new URLSearchParams(requestQuery),
        decision,
        decision === 'allow' ? consentToken : undefined
      )
      // Leaves the app for the client's redirect_uri; stay disabled meanwhile.
      leaveTo(location)
    } catch {
      setError('Something went wrong. Please try again.')
      setPending(false)
    }
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

        {error && <Alert tone="danger">{error}</Alert>}

        <div className="flex gap-3">
          <Button className="flex-1" disabled={pending} onClick={() => void decide('allow')}>
            Approve
          </Button>
          <Button
            variant="secondary"
            className="flex-1"
            disabled={pending}
            onClick={() => void decide('deny')}
          >
            Deny
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
