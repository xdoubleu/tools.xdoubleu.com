import { useState } from 'react'
import type { AgentAccount } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { useAgentActions, useAgents, type AgentLevel } from '@/hooks/useUserManagement'
import AgentDialog from '@/components/user-management/AgentDialog'
import AgentStepUpDialog from '@/components/user-management/AgentStepUpDialog'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { Select } from '@/components/ui/select'
import { ConfirmDialog } from '@/components/ui/dialog'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

const AGENT_APPS = [
  'factory',
  'books',
  'feeds',
  'games',
  'learningpaths',
  'mealplans',
  'movies',
  'podcasts',
  'recipes',
  'shoppinglist',
  'taste',
  'trains'
]

const LEVELS: AgentLevel[] = ['none', 'read', 'write']

function levelOf(account: AgentAccount, app: string): AgentLevel {
  if (account.writeAccess.includes(app)) return 'write'
  return account.appAccess.includes(app) ? 'read' : 'none'
}

/** What the agent may do in each app: none, read or write. */
function Grants({ account }: { account: AgentAccount }) {
  const { setAccess } = useAgentActions()
  const [failed, setFailed] = useState(false)

  const change = async (app: string, level: AgentLevel) => {
    setFailed(false)
    try {
      await setAccess(account.userId, app, level)
    } catch {
      setFailed(true)
    }
  }

  return (
    <div className="w-full space-y-2">
      <div
        role="group"
        aria-label={`App access for ${account.name}`}
        className="grid grid-cols-2 gap-2 sm:grid-cols-3"
      >
        {AGENT_APPS.map((app) => (
          <Field key={app} label={app} htmlFor={`${account.id}-${app}`} className="capitalize">
            <Select
              id={`${account.id}-${app}`}
              aria-label={`${app} access for ${account.name}`}
              value={levelOf(account, app)}
              onChange={(e) => {
                const level = LEVELS.find((l) => l === e.target.value)
                if (level) void change(app, level)
              }}
            >
              {LEVELS.map((level) => (
                <option key={level} value={level}>
                  {level}
                </option>
              ))}
            </Select>
          </Field>
        ))}
      </div>
      {failed && <Alert tone="danger">Couldn&apos;t change the access. Try again.</Alert>}
    </div>
  )
}

type StepUp = { account: AgentAccount; action: 'rotate' | 'join' }

/** The agents: grant them apps, rotate their secret, put them in your family, or revoke them. */
export default function AgentsClient() {
  const { data, error, isLoading } = useAgents()
  const { revoke: revokeAgent, rotate, joinFamily, leaveFamily } = useAgentActions()
  const [creating, setCreating] = useState(false)
  const [stepUp, setStepUp] = useState<StepUp | null>(null)
  const [revoking, setRevoking] = useState<AgentAccount | null>(null)
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const revoke = async (account: AgentAccount) => {
    setPending(true)
    setFailed(false)
    try {
      await revokeAgent(account.id)
      setRevoking(null)
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  const leave = async (account: AgentAccount) => {
    setFailed(false)
    try {
      await leaveFamily(account.id)
    } catch {
      setFailed(true)
    }
  }

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Agents"
        breadcrumb={[{ label: 'User management', href: '/user-management' }, { label: 'Agents' }]}
        actions={<Button onClick={() => setCreating(true)}>New agent</Button>}
      />
      {failed && (
        <Alert tone="danger" className="mb-4">
          Couldn&apos;t complete that. Try again.
        </Alert>
      )}
      {error && !data && <ErrorState what="agents" />}
      {isLoading && !data && <LoadingState label="agents" />}
      {data && data.accounts.length === 0 && <EmptyState>No agents yet.</EmptyState>}
      {data && data.accounts.length > 0 && (
        <ul className="space-y-2" aria-label="Agents">
          {data.accounts.map((account) => (
            <li key={account.id}>
              <Card variant="inset" className="flex flex-wrap items-center gap-3">
                <div className="min-w-0 flex-1">
                  <p className="break-words font-medium">{account.name}</p>
                  <p className="break-all text-xs text-muted">{account.clientId}</p>
                </div>
                {account.revokedAt ? (
                  <Badge variant="secondary">Revoked</Badge>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => setStepUp({ account, action: 'rotate' })}
                    >
                      Rotate secret
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => setStepUp({ account, action: 'join' })}
                    >
                      Join my family
                    </Button>
                    <Button size="sm" variant="secondary" onClick={() => void leave(account)}>
                      Leave family
                    </Button>
                    <Button size="sm" variant="destructive" onClick={() => setRevoking(account)}>
                      Revoke
                    </Button>
                  </div>
                )}
                {!account.revokedAt && <Grants account={account} />}
              </Card>
            </li>
          ))}
        </ul>
      )}
      {creating && <AgentDialog onClose={() => setCreating(false)} />}
      {stepUp?.action === 'rotate' && (
        <AgentStepUpDialog
          title={`Rotate ${stepUp.account.name}'s secret`}
          description="You get a new secret. The old one keeps working for 24 hours, so you can swap it without downtime."
          submitLabel="Rotate"
          run={(code) => rotate(stepUp.account.id, code)}
          onClose={() => setStepUp(null)}
        />
      )}
      {stepUp?.action === 'join' && (
        <AgentStepUpDialog
          title={`Add ${stepUp.account.name} to your family`}
          description="It shares what your family shares, for the apps it holds. It leaves its own family."
          submitLabel="Add"
          run={(code) => joinFamily(stepUp.account.id, code)}
          onClose={() => setStepUp(null)}
        />
      )}
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={`Revoke ${revoking.name}?`}
          description="Its access ends at once and it can't be restored. Create a new agent to replace it."
          confirmLabel="Revoke"
          pendingLabel="Revoking…"
          destructive
          pending={pending}
          onConfirm={() => void revoke(revoking)}
        />
      )}
    </PageContainer>
  )
}
