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

export interface ProgressPoint {
  label: string
  value: number
}

interface BooksProgressChartProps {
  data: ProgressPoint[]
}

export default function BooksProgressChart({ data }: BooksProgressChartProps) {
  if (data.length === 0) {
    return <p className="text-muted">No progress data available.</p>
  }

  return (
    <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="rgb(var(--color-border))" />
          <XAxis dataKey="label" width={80} tick={{ fill: 'rgb(var(--color-muted))' }} />
          <YAxis tick={{ fill: 'rgb(var(--color-muted))' }} />
          <Tooltip
            contentStyle={{
              backgroundColor: 'rgb(var(--color-card))',
              borderColor: 'rgb(var(--color-border))',
              borderRadius: '0.75rem',
              color: 'rgb(var(--color-fg))'
            }}
          />
          <Line
            type="monotone"
            dataKey="value"
            stroke="rgb(var(--color-accent))"
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
