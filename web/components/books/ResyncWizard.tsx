'use client'

import { useEffect, useState } from 'react'
import { mutate } from 'swr'
import type { SourceBook } from '@/lib/gen/books/v1/catalog_pb'
import { useResyncProposals, useApplyResyncChoice } from '@/hooks/useBooks'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { swrKeys } from '@/lib/swrKeys'

const SOURCE_LABELS: Record<string, string> = {
  '': 'Keep library',
  openlibrary: 'Open Library',
  googlebooks: 'Google Books',
  unicat: 'UniCat'
}

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
    case 'cover_url':
      return s.coverUrl
    default:
      return ''
  }
}

// SourceCard shows one candidate's fields, highlighting the ones that differ
// from the library row (differs is empty for the library card itself).
function SourceCard({
  label,
  source,
  fields
}: {
  label: string
  source: SourceBook
  fields: string[]
}) {
  return (
    <div className="rounded-xl border border-border bg-surface p-3 text-sm">
      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">{label}</p>
      <ul className="space-y-1">
        {fields.map((field) => {
          const value = fieldValue(source, field)
          const differs = source.differs.includes(field)
          return (
            <li key={field} className="flex items-start justify-between gap-2">
              <span className="text-xs text-muted">{field.replace('_', ' ')}</span>
              <span
                className={differs ? 'text-right font-medium text-fg' : 'text-right text-muted'}
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

const FIELDS = ['title', 'authors', 'description', 'page_count', 'isbn13', 'cover_url']

export default function ResyncWizard() {
  const { data, isLoading } = useResyncProposals()
  const applyChoice = useApplyResyncChoice()
  const [index, setIndex] = useState(0)
  const [choice, setChoice] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const proposals = data?.proposals ?? []
  // Clamp after the list shrinks (e.g. applying the last book on the page).
  useEffect(() => {
    if (index > 0 && index >= proposals.length) setIndex(proposals.length - 1)
  }, [index, proposals.length])
  const current = proposals[index]

  function goTo(next: number) {
    setIndex(Math.max(0, Math.min(next, proposals.length - 1)))
    setChoice('')
    setError(null)
  }

  async function handleApply() {
    if (!current) return
    setBusy(true)
    setError(null)
    try {
      await applyChoice(current.bookId, choice)
      await mutate(swrKeys.resyncProposals)
      await mutate(swrKeys.books)
      // The list shrinks by one — stay on the same index to see the next book.
      setChoice('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to apply.')
    } finally {
      setBusy(false)
    }
  }

  if (isLoading) return <p className="text-xs text-muted">Loading…</p>
  if (proposals.length === 0) {
    return (
      <p className="text-xs text-muted">No flagged differences. Run a scan to check for updates.</p>
    )
  }
  // Briefly undefined the render after the list shrinks, before the clamp effect runs.
  if (!current) return null

  return (
    <Card className="mt-2 rounded-2xl p-4">
      <div className="mb-3 flex items-center justify-between text-xs text-muted">
        <span>
          Book {index + 1} of {proposals.length}
        </span>
        <span className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            disabled={index === 0}
            onClick={() => goTo(index - 1)}
          >
            Prev
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={index === proposals.length - 1}
            onClick={() => goTo(index + 1)}
          >
            Next
          </Button>
        </span>
      </div>

      <p className="mb-3 text-lg font-semibold text-fg">
        {current.library?.title || '(untitled)'}
        {current.library && current.library.authors.length > 0 && (
          <span className="ml-2 text-sm font-normal text-muted">
            {current.library.authors.join(', ')}
          </span>
        )}
      </p>

      {current.sources.length === 0 ? (
        <p className="rounded-xl border border-border bg-surface p-3 text-sm text-muted">
          No configured source (Open Library, Google Books, UniCat) has this book. Consider adding a
          new source, or dismiss if this is expected.
        </p>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2">
          {current.library && (
            <SourceCard label="Library" source={current.library} fields={FIELDS} />
          )}
          {current.sources.map((s) => (
            <SourceCard
              key={s.source}
              label={SOURCE_LABELS[s.source] ?? s.source}
              source={s}
              fields={FIELDS}
            />
          ))}
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <RadioGroup name="resync-source" value={choice} onChange={setChoice}>
          <RadioGroupItem value="" label="Keep library" />
          {current.sources.map((s) => (
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
            {busy ? 'Applying…' : choice === '' ? 'Dismiss' : 'Apply & next'}
          </Button>
        </span>
      </div>

      <Badge variant={current.sources.length === 0 ? 'warn' : 'secondary'} className="mt-3">
        {current.sources.length === 0
          ? 'Not found in any source'
          : `${current.sources.length} source${current.sources.length !== 1 ? 's' : ''} differ`}
      </Badge>
    </Card>
  )
}
