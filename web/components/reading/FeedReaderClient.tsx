'use client'

import { useCallback, useMemo, useState } from 'react'
import { useLibrary } from '@/hooks/useBooks'
import { useFeedItemBooks } from '@/hooks/useBookFeeds'
import ArticleReaderDialog from '@/components/reading/ArticleReaderDialog'
import BookCover from '@/components/reading/BookCover'
import { Button } from '@/components/ui/button'
import { formatDate } from '@/lib/dates'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'

export default function FeedReaderClient() {
  const { data: libraryData, error: libraryError, isLoading: libraryLoading } = useLibrary()
  const { data: feedItemsData } = useFeedItemBooks()
  const [dismissed, setDismissed] = useState<Set<string>>(new Set())

  const feedTitleByBookId = useMemo(() => {
    const map = new Map<string, string>()
    for (const item of feedItemsData?.items ?? []) {
      map.set(item.bookId, item.feedTitle)
    }
    return map
  }, [feedItemsData])

  const unread = useMemo(() => {
    const rss = libraryData?.library?.rss ?? []
    return rss
      .filter((ub) => ub.status !== 'read' && !dismissed.has(ub.bookId))
      .sort((a, b) => (a.addedAt < b.addedAt ? 1 : -1))
  }, [libraryData, dismissed])

  const handleSettled = useCallback((bookId: string) => {
    setDismissed((prev) => new Set(prev).add(bookId))
  }, [])

  if (libraryLoading) return <p className="text-muted">Loading…</p>
  if (libraryError) return <p className="text-danger">Failed to load feed items.</p>

  if (unread.length === 0) {
    return <p className="py-16 text-center text-sm text-muted">No unread feed items.</p>
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {unread.map((userBook) => (
        <FeedReaderCard
          key={userBook.id}
          userBook={userBook}
          feedTitle={feedTitleByBookId.get(userBook.bookId)}
          onSettled={handleSettled}
        />
      ))}
    </div>
  )
}

interface FeedReaderCardProps {
  userBook: UserBook
  feedTitle?: string
  onSettled: (bookId: string) => void
}

function FeedReaderCard({ userBook, feedTitle, onSettled }: FeedReaderCardProps) {
  const book = userBook.book
  const [readerOpen, setReaderOpen] = useState(false)
  if (!book) return null

  const noContent = !book.hasContent

  return (
    <div className="flex flex-col gap-2 rounded-2xl border border-border bg-card p-3 shadow-card">
      <div className="flex items-start gap-3">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="thumb" />
        <div className="min-w-0 flex-1">
          <Button
            type="button"
            variant="link"
            onClick={() => setReaderOpen(true)}
            className="h-auto p-0 font-semibold text-sm leading-snug text-fg no-underline hover:text-accent"
          >
            {book.title}
          </Button>
          {feedTitle && <p className="text-xs text-muted">{feedTitle}</p>}
          <p className="text-xs text-muted">{formatDate(userBook.addedAt)}</p>
          {noContent && <p className="text-xs text-subtle">No in-app content</p>}
        </div>
      </div>

      <ArticleReaderDialog
        bookId={book.id}
        title={book.title}
        sourceUrl={book.sourceUrl}
        open={readerOpen}
        onOpenChange={setReaderOpen}
        userBook={userBook}
        onSettled={onSettled}
      />
    </div>
  )
}
