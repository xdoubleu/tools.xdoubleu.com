'use client'

import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import { Badge } from '@/components/ui/badge'
import { LinkCard } from '@/components/ui/link-card'
import { displayTags } from '@/lib/books/bookShelves'

interface BookCardProps {
  userBook: UserBook
  onSaved: () => void
  /** Carried into the detail link so the breadcrumb can restore it. */
  query?: string
}

export default function BookCard({ userBook, onSaved, query }: BookCardProps) {
  const book = userBook.book
  if (!book) return null

  const isRead = userBook.status === 'read'
  const isReading = userBook.status === 'currently-reading'
  const href = query
    ? `/books/${userBook.id}?q=${encodeURIComponent(query)}`
    : `/books/${userBook.id}`
  const tags = displayTags(userBook.tags)

  return (
    <LinkCard
      href={href}
      aria-label={book.title}
      linkClassName="flex items-start gap-3"
      actions={
        <div className="w-full min-w-0 space-y-2">
          {isRead && (
            <div className="flex items-center gap-2">
              <BookRatingStars userBook={userBook} readOnly />
              <BookFavouriteButton userBook={userBook} onSaved={onSaved} />
            </div>
          )}
          <BookOwnershipToggles userBook={userBook} onSaved={onSaved} hideLabel />
          {isReading && <BookProgressEditor userBook={userBook} onSaved={onSaved} />}
        </div>
      }
    >
      <div className="shrink-0">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      </div>
      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-semibold leading-snug">{book.title}</h3>
        <p className="truncate text-xs text-muted">{book.authors.join(', ')}</p>
        <div className="mt-1 flex flex-wrap items-center gap-2">
          <Badge variant="secondary" className="capitalize">
            {userBook.status.replace(/-/g, ' ')}
          </Badge>
          {tags.length > 0 && (
            <span className="min-w-0 truncate text-xs text-muted">{tags.join(', ')}</span>
          )}
        </div>
      </div>
    </LinkCard>
  )
}
