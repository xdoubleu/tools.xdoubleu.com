import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { axisDate } from '@/lib/dates'

// Stryker disable next-line ObjectLiteral: chart styling.
const AXIS_TICK = { fontSize: 11 }

interface SteamProgressChartProps {
  data: { label: string; value: number }[]
}

export default function SteamProgressChart({ data }: SteamProgressChartProps) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="label" tick={AXIS_TICK} tickFormatter={axisDate} />
        <YAxis />
        <Tooltip
          cursor={{ stroke: 'var(--color-border)' }}
          contentStyle={{
            backgroundColor: 'var(--color-surface)',
            border: '1px solid var(--color-border)',
            borderRadius: '0.75rem',
            color: 'var(--color-fg)'
          }}
        />
        <Line
          type="monotone"
          dataKey="value"
          stroke="var(--color-accent)"
          strokeWidth={2}
          dot={false}
        />
      </LineChart>
    </ResponsiveContainer>
  )
}
