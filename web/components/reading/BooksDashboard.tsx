'use client'

import { useState } from 'react'
import Link from 'next/link'
import { mutate } from 'swr'
import { useLibrary, useBooksProgress } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import BookCover from '@/components/reading/BookCover'
import BookSearchBar from '@/components/reading/BookSearchBar'
import BooksProgressChart from '@/components/reading/BooksProgressChart'
import BookProgressBar from '@/components/reading/BookProgressBar'
import AddToLibraryDialog from '@/components/reading/AddToLibraryDialog'
import SubscribedFeedsCard from '@/components/reading/SubscribedFeedsCard'
import ProfileShareButton from '@/components/profile/ProfileShareButton'
import { Button } from '@/components/ui/button'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { DateInput } from '@/components/ui/date-input'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/dates'
import { ytdProgress } from '@/lib/reading/ytdProgress'
import { statusLabel } from '@/lib/reading/bookShelves'
import { swrKeys } from '@/lib/swrKeys'

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-3">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-xl font-bold mt-0.5">{value}</p>
    </Card>
  )
}

function ReadingBookCard({ userBook }: { userBook: UserBook }) {
  const book = userBook.book
  if (!book) return null
  return (
    <Link
      href={`/reading/${userBook.id}`}
      className={cn(
        interactiveCardClass,
        'relative flex w-full gap-3 p-4 text-left sm:w-60 self-start'
      )}
    >
      <CardLinkStatus />
      <BookCover coverUrl={book.coverUrl} title={book.title} size="md" />
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold truncate">{book.title}</h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
        <div className="mt-2">
          <BookProgressBar userBook={userBook} />
        </div>
      </div>
    </Link>
  )
}

export default function BooksDashboard() {
  const [view, setView] = useState<'ytd' | 'all'>('ytd')
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())
  const [addOpen, setAddOpen] = useState(false)

  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const { data: progressData } = useBooksProgress(
    view === 'all' ? progressStart : undefined,
    view === 'all' ? progressEnd : undefined
  )

  const library = libraryData?.library
  const reading = library?.reading ?? []
  const rss = library?.rss ?? []
  const rssRead = rss.filter((ub) => ub.status === 'read').length

  const ytd = ytdProgress(library?.finished ?? [])

  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

  const handleRefresh = () => {
    void mutate(swrKeys.books)
  }

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <div className="mr-auto w-full max-w-md">
          <BookSearchBar onAdded={handleRefresh} />
        </div>
        <Button onClick={() => setAddOpen(true)}>Add to library</Button>
        <ProfileShareButton app="reading" />
        <Button asChild variant="secondary">
          <Link href="/reading/library">Browse full library</Link>
        </Button>
      </div>
      <AddToLibraryDialog open={addOpen} onOpenChange={setAddOpen} onAdded={handleRefresh} />

      {libLoading && <p className="text-muted">Loading dashboard…</p>}
      {libError && <p className="text-danger">Failed to load books.</p>}

      {library && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <StatCard
            label="Total books"
            value={reading.length + library.wishlist.length + library.finished.length}
          />
          <StatCard label={statusLabel('currently-reading')} value={reading.length} />
          <StatCard label={statusLabel('read')} value={library.finished.length} />
          <StatCard label="Read this year" value={ytd.total} />
          <StatCard label={statusLabel('to-read')} value={library.wishlist.length} />
        </div>
      )}

      {library && rss.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="RSS items" value={rss.length} />
          <StatCard label="RSS read" value={rssRead} />
        </div>
      )}

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col gap-3">
          <div className="flex min-h-0 flex-col lg:flex-1">
            <h2 className="mb-2 text-base font-semibold">Currently reading</h2>
            {!libLoading && reading.length === 0 && (
              <p className="text-muted text-sm">No books in progress.</p>
            )}
            {reading.length > 0 && (
              <div className="flex min-h-0 flex-wrap content-start gap-3 overflow-y-auto pr-1 lg:flex-1">
                {reading.map((ub) => (
                  <ReadingBookCard key={ub.id} userBook={ub} />
                ))}
              </div>
            )}
          </div>
          <div className="shrink-0 lg:max-h-56 lg:overflow-y-auto">
            <SubscribedFeedsCard />
          </div>
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
                    value={progressStart}
                    onChange={setProgressStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="books-dash-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="books-dash-to"
                    value={progressEnd}
                    onChange={setProgressEnd}
                    className="h-9 w-40"
                  />
                </div>
              </div>
            )}
          </div>

          {view === 'ytd' && (
            <>
              {!libLoading && ytd.series.length === 0 && (
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
