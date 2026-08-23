'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { useLogs } from '@/hooks/useMonitoring'
import { formatDateTime } from '@/lib/dates'

const SOURCE_OPTIONS = ['', 'api', 'web']
const LEVEL_OPTIONS = ['', 'debug', 'info', 'warn', 'error']

function levelVariant(level: string): 'default' | 'secondary' | 'warn' | 'danger' {
  switch (level.toLowerCase()) {
    case 'error':
      return 'danger'
    case 'warn':
      return 'warn'
    case 'debug':
      return 'secondary'
    default:
      return 'default'
  }
}

export default function LogsCard() {
  const [source, setSource] = useState('')
  const [minLevel, setMinLevel] = useState('')
  const { data, isLoading } = useLogs(source, minLevel)
  const entries = data?.entries ?? []

  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <CardTitle>Logs</CardTitle>
            <CardDescription>Recent application logs from api and web.</CardDescription>
          </div>
          <div className="flex gap-2">
            <Select
              value={source}
              onChange={(e) => setSource(e.target.value)}
              className="h-9 w-auto"
              aria-label="Source"
            >
              {SOURCE_OPTIONS.map((s) => (
                <option key={s} value={s}>
                  {s === '' ? 'All sources' : s}
                </option>
              ))}
            </Select>
            <Select
              value={minLevel}
              onChange={(e) => setMinLevel(e.target.value)}
              className="h-9 w-auto"
              aria-label="Minimum level"
            >
              {LEVEL_OPTIONS.map((l) => (
                <option key={l} value={l}>
                  {l === '' ? 'All levels' : l}
                </option>
              ))}
            </Select>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : entries.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No logs found.</p>
        ) : (
          <div className="max-h-96 space-y-2 overflow-auto">
            {entries.map((entry, i) => (
              <div
                key={`${entry.occurredAt}-${i}`}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="mb-1 flex flex-wrap items-center gap-2">
                  <Badge variant={levelVariant(entry.level)}>{entry.level || 'info'}</Badge>
                  <Badge variant="secondary">{entry.source}</Badge>
                  <span className="text-xs text-muted">{formatDateTime(entry.occurredAt)}</span>
                </div>
                <p className="break-words text-fg">{entry.message}</p>
                {entry.attrsJson && (
                  <pre className="mt-1 overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs text-muted">
                    {entry.attrsJson}
                  </pre>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
