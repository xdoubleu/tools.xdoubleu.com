'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  GetTransactionLatencyHistoryResponse,
  TransactionLatencyPoint
} from '@/lib/gen/observability/v1/observability_pb'
import { formatDuration } from '@/lib/observability'
import MultiSeriesChart, { type SeriesMeta, type SeriesPoint } from './MultiSeriesChart'

function seriesKey(project: string, transaction: string): string {
  return `${project}/${transaction}`
}

export function toSeriesPoints(points: TransactionLatencyPoint[]): SeriesPoint[] {
  return points.map((p) => ({
    day: p.day,
    seriesKey: seriesKey(p.project, p.transaction),
    value: p.p95DurationMs
  }))
}

export function toSeriesMeta(points: TransactionLatencyPoint[]): SeriesMeta[] {
  const seen = new Map<string, SeriesMeta>()
  for (const p of points) {
    const key = seriesKey(p.project, p.transaction)
    if (!seen.has(key)) {
      seen.set(key, { key, label: `${p.project} · ${p.transaction}` })
    }
  }
  return [...seen.values()]
}

export default function TransactionLatencyHistoryCard({
  data
}: {
  data?: GetTransactionLatencyHistoryResponse
}) {
  const points = data?.points ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Span latency history</CardTitle>
          <Badge variant="secondary">Sentry</Badge>
        </div>
        <CardDescription>
          Select one or more spans to plot their p95 duration over time.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {points.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No latency history yet.</p>
        ) : (
          <MultiSeriesChart
            points={toSeriesPoints(points)}
            meta={toSeriesMeta(points)}
            valueLabel="p95 duration"
            valueFormatter={formatDuration}
            searchPlaceholder="Filter spans…"
          />
        )}
      </CardContent>
    </Card>
  )
}
