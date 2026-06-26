'use client'

import Link from 'next/link'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCover from '@/components/backlog/BookCover'
import BookProgressEditor from '@/components/backlog/BookProgressEditor'
import BookRatingStars from '@/components/backlog/BookRatingStars'
import BookFavouriteButton from '@/components/backlog/BookFavouriteButton'
import BookOwnershipToggles from '@/components/backlog/BookOwnershipToggles'
import BookShelfPopover from '@/components/backlog/BookShelfPopover'
import { interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'

interface BookCardProps {
  userBook: UserBook
  knownShelves: string[]
  knownTags: string[]
  onSaved: () => void
}

export default function BookCard({ userBook, knownShelves, knownTags, onSaved }: BookCardProps) {
  const book = userBook.book
  if (!book) return null

  const isRead = userBook.status === 'read'

  return (
    <div className={cn(interactiveCardClass, 'relative p-3 flex gap-3 items-start')}>
      {/* Stretched link covers the whole card; interactive controls sit above it via z-10 */}
      <Link
        href={`/backlog/books/${userBook.id}`}
        className="absolute inset-0 rounded-2xl"
        aria-label={book.title}
      />
      {/* Cover sits below the link — clicking it navigates */}
      <div className="shrink-0">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      </div>
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold text-sm leading-snug">{book.title}</h3>
        <p className="text-xs text-muted">{book.authors.join(', ')}</p>

        {/* Status pill — non-interactive, navigates with the card */}
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {userBook.status.replace(/-/g, ' ')}
          </span>

          {/* Rating + favourite — only for finished books; lifted above link via z-10 */}
          {isRead && (
            <div className="relative z-10 flex items-center gap-2">
              <BookRatingStars userBook={userBook} onSaved={onSaved} />
              <BookFavouriteButton userBook={userBook} onSaved={onSaved} />
            </div>
          )}
        </div>

        {/* Ownership / format chips */}
        <div className="relative z-10">
          <BookOwnershipToggles userBook={userBook} onSaved={onSaved} />
        </div>

        {/* Progress bar (currently-reading only) */}
        {userBook.status === 'currently-reading' && (
          <div className="relative z-10 mt-2">
            <BookProgressEditor userBook={userBook} onSaved={onSaved} />
          </div>
        )}
      </div>

      {/* Status + shelves/tags popover */}
      <div className="relative z-10 shrink-0">
        <BookShelfPopover
          userBook={userBook}
          knownShelves={knownShelves}
          knownTags={knownTags}
          onSaved={onSaved}
        />
      </div>
    </div>
  )
}
