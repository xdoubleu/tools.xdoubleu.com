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
 * Library-table "Progress" column cell — lets a currently-reading book's
 * progress be updated inline from the table row, without navigating to the
 * book detail page. Uses the shared Popover primitive (as the "Shelf & tags"
 * column does) so the edit form portals out of the table's overflow-x-auto
 * wrapper and stays positioned within the viewport on mobile.
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
