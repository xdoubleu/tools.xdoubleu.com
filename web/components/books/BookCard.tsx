'use client'

import { useId, useState } from 'react'
import Link from 'next/link'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import BookProgressEditor from '@/components/books/BookProgressEditor'
import BookRatingStars from '@/components/books/BookRatingStars'
import BookFavouriteButton from '@/components/books/BookFavouriteButton'
import BookOwnershipToggles from '@/components/books/BookOwnershipToggles'
import BookDescription from '@/components/books/BookDescription'
import OfflineBookBadge from '@/components/books/OfflineBookBadge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { LinkCard } from '@/components/ui/link-card'
import { displayTags } from '@/lib/books/bookShelves'
import { readerFormats } from '@/lib/books/readerSettings'

interface BookCardProps {
  userBook: UserBook
  onSaved?: () => void
  /** Carried into the detail link so the breadcrumb can restore it. */
  query?: string
  /** Adds a button that expands the description inside the card. */
  showDescriptionToggle?: boolean
}

export default function BookCard({
  userBook,
  onSaved,
  query,
  showDescriptionToggle = false
}: BookCardProps) {
  const [descriptionOpen, setDescriptionOpen] = useState(false)
  const descriptionId = useId()
  const book = userBook.book
  if (!book) return null

  const isRead = userBook.status === 'read'
  const isReading = userBook.status === 'currently-reading'
  const href = query
    ? `/books/${userBook.id}?q=${encodeURIComponent(query)}`
    : `/books/${userBook.id}`
  const tags = displayTags(userBook.tags)
  const canPeek = showDescriptionToggle && book.description !== ''
  const canRead = readerFormats(userBook.formats).length > 0

  return (
    <LinkCard
      href={href}
      aria-label={book.title}
      linkClassName="flex items-start gap-3"
      actions={
        <div className="w-full min-w-0 space-y-2">
          {canRead && (
            <Button asChild variant="default" className="w-full" size="sm">
              <Link href={`/books/${userBook.id}/read`}>
                {isReading ? 'Continue reading' : 'Read'}
              </Link>
            </Button>
          )}
          {isRead && (
            <div className="flex items-center gap-2">
              <BookRatingStars userBook={userBook} readOnly />
              <BookFavouriteButton userBook={userBook} onSaved={onSaved} />
            </div>
          )}
          <BookOwnershipToggles userBook={userBook} onSaved={onSaved} hideLabel />
          {isReading && <BookProgressEditor userBook={userBook} onSaved={onSaved} />}
          {canPeek && (
            <>
              <Button
                type="button"
                variant="link"
                className="text-sm"
                aria-expanded={descriptionOpen}
                aria-controls={descriptionId}
                onClick={() => setDescriptionOpen((open) => !open)}
              >
                {descriptionOpen ? 'Hide description' : 'Show description'}
              </Button>
              {descriptionOpen && (
                <BookDescription id={descriptionId} description={book.description} />
              )}
            </>
          )}
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
          {userBook.tags.includes('own-bol') && (
            <Badge variant="secondary">bol.com</Badge>
          )}
          <OfflineBookBadge bookId={userBook.bookId} />
          {tags.length > 0 && (
            <span className="min-w-0 truncate text-xs text-muted">{tags.join(', ')}</span>
          )}
        </div>
      </div>
    </LinkCard>
  )
}
