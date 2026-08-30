'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  GetDatabaseSizeHistoryResponse,
  DBSizeHistoryPoint
} from '@/lib/gen/observability/v1/observability_pb'
import { formatBytes } from '@/lib/observability'
import MultiSeriesChart, { type SeriesMeta, type SeriesPoint } from './MultiSeriesChart'

function seriesKey(schemaName: string, tableName: string): string {
  return `${schemaName}.${tableName}`
}

export function toSeriesPoints(points: DBSizeHistoryPoint[]): SeriesPoint[] {
  return points.map((p) => ({
    day: p.day,
    seriesKey: seriesKey(p.schemaName, p.tableName),
    value: Number(p.sizeBytes)
  }))
}

export function toSeriesMeta(points: DBSizeHistoryPoint[]): SeriesMeta[] {
  const seen = new Map<string, SeriesMeta>()
  for (const p of points) {
    const key = seriesKey(p.schemaName, p.tableName)
    if (!seen.has(key)) {
      seen.set(key, { key, label: key })
    }
  }
  return [...seen.values()]
}

export default function DatabaseSizeHistoryCard({
  data
}: {
  data?: GetDatabaseSizeHistoryResponse
}) {
  const points = data?.points ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Database size history</CardTitle>
          <Badge variant="secondary">PostgreSQL</Badge>
        </div>
        <CardDescription>
          Select one or more schemas or tables to plot their on-disk size over time.
        </CardDescription>
      </CardHeader>
      <CardContent>
        {points.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No size history yet.</p>
        ) : (
          <MultiSeriesChart
            points={toSeriesPoints(points)}
            meta={toSeriesMeta(points)}
            valueLabel="size"
            valueFormatter={formatBytes}
            searchPlaceholder="Filter schemas or tables…"
          />
        )}
      </CardContent>
    </Card>
  )
}
