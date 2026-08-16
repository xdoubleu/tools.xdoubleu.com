'use client'

import type { ReactNode } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import BooksDashboardView from '@/components/books/BooksDashboardView'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

// ReadingDashboardLayout composes the presentational BooksDashboardView
// (library + progress chart) with the feeds widget — shared by the private
// (ReadingDashboard) and public (ReadingDashboardPublicClient) wrappers so
// the merged books+feeds "reading dashboard" (issue #737) can't drift
// between the two. feedsCard is a slot rather than a fixed component/prop
// shape: the private view shows an unread-items digest, the public view
// shows a plain subscribed-feeds list — genuinely different data.
export default function ReadingDashboardLayout({
  library,
  chart,
  allTimeChartData,
  renderReadingCard,
  actions,
  feedsCard
}: {
  library: LibraryResponse
  chart: DashboardChartState<'ytd' | 'all'>
  allTimeChartData: { label: string; value: number }[]
  renderReadingCard: (ub: UserBook) => ReactNode
  actions: ReactNode
  feedsCard?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      {feedsCard}
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
