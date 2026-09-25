'use client'

import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookProgressBar from '@/components/books/BookProgressBar'
import BookProgressForm from '@/components/books/BookProgressForm'
import { Popover, PopoverTrigger } from '@/components/ui/popover'

interface BookProgressCellProps {
  userBook: UserBook
  onSaved?: () => void
}

/**
 * Inline progress editor for the library table. The Popover portals out of
 * the table's overflow wrapper so it stays on-screen on mobile.
 */
export default function BookProgressCell({ userBook, onSaved }: BookProgressCellProps) {
  if (userBook.status !== 'currently-reading') return null

  return (
    <Popover
      align="left"
      trigger={({ onClick }) => (
        <PopoverTrigger
          onClick={onClick}
          className="block w-full text-left px-1 py-1.5 -mx-1"
          aria-label={`Edit reading progress for ${userBook.book?.title ?? 'book'}`}
        >
          <BookProgressBar userBook={userBook} />
        </PopoverTrigger>
      )}
    >
      <BookProgressForm userBook={userBook} onSaved={onSaved} />
    </Popover>
  )
}
