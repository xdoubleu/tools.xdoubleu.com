'use client'

import { useState } from 'react'
import type { SeriesEntry } from '@/lib/gen/books/v1/library_pb'
import BookCover from '@/components/books/BookCover'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { LinkCard } from '@/components/ui/link-card'
import { useCreateBook } from '@/hooks/useBooks'
import { statusLabel } from '@/lib/books/bookShelves'
import { formatSeriesPosition } from '@/lib/books/series'

interface SeriesEntryRowProps {
  entry: SeriesEntry
  onAdded: () => void
}

/** One volume: a link to an owned book, or a missing one with an add action. */
export default function SeriesEntryRow({ entry, onAdded }: SeriesEntryRowProps) {
  const position = formatSeriesPosition(entry.position)
  const owned = entry.userBook
  if (owned?.book) {
    return (
      <LinkCard
        href={`/books/${owned.id}`}
        aria-label={owned.book.title}
        linkClassName="flex items-start gap-3"
      >
        <BookCover coverUrl={owned.book.coverUrl} title={owned.book.title} size="sm" />
        <div className="min-w-0 flex-1">
          {position && <p className="text-xs text-muted">{position}</p>}
          <h3 className="text-sm font-semibold leading-snug">{owned.book.title}</h3>
          <Badge variant={owned.status === 'read' ? 'success' : 'secondary'} className="mt-1">
            {statusLabel(owned.status)}
          </Badge>
        </div>
      </LinkCard>
    )
  }
  if (!entry.external) return null
  return <MissingEntry entry={entry} position={position} onAdded={onAdded} />
}

function MissingEntry({
  entry,
  position,
  onAdded
}: {
  entry: SeriesEntry
  position: string
  onAdded: () => void
}) {
  const ext = entry.external!
  const addBook = useCreateBook()
  const [adding, setAdding] = useState(false)
  const [error, setError] = useState(false)

  const handleAdd = async () => {
    setAdding(true)
    setError(false)
    try {
      await addBook({
        provider: ext.provider,
        providerId: ext.providerId,
        title: ext.title,
        author: ext.authors.join(', '),
        status: 'to-read',
        isbn13: ext.isbn13,
        coverUrl: ext.coverUrl,
        description: ext.description,
        seriesName: ext.seriesName,
        seriesPosition: ext.seriesPosition,
        seriesTotal: ext.seriesTotal
      })
      onAdded()
    } catch {
      setError(true)
    } finally {
      setAdding(false)
    }
  }

  return (
    <Card
      className="flex flex-col gap-3 p-3 sm:flex-row sm:items-start"
      data-testid="series-missing"
    >
      <div className="flex min-w-0 flex-1 items-start gap-3">
        <BookCover coverUrl={ext.coverUrl} title={ext.title} size="sm" />
        <div className="min-w-0 flex-1">
          {position && <p className="text-xs text-muted">{position}</p>}
          <h3 className="text-sm font-semibold leading-snug">{ext.title}</h3>
          {ext.authors.length > 0 && <p className="text-xs text-muted">{ext.authors.join(', ')}</p>}
          <Badge variant="warn" className="mt-1">
            Not in library
          </Badge>
          {error && <p className="mt-1 text-xs text-danger">Failed to add. Try again.</p>}
        </div>
      </div>
      <Button size="sm" variant="secondary" disabled={adding} onClick={handleAdd}>
        {adding ? 'Adding…' : `Add to ${statusLabel('to-read')}`}
      </Button>
    </Card>
  )
}
