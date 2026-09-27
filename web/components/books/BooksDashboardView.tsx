'use client'

import { Fragment, type ReactNode } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import { StatTile } from '@/components/ui/stat'
import BooksProgressChart from '@/components/books/BooksProgressChart'
import { Field } from '@/components/ui/field'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { DateInput } from '@/components/ui/date-input'
import { ytdProgress } from '@/lib/books/ytdProgress'
import { statusLabel } from '@/lib/books/bookShelves'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

/**
 * Books dashboard view shared by the private and public wrappers so they
 * can't drift; the public one passes no mutating actions.
 */
export default function BooksDashboardView({
  library,
  chart,
  allTimeChartData,
  renderReadingCard,
  actions
}: {
  library: LibraryResponse
  chart: DashboardChartState<'ytd' | 'all'>
  allTimeChartData: { label: string; value: number }[]
  renderReadingCard: (ub: UserBook) => ReactNode
  actions: ReactNode
}) {
  const { view, setView, start, setStart, end, setEnd } = chart
  const reading = library.reading
  const ytd = ytdProgress(library.finished)

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">{actions}</div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <StatTile
          label="Total books"
          value={reading.length + library.wishlist.length + library.finished.length}
        />
        <StatTile label={statusLabel('currently-reading')} value={reading.length} />
        <StatTile label={statusLabel('read')} value={library.finished.length} />
        <StatTile label="Read this year" value={ytd.total} />
        <StatTile label={statusLabel('to-read')} value={library.wishlist.length} />
      </div>

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
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
            <SegmentedTabs
              aria-label="Chart view"
              value={view}
              onChange={setView}
              options={[
                { value: 'ytd', label: 'This year' },
                { value: 'all', label: 'All time' }
              ]}
            />

            {view === 'all' && (
              <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
                <Field label="From" htmlFor="books-dash-from">
                  <DateInput
                    id="books-dash-from"
                    value={start}
                    onChange={setStart}
                    className="sm:w-40"
                  />
                </Field>
                <Field label="To" htmlFor="books-dash-to">
                  <DateInput id="books-dash-to" value={end} onChange={setEnd} className="sm:w-40" />
                </Field>
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
