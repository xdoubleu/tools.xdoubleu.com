'use client'

import { useCallback, useMemo, useState } from 'react'
import { useLibrary } from '@/hooks/useBooks'
import { useFeedItemBooks } from '@/hooks/useBookFeeds'
import FeedItemMarkReadButton from '@/components/reading/FeedItemMarkReadButton'
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
    <ul className="flex flex-col gap-2">
      {unread.map((userBook) => (
        <FeedReaderRow
          key={userBook.id}
          userBook={userBook}
          feedTitle={feedTitleByBookId.get(userBook.bookId)}
          onSettled={handleSettled}
        />
      ))}
    </ul>
  )
}

interface FeedReaderRowProps {
  userBook: UserBook
  feedTitle?: string
  onSettled: (bookId: string) => void
}

function FeedReaderRow({ userBook, feedTitle, onSettled }: FeedReaderRowProps) {
  const book = userBook.book
  if (!book) return null

  return (
    <li className="flex items-center gap-3 rounded-2xl border border-border bg-card p-3 shadow-card">
      <div className="min-w-0 flex-1">
        {book.sourceUrl ? (
          <a
            href={book.sourceUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="font-semibold text-sm leading-snug hover:text-accent"
          >
            {book.title}
          </a>
        ) : (
          <span className="font-semibold text-sm leading-snug">{book.title}</span>
        )}
        {feedTitle && <p className="text-xs text-muted">{feedTitle}</p>}
      </div>

      <FeedItemMarkReadButton userBook={userBook} onSettled={onSettled} />
    </li>
  )
}
