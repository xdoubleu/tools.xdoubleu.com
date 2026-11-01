import { useState } from 'react'
import type { Runner } from '@/lib/gen/factory/v1/runners_pb'
import {
  activeRunner,
  RUNNER_DOWNLOAD_PATH,
  runnerState,
  runnerUpdate,
  runnersOf,
  shortRelease
} from '@/lib/factory/board'
import { useFactoryActions, useFactoryRunners, useRunnerRelease } from '@/hooks/useFactory'
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
  const { data: latest } = useRunnerRelease()
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
      <Card variant="inset" className="mb-4 space-y-2 px-4 py-3" data-testid="runner-install">
        <p className="text-sm font-medium">Install the runner</p>
        <p className="text-xs text-muted">
          Apple Silicon Mac. Download it, then run{' '}
          <code className="break-all">
            xattr -d com.apple.quarantine factory-runner-darwin-arm64
          </code>
          , make it executable and run <code>./factory-runner-darwin-arm64 install</code>. It
          updates itself once idle.
        </p>
        <Button asChild>
          <a href={RUNNER_DOWNLOAD_PATH} download>
            Download factory-runner
          </a>
        </Button>
        <RunnerVersion runners={runnersOf(data)} latest={latest} />
        <p className="text-xs text-muted">
          While a job runs the Mac is kept awake, but a closed lid only stays awake on AC power with
          an external display attached.
        </p>
      </Card>
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
                      {runner.version
                        ? `Version ${shortRelease(runner.version)}`
                        : 'Not paired yet'}
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

function RunnerVersion({ runners, latest }: { runners: Runner[]; latest: string | undefined }) {
  const runner = activeRunner(runners)
  if (!runner?.version && !latest) return null
  const update = runnerUpdate(runner, latest)
  return (
    <p className="text-xs text-muted" data-testid="runner-version">
      Running {runner?.version ? shortRelease(runner.version) : 'none'} · latest{' '}
      {latest ? shortRelease(latest) : 'unknown'}
      {update === 'pending' && ' · update available'}
    </p>
  )
}
