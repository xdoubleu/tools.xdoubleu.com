'use client'

import { useMemo, useState } from 'react'
import { mutate } from 'swr'
import type { BookMismatch, CompareCSVResponse } from '@/lib/gen/books/v1/catalog_pb'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/select'
import { useApplyCSVFix } from '@/hooks/useBooks'
import { swrKeys } from '@/lib/swrKeys'

type Props = {
  result: CompareCSVResponse
  csvData: string
  onFixed: () => void | Promise<void>
}

type Group = {
  label: string
  tag: string
  items: BookMismatch[]
}

// fixableTags lists the differences ApplyCSVFix can resolve. missing-in-csv
// (a library book absent from the CSV) has no fix — nothing is ever deleted.
const fixableTags = new Set(['missing-in-library', 'status', 'isbn', 'title', 'tags'])

function bookLabel(m: BookMismatch, side: 'csv' | 'library'): string {
  const ref = side === 'csv' ? m.csv : m.library
  if (!ref) return '(unknown)'
  const author = ref.authors[0] ?? ''
  return author ? `${ref.title} — ${author}` : ref.title
}

// tagsDelta compares the CSV's and library's tag sets and returns what a
// "tags" fix would add/remove (CSV is the source of truth).
function tagsDelta(m: BookMismatch): { added: string[]; removed: string[] } {
  const libTags = new Set(m.library?.tags ?? [])
  const csvTags = new Set(m.csv?.tags ?? [])
  return {
    added: [...csvTags].filter((t) => !libTags.has(t)),
    removed: [...libTags].filter((t) => !csvTags.has(t))
  }
}

