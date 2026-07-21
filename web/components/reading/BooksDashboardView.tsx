'use client'

import { Fragment, type ReactNode } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/reading/v1/library_pb'
// ponytail: GamesStatCard is the shared stat card for both apps; not renamed to
// avoid churning imports across ~5 files for no behaviour change.
import GamesStatCard from '@/components/games/GamesStatCard'
import BooksProgressChart from '@/components/reading/BooksProgressChart'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { ytdProgress } from '@/lib/reading/ytdProgress'
import { statusLabel } from '@/lib/reading/bookShelves'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

/**
 * Presentational books dashboard shared by the private (`BooksDashboard`) and
 * public (`ProfileBooksClient`) wrappers so their cards/charts can't drift.
 * The wrappers supply data, the reading-card renderer, an optional feeds slot,
 * and owner actions; the public one passes no mutating controls.
 */
export default function BooksDashboardView({
  library,
  chart,
  allTimeChartData,
  renderReadingCard,
  feedsSlot,
  actions
}: {
  library: LibraryResponse
  chart: DashboardChartState<'ytd' | 'all'>
  allTimeChartData: { label: string; value: number }[]
  renderReadingCard: (ub: UserBook) => ReactNode
  feedsSlot?: ReactNode
  actions: ReactNode
}) {
  const { view, setView, start, setStart, end, setEnd } = chart
  const reading = library.reading
  const rss = library.rss
  const rssRead = rss.filter((ub) => ub.status === 'read').length
  const ytd = ytdProgress(library.finished)

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">{actions}</div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <GamesStatCard
          label="Total books"
          value={reading.length + library.wishlist.length + library.finished.length}
        />
        <GamesStatCard label={statusLabel('currently-reading')} value={reading.length} />
        <GamesStatCard label={statusLabel('read')} value={library.finished.length} />
        <GamesStatCard label="Read this year" value={ytd.total} />
        <GamesStatCard label={statusLabel('to-read')} value={library.wishlist.length} />
      </div>

      {rss.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <GamesStatCard label="RSS items" value={rss.length} />
          <GamesStatCard label="RSS read" value={rssRead} />
        </div>
      )}

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col gap-3">
          <div className="flex min-h-0 flex-col lg:flex-1">
            <h2 className="mb-2 text-base font-semibold">Currently reading</h2>
            {reading.length === 0 && <p className="text-muted text-sm">No books in progress.</p>}
            {reading.length > 0 && (
              <div className="flex min-h-0 flex-wrap content-start gap-3 overflow-y-auto pr-1 lg:flex-1">
                {reading.map((ub) => (
                  <Fragment key={ub.id}>{renderReadingCard(ub)}</Fragment>
                ))}
              </div>
            )}
          </div>
          {feedsSlot && <div className="shrink-0 lg:max-h-56 lg:overflow-y-auto">{feedsSlot}</div>}
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
            <div
              role="tablist"
              aria-label="Chart view"
              className="flex gap-1 rounded-xl border border-border bg-surface p-1"
            >
              <Button
                role="tab"
                aria-selected={view === 'ytd'}
                size="sm"
                variant={view === 'ytd' ? 'default' : 'ghost'}
                onClick={() => setView('ytd')}
              >
                This year
              </Button>
              <Button
                role="tab"
                aria-selected={view === 'all'}
                size="sm"
                variant={view === 'all' ? 'default' : 'ghost'}
                onClick={() => setView('all')}
              >
                All time
              </Button>
            </div>

            {view === 'all' && (
              <div className="flex gap-3">
                <div>
                  <label htmlFor="books-dash-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="books-dash-from"
                    value={start}
                    onChange={setStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="books-dash-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="books-dash-to"
                    value={end}
                    onChange={setEnd}
                    className="h-9 w-40"
                  />
                </div>
              </div>
            )}
          </div>

          {view === 'ytd' && (
            <>
              {ytd.series.length === 0 && (
                <p className="text-muted text-sm">No books finished this year yet.</p>
              )}
              {ytd.series.length > 0 && <BooksProgressChart data={ytd.series} />}
            </>
          )}

          {view === 'all' && <BooksProgressChart data={allTimeChartData} />}
        </div>
      </div>
    </section>
  )
}
