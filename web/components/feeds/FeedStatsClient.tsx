'use client'

import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { useFeedStats } from '@/hooks/useFeeds'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const tooltipContentStyle = {
  backgroundColor: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: '0.75rem',
  color: 'var(--color-fg)'
}

// formatInterval renders a mean posting gap as a compact human string; 0
// means the feed has fewer than 2 items to establish a cadence yet.
function formatInterval(hours: number): string {
  if (hours <= 0) return 'not enough data yet'
  if (hours < 24) return `every ~${Math.round(hours)}h`
  return `every ~${(hours / 24).toFixed(1)}d`
}

function formatPct(fraction: number): string {
  return `${Math.round(fraction * 100)}%`
}

export default function FeedStatsClient() {
  const { data, error, isLoading } = useFeedStats()

  if (isLoading) return <p className="text-muted">Loading…</p>
  if (error) return <p className="text-danger">Failed to load feed stats.</p>
  if (!data || data.stats.length === 0) {
    return <p className="text-muted">No feed activity yet.</p>
  }

  const chartData = data.itemsPerDay.map((d) => ({
    day: new Date(d.day).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }),
    count: d.count
  }))

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>New items per day (last 90 days)</CardTitle>
        </CardHeader>
        <CardContent>
          {chartData.length === 0 ? (
            <p className="text-muted">No items ingested in the last 90 days.</p>
          ) : (
            <div className="h-64 w-full">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="day" tick={{ fontSize: 11 }} />
                  <YAxis allowDecimals={false} />
                  <Tooltip
                    formatter={(value) => [value, 'Items']}
                    cursor={{ fill: 'rgb(var(--hover-rgb) / 0.5)' }}
                    contentStyle={tooltipContentStyle}
                    labelStyle={{ color: 'var(--color-fg)' }}
                    itemStyle={{ color: 'var(--color-fg)' }}
                  />
                  <Bar dataKey="count" fill="#3b82f6" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {data.stats.map((s) => (
          <Card key={s.feedId} className="p-3">
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
                <dd className="text-fg">{Math.round(s.avgReadProgressPct)}%</dd>
              </div>
            </dl>
          </Card>
        ))}
      </div>
    </div>
  )
}
