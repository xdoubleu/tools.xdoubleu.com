'use client'

import { useState } from 'react'
import { useSourceStats, useSourceUniqueBooks } from '@/hooks/useBooks'
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

// UniqueBooksDialog lists the actual books behind a source's Unique count.
function UniqueBooksDialog({
  source,
  onOpenChange
}: {
  source: string
  onOpenChange: (open: boolean) => void
}) {
  const { data, isLoading, error } = useSourceUniqueBooks(source)

  return (
    <Dialog open onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Unique to {SOURCE_LABELS[source] ?? source}</DialogTitle>
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
// found in each source) and uniqueness (how many books were found ONLY in
// that source, nowhere else). Clicking a Unique count opens the list of
// those books.
export default function SourceStats() {
  const { data, isLoading, error } = useSourceStats()
  const [openSource, setOpenSource] = useState<string | null>(null)

  if (isLoading) return <p className="text-xs text-muted">Loading…</p>
  if (error || !data) return <p className="text-xs text-danger">Failed to load source stats.</p>

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
              <td className="py-1.5">{SOURCE_LABELS[s.source] ?? s.source}</td>
              <td className="py-1.5 text-right tabular-nums">{s.foundCount}</td>
              <td className="py-1.5 text-right tabular-nums">
                {s.uniqueCount > 0 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-auto px-1 py-0 tabular-nums"
                    onClick={() => setOpenSource(s.source)}
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
      {openSource && (
        <UniqueBooksDialog
          source={openSource}
          onOpenChange={(open) => !open && setOpenSource(null)}
        />
      )}
    </Card>
  )
}
