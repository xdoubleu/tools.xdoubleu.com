'use client'

import { useState } from 'react'
import type { SourceBook, ResyncProposal } from '@/lib/gen/books/v1/catalog_pb'
import BookCover from '@/components/books/BookCover'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export const SOURCE_LABELS: Record<string, string> = {
  '': 'Keep library',
  openlibrary: 'Open Library',
  googlebooks: 'Google Books',
  unicat: 'UniCat'
}

// cover_url is rendered as an image via BookCover, not as a text field.
const FIELDS = ['title', 'authors', 'description', 'page_count', 'isbn13']

function fieldValue(s: SourceBook, field: string): string {
  switch (field) {
    case 'title':
      return s.title
    case 'authors':
      return s.authors.join(', ')
    case 'description':
      return s.description
    case 'page_count':
      return s.pageCount ? String(s.pageCount) : ''
    case 'isbn13':
      return s.isbn13
    default:
      return ''
  }
}

// SourceCard shows one candidate's cover and fields, highlighting the ones
// that differ from the library row (differs is empty for the library card
// itself).
function SourceCard({ label, source }: { label: string; source: SourceBook }) {
  return (
    <div className="rounded-xl border border-border bg-surface p-3 text-sm">
      <div className="mb-2 flex items-center gap-2">
        <BookCover coverUrl={source.coverUrl} title={source.title} size="sm" />
        <p className="text-xs font-semibold uppercase tracking-wide text-muted">{label}</p>
      </div>
      <ul className="space-y-1">
        {FIELDS.map((field) => {
          const value = fieldValue(source, field)
          const differs = source.differs.includes(field)
          return (
            <li key={field} className="flex items-start justify-between gap-2">
              <span className="shrink-0 text-xs text-muted">{field.replace('_', ' ')}</span>
              <span
                className={
                  (differs ? 'font-medium text-fg' : 'text-muted') +
                  ' min-w-0 break-words text-right'
                }
              >
                {value || <span className="italic text-muted">none</span>}
              </span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}

interface SourceCompareProps {
  proposal: ResyncProposal
  onApply: (source: string) => Promise<void>
  applyLabel: (choice: string) => string
  // When set, renders editable title/author search fields so the admin can
  // re-run the live source search with tweaked terms (for books whose stored
  // title/author is slightly off and matches nothing).
  onSearch?: (title: string, author: string) => void
}

// SearchOverrideForm lets the admin steer the source search with hand-tweaked
// title/author terms.
function SearchOverrideForm({
  proposal,
  onSearch
}: {
  proposal: ResyncProposal
  onSearch: (title: string, author: string) => void
}) {
  const [title, setTitle] = useState(proposal.library?.title ?? '')
  const [author, setAuthor] = useState(proposal.library?.authors[0] ?? '')

  return (
    <div className="mb-3 flex flex-wrap items-end gap-2">
      <label className="min-w-0 flex-1 text-xs text-muted">
        Search title
        <Input
          className="mt-1"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Title"
        />
      </label>
      <label className="min-w-0 flex-1 text-xs text-muted">
        Search author
        <Input
          className="mt-1"
          value={author}
          onChange={(e) => setAuthor(e.target.value)}
          placeholder="Author"
        />
      </label>
      <Button variant="secondary" size="sm" onClick={() => onSearch(title, author)}>
        Search with these terms
      </Button>
    </div>
  )
}

// SourceCompare renders one book's library row alongside its external source
// candidates, with a radio picker and apply action. Shared by the resync
// wizard (stepping through flagged books) and the book detail page's
// on-demand "sync source" control (any single book).
export default function SourceCompare({
  proposal,
  onApply,
  applyLabel,
  onSearch
}: SourceCompareProps) {
  const [choice, setChoice] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleApply() {
    setBusy(true)
    setError(null)
    try {
      await onApply(choice)
      setChoice('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to apply.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div>
      <p className="mb-3 min-w-0 break-words text-lg font-semibold text-fg">
        {proposal.library?.title || '(untitled)'}
        {proposal.library && proposal.library.authors.length > 0 && (
          <span className="ml-2 text-sm font-normal text-muted">
            {proposal.library.authors.join(', ')}
          </span>
        )}
      </p>

      {onSearch && <SearchOverrideForm proposal={proposal} onSearch={onSearch} />}

      {proposal.sources.length === 0 ? (
        <p className="rounded-xl border border-border bg-surface p-3 text-sm text-muted">
          No configured source (Open Library, Google Books, UniCat) has this book. Consider adding a
          new source, or dismiss if this is expected.
        </p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {proposal.library && <SourceCard label="Library" source={proposal.library} />}
          {proposal.sources.map((s) => (
            <SourceCard key={s.source} label={SOURCE_LABELS[s.source] ?? s.source} source={s} />
          ))}
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <RadioGroup name="source-compare" value={choice} onChange={setChoice}>
          <RadioGroupItem value="" label="Keep library" />
          {proposal.sources.map((s) => (
            <RadioGroupItem
              key={s.source}
              value={s.source}
              label={SOURCE_LABELS[s.source] ?? s.source}
            />
          ))}
        </RadioGroup>
        <span className="flex items-center gap-2">
          {error && <span className="text-xs text-danger">{error}</span>}
          <Button variant="default" disabled={busy} onClick={() => void handleApply()}>
            {busy ? 'Applying…' : applyLabel(choice)}
          </Button>
        </span>
      </div>
    </div>
  )
}
