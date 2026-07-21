'use client'

import Link from 'next/link'
import { useSharedLibrary, useSharedBooksProgress, useSharedFeeds } from '@/hooks/useProfile'
import type { GetSharedLibraryResponse } from '@/lib/gen/reading/v1/public_pb'
import ProfileBookCard from '@/components/profile/ProfileBookCard'
import BooksDashboardView from '@/components/reading/BooksDashboardView'
import FeedList from '@/components/reading/FeedList'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { formatDateTime } from '@/lib/dates'

export default function ProfileBooksClient({
  token,
  initialData
}: {
  token: string
  initialData?: GetSharedLibraryResponse
}) {
  const chart = useDashboardChartState<'ytd' | 'all'>('ytd')

  const { data, error, isLoading } = useSharedLibrary(token, initialData)
  const { data: progressData } = useSharedBooksProgress(
    chart.view === 'all' ? token : '',
    chart.start,
    chart.end
  )
  const { data: feedsData, error: feedsError, isLoading: feedsLoading } = useSharedFeeds(token)
  const feeds = feedsData?.feeds ?? []

  const library = data?.library
  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

  if (isLoading && !library) return <p className="text-muted">Loading books…</p>
  if (error && !library) return <p className="text-danger">Failed to load books.</p>
  if (!library) return null

  return (
    <BooksDashboardView
      library={library}
      chart={chart}
      allTimeChartData={allTimeChartData}
      renderReadingCard={(ub) => (
        <div className="w-full sm:w-72">
          <ProfileBookCard userBook={ub} />
        </div>
      )}
      feedsSlot={
        feeds.length > 0 ? (
          <Card className="flex min-h-0 flex-col p-4">
            <h2 className="mb-2 text-base font-semibold">Subscribed feeds</h2>
            <FeedList feeds={feeds} isLoading={feedsLoading} error={feedsError} />
          </Card>
        ) : null
      }
      actions={
        <>
          {data?.lastSyncedAt ? (
            <p className="mr-auto text-xs text-muted">
              Last synced: {formatDateTime(data.lastSyncedAt)}
            </p>
          ) : (
            <span className="mr-auto" />
          )}
          <Button asChild variant="secondary">
            <Link href={`/profile/reading/${token}/library`}>Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