function MismatchRow({
  m,
  tag,
  csvData,
  onFixed
}: {
  m: BookMismatch
  tag: string
  csvData: string
  onFixed: () => void | Promise<void>
}) {
  const applyCSVFix = useApplyCSVFix()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)

  async function handleFix() {
    setBusy(true)
    setError(false)
    try {
      await applyCSVFix(csvData, m.id, tag)
      await mutate(swrKeys.books)
      await onFixed()
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const fixButton = fixableTags.has(tag) && (
    <Button
      variant="secondary"
      className="ml-2 h-6 px-2 text-xs"
      disabled={busy}
      onClick={handleFix}
    >
      {busy ? 'Fixing…' : 'Fix'}
    </Button>
  )

  let body: React.ReactNode
  if (tag === 'missing-in-library') {
    body = (
      <>
        <span className="font-medium">{bookLabel(m, 'csv')}</span>
        <span className="ml-2 text-xs text-muted">will add to library</span>
      </>
    )
  } else if (tag === 'missing-in-csv') {
    body = (
      <>
        <span className="font-medium">{bookLabel(m, 'library')}</span>
        {m.library?.isbn13 && <span className="ml-2 text-xs text-muted">{m.library.isbn13}</span>}
      </>
    )
  } else {
    // status / isbn / title / tags diff — show the fix as before → after
    body = (
      <>
        <span className="font-medium">{bookLabel(m, 'csv')}</span>
        {tag === 'status' && (
          <span className="ml-2 text-xs text-muted">
            <Badge variant="secondary">{m.library?.status || 'none'}</Badge> {'→'}{' '}
            <Badge variant="secondary">{m.csv?.status || 'none'}</Badge>
          </span>
        )}
        {tag === 'isbn' && (
          <span className="ml-2 text-xs text-muted">
            {m.library?.isbn13 || 'none'} {'→'} {m.csv?.isbn13 || 'none'}
          </span>
        )}
        {tag === 'title' && (
          <span className="ml-2 text-xs text-muted">
            {m.library?.title} {'→'} {m.csv?.title}
          </span>
        )}
        {tag === 'tags' && (
          <span className="ml-2 text-xs text-muted">
            {tagsDelta(m).added.map((t) => (
              <Badge key={`add-${t}`} variant="secondary" className="ml-1">
                +{t}
              </Badge>
            ))}
            {tagsDelta(m).removed.map((t) => (
              <Badge key={`rm-${t}`} variant="secondary" className="ml-1">
                −{t}
              </Badge>
            ))}
          </span>
        )}
      </>
    )
  }

  return (
    <li className="flex items-center justify-between py-1 text-sm">
      <span>{body}</span>
      <span className="flex items-center">
        {error && <span className="mr-2 text-xs text-danger">Fix failed</span>}
        {fixButton}
      </span>
    </li>
  )
}

function GroupSection({
  group,
  csvData,
  onFixed
}: {
  group: Group
  csvData: string
  onFixed: () => void | Promise<void>
}) {
  const [open, setOpen] = useState(true)
  if (group.items.length === 0) return null
  return (
    <div className="mt-4">
      <Button
        variant="ghost"
        className="mb-1 w-full justify-between px-0 text-sm font-semibold text-fg hover:bg-transparent"
        onClick={() => setOpen((o) => !o)}
      >
        <span>
          {group.label}
          <span className="ml-2 text-xs font-normal text-muted">({group.items.length})</span>
        </span>
        <span className="text-xs text-muted">{open ? 'hide' : 'show'}</span>
      </Button>
      {open && (
        <ul className="divide-y divide-border rounded-2xl border border-border bg-card px-4 py-1">
          {group.items.map((m, i) => (
            // ponytail: index key is fine — list is static after render
            <MismatchRow key={i} m={m} tag={group.tag} csvData={csvData} onFixed={onFixed} />
          ))}
        </ul>
      )}
    </div>
  )
}

export default function CompareReport({ result, csvData, onFixed }: Props) {
  const [shelf, setShelf] = useState('')

  const shelves = useMemo(() => {
    const set = new Set<string>()
    for (const m of result.mismatches) {
      if (m.csv?.status) set.add(m.csv.status)
      if (m.library?.status) set.add(m.library.status)
    }
    return [...set].sort()
  }, [result.mismatches])

  const mismatches = shelf
    ? result.mismatches.filter((m) => m.csv?.status === shelf || m.library?.status === shelf)
    : result.mismatches

  const groups: Group[] = [
    {
      label: 'Only in CSV (not in library)',
      tag: 'missing-in-library',
      items: mismatches.filter((m) => m.differences.includes('missing-in-library'))
    },
    {
      label: 'Only in library (not in CSV)',
      tag: 'missing-in-csv',
      items: mismatches.filter((m) => m.differences.includes('missing-in-csv'))
    },
    {
      label: 'Reading state differs',
      tag: 'status',
      items: mismatches.filter((m) => m.differences.includes('status'))
    },
    {
      label: 'ISBN differs',
      tag: 'isbn',
      items: mismatches.filter((m) => m.differences.includes('isbn'))
    },
    {
      label: 'Title differs',
      tag: 'title',
      items: mismatches.filter((m) => m.differences.includes('title'))
    },
    {
      label: 'Tags differ',
      tag: 'tags',
      items: mismatches.filter((m) => m.differences.includes('tags'))
    }
  ]

  const allMatch = result.mismatches.length === 0

  return (
    <Card className="mt-4 rounded-2xl p-4">
      <div className="mb-3 flex flex-wrap items-center gap-4 text-sm text-muted">
        <span>
          CSV: <strong className="text-fg">{result.csvCount}</strong>
        </span>
        <span>
          Library: <strong className="text-fg">{result.libraryCount}</strong>
        </span>
        <span>
          Matched: <strong className="text-fg">{result.matchedCount}</strong>
        </span>
        <span>
          Mismatches: <strong className="text-fg">{result.mismatches.length}</strong>
        </span>
        {shelves.length > 0 && (
          <Select
            value={shelf}
            onChange={(e) => setShelf(e.target.value)}
            className="ml-auto h-8 w-auto text-xs"
            aria-label="Filter by shelf"
          >
            <option value="">All shelves</option>
            {shelves.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </Select>
        )}
      </div>
      {allMatch ? (
        <p className="text-sm text-success">CSV matches library exactly.</p>
      ) : (
        groups.map((g) => (
          <GroupSection key={g.tag} group={g} csvData={csvData} onFixed={onFixed} />
        ))
      )}
    </Card>
  )
}
