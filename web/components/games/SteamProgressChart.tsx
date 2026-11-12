import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { axisTick, gridProps, lineProps, lineTooltipProps, seriesColor } from '@/lib/chartTheme'
import { axisDate } from '@/lib/dates'

interface SteamProgressChartProps {
  data: { label: string; value: number }[]
}

export default function SteamProgressChart({ data }: SteamProgressChartProps) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <LineChart data={data}>
        <CartesianGrid {...gridProps} />
        <XAxis dataKey="label" tick={axisTick} tickFormatter={axisDate} />
        <YAxis tick={axisTick} />
        <Tooltip {...lineTooltipProps} />
        <Line dataKey="value" stroke={seriesColor(1)} {...lineProps} />
      </LineChart>
    </ResponsiveContainer>
  )
}
