'use client'

import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type {
  GetStorageStatsResponse,
  StorageSnapshot
} from '@/lib/gen/observability/v1/observability_pb'
import {
  CATEGORICAL_PALETTE,
  chartTooltipStyle,
  formatBytes,
  formatCount
} from '@/lib/observability'

function CleanupNotice({ latest }: { latest: StorageSnapshot }) {
  const orphans = Number(latest.orphanCount)
  const stale = Number(latest.staleUploadCount)
  if (orphans === 0 && stale === 0) {
    return <Badge variant="secondary">No cleanup needed</Badge>
  }
  return (
    <div className="flex flex-wrap gap-2">
      {orphans > 0 && (
        <Badge variant="danger">
          {formatCount(latest.orphanCount)} orphaned ({formatBytes(latest.orphanSizeBytes)})
        </Badge>
      )}
      {stale > 0 && (
        <Badge variant="danger">
          {formatCount(latest.staleUploadCount)} stale uploads (
          {formatBytes(latest.staleUploadSizeBytes)})
        </Badge>
      )}
    </div>
  )
}

export default function StorageCard({ data }: { data?: GetStorageStatsResponse }) {
  const latest = data?.latest
  const history = data?.history ?? []
  const chartData = history.map((s) => ({
    date: s.scannedAt.slice(0, 10),
    size: Number(s.totalSizeBytes)
  }))

  return (
    <Card>
      <CardHeader>
        <CardTitle>R2 object storage</CardTitle>
        <CardDescription>
          {latest
            ? `${formatBytes(latest.totalSizeBytes)} across ${formatCount(latest.objectCount)} objects`
            : 'No scan recorded yet.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {latest && (
          <div className="mb-4">
            <CleanupNotice latest={latest} />
          </div>
        )}

        {chartData.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No snapshot history.</p>
        ) : (
          <div className="h-56 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={chartData} margin={{ left: 8, right: 16 }}>
                <defs>
                  <linearGradient id="storageFill" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="0%" stopColor={CATEGORICAL_PALETTE[0]} stopOpacity={0.4} />
                    <stop offset="100%" stopColor={CATEGORICAL_PALETTE[0]} stopOpacity={0.02} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" vertical={false} />
                <XAxis dataKey="date" tick={{ fontSize: 11 }} minTickGap={24} />
                <YAxis tickFormatter={(v: number) => formatBytes(v)} tick={{ fontSize: 11 }} />
                <Tooltip
                  formatter={(value) => [formatBytes(Number(value)), 'Total size']}
                  contentStyle={chartTooltipStyle}
                  labelStyle={{ color: 'var(--color-fg)' }}
                  itemStyle={{ color: 'var(--color-fg)' }}
                />
                <Area
                  type="monotone"
                  dataKey="size"
                  stroke={CATEGORICAL_PALETTE[0]}
                  strokeWidth={2}
                  fill="url(#storageFill)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        )}

        {latest && latest.prefixBreakdown.length > 0 && (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border">
                <tr>
                  <th className="py-2 pr-3 font-semibold text-subtle">Prefix</th>
                  <th className="py-2 pr-3 text-right font-semibold text-subtle">Size</th>
                  <th className="py-2 text-right font-semibold text-subtle">Objects</th>
                </tr>
              </thead>
              <tbody>
                {latest.prefixBreakdown.map((p) => (
                  <tr key={p.prefix} className="border-b border-border last:border-0">
                    <td className="py-2 pr-3 text-fg">{p.prefix}</td>
                    <td className="py-2 pr-3 text-right text-fg">{formatBytes(p.sizeBytes)}</td>
                    <td className="py-2 text-right text-fg">{formatCount(p.count)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
