'use client'

import { useState } from 'react'
import { useKoboDeviceLogs, useClearKoboDeviceLogs } from '@/hooks/useBooks'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { LoadingState } from '@/components/ui/states'

interface KoboDeviceLogsProps {
  deviceId: string
}

function formatLogTime(time: string): string {
  const d = new Date(time)
  if (Number.isNaN(d.getTime())) return time
  return d.toLocaleString()
}

export default function KoboDeviceLogs({ deviceId }: KoboDeviceLogsProps) {
  const { data, isLoading, mutate } = useKoboDeviceLogs(deviceId, true)
  const clearLogs = useClearKoboDeviceLogs()
  const [clearing, setClearing] = useState(false)

  const entries = data?.entries ?? []

  async function handleClear() {
    setClearing(true)
    try {
      await clearLogs(deviceId)
      await mutate()
    } finally {
      setClearing(false)
    }
  }

  return (
    <Card variant="inset" className="mt-3" data-testid="kobo-logs">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <p className="min-w-0 flex-1 basis-48 text-xs text-muted">
          Captured in memory only — resets when the server restarts.
        </p>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => mutate()}
            data-testid="kobo-logs-refresh"
          >
            Refresh
          </Button>
          <Button
            type="button"
            variant="destructive"
            size="sm"
            disabled={clearing || entries.length === 0}
            onClick={handleClear}
            data-testid="kobo-logs-clear"
          >
            {clearing ? 'Clearing…' : 'Clear'}
          </Button>
        </div>
      </div>

      {isLoading ? (
        <LoadingState label="logs" className="text-xs" />
      ) : entries.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted" data-testid="kobo-logs-empty">
          No requests captured yet. Sync your Kobo, then refresh.
        </p>
      ) : (
        <ul className="space-y-2" data-testid="kobo-logs-list">
          {entries
            .slice()
            .reverse()
            .map((entry, i) => (
              <li key={`${entry.time}-${i}`}>
                <Card className="rounded-lg p-2 text-xs">
                  <div className="flex flex-wrap items-center gap-2 font-mono">
                    <span className="font-semibold">{entry.method}</span>
                    <span className="min-w-0 break-all">{entry.path}</span>
                    <span className="text-muted">→ {entry.status}</span>
                    <span className="ml-auto text-muted">{formatLogTime(entry.time)}</span>
                  </div>
                  {entry.query && (
                    <p className="mt-1 break-all font-mono text-muted">?{entry.query}</p>
                  )}
                  {entry.requestBody && (
                    <div className="mt-1 overflow-x-auto">
                      <p className="text-muted">Request:</p>
                      <pre className="whitespace-pre-wrap break-all font-mono">
                        {entry.requestBody}
                      </pre>
                    </div>
                  )}
                  {entry.responseBody && (
                    <div className="mt-1 overflow-x-auto">
                      <p className="text-muted">Response:</p>
                      <pre className="whitespace-pre-wrap break-all font-mono">
                        {entry.responseBody}
                      </pre>
                    </div>
                  )}
                </Card>
              </li>
            ))}
        </ul>
      )}
    </Card>
  )
}
