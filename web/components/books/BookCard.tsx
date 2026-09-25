'use client'

import Link from 'next/link'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
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
  const href = query
    ? `/books/${userBook.id}?q=${encodeURIComponent(query)}`
    : `/books/${userBook.id}`

  return (
    <div className={cn(interactiveCardClass, 'relative p-3 flex gap-3 items-start')}>
      {/* Stretched link; interactive controls sit above it via z-10 */}
      <Link href={href} className="absolute inset-0 rounded-2xl" aria-label={book.title}>
        <CardLinkStatus />
      </Link>
      <div className="shrink-0">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      </div>
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold text-sm leading-snug truncate">{book.title}</h3>
        <p className="text-xs text-muted truncate">{book.authors.join(', ')}</p>

        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {userBook.status.replace(/-/g, ' ')}
          </span>

          {isRead && (
            <div className="relative z-10 flex items-center gap-2">
              <BookRatingStars userBook={userBook} readOnly />
              <BookFavouriteButton userBook={userBook} onSaved={onSaved} />
            </div>
          )}
        </div>

        <div className="relative z-10">
          <BookOwnershipToggles userBook={userBook} onSaved={onSaved} />
        </div>

        {userBook.status === 'currently-reading' && (
          <div className="relative z-10 mt-2">
            <BookProgressEditor userBook={userBook} onSaved={onSaved} />
          </div>
        )}
      </div>

      {displayTags(userBook.tags).length > 0 && (
        <div className="shrink-0 text-right text-xs text-muted max-w-24 truncate">
          {displayTags(userBook.tags).join(', ')}
        </div>
      )}
    </div>
  )
}
