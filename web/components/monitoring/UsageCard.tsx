'use client'

import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer
} from 'recharts'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetUsageStatsResponse } from '@/lib/gen/observability/v1/observability_pb'
import {
  CATEGORICAL_PALETTE,
  OTHER_COLOR,
  chartTooltipStyle,
  formatBytes,
  formatCount
} from '@/lib/observability'
import { aggregateUsage, OTHER_LABEL } from './usageData'

function seriesColor(app: string, index: number): string {
  if (app === OTHER_LABEL) return OTHER_COLOR
  return CATEGORICAL_PALETTE[index] ?? OTHER_COLOR
}

export default function UsageCard({ data }: { data?: GetUsageStatsResponse }) {
  const { rows, apps, endpoints } = aggregateUsage(data?.entries ?? [])
  const unusedApps = data?.unusedApps ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Feature usage</CardTitle>
          <Badge variant="secondary">Internal</Badge>
        </div>
        <CardDescription>Requests per day by app, across all traffic.</CardDescription>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No usage recorded yet.</p>
        ) : (
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={rows} margin={{ left: 8, right: 16 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="day" tick={{ fontSize: 11 }} minTickGap={24} />
                <YAxis allowDecimals={false} tick={{ fontSize: 11 }} />
                <Tooltip
                  contentStyle={chartTooltipStyle}
                  labelStyle={{ color: 'var(--color-fg)' }}
                  itemStyle={{ color: 'var(--color-fg)' }}
                />
                <Legend wrapperStyle={{ fontSize: 12 }} />
                {apps.map((app, i) => (
                  <Area
                    key={app}
                    type="monotone"
                    dataKey={app}
                    stackId="usage"
                    stroke={seriesColor(app, i)}
                    fill={seriesColor(app, i)}
                    fillOpacity={0.65}
                    strokeWidth={1}
                  />
                ))}
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}

        {endpoints.length > 0 && (
          <div className="mt-4 max-h-72 overflow-auto">
            <table className="w-full text-left text-sm">
              <thead className="sticky top-0 border-b border-border bg-card">
                <tr>
                  <th className="py-2 pr-3 font-semibold text-subtle">App</th>
                  <th className="py-2 pr-3 font-semibold text-subtle">Endpoint</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Requests</th>
                  <th className="py-2 text-right font-semibold text-subtle">Served</th>
                </tr>
              </thead>
              <tbody>
                {endpoints.map((e) => (
                  <tr
                    key={`${e.app}-${e.endpoint}`}
                    className="border-b border-border last:border-0"
                  >
                    <td className="py-2 pr-3 text-fg">{e.app}</td>
                    <td className="py-2 pr-3 font-mono text-xs text-fg">{e.endpoint}</td>
                    <td className="py-2 pr-3 text-right text-fg">{formatCount(e.count)}</td>
                    <td className="py-2 text-right text-fg">{formatBytes(e.bytes)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {unusedApps.length > 0 && (
          <p className="mt-4 text-sm text-muted">Unused in this window: {unusedApps.join(', ')}</p>
        )}
      </CardContent>
    </Card>
  )
}
