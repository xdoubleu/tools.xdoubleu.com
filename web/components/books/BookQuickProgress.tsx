'use client'

import { useState } from 'react'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressForm from '@/components/books/BookProgressForm'
import { Button } from '@/components/ui/button'

/**
 * Reading progress display: the progress bar, tapped to open the exact-entry
 * `BookProgressForm`. Built for the reading dashboard's currently-reading
 * cards, where a trip to the detail page (especially on mobile) is the main
 * cost of correcting progress.
 */
export default function BookQuickProgress({
  userBook,
  onSaved
}: {
  userBook: UserBook
  onSaved?: () => void
}) {
  const [editing, setEditing] = useState(false)

  if (editing) {
    return (
      <BookProgressForm userBook={userBook} onSaved={onSaved} onClose={() => setEditing(false)} />
    )
  }

  return (
    <Button
      variant="ghost"
      onClick={() => setEditing(true)}
      aria-label="Edit reading progress"
      className="-my-2 h-auto w-full min-w-0 justify-start rounded-lg px-0 py-2 text-left hover:bg-transparent"
    >
      <BookProgressBar userBook={userBook} />
    </Button>
  )
}
