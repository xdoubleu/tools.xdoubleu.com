'use client'

import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from 'recharts'
import { useMovieStats } from '@/hooks/useMovies'
import { monthLabel } from '@/lib/movies/format'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { RatingStars } from '@/components/ui/rating-stars'
import { StatTile } from '@/components/ui/stat'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

const tooltipContentStyle = {
  backgroundColor: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: '0.75rem',
  color: 'var(--color-fg)'
}

/** Watching totals, watches per month, top genres and ratings. */
export default function MoviesStatsClient() {
  const { data, error, isLoading } = useMovieStats()

  if (isLoading && !data) return <LoadingState label="stats" />
  if (error && !data) return <ErrorState what="stats" />
  if (!data) return null
  if (data.moviesWatched + data.seasonsWatched === 0) {
    return (
      <EmptyState>Nothing watched yet. Mark a movie or season watched to see stats.</EmptyState>
    )
  }

  const months = data.months.map((m) => ({
    month: monthLabel(m.month),
    Movies: m.movies,
    Seasons: m.seasons
  }))
  const rated = data.ratings.some((n) => n > 0)

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <StatTile label="Movies watched" value={data.moviesWatched} />
        <StatTile label="Series watched" value={data.seriesWatched} />
        <StatTile label="Seasons watched" value={data.seasonsWatched} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Watched per month</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={months}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                <XAxis dataKey="month" tick={{ fontSize: 11, fill: 'var(--color-muted)' }} />
                <YAxis allowDecimals={false} tick={{ fill: 'var(--color-muted)' }} width={32} />
                <Tooltip
                  cursor={{ fill: 'rgb(var(--hover-rgb) / 0.5)' }}
                  contentStyle={tooltipContentStyle}
                  labelStyle={{ color: 'var(--color-fg)' }}
                  itemStyle={{ color: 'var(--color-fg)' }}
                />
                <Legend />
                <Bar dataKey="Movies" stackId="watches" fill="var(--color-accent)" />
                <Bar dataKey="Seasons" stackId="watches" fill="var(--color-star)" />
              </BarChart>
            </ResponsiveContainer>
          </div>
          <p className="mt-2 text-xs text-muted">Watches without a date aren&apos;t charted.</p>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Top genres</CardTitle>
          </CardHeader>
          <CardContent>
            {data.genres.length === 0 ? (
              <p className="text-sm text-muted">No genres yet.</p>
            ) : (
              <ol className="space-y-1 text-sm">
                {data.genres.map((g) => (
                  <li key={g.genre} className="flex justify-between gap-3">
                    <span className="min-w-0 break-words">{g.genre}</span>
                    <span className="text-muted">{g.count}</span>
                  </li>
                ))}
              </ol>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Ratings</CardTitle>
          </CardHeader>
          <CardContent>
            {rated ? (
              <ul className="space-y-1 text-sm">
                {[5, 4, 3, 2, 1].map((stars) => (
                  <li key={stars} className="flex items-center justify-between gap-3">
                    <RatingStars value={stars} />
                    <span className="text-muted">{data.ratings[stars - 1] ?? 0}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted">No ratings yet.</p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
