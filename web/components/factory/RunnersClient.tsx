import { useState } from 'react'
import type { Runner } from '@/lib/gen/factory/v1/runners_pb'
import { runnerState } from '@/lib/factory/board'
import { useFactoryActions, useFactoryRunners } from '@/hooks/useFactory'
import RunnerDialog from '@/components/factory/RunnerDialog'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { ConfirmDialog } from '@/components/ui/dialog'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

/** The runners: pair the Mac, see whether it is online, and revoke it. */
export default function RunnersClient() {
  const { data, error, isLoading } = useFactoryRunners()
  const { revokeRunner } = useFactoryActions()
  const [creating, setCreating] = useState(false)
  const [revoking, setRevoking] = useState<Runner | null>(null)
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const revoke = async (runner: Runner) => {
    setPending(true)
    setFailed(false)
    try {
      await revokeRunner(runner.id)
      setRevoking(null)
    } catch {
      setFailed(true)
    }
    setPending(false)
  }

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Runner"
        breadcrumb={[{ label: 'Factory', href: '/factory' }, { label: 'Runner' }]}
        actions={<Button onClick={() => setCreating(true)}>Pair a runner</Button>}
      />
      {failed && (
        <Alert tone="danger" className="mb-4">
          Couldn&apos;t revoke the runner. Try again.
        </Alert>
      )}
      {error && !data && <ErrorState what="runners" />}
      {isLoading && !data && <LoadingState label="runners" />}
      {data && data.runners.length === 0 && <EmptyState>No runner paired yet.</EmptyState>}
      {data && data.runners.length > 0 && (
        <ul className="space-y-2" aria-label="Runners">
          {data.runners.map((runner) => {
            const state = runnerState(runner, Date.now())
            return (
              <li key={runner.id}>
                <Card variant="inset" className="flex flex-wrap items-center gap-3">
                  <div className="min-w-0 flex-1">
                    <p className="break-words font-medium">{runner.name}</p>
                    <p className="text-xs text-muted">
                      {runner.version ? `Version ${runner.version}` : 'Not paired yet'}
                    </p>
                  </div>
                  <Badge variant={state.online ? 'success' : 'secondary'}>{state.label}</Badge>
                  {runner.revokedAt === '' && (
                    <Button variant="destructive" onClick={() => setRevoking(runner)}>
                      Revoke
                    </Button>
                  )}
                </Card>
              </li>
            )
          })}
        </ul>
      )}
      {creating && <RunnerDialog onClose={() => setCreating(false)} />}
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={`Revoke ${revoking.name}?`}
          description="Its token stops working at once, so the runner goes offline. Pair it again to bring it back."
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
