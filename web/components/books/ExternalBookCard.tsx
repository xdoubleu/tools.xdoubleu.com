'use client'

import Link from 'next/link'
import type { ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import { Badge } from '@/components/ui/badge'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
import { providerLabel } from '@/lib/books/bookShelves'

interface ExternalBookCardProps {
  book: ExternalBookResult
}

// Card for a search result not yet in the library. Visually distinct from
// BookCard via the source badge; clicking opens the external detail page
// instead of a library book page.
//
// provider_id is the result's ISBN13 (see protoExternalBook) — both
// configured providers only support fetch-by-ISBN, so a result with no ISBN
// has no detail page to link to and renders as a plain, non-clickable card.
export default function ExternalBookCard({ book }: ExternalBookCardProps) {
  const content = (
    <>
      <div className="shrink-0">
        <BookCover coverUrl={book.coverUrl} title={book.title} size="sm" />
      </div>
      <div className="flex-1 min-w-0">
        <h3 className="font-semibold text-sm leading-snug">{book.title}</h3>
        {book.authors.length > 0 && <p className="text-xs text-muted">{book.authors.join(', ')}</p>}
        <Badge variant="secondary" className="mt-1">
          {providerLabel(book.provider)}
        </Badge>
      </div>
    </>
  )

  if (!book.providerId) {
    return (
      <div className={cn('relative p-3 flex gap-3 items-start rounded-2xl border border-border')}>
        {content}
      </div>
    )
  }

  return (
    <Link
      href={`/books/external/${book.provider}/${book.providerId}`}
      className={cn(interactiveCardClass, 'relative p-3 flex gap-3 items-start')}
    >
      <CardLinkStatus />
      {content}
    </Link>
  )
}
