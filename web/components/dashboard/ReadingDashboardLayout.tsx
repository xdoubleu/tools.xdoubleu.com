'use client'

import type { ReactNode } from 'react'
import type { LibraryResponse, UserBook } from '@/lib/gen/books/v1/library_pb'
import BooksDashboardView from '@/components/books/BooksDashboardView'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

// Shared by the private and public reading dashboards so they can't drift;
// feedsCard is a slot since each shows different feeds data.
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
