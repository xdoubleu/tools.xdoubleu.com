'use client'

import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell
} from 'recharts'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  GetDatabaseStatsResponse,
  GetDatabaseSizeHistoryResponse,
  DBSizeHistoryPoint
} from '@/lib/gen/observability/v1/observability_pb'
import {
  bytesTooltipFormatter,
  CATEGORICAL_PALETTE,
  chartTooltipStyle,
  formatBytes
} from '@/lib/observability'
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

export default function DatabaseCard({
  data,
  history
}: {
  data?: GetDatabaseStatsResponse
  history?: GetDatabaseSizeHistoryResponse
}) {
  const schemas = data?.schemas ?? []
  const historyData = (data?.history ?? []).map((s) => ({
    date: s.sampledAt.slice(0, 10),
    size: Number(s.totalSizeBytes)
  }))
  const chartData = schemas.map((s) => ({
    name: s.name,
    size: Number(s.sizeBytes),
    tables: Number(s.tableCount)
  }))
  const historyPoints = history?.points ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Database usage</CardTitle>
          <Badge variant="secondary">PostgreSQL</Badge>
        </div>
        <CardDescription>
          {data ? `${formatBytes(data.totalSizeBytes)} total on disk` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="mb-5">
          <h4 className="mb-2 text-sm font-semibold text-subtle">Total size over time</h4>
          {historyData.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted">No snapshot history.</p>
          ) : (
            <div className="h-56 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={historyData} margin={{ left: 8, right: 16 }}>
                  <defs>
                    <linearGradient id="databaseFill" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={CATEGORICAL_PALETTE[0]} stopOpacity={0.4} />
                      <stop offset="100%" stopColor={CATEGORICAL_PALETTE[0]} stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} />
                  <XAxis dataKey="date" tick={{ fontSize: 11 }} minTickGap={24} />
                  <YAxis tickFormatter={(v: number) => formatBytes(v)} tick={{ fontSize: 11 }} />
                  <Tooltip
                    formatter={bytesTooltipFormatter('Total size')}
                    contentStyle={chartTooltipStyle}
                    labelStyle={{ color: 'var(--color-fg)' }}
                    itemStyle={{ color: 'var(--color-fg)' }}
                  />
                  <Area
                    type="monotone"
                    dataKey="size"
                    stroke={CATEGORICAL_PALETTE[0]}
                    strokeWidth={2}
                    fill="url(#databaseFill)"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>

        {chartData.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No schema data.</p>
        ) : (
          <>
            <h4 className="mb-2 text-sm font-semibold text-subtle">Schemas</h4>
            <div className="h-64 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData} layout="vertical" margin={{ left: 8, right: 16 }}>
                  <CartesianGrid strokeDasharray="3 3" horizontal={false} />
                  <XAxis
                    type="number"
                    tickFormatter={(v: number) => formatBytes(v)}
                    tick={{ fontSize: 11 }}
                  />
                  <YAxis type="category" dataKey="name" width={96} tick={{ fontSize: 12 }} />
                  <Tooltip
                    formatter={bytesTooltipFormatter('Size')}
                    cursor={{ fill: 'rgb(var(--hover-rgb) / 0.5)' }}
                    contentStyle={chartTooltipStyle}
                    labelStyle={{ color: 'var(--color-fg)' }}
                    itemStyle={{ color: 'var(--color-fg)' }}
                  />
                  <Bar dataKey="size" radius={[0, 4, 4, 0]}>
                    {chartData.map((entry) => (
                      <Cell key={entry.name} fill={CATEGORICAL_PALETTE[0]} />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
            <div className="mt-4 overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-border">
                  <tr>
                    <th className="py-2 pr-3 font-semibold text-subtle">Schema</th>
                    <th className="py-2 pr-3 text-right font-semibold text-subtle">Size</th>
                    <th className="py-2 text-right font-semibold text-subtle">Tables</th>
                  </tr>
                </thead>
                <tbody>
                  {chartData.map((s) => (
                    <tr key={s.name} className="border-b border-border last:border-0">
                      <td className="py-2 pr-3 text-fg">{s.name}</td>
                      <td className="py-2 pr-3 text-right text-fg">{formatBytes(s.size)}</td>
                      <td className="py-2 text-right text-fg">{s.tables}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}

        <div className="mt-5">
          <h4 className="mb-2 text-sm font-semibold text-subtle">Schema &amp; table history</h4>
          <p className="mb-3 text-xs text-muted">
            Select one or more schemas or tables to plot their on-disk size over time.
          </p>
          {historyPoints.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted">No size history yet.</p>
          ) : (
            <MultiSeriesChart
              points={toSeriesPoints(historyPoints)}
              meta={toSeriesMeta(historyPoints)}
              valueLabel="size"
              valueFormatter={formatBytes}
              searchPlaceholder="Filter schemas or tables…"
            />
          )}
        </div>
      </CardContent>
    </Card>
  )
}
