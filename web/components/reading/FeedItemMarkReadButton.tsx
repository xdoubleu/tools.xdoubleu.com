'use client'

import { useEffect, useRef, useState } from 'react'
import { mutate } from 'swr'
import { Button } from '@/components/ui/button'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import { swrKeys } from '@/lib/swrKeys'

const UNDO_WINDOW_MS = 4000

interface FeedItemMarkReadButtonProps {
  userBook: UserBook
  /** Called once the undo window has elapsed without the user reverting. */
  onSettled: (bookId: string) => void
}

// FeedItemMarkReadButton marks an rss item read with a brief Undo window
// (issue #476) — same optimistic-toggle shape as BookFavouriteButton, but
// toggling `status` (read/unread) instead of the favourite tag.
export default function FeedItemMarkReadButton({
  userBook,
  onSettled
}: FeedItemMarkReadButtonProps) {
  const [justRead, setJustRead] = useState(false)
  const updateBookStatus = useUpdateBookStatus()
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current)
    }
  }, [])

  const save = async (status: string) => {
    await updateBookStatus({
      bookId: userBook.bookId,
      status,
      favourite: userBook.tags.includes('favourite'),
      rating: String(userBook.rating)
    })
    mutate(swrKeys.books)
  }

  const handleMarkRead = async () => {
    setJustRead(true)
    try {
      await save('read')
      timeoutRef.current = setTimeout(() => onSettled(userBook.bookId), UNDO_WINDOW_MS)
    } catch {
      setJustRead(false)
    }
  }

  const handleUndo = async () => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current)
    setJustRead(false)
    try {
      await save(userBook.status)
    } catch {
      // Best-effort: the row already reverted locally.
    }
  }

  if (justRead) {
    return (
      <span className="flex items-center gap-1 text-xs text-muted whitespace-nowrap">
        Marked as read
        <Button variant="link" size="sm" className="h-auto p-0 text-xs" onClick={handleUndo}>
          Undo
        </Button>
      </span>
    )
  }

  return (
    <Button variant="ghost" size="sm" onClick={handleMarkRead}>
      Mark read
    </Button>
  )
}
