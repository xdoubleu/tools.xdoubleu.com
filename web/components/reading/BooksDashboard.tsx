'use client'

import { useState } from 'react'
import Link from 'next/link'
import { mutate } from 'swr'
import { useLibrary, useBooksProgress } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import BookCover from '@/components/reading/BookCover'
import BookSearchBar from '@/components/reading/BookSearchBar'
import BookProgressBar from '@/components/reading/BookProgressBar'
import BooksDashboardView from '@/components/reading/BooksDashboardView'
import AddToLibraryDialog from '@/components/reading/AddToLibraryDialog'
import SubscribedFeedsCard from '@/components/reading/SubscribedFeedsCard'
import ProfileShareButton from '@/components/profile/ProfileShareButton'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { swrKeys } from '@/lib/swrKeys'

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
  const [addOpen, setAddOpen] = useState(false)
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

  const handleRefresh = () => {
    void mutate(swrKeys.books)
  }

  if (libLoading && !library) return <p className="text-muted">Loading dashboard…</p>
  if (libError && !library) return <p className="text-danger">Failed to load books.</p>
  if (!library) return null

  return (
    <>
      <AddToLibraryDialog open={addOpen} onOpenChange={setAddOpen} onAdded={handleRefresh} />
      <BooksDashboardView
        library={library}
        chart={chart}
        allTimeChartData={allTimeChartData}
        renderReadingCard={(ub) => <ReadingBookCard userBook={ub} />}
        feedsSlot={<SubscribedFeedsCard />}
        actions={
          <>
            <div className="mr-auto w-full max-w-md">
              <BookSearchBar onAdded={handleRefresh} />
            </div>
            <Button onClick={() => setAddOpen(true)}>Add to library</Button>
            <ProfileShareButton app="reading" />
            <Button asChild variant="secondary">
              <Link href="/reading/library">Browse full library</Link>
            </Button>
          </>
        }
      />
    </>
  )
}
