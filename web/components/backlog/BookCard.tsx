'use client'

import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import BookCover from '@/components/backlog/BookCover'
import BookProgressBar from '@/components/backlog/BookProgressBar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

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
  onEdit: (ub: UserBook) => void
}

export default function BookCard({ userBook, onEdit }: BookCardProps) {
  const book = userBook.book
  if (!book) return null

  return (
    <div className="rounded-2xl border border-border bg-card shadow-card p-4 flex gap-4">
      <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold">{book.title}</h3>
        <p className="text-sm text-muted">{book.authors.join(', ')}</p>
        <div className="flex items-center gap-2 mt-1 flex-wrap">
          <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-subtle capitalize">
            {userBook.status}
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
      <Button
        variant="secondary"
        size="sm"
        onClick={() => onEdit(userBook)}
        className="shrink-0 self-start"
      >
        Edit
      </Button>
    </div>
  )
}

export { OwnershipBadges }
