import { useState } from 'react'
import type { AgentAccount } from '@/lib/gen/factory/v1/agent_accounts_pb'
import { useFactoryActions, useFactoryAgents } from '@/hooks/useFactory'
import AgentDialog from '@/components/factory/AgentDialog'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/dialog'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

/** The agent accounts: create one, or revoke it to end its access at once. */
export default function AgentsClient() {
  const { data, error, isLoading } = useFactoryAgents()
  const { revokeAgent } = useFactoryActions()
  const [creating, setCreating] = useState(false)
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

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Agents"
        breadcrumb={[{ label: 'Factory', href: '/factory' }, { label: 'Agents' }]}
        actions={<Button onClick={() => setCreating(true)}>New agent</Button>}
      />
      {failed && (
        <Alert tone="danger" className="mb-4">
          Couldn&apos;t revoke the agent. Try again.
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
                  <Button variant="destructive" onClick={() => setRevoking(account)}>
                    Revoke
                  </Button>
                )}
              </Card>
            </li>
          ))}
        </ul>
      )}
      {creating && <AgentDialog onClose={() => setCreating(false)} />}
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
