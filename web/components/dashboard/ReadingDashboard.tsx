'use client'

import Link from 'next/link'
import { useLibrary, useBooksProgress } from '@/hooks/useBooks'
import { useFeedsSummary } from '@/hooks/useFeeds'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookSearchBar from '@/components/books/BookSearchBar'
import BookProgressBar from '@/components/books/BookProgressBar'
import ReadingDashboardLayout from '@/components/dashboard/ReadingDashboardLayout'
import DashboardShareButton from '@/components/dashboard/DashboardShareButton'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'

function ReadingBookCard({ userBook }: { userBook: UserBook }) {
  const book = userBook.book
  if (!book) return null
  return (
    <Link
      href={`/books/${userBook.id}`}
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

// ReadingDashboard is the owner's private dashboard for books+feeds merged
// into one "reading" view (issue #737) — was BooksDashboard, renamed now
// that it also surfaces a feeds summary.
export default function ReadingDashboard() {
  const chart = useDashboardChartState<'ytd' | 'all'>('ytd')

  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const { data: progressData } = useBooksProgress(
    chart.view === 'all' ? chart.start : undefined,
    chart.view === 'all' ? chart.end : undefined
  )
  const { data: feedsSummary } = useFeedsSummary()

  const library = libraryData?.library
  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

  if (libLoading && !library) return <p className="text-muted">Loading dashboard…</p>
  if (libError && !library) return <p className="text-danger">Failed to load books.</p>
  if (!library) return null

  return (
    <ReadingDashboardLayout
      library={library}
      chart={chart}
      allTimeChartData={allTimeChartData}
      renderReadingCard={(ub) => <ReadingBookCard userBook={ub} />}
      feedsSummary={feedsSummary}
      feedsHref="/feeds"
      actions={
        <>
          <div className="mr-auto w-full max-w-md">
            <BookSearchBar />
          </div>
          <DashboardShareButton kind="reading" />
          <Button asChild variant="secondary">
            <Link href="/books/library">Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
