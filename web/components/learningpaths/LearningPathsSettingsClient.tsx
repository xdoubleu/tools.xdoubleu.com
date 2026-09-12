'use client'

import { useState } from 'react'
import { useSearchParams } from 'next/navigation'
import {
  useTodoistConnection,
  useConnectTodoist,
  useDisconnectTodoist
} from '@/hooks/useTodoistConnection'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
    <PageContainer size="narrow" className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Learning Paths', href: '/learningpaths/list' }, { label: 'Settings' }]}
      />
      <h1 className="mb-6 text-3xl font-bold">Learning Paths Settings</h1>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-muted">Todoist</h2>
        <p className="mb-3 text-xs text-muted">
          Connect your own Todoist account to send a path item as a task. This is one-way — Claude
          never reads your Todoist tasks back, and completing one there doesn&apos;t check the item
          off here.
        </p>

        {todoistError && (
          <p className="mb-3 text-xs text-danger">Connecting Todoist failed. Please try again.</p>
        )}

        {isLoading ? (
          <p className="text-sm text-muted">Loading…</p>
        ) : (
          <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-surface p-3 text-sm">
            <div>
              <div className="flex items-center gap-2">
                <span className="font-medium text-fg">Todoist</span>
                <Badge variant={data?.connected ? 'success' : 'secondary'}>
                  {data?.connected ? 'Connected' : 'Not connected'}
                </Badge>
              </div>
              {data?.connected && data.connectedAt && (
                <p className="mt-1 text-xs text-muted">
                  Connected on {formatDateTime(data.connectedAt)}
                </p>
              )}
            </div>
            {data?.connected ? (
              <Button variant="destructive" size="sm" disabled={busy} onClick={handleDisconnect}>
                Disconnect
              </Button>
            ) : (
              <Button variant="secondary" size="sm" disabled={busy} onClick={handleConnect}>
                Connect
              </Button>
            )}
          </div>
        )}
      </section>
    </PageContainer>
  )
}
