'use client'

import { useState } from 'react'
import { useSearchParams } from 'next/navigation'
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

  const [busy, setBusy] = useState(false)

  const todoistError = searchParams.get('todoist_error') !== null

  const handleConnect = async () => {
    setBusy(true)
    try {
      await connectTodoist()
    } finally {
      setBusy(false)
    }
  }

  const handleDisconnect = async () => {
    setBusy(true)
    try {
      await disconnectTodoist()
      await mutate()
    } finally {
      setBusy(false)
    }
  }

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
          Connect your own Todoist account to send a path item as a task. This is one-way — Claude
          never reads your Todoist tasks back, and completing one there doesn&apos;t check the item
          off here.
        </p>

        {todoistError && (
          <Alert tone="danger" className="mb-3">
            Connecting Todoist failed. Please try again.
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
                <Button variant="destructive" size="sm" disabled={busy} onClick={handleDisconnect}>
                  Disconnect
                </Button>
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
