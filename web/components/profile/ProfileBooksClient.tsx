'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useSharedLibrary, useSharedBooksProgress, useSharedFeeds } from '@/hooks/useProfile'
import type { GetSharedLibraryResponse } from '@/lib/gen/reading/v1/public_pb'
import ProfileBookCard from '@/components/profile/ProfileBookCard'
import BooksProgressChart from '@/components/reading/BooksProgressChart'
import FeedList from '@/components/reading/FeedList'
import GamesStatCard from '@/components/games/GamesStatCard'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { formatDateTime, oneYearAgo, today } from '@/lib/dates'
import { ytdProgress } from '@/lib/reading/ytdProgress'
import { statusLabel } from '@/lib/reading/bookShelves'

export default function ProfileBooksClient({
  token,
  initialData
}: {
  token: string
  initialData?: GetSharedLibraryResponse
}) {
  const { data, error, isLoading } = useSharedLibrary(token, initialData)

  const [view, setView] = useState<'ytd' | 'all'>('ytd')
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())
  const { data: progressData } = useSharedBooksProgress(
    view === 'all' ? token : '',
    progressStart,
    progressEnd
  )
  const { data: feedsData, error: feedsError, isLoading: feedsLoading } = useSharedFeeds(token)
  const feeds = feedsData?.feeds ?? []

  const library = data?.library
  const reading = library?.reading ?? []
  const rss = library?.rss ?? []
  const rssRead = rss.filter((ub) => ub.status === 'read').length
  const ytd = ytdProgress(library?.finished ?? [])

  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

  if (isLoading && !library) return <p className="text-muted">Loading books…</p>
  if (error && !library) return <p className="text-danger">Failed to load books.</p>
  if (!library) return null

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {data?.lastSyncedAt ? (
          <p className="text-xs text-muted">Last synced: {formatDateTime(data.lastSyncedAt)}</p>
        ) : (
          <span />
        )}
        <Button asChild variant="secondary">
          <Link href={`/profile/reading/${token}/library`}>Browse full library</Link>
        </Button>
      </div>

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

      {feeds.length > 0 && (
        <Card className="flex min-h-0 flex-col p-4">
          <h2 className="mb-2 text-base font-semibold">Subscribed feeds</h2>
          <FeedList feeds={feeds} isLoading={feedsLoading} error={feedsError} />
        </Card>
      )}

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <h2 className="mb-2 text-base font-semibold">Currently reading</h2>
          {reading.length === 0 && <p className="text-muted text-sm">No books in progress.</p>}
          {reading.length > 0 && (
            <div className="flex min-h-0 flex-wrap content-start gap-3 overflow-y-auto pr-1 lg:flex-1">
              {reading.map((ub) => (
                <div key={ub.id} className="w-full sm:w-72">
                  <ProfileBookCard userBook={ub} />
                </div>
              ))}
            </div>
          )}
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
                  <label htmlFor="profile-books-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="profile-books-from"
                    value={progressStart}
                    onChange={setProgressStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="profile-books-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="profile-books-to"
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
