'use client'

import { useState } from 'react'
import type { BookMismatch, CompareCSVResponse } from '@/lib/gen/books/v1/catalog_pb'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

type Props = {
  result: CompareCSVResponse
}

type Group = {
  label: string
  tag: string
  items: BookMismatch[]
}

function bookLabel(m: BookMismatch, side: 'csv' | 'library'): string {
  const ref = side === 'csv' ? m.csv : m.library
  if (!ref) return '(unknown)'
  const author = ref.authors[0] ?? ''
  return author ? `${ref.title} — ${author}` : ref.title
}

function MismatchRow({ m, tag }: { m: BookMismatch; tag: string }) {
  if (tag === 'missing-in-library') {
    return (
      <li className="py-1 text-sm">
        <span className="font-medium">{bookLabel(m, 'csv')}</span>
        {m.csv?.isbn13 && <span className="ml-2 text-xs text-muted">{m.csv.isbn13}</span>}
      </li>
    )
  }
  if (tag === 'missing-in-csv') {
    return (
      <li className="py-1 text-sm">
        <span className="font-medium">{bookLabel(m, 'library')}</span>
        {m.library?.isbn13 && <span className="ml-2 text-xs text-muted">{m.library.isbn13}</span>}
      </li>
    )
  }
  // status / isbn / title diff — show both sides
  return (
    <li className="py-1 text-sm">
      <span className="font-medium">{bookLabel(m, 'csv')}</span>
      {tag === 'status' && (
        <span className="ml-2 text-xs text-muted">
          CSV: <Badge variant="secondary">{m.csv?.status || 'none'}</Badge> Library:{' '}
          <Badge variant="secondary">{m.library?.status || 'none'}</Badge>
        </span>
      )}
      {tag === 'isbn' && (
        <span className="ml-2 text-xs text-muted">
          CSV: {m.csv?.isbn13 || 'none'} / Library: {m.library?.isbn13 || 'none'}
        </span>
      )}
      {tag === 'title' && (
        <span className="ml-2 text-xs text-muted">Library title: {m.library?.title}</span>
      )}
    </li>
  )
}

function GroupSection({ group }: { group: Group }) {
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
            <MismatchRow key={i} m={m} tag={group.tag} />
          ))}
        </ul>
      )}
    </div>
  )
}

export default function CompareReport({ result }: Props) {
  const groups: Group[] = [
    {
      label: 'Only in CSV (not in library)',
      tag: 'missing-in-library',
      items: result.mismatches.filter((m) => m.differences.includes('missing-in-library'))
    },
    {
      label: 'Only in library (not in CSV)',
      tag: 'missing-in-csv',
      items: result.mismatches.filter((m) => m.differences.includes('missing-in-csv'))
    },
    {
      label: 'Reading state differs',
      tag: 'status',
      items: result.mismatches.filter((m) => m.differences.includes('status'))
    },
    {
      label: 'ISBN differs',
      tag: 'isbn',
      items: result.mismatches.filter((m) => m.differences.includes('isbn'))
    },
    {
      label: 'Title differs',
      tag: 'title',
      items: result.mismatches.filter((m) => m.differences.includes('title'))
    }
  ]

  const allMatch = result.mismatches.length === 0

  return (
    <Card className="mt-4 rounded-2xl p-4">
      <div className="mb-3 flex gap-4 text-sm text-muted">
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
      </div>
      {allMatch ? (
        <p className="text-sm text-success">CSV matches library exactly.</p>
      ) : (
        groups.map((g) => <GroupSection key={g.tag} group={g} />)
      )}
    </Card>
  )
}
