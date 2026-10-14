'use client'

import { useState } from 'react'
import { ConnectError } from '@connectrpc/connect'
import { formatDateTime } from '@/lib/dates'
import { Button } from '@/components/ui/button'

/** Queues a run, disabled while one is pending or until waitUntil (RFC3339; '' for now). */
export default function RefreshButton({
  refresh,
  waitUntil,
  pending,
  label = 'Refresh'
}: {
  refresh: () => Promise<void>
  waitUntil: string
  pending: boolean
  label?: string
}) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function onClick() {
    setBusy(true)
    setError('')
    try {
      await refresh()
    } catch (err) {
      setError(ConnectError.from(err).rawMessage)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        size="sm"
        variant="secondary"
        onClick={onClick}
        disabled={pending || busy || waitUntil !== ''}
      >
        {pending || busy ? 'Refreshing…' : label}
      </Button>
      {waitUntil !== '' && !pending && (
        <p className="max-w-32 text-right text-xs text-muted" suppressHydrationWarning>
          Again after {formatDateTime(waitUntil)}
        </p>
      )}
      {error && (
        <p role="alert" className="text-xs text-danger">
          {error}
        </p>
      )}
    </div>
  )
}
