'use client'

import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Popover, PopoverTrigger } from '@/components/ui/popover'
import BookShelfTagFields from '@/components/books/BookShelfTagFields'
import { displayTags } from '@/lib/books/bookShelves'

interface BookShelfPopoverProps {
  userBook: UserBook
  knownShelves: string[]
  knownTags: string[]
  onSaved?: () => void
}

export default function BookShelfPopover({
  userBook,
  knownShelves,
  knownTags,
  onSaved
}: BookShelfPopoverProps) {
  const tagCount = displayTags(userBook.tags).length
  const triggerLabel = tagCount > 0 ? `Shelves & tags (${tagCount})` : 'Shelves & tags'

  return (
    <Popover
      align="right"
      trigger={({ open, onClick }) => (
        <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Edit shelves and tags">
          {triggerLabel}
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
