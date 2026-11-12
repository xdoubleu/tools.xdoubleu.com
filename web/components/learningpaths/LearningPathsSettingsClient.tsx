import { useState } from 'react'
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
import { ConfirmDialog } from '@/components/ui/dialog'
import { Alert } from '@/components/ui/alert'
import { ConnectionRow } from '@/components/ui/connection-row'
import { SettingsSection, SettingsSections } from '@/components/ui/settings-section'
import { LoadingState } from '@/components/ui/states'
import { formatDateTime } from '@/lib/dates'

export default function LearningPathsSettingsClient() {
  const { data, isLoading, mutate } = useTodoistConnection()
  const connectTodoist = useConnectTodoist()
  const disconnectTodoist = useDisconnectTodoist()
  const searchParams = useSearchParams()

  const { run, pending: busy, error } = useAsyncAction()
  const [confirmingDisconnect, setConfirmingDisconnect] = useState(false)

  const todoistError = searchParams.get('todoist_error') !== null

  const handleConnect = () => run(connectTodoist, "Couldn't connect Todoist. Try again.")

  const handleDisconnect = () =>
    run(async () => {
      await disconnectTodoist()
      await mutate()
    }, "Couldn't disconnect Todoist. Try again.").finally(() => setConfirmingDisconnect(false))

  return (
    <PageContainer size="narrow">
      <PageHeader
        title="Learning paths settings"
        breadcrumb={[
          { label: 'Learning paths', href: '/learningpaths/list' },
          { label: 'Settings' }
        ]}
      />

      <SettingsSections>
        <SettingsSection
          title="Todoist"
          description="Connect your own Todoist account for reminders: each path gets its own Todoist project holding the current week’s items as scheduled tasks. This is one-way — completing a task there doesn’t check the item off here."
        >
          {todoistError && (
            <Alert tone="danger">Connecting Todoist failed. Please try again.</Alert>
          )}

          {error && <Alert tone="danger">{error}</Alert>}

          {data?.needsReconnect && (
            <Alert tone="warn">
              Reconnect Todoist: reminders now need permission to create projects and remove
              finished tasks, so they&apos;re paused until you do.
            </Alert>
          )}

          {isLoading ? (
            <LoadingState placement="inline" />
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
                      <Button variant="secondary" disabled={busy} onClick={handleConnect}>
                        Reconnect
                      </Button>
                    )}
                    <Button
                      variant="destructive"

                      disabled={busy}
                      onClick={() => setConfirmingDisconnect(true)}
                    >
                      Disconnect
                    </Button>
                  </>
                ) : (
                  <Button variant="secondary" disabled={busy} onClick={handleConnect}>
                    Connect
                  </Button>
                )
              }
            />
          )}
          <ConfirmDialog
            open={confirmingDisconnect}
            onOpenChange={setConfirmingDisconnect}
            title="Disconnect Todoist?"
            description="Reminders stop, and the projects already created in Todoist stay there."
            confirmLabel="Disconnect"
            pendingLabel="Disconnecting…"
            destructive
            pending={busy}
            onConfirm={() => void handleDisconnect()}
          />
        </SettingsSection>
      </SettingsSections>
    </PageContainer>
  )
}
