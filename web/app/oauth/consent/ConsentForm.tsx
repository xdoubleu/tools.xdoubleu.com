import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { leaveTo } from '@/lib/navigate'
import { decideAuthorization } from '@/lib/oauth2as/consentClient'
import { Alert } from '@/components/ui/alert'
import { useAgentActions } from '@/hooks/useUserManagement'
import { failureMessage } from '@/components/user-management/AgentDialog'
import { AGENT_APPS } from '@/components/user-management/AgentGrants'
import AgentChoice, { NEW_AGENT_GRANTS, type AgentChoiceState } from './AgentChoice'

interface ConsentFormProps {
  requestQuery: string
  consentToken: string
  clientName: string
  scope: string
  canDelegate?: boolean
}

/** A failure to set up the agent, with a message to show as is. */
class AgentSetupError extends Error {}

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
  scope,
  canDelegate = false
}: ConsentFormProps) {
  const { create, setAccess } = useAgentActions()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [choice, setChoice] = useState<AgentChoiceState>({
    mode: 'self',
    name: clientName,
    grants: NEW_AGENT_GRANTS,
    existingId: ''
  })
  // A new agent made by a failed attempt is reused on retry, not made twice.
  const [createdId, setCreatedId] = useState('')

  const scopes = scope.split(' ').filter(Boolean)
  const incomplete =
    (choice.mode === 'new' && !createdId && !choice.name.trim()) ||
    (choice.mode === 'existing' && !choice.existingId)

  // The agent account an approval delegates to; '' signs in as the user.
  async function delegateTo(): Promise<string> {
    if (choice.mode === 'existing') return choice.existingId
    if (choice.mode === 'self') return ''
    if (createdId) return createdId
    let created
    try {
      created = await create(choice.name.trim())
    } catch (err) {
      throw new AgentSetupError(failureMessage(err, "Couldn't create the agent. Try again."))
    }
    const account = created.account
    if (!account) throw new AgentSetupError("Couldn't create the agent. Try again.")
    setCreatedId(account.id)
    for (const app of AGENT_APPS) {
      const level = choice.grants[app] ?? 'none'
      if (level !== (NEW_AGENT_GRANTS[app] ?? 'none')) {
        await setAccess(account.userId, app, level)
      }
    }
    return account.id
  }

  async function decide(decision: 'allow' | 'deny') {
    setError(null)
    setPending(true)
    try {
      const agentAccountId = decision === 'allow' ? await delegateTo() : ''
      const location = await decideAuthorization(
        new URLSearchParams(requestQuery),
        decision,
        decision === 'allow' ? consentToken : undefined,
        agentAccountId || undefined
      )
      // Leaves the app for the client's redirect_uri; stay disabled meanwhile.
      leaveTo(location)
    } catch (err) {
      setError(
        err instanceof AgentSetupError ? err.message : 'Something went wrong. Please try again.'
      )
      setPending(false)
    }
  }

  return (
    <Card padding="none" className="mx-auto max-w-md">
      <CardHeader>
        <CardTitle className="text-xl">Authorize {clientName}</CardTitle>
        <p className="text-sm text-muted">
          {choice.mode === 'self'
            ? `${clientName} wants read-only access to your tools.xdoubleu.com monitoring data.`
            : `${clientName} will act as the agent, reaching only the apps it is granted.`}
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

        {canDelegate && (
          <AgentChoice
            state={choice}
            onChange={(next) => setChoice((prev) => ({ ...prev, ...next }))}
          />
        )}

        {error && <Alert tone="danger">{error}</Alert>}

        <div className="flex gap-3">
          <Button
            className="flex-1"
            disabled={pending || incomplete}
            onClick={() => void decide('allow')}
          >
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
