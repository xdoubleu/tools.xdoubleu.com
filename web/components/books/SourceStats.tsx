'use client'

import { useState } from 'react'
import { useSourceStats, useBooksInExactSources } from '@/hooks/useBooks'
import { SOURCE_LABELS } from '@/components/books/SourceCompare'
import { Card } from '@/components/ui/card'
import { LoadingState, ErrorState } from '@/components/ui/states'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '@/components/ui/table'
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

// External providers only; excludes the '' "Keep library" entry.
const TOTAL_SOURCES = Object.keys(SOURCE_LABELS).filter((s) => s !== '').length

function comboLabel(sources: string[]): string {
  if (sources.length >= TOTAL_SOURCES) return 'All sources'
  return sources.map(sourceLabel).join(' + ')
}

// Books found by exactly the given set of sources.
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
          <DialogClose />
        </DialogHeader>
        {isLoading && <LoadingState className="text-xs" />}
        {error && <ErrorState what="books" className="text-xs" />}
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

// SourceStats shows per-source coverage, uniqueness and overlap from the last
// scan; clicking a count lists those books.
export default function SourceStats() {
  const { data, isLoading, error } = useSourceStats()
  const [openSources, setOpenSources] = useState<string[] | null>(null)

  if (isLoading) return <LoadingState className="text-xs" />
  if (error || !data) return <ErrorState what="source stats" className="text-xs" />

  const overlaps = data.overlaps.filter((o) => o.count > 0)
  const missedOverlaps = data.missedOverlaps.filter((o) => o.count > 0)
  const foundTotal = data.totalBooks - data.notFoundAnywhere - data.neverScanned

  return (
    <div className="space-y-3">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>Source</TableHead>
            <TableHead className="text-right">Found</TableHead>
            <TableHead className="text-right">Missed</TableHead>
            <TableHead className="text-right">Unique</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.sources.map((s) => (
            <TableRow key={s.source}>
              <TableCell>{sourceLabel(s.source)}</TableCell>
              <TableCell className="text-right tabular-nums">{s.foundCount}</TableCell>
              <TableCell className="text-right tabular-nums">{s.missedCount}</TableCell>
              <TableCell className="text-right tabular-nums">
                {s.uniqueCount > 0 ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="min-w-11 px-2 tabular-nums sm:min-w-0"
                    onClick={() => setOpenSources([s.source])}
                  >
                    {s.uniqueCount}
                  </Button>
                ) : (
                  s.uniqueCount
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Card className="rounded-2xl p-4">
        <div className="space-y-0.5 text-xs text-muted">
          <p className="font-medium text-fg">
            {foundTotal} found across all sources (in at least one).
          </p>
          <p>{data.totalBooks} books in the catalog.</p>
          <p>{data.notFoundAnywhere} missing from all sources.</p>
          <p>{data.neverScanned} never scanned.</p>
        </div>
        {overlaps.length > 0 && (
          <div className="mt-4 border-t border-border pt-3">
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
              Overlap — found in exactly these sources
            </p>
            <ul className="space-y-1">
              {overlaps.map((o) => (
                <li key={o.sources.join('+')} className="flex items-center justify-between gap-2">
                  <span className="text-sm">{comboLabel(o.sources)}</span>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="min-w-11 px-2 tabular-nums sm:min-w-0"
                    onClick={() => setOpenSources(o.sources)}
                  >
                    {o.count}
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        )}
        {missedOverlaps.length > 0 && (
          <div className="mt-4 border-t border-border pt-3">
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
              Missed overlaps — missed by exactly these sources
            </p>
            <ul className="space-y-1">
              {missedOverlaps.map((o) => (
                <li key={o.sources.join('+')} className="flex items-center justify-between gap-2">
                  <span className="text-sm">{comboLabel(o.sources)}</span>
                  <span className="text-sm tabular-nums">{o.count}</span>
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
    </div>
  )
}
