import { useSearchParams } from '@/hooks/navigation'
import { useAsyncAction } from '@/hooks/useAsyncAction'
import {
  useTodoistConnection,
  useConnectTodoist,
  useDisconnectTodoist
} from '@/hooks/useTodoistConnection'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { Button } from '@/components/ui/button'
import { Alert } from '@/components/ui/alert'
import { ConnectionRow } from '@/components/ui/connection-row'
import { LoadingState } from '@/components/ui/states'
import { formatDateTime } from '@/lib/dates'

export default function LearningPathsSettingsClient() {
  const { data, isLoading, mutate } = useTodoistConnection()
  const connectTodoist = useConnectTodoist()
  const disconnectTodoist = useDisconnectTodoist()
  const searchParams = useSearchParams()

  const { run, pending: busy, error } = useAsyncAction()

  const todoistError = searchParams.get('todoist_error') !== null

  const handleConnect = () => run(connectTodoist, "Couldn't connect Todoist. Try again.")

  const handleDisconnect = () =>
    run(async () => {
      await disconnectTodoist()
      await mutate()
    }, "Couldn't disconnect Todoist. Try again.")

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Learning Paths Settings"
        breadcrumb={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: 'Settings' }
        ]}
      />

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Todoist</h2>
        <p className="mb-3 text-xs text-muted">
          Connect your own Todoist account for reminders: each path gets its own Todoist project
          holding the current week&apos;s items as scheduled tasks. This is one-way — completing a
          task there doesn&apos;t check the item off here.
        </p>

        {todoistError && (
          <Alert tone="danger" className="mb-3">
            Connecting Todoist failed. Please try again.
          </Alert>
        )}

        {error && (
          <Alert tone="danger" className="mb-3">
            {error}
          </Alert>
        )}

        {data?.needsReconnect && (
          <Alert tone="warn" className="mb-3">
            Reconnect Todoist: reminders now need permission to create projects and remove finished
            tasks, so they&apos;re paused until you do.
          </Alert>
        )}

        {isLoading ? (
          <LoadingState className="text-sm" />
        ) : (
          <ConnectionRow
            name="Todoist"
            connected={!!data?.connected}
            detail={
              data?.connected &&
              data.connectedAt &&
              `Connected on ${formatDateTime(data.connectedAt)}`
            }
            actions={
              data?.connected ? (
                <>
                  {data.needsReconnect && (
                    <Button variant="secondary" size="sm" disabled={busy} onClick={handleConnect}>
                      Reconnect
                    </Button>
                  )}
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={busy}
                    onClick={handleDisconnect}
                  >
                    Disconnect
                  </Button>
                </>
              ) : (
                <Button variant="secondary" size="sm" disabled={busy} onClick={handleConnect}>
                  Connect
                </Button>
              )
            }
          />
        )}
      </section>
    </PageContainer>
  )
}
