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
import { axisTick, countAxisProps, gridProps, seriesColor, tooltipProps } from '@/lib/chartTheme'
import { EmptyState } from '@/components/ui/states'

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
    return <EmptyState placement="inline">No distribution data available.</EmptyState>
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
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="range" tick={axisTick} />
          <YAxis {...countAxisProps} />
          <Tooltip {...tooltipProps} />
          <Bar
            dataKey="count"
            name="Games"
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
              <Cell key={entry.range} fill={seriesColor(1)} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
