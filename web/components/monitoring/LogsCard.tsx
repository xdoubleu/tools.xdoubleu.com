'use client'

import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { useLogs } from '@/hooks/useMonitoring'
import { formatDateTime } from '@/lib/dates'

const SOURCE_OPTIONS = ['', 'api', 'web']
const LEVEL_OPTIONS = ['', 'debug', 'info', 'warn', 'error']

function levelColor(level: string): string {
  switch (level.toLowerCase()) {
    case 'error':
      return 'text-red-400'
    case 'warn':
      return 'text-amber-400'
    case 'debug':
      return 'text-neutral-500'
    default:
      return 'text-sky-400'
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
            <div className="flex items-center gap-2">
              <CardTitle>Logs</CardTitle>
              <Badge variant="secondary">Internal</Badge>
            </div>
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
          <div className="max-h-96 overflow-auto rounded-lg border border-border bg-neutral-950 p-3 font-mono text-xs leading-relaxed">
            {entries.map((entry, i) => (
              <div key={`${entry.occurredAt}-${i}`} className="whitespace-pre-wrap break-words">
                <span className="text-neutral-500">{formatDateTime(entry.occurredAt)}</span>{' '}
                <span className={levelColor(entry.level)}>
                  [{(entry.level || 'info').toUpperCase()}]
                </span>{' '}
                <span className="text-neutral-500">{entry.source}</span>{' '}
                <span className="text-neutral-100">{entry.message}</span>
                {entry.attrsJson && <div className="pl-4 text-neutral-500">{entry.attrsJson}</div>}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
