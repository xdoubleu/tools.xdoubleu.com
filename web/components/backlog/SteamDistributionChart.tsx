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
    bucket: i * 10
  }))

  return (
    <div className="w-full h-64">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={chartData} style={onBucketClick ? { cursor: 'pointer' } : undefined}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="range" tick={{ fontSize: 11 }} />
          <YAxis allowDecimals={false} />
          <Tooltip formatter={(value) => [value, 'Games']} />
          <Bar
            dataKey="count"
            onClick={
              onBucketClick
                ? (entry) => onBucketClick((entry as unknown as { bucket: number }).bucket)
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
