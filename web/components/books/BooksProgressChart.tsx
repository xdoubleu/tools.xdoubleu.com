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
import { EmptyState } from '@/components/ui/states'

export interface ProgressPoint {
  label: string
  value: number
}

interface BooksProgressChartProps {
  data: ProgressPoint[]
}

export default function BooksProgressChart({ data }: BooksProgressChartProps) {
  if (data.length === 0) {
    return <EmptyState>No progress data available.</EmptyState>
  }

  return (
    <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="label" width={80} tick={axisTick} tickFormatter={axisDate} />
          <YAxis tick={axisTick} />
          <Tooltip {...lineTooltipProps} />
          <Line dataKey="value" stroke={seriesColor(1)} {...lineProps} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
