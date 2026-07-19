'use client'

import {
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
import type { GetDatabaseStatsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { CATEGORICAL_PALETTE, chartTooltipStyle, formatBytes } from '@/lib/observability'

export default function DatabaseCard({ data }: { data?: GetDatabaseStatsResponse }) {
  const schemas = data?.schemas ?? []
  const chartData = schemas.map((s) => ({
    name: s.name,
    size: Number(s.sizeBytes),
    tables: Number(s.tableCount)
  }))

  return (
    <Card>
      <CardHeader>
        <CardTitle>Database usage</CardTitle>
        <CardDescription>
          {data ? `${formatBytes(data.totalSizeBytes)} total on disk` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {chartData.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No schema data.</p>
        ) : (
          <>
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
                    formatter={(value) => [formatBytes(Number(value)), 'Size']}
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
      </CardContent>
    </Card>
  )
}
