'use client'

import { useState } from 'react'
import type { SourceBook, ResyncProposal } from '@/lib/gen/books/v1/catalog_pb'
import BookCover from '@/components/books/BookCover'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

export const SOURCE_LABELS: Record<string, string> = {
  '': 'Keep library',
  unicat: 'UniCat',
  hardcover: 'Hardcover'
}

// cover_url renders via BookCover.
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

// Highlights fields differing from the library row.
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
  onApply: (source: string, index: number) => Promise<void>
  applyLabel: (choice: string) => string
  // Shows editable search terms to re-run the live search.
  onSearch?: (title: string, author: string) => void
}

// Encodes (source, index) as one RadioGroup value; "" means keep library.
function choiceValue(source: string, index: number): string {
  return source === '' ? '' : `${source}:${index}`
}

function parseChoice(value: string): { source: string; index: number } {
  if (value === '') return { source: '', index: 0 }
  const sepIndex = value.lastIndexOf(':')
  return { source: value.slice(0, sepIndex), index: Number(value.slice(sepIndex + 1)) }
}

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

// One book's library row versus its source candidates, with a picker and
// apply. Shared by the resync wizard and the book page's sync control.
export default function SourceCompare({
  proposal,
  onApply,
  applyLabel,
  onSearch
}: SourceCompareProps) {
  const [choice, setChoice] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Usually one candidate per source; an override search can return up to 5.
  const groups = new Map<string, SourceBook[]>()
  for (const s of proposal.sources) {
    groups.set(s.source, [...(groups.get(s.source) ?? []), s])
  }

  async function handleApply() {
    setBusy(true)
    setError(null)
    try {
      const { source, index } = parseChoice(choice)
      await onApply(source, index)
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
          No configured source (UniCat, Hardcover) has this book. Consider adding a new source, or
          dismiss if this is expected.
        </p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {proposal.library && <SourceCard label="Library" source={proposal.library} />}
          {Array.from(groups.entries()).map(([source, candidates]) =>
            candidates.length === 1 ? (
              <SourceCard
                key={source}
                label={SOURCE_LABELS[source] ?? source}
                source={candidates[0]}
              />
            ) : (
              <div key={source} className="sm:col-span-2">
                <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
                  {SOURCE_LABELS[source] ?? source} ({candidates.length} candidates)
                </p>
                <div className="grid max-h-80 gap-3 overflow-y-auto sm:grid-cols-2">
                  {candidates.map((s) => (
                    <SourceCard key={s.index} label={`Candidate ${s.index + 1}`} source={s} />
                  ))}
                </div>
              </div>
            )
          )}
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <RadioGroup name="source-compare" value={choice} onChange={setChoice}>
          <RadioGroupItem value="" label="Keep library" />
          {Array.from(groups.entries()).flatMap(([source, candidates]) =>
            candidates.map((s) => (
              <RadioGroupItem
                key={choiceValue(source, s.index)}
                value={choiceValue(source, s.index)}
                label={
                  candidates.length === 1
                    ? (SOURCE_LABELS[source] ?? source)
                    : `${SOURCE_LABELS[source] ?? source} #${s.index + 1}`
                }
              />
            ))
          )}
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
