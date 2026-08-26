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
import { Badge } from '@/components/ui/badge'
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

// hostMetricDangerThreshold/hostMetricWarnThreshold are client-side alerting
// thresholds for CPU/memory/disk usage — no backend threshold config exists
// for these raw node_exporter percentages (issue #1261).
const hostMetricDangerThreshold = 90
const hostMetricWarnThreshold = 75

export function hostMetricTone(value: number): 'danger' | 'warn' | 'default' {
  if (value >= hostMetricDangerThreshold) return 'danger'
  if (value >= hostMetricWarnThreshold) return 'warn'
  return 'default'
}

function toChartData(points: HostMetricPoint[]): { timestamp: string; value: number }[] {
  return points.map((p) => ({ timestamp: p.timestamp, value: p.value }))
}

// Extracted (rather than inlined as chart-prop closures) so they're directly
// unit-testable: recharts' <ResponsiveContainer> never actually invokes its
// children's formatter callbacks under jsdom, since it measures zero layout
// width/height there and skips rendering the chart body entirely.
export function xAxisTickFormatter(value: string): string {
  return formatDateTime(value).split(',')[1]?.trim() ?? value
}

export function yAxisTickFormatter(value: number): string {
  return `${value}%`
}

export function tooltipLabelFormatter(value: unknown): string {
  return formatDateTime(typeof value === 'string' ? value : '')
}

export function tooltipValueFormatter(value: unknown, label: string): [string, string] {
  return [formatPercent(Number(value)), label]
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
                tickFormatter={xAxisTickFormatter}
              />
              <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} tickFormatter={yAxisTickFormatter} />
              <Tooltip
                contentStyle={chartTooltipStyle}
                labelStyle={{ color: 'var(--color-fg)' }}
                itemStyle={{ color: 'var(--color-fg)' }}
                labelFormatter={tooltipLabelFormatter}
                formatter={(value) => tooltipValueFormatter(value, label)}
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
    {
      label: 'CPU',
      value: data ? formatPercent(data.cpuPercent) : '—',
      tone: data ? hostMetricTone(data.cpuPercent) : ('default' as const)
    },
    {
      label: 'Memory',
      value: data ? formatPercent(data.memoryPercent) : '—',
      tone: data ? hostMetricTone(data.memoryPercent) : ('default' as const)
    },
    {
      label: 'Disk',
      value: data ? formatPercent(data.diskPercent) : '—',
      tone: data ? hostMetricTone(data.diskPercent) : ('default' as const)
    }
  ]

  return (
    <Card className="lg:col-span-2">
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Host metrics</CardTitle>
          <Badge variant="secondary">Host</Badge>
        </div>
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
