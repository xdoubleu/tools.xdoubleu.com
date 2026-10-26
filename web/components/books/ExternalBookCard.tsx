import type { ExternalBookResult } from '@/lib/gen/books/v1/library_pb'
import { Artwork } from '@/components/ui/artwork'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { LinkCard } from '@/components/ui/link-card'
import { providerLabel } from '@/lib/books/bookShelves'

interface ExternalBookCardProps {
  book: ExternalBookResult
}

// Card for a not-in-library search result, linking to its external page.
// provider_id is the ISBN13; without one there's no detail page, so it's
// not clickable.
export default function ExternalBookCard({ book }: ExternalBookCardProps) {
  const content = (
    <>
      <div className="shrink-0">
        <Artwork src={book.coverUrl} title={book.title} alt={book.title} initials={2} size="sm" />
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
    return <Card className="flex items-start gap-3 p-3">{content}</Card>
  }

  return (
    <LinkCard
      href={`/books/external/${book.provider}/${book.providerId}`}
      linkClassName="flex items-start gap-3"
    >
      {content}
    </LinkCard>
  )
}
