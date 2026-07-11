'use client'

import { useState } from 'react'
import { useSourceStats, useBooksInExactSources } from '@/hooks/useBooks'
import { SOURCE_LABELS } from '@/components/books/SourceCompare'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import BookCover from '@/components/books/BookCover'

function sourceLabel(source: string): string {
  return SOURCE_LABELS[source] ?? source
}

function comboLabel(sources: string[]): string {
  if (sources.length >= 3) return 'All three'
  return sources.map(sourceLabel).join(' + ')
}

// ExactSourcesDialog lists the actual books found by exactly the given set
// of sources — one source is a Unique count, two or three is an overlap combo.
function ExactSourcesDialog({
  sources,
  onOpenChange
}: {
  sources: string[]
  onOpenChange: (open: boolean) => void
}) {
  const { data, isLoading, error } = useBooksInExactSources(sources)
  const title =
    sources.length === 1
      ? `Unique to ${sourceLabel(sources[0])}`
      : `Found in ${comboLabel(sources)}`

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>
        {isLoading && <p className="text-xs text-muted">Loading…</p>}
        {error && <p className="text-xs text-danger">Failed to load books.</p>}
        {data && (
          <ul className="space-y-2">
            {data.books.map((b) => (
              <li key={b.id} className="flex items-center gap-3">
                <BookCover coverUrl={b.coverUrl} title={b.title} size="sm" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-fg">{b.title}</p>
                  {b.authors.length > 0 && (
                    <p className="truncate text-xs text-muted">{b.authors.join(', ')}</p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </DialogContent>
    </Dialog>
  )
}

// SourceStats reports per-source coverage (how many books the last scan
// found in each source), uniqueness (how many books were found ONLY in that
// source), and pairwise/all-three overlap. Clicking a count opens the list
// of those books.
export default function SourceStats() {
  const { data, isLoading, error } = useSourceStats()
  const [openSources, setOpenSources] = useState<string[] | null>(null)

  if (isLoading) return <p className="text-xs text-muted">Loading…</p>
  if (error || !data) return <p className="text-xs text-danger">Failed to load source stats.</p>

  const overlaps = data.overlaps.filter((o) => o.count > 0)

  return (
    <Card className="rounded-2xl p-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs uppercase tracking-wide text-muted">
            <th className="pb-2 font-semibold">Source</th>
            <th className="pb-2 text-right font-semibold">Found</th>
            <th className="pb-2 text-right font-semibold">Unique</th>
          </tr>
        </thead>
        <tbody>
          {data.sources.map((s) => (
            <tr key={s.source} className="border-t border-border">
              <td className="py-1.5">{sourceLabel(s.source)}</td>
              <td className="py-1.5 text-right tabular-nums">{s.foundCount}</td>
              <td className="py-1.5 text-right tabular-nums">
                {s.uniqueCount > 0 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-auto px-1 py-0 tabular-nums"
                    onClick={() => setOpenSources([s.source])}
                  >
                    {s.uniqueCount}
                  </Button>
                ) : (
                  s.uniqueCount
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="mt-3 space-y-0.5 text-xs text-muted">
        <p>{data.totalBooks} books in the catalog.</p>
        <p>{data.notFoundAnywhere} scanned but not found in any source.</p>
        <p>{data.neverScanned} never scanned.</p>
      </div>
      {overlaps.length > 0 && (
        <div className="mt-4 border-t border-border pt-3">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
            Overlap (2+ sources)
          </p>
          <ul className="space-y-1">
            {overlaps.map((o) => (
              <li key={o.sources.join('+')} className="flex items-center justify-between">
                <span className="text-sm">{comboLabel(o.sources)}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-auto px-1 py-0 tabular-nums"
                  onClick={() => setOpenSources(o.sources)}
                >
                  {o.count}
                </Button>
              </li>
            ))}
          </ul>
        </div>
      )}
      {openSources && (
        <ExactSourcesDialog
          sources={openSources}
          onOpenChange={(open) => !open && setOpenSources(null)}
        />
      )}
    </Card>
  )
}
