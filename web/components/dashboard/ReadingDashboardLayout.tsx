'use client'

import type { ReactNode } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import BooksDashboardView from '@/components/books/BooksDashboardView'
import ReadingFeedsSummaryCard from '@/components/dashboard/ReadingFeedsSummaryCard'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'
import type { FeedsSummary } from '@/hooks/useFeeds'

// ReadingDashboardLayout composes the presentational BooksDashboardView
// (library + progress chart) with the feeds summary widget — shared by the
// private (ReadingDashboard) and public (ReadingDashboardPublicClient)
// wrappers so the merged books+feeds "reading dashboard" (issue #737) can't
// drift between the two.
export default function ReadingDashboardLayout({
  library,
  chart,
  allTimeChartData,
  renderReadingCard,
  actions,
  feedsSummary,
  feedsHref
}: {
  library: LibraryResponse
  chart: DashboardChartState<'ytd' | 'all'>
  allTimeChartData: { label: string; value: number }[]
  renderReadingCard: (ub: UserBook) => ReactNode
  actions: ReactNode
  feedsSummary?: FeedsSummary
  feedsHref?: string
}) {
  return (
    <div className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <ReadingFeedsSummaryCard summary={feedsSummary} href={feedsHref} />
      <div className="lg:min-h-0 lg:flex-1">
        <BooksDashboardView
          library={library}
          chart={chart}
          allTimeChartData={allTimeChartData}
          renderReadingCard={renderReadingCard}
          actions={actions}
        />
      </div>
    </div>
  )
}
