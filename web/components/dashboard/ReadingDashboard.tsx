'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useLibrary, useBooksProgress } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookSearchBar from '@/components/books/BookSearchBar'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import MarkAsCompletedDialog from '@/components/books/MarkAsCompletedDialog'
import ReadingDashboardLayout from '@/components/dashboard/ReadingDashboardLayout'
import DashboardShareButton from '@/components/dashboard/DashboardShareButton'
import { Button } from '@/components/ui/button'
import { LinkCard } from '@/components/ui/link-card'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'

function ReadingBookCard({ userBook }: { userBook: UserBook }) {
  const [completing, setCompleting] = useState(false)
  const book = userBook.book
  if (!book) return null
  return (
    <LinkCard
      href={`/books/${userBook.id}`}
      aria-label={book.title}
      className="w-full self-start sm:w-72"
      linkClassName="flex gap-3 p-4"
      actions={
        <>
          <BookProgressEditor
            userBook={userBook}
            actions={
              <Button variant="secondary" size="sm" onClick={() => setCompleting(true)}>
                Mark as completed
              </Button>
            }
          />
          {/* Outside the link: portalled dialog events still bubble through React. */}
          <MarkAsCompletedDialog
            userBook={userBook}
            open={completing}
            onOpenChange={setCompleting}
          />
        </>
      }
    >
      <BookCover coverUrl={book.coverUrl} title={book.title} size="md" />
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold break-words">{book.title}</h3>
        <p className="text-sm text-muted truncate">{book.authors.join(', ')}</p>
      </div>
    </LinkCard>
  )
}

// ReadingDashboard is the owner's private books+feeds dashboard.
export default function ReadingDashboard() {
  const chart = useDashboardChartState<'ytd' | 'all'>('ytd')

  const { data: libraryData, error: libError, isLoading: libLoading } = useLibrary()
  const { data: progressData } = useBooksProgress(
    chart.view === 'all' ? chart.start : undefined,
    chart.view === 'all' ? chart.end : undefined
  )

  const library = libraryData?.library
  const allTimeChartData =
    progressData?.progress?.labels?.map((label: string, idx: number) => ({
      label,
      value: parseInt(progressData.progress?.values?.[idx] ?? '0', 10)
    })) ?? []

  if (libLoading && !library) return <LoadingState label="dashboard" />
  if (libError && !library) return <ErrorState what="books" />
  if (!library) return null

  return (
    <ReadingDashboardLayout
      library={library}
      chart={chart}
      allTimeChartData={allTimeChartData}
      renderReadingCard={(ub) => <ReadingBookCard userBook={ub} />}
      // Feeds are hidden from the reading dashboard for now.
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
