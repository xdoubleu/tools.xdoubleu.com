import { Bar, BarChart, CartesianGrid, Legend, Tooltip, XAxis, YAxis } from 'recharts'
import { axisTick, countAxisProps, gridProps, seriesColor, tooltipProps } from '@/lib/chartTheme'
import { useMovieStats } from '@/hooks/useMovies'
import { monthLabel } from '@/lib/dates'
import { RatingStars } from '@/components/ui/rating-stars'
import { ChartCard, RankedList, StatTile, StatTileGrid } from '@/components/ui/stat'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

/** Watching totals, watches per month, top genres and ratings. */
export default function MoviesStatsClient() {
  const { data, error, isLoading } = useMovieStats()

  if (isLoading && !data) return <LoadingState label="stats" placement="page" />
  if (error && !data) return <ErrorState what="stats" placement="page" />
  if (!data) return null
  const rated = data.ratings.some((n) => n > 0)
  if (data.moviesWatched + data.seriesWatched + data.seasonsWatched === 0 && !rated) {
    return (
      <EmptyState placement="page">
        Nothing watched yet. Mark a movie or season watched to see stats.
      </EmptyState>
    )
  }

  const months = data.months.map((m) => ({
    month: monthLabel(m.month),
    Movies: m.movies,
    Seasons: m.seasons
  }))

  return (
    <div className="space-y-6">
      <StatTileGrid columns={3}>
        <StatTile label="Movies watched" value={data.moviesWatched} />
        <StatTile label="Series watched" value={data.seriesWatched} />
        <StatTile label="Seasons watched" value={data.seasonsWatched} />
      </StatTileGrid>

      <ChartCard title="Watched per month" note="Watches without a date aren’t charted.">
        <BarChart data={months}>
          <CartesianGrid {...gridProps} />
          <XAxis dataKey="month" tick={axisTick} />
          <YAxis {...countAxisProps} />
          <Tooltip {...tooltipProps} />
          <Legend />
          <Bar dataKey="Movies" stackId="watches" fill={seriesColor(1)} />
          <Bar dataKey="Seasons" stackId="watches" fill={seriesColor(2)} />
        </BarChart>
      </ChartCard>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <RankedList
          title="Top genres"
          empty="No genres yet."
          items={data.genres.map((g) => ({ key: g.genre, label: g.genre, value: g.count }))}
        />
        <RankedList
          title="Ratings"
          empty="No ratings yet."
          items={
            rated
              ? [5, 4, 3, 2, 1].map((stars) => ({
                  key: String(stars),
                  label: <RatingStars value={stars} />,
                  value: data.ratings[stars - 1] ?? 0
                }))
              : []
          }
        />
      </div>
    </div>
  )
}
