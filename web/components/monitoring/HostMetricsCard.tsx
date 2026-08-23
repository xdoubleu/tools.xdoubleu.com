'use client'

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import type {
  GetHostMetricsResponse,
  HostMetricPoint
} from '@/lib/gen/observability/v1/observability_pb'
import { CATEGORICAL_PALETTE, chartTooltipStyle } from '@/lib/observability'
import { formatDateTime } from '@/lib/dates'
import StatTiles from './StatTiles'

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

function toChartData(points: HostMetricPoint[]): { timestamp: string; value: number }[] {
  return points.map((p) => ({ timestamp: p.timestamp, value: p.value }))
}

function HistoryChart({
  label,
  points,
  color
}: {
  label: string
  points: HostMetricPoint[]
  color: string
}) {
  const data = toChartData(points)

  return (
    <div>
      <p className="mb-1 text-xs font-medium uppercase tracking-wide text-muted">{label}</p>
      {data.length === 0 ? (
        <p className="py-6 text-center text-sm text-muted">No history yet.</p>
      ) : (
        <div className="h-40 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ left: 8, right: 16 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="timestamp"
                tick={{ fontSize: 11 }}
                minTickGap={32}
                tickFormatter={(v: string) => formatDateTime(v).split(',')[1]?.trim() ?? v}
              />
              <YAxis
                domain={[0, 100]}
                tick={{ fontSize: 11 }}
                tickFormatter={(v: number) => `${v}%`}
              />
              <Tooltip
                contentStyle={chartTooltipStyle}
                labelStyle={{ color: 'var(--color-fg)' }}
                itemStyle={{ color: 'var(--color-fg)' }}
                labelFormatter={(v) => formatDateTime(typeof v === 'string' ? v : '')}
                formatter={(value) => [formatPercent(Number(value)), label]}
              />
              <Line
                type="monotone"
                dataKey="value"
                stroke={color}
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export default function HostMetricsCard({ data }: { data?: GetHostMetricsResponse }) {
  const tiles = [
    { label: 'CPU', value: data ? formatPercent(data.cpuPercent) : '—' },
    { label: 'Memory', value: data ? formatPercent(data.memoryPercent) : '—' },
    { label: 'Disk', value: data ? formatPercent(data.diskPercent) : '—' }
  ]

  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <CardTitle>Host metrics</CardTitle>
        <CardDescription>CPU, memory, and disk usage, scraped from node_exporter.</CardDescription>
      </CardHeader>
      <CardContent>
        <StatTiles tiles={tiles} />
        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
          <HistoryChart
            label="CPU"
            points={data?.cpuHistory ?? []}
            color={CATEGORICAL_PALETTE[0]}
          />
          <HistoryChart
            label="Memory"
            points={data?.memoryHistory ?? []}
            color={CATEGORICAL_PALETTE[1]}
          />
          <HistoryChart
            label="Disk"
            points={data?.diskHistory ?? []}
            color={CATEGORICAL_PALETTE[2]}
          />
        </div>
      </CardContent>
    </Card>
  )
}
