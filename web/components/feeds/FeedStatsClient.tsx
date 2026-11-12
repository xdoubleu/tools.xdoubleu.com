import { Bar, BarChart, CartesianGrid, Tooltip, XAxis, YAxis } from 'recharts'
import { axisTick, countAxisProps, gridProps, seriesColor, tooltipProps } from '@/lib/chartTheme'
import { useFeedStats } from '@/hooks/useFeeds'
import { Card } from '@/components/ui/card'
import { ChartCard, StatTile, StatTileGrid } from '@/components/ui/stat'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { formatDate } from '@/lib/dates'
import { formatNumber, formatPercent } from '@/lib/format'

// Stryker disable next-line ArrayDeclaration: chart styling.
const BAR_RADIUS: [number, number, number, number] = [4, 4, 0, 0]

// 0 means fewer than 2 items, so no cadence yet.
function formatInterval(hours: number): string {
  if (hours <= 0) return 'not enough data yet'
  if (hours < 24) return `every ~${Math.round(hours)}h`
  return `every ~${formatNumber(hours / 24)}d`
}

function formatPct(fraction: number): string {
  return `${Math.round(fraction * 100)}%`
}

export default function FeedStatsClient() {
  const { data, error, isLoading } = useFeedStats()

  if (isLoading) return <LoadingState placement="page" />
  if (error) return <ErrorState placement="page" what="feed stats" />
  if (!data || data.stats.length === 0) {
    return <EmptyState placement="page">No feed activity yet.</EmptyState>
  }

  const chartData = data.itemsPerDay.map((d) => ({
    day: formatDate(d.day),
    count: d.count
  }))

  const totalItems = data.stats.reduce((sum, s) => sum + s.itemCount, 0)
  const recentItems = data.itemsPerDay.reduce((sum, d) => sum + d.count, 0)

  return (
    <div className="flex flex-col gap-6">
      <StatTileGrid columns={3}>
        <StatTile label="Feeds" value={data.stats.length} />
        <StatTile label="Items" value={formatNumber(totalItems)} />
        <StatTile label="Last 90 days" value={formatNumber(recentItems)} />
      </StatTileGrid>

      <ChartCard
        title="New items per day (last 90 days)"
        empty={chartData.length === 0 ? 'No items ingested in the last 90 days.' : undefined}
      >
        <BarChart data={chartData}>
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="day" tick={axisTick} />
          <YAxis {...countAxisProps} />
          <Tooltip {...tooltipProps} />
          <Bar dataKey="count" name="Items" fill={seriesColor(1)} radius={BAR_RADIUS} />
        </BarChart>
      </ChartCard>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {data.stats.map((s) => (
          <Card key={s.feedId} padding="sm">
            <p className="truncate text-sm font-semibold">{s.feedTitle || 'Untitled feed'}</p>
            <dl className="mt-2 space-y-1 text-xs text-muted">
              <div className="flex justify-between">
                <dt>Items</dt>
                <dd className="text-fg">{s.itemCount}</dd>
              </div>
              <div className="flex justify-between">
                <dt>Cadence</dt>
                <dd className="text-fg">{formatInterval(s.avgIntervalHours)}</dd>
              </div>
              <div className="flex justify-between">
                <dt>Read rate</dt>
                <dd className="text-fg">{formatPct(s.readRate)}</dd>
              </div>
              <div className="flex justify-between">
                <dt>Avg completion</dt>
                <dd className="text-fg">{formatPercent(s.avgReadProgressPct)}</dd>
              </div>
            </dl>
          </Card>
        ))}
      </div>
    </div>
  )
}
