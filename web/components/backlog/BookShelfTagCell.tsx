'use client'

import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { Popover, PopoverTrigger } from '@/components/ui/popover'
import BookShelfTagFields from '@/components/backlog/BookShelfTagFields'
import { statusLabel, displayTags } from '@/lib/backlog/bookShelves'

interface BookShelfTagCellProps {
  userBook: UserBook
  /** All known shelf names for the radio list (custom + built-in). */
  knownShelves: string[]
  /** All known tag names for the checkbox list. */
  knownTags: string[]
  onSaved?: () => void
}

export default function BookShelfTagCell({
  userBook,
  knownShelves,
  knownTags,
  onSaved
}: BookShelfTagCellProps) {
  const shelfDisplay = statusLabel(userBook.status)
  const tagCount = displayTags(userBook.tags).length

  return (
    <Popover
      align="right"
      trigger={({ open, onClick }) => (
        <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Edit shelf and tags">
          <span className="text-sm">{shelfDisplay}</span>
          {tagCount > 0 && <span className="ml-1 text-xs text-muted">+{tagCount}</span>}
        </PopoverTrigger>
      )}
    >
      <BookShelfTagFields
        userBook={userBook}
        knownShelves={knownShelves}
        knownTags={knownTags}
        onSaved={onSaved}
      />
    </Popover>
  )
}
