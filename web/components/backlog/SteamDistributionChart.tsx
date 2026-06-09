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

const BUCKET_LABELS = [
  '0-9%',
  '10-19%',
  '20-29%',
  '30-39%',
  '40-49%',
  '50-59%',
  '60-69%',
  '70-79%',
  '80-89%',
  '90-99%',
  '100%'
]

const tooltipContentStyle = {
  backgroundColor: 'rgb(var(--color-surface))',
  border: '1px solid rgb(var(--color-border))',
  borderRadius: '0.75rem',
  color: 'rgb(var(--color-fg))'
}

interface SteamDistributionChartProps {
  distribution: number[]
  onBucketClick?: (bucket: number) => void
}

export default function SteamDistributionChart({
  distribution,
  onBucketClick
}: SteamDistributionChartProps) {
  if (!distribution || distribution.length === 0) {
    return <p className="text-muted">No distribution data available.</p>
  }

  const chartData = distribution.map((count, i) => ({
    range: BUCKET_LABELS[i] ?? `${i * 10}%`,
    count,
    bucket: i
  }))

  return (
    <div className="h-full min-h-0 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={chartData} style={onBucketClick ? { cursor: 'pointer' } : undefined}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="range" tick={{ fontSize: 11 }} />
          <YAxis allowDecimals={false} />
          <Tooltip
            formatter={(value) => [value, 'Games']}
            cursor={{ fill: 'rgb(var(--color-hover) / 0.5)' }}
            contentStyle={tooltipContentStyle}
            labelStyle={{ color: 'rgb(var(--color-fg))' }}
            itemStyle={{ color: 'rgb(var(--color-fg))' }}
          />
          <Bar
            dataKey="count"
            onClick={
              onBucketClick
                ? (entry: unknown) => {
                    if (entry !== null && typeof entry === 'object' && 'bucket' in entry) {
                      const bucket = (entry as Record<string, unknown>)['bucket']
                      if (typeof bucket === 'number') {
                        onBucketClick(bucket)
                      }
                    }
                  }
                : undefined
            }
          >
            {chartData.map((entry) => (
              <Cell key={entry.range} fill="#3b82f6" />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
