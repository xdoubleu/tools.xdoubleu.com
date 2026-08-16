'use client'

import Link from 'next/link'
import {
  useSharedLibrary,
  useSharedBooksProgress,
  useSharedFeedsSummary
} from '@/hooks/useDashboardShare'
import type { GetSharedLibraryResponse } from '@/lib/gen/dashboard/v1/reading_pb'
import DashboardBookCard from '@/components/dashboard/DashboardBookCard'
import ReadingDashboardLayout from '@/components/dashboard/ReadingDashboardLayout'
import SharedFeedsCard from '@/components/dashboard/SharedFeedsCard'
import { Button } from '@/components/ui/button'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { formatDateTime } from '@/lib/dates'

export default function ReadingDashboardPublicClient({
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
  const { data: feedsSummaryData } = useSharedFeedsSummary(token)
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
    <ReadingDashboardLayout
      library={library}
      chart={chart}
      allTimeChartData={allTimeChartData}
      renderReadingCard={(ub) => (
        <div className="w-full sm:w-72">
          <DashboardBookCard userBook={ub} />
        </div>
      )}
      feedsCard={<SharedFeedsCard feeds={feedsSummaryData?.feeds} />}
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
            <Link href={`/dashboard/reading/${token}/library`}>Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
