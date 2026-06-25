'use client'

import Link from 'next/link'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCover from '@/components/backlog/BookCover'
import BookProgressBar from '@/components/backlog/BookProgressBar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'

export type BookActionKind = 'entry' | 'shelf' | 'progress'

function OwnershipBadges({ userBook }: { userBook: UserBook }) {
  const physical = userBook.tags.includes('own-physical')
  const digital = userBook.tags.includes('own-digital')
  const hasPdf = userBook.formats.includes('pdf')
  const hasEpub = userBook.formats.includes('epub')

  if (!physical && !digital && !hasPdf && !hasEpub) return null

  return (
    <div className="flex items-center gap-1 flex-wrap mt-1">
      {physical && <Badge variant="secondary">Physical</Badge>}
      {digital && <Badge variant="secondary">Digital</Badge>}
      {hasPdf && <Badge variant="default">PDF</Badge>}
      {hasEpub && <Badge variant="default">EPUB</Badge>}
    </div>
  )
}

interface BookCardProps {
  userBook: UserBook
  onAction: (kind: BookActionKind, ub: UserBook) => void
}

export default function BookCard({ userBook, onAction }: BookCardProps) {
  const book = userBook.book
  if (!book) return null

  return (
    <div className={cn(interactiveCardClass, 'relative p-3 flex gap-3 items-start')}>
      {/* Stretched link covers the whole card; action buttons sit above it via z-10 */}
      <Link
        href={`/backlog/books/${userBook.id}`}
        className="absolute inset-0 rounded-2xl"
        aria-label={book.title}
      />
      <div className="relative z-10 shrink-0">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      </div>
      <div className="relative z-10 flex-1 min-w-0">
        <h3 className="font-semibold text-sm leading-snug">{book.title}</h3>
        <p className="text-xs text-muted">{book.authors.join(', ')}</p>
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {userBook.status.replace(/-/g, ' ')}
          </span>
          {userBook.rating > 0 && <span className="text-xs text-muted">{userBook.rating}★</span>}
          {userBook.tags.includes('favourite') && <span className="text-xs text-amber-500">♥</span>}
        </div>
        <OwnershipBadges userBook={userBook} />
        {userBook.status === 'currently-reading' && (
          <div className="mt-2">
            <BookProgressBar userBook={userBook} />
          </div>
        )}
      </div>
      <div className="relative z-10 shrink-0 flex flex-col gap-1 items-end">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onAction('entry', userBook)}
          className="text-xs"
        >
          Entry
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onAction('shelf', userBook)}
          className="text-xs"
        >
          Shelf
        </Button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onAction('progress', userBook)}
          className="text-xs"
        >
          Progress
        </Button>
      </div>
    </div>
  )
}

export { OwnershipBadges }
