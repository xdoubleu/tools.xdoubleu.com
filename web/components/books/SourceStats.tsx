'use client'

import { useSourceStats } from '@/hooks/useBooks'
import { SOURCE_LABELS } from '@/components/books/SourceCompare'
import { Card } from '@/components/ui/card'

// SourceStats reports per-source coverage (how many books the last scan
// found in each source) and applied provenance (how many books' metadata was
// last applied from each source — the admin's manual quality verdict).
export default function SourceStats() {
  const { data, isLoading, error } = useSourceStats()

  if (isLoading) return <p className="text-xs text-muted">Loading…</p>
  if (error || !data) return <p className="text-xs text-danger">Failed to load source stats.</p>

  return (
    <Card className="rounded-2xl p-4">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-xs uppercase tracking-wide text-muted">
            <th className="pb-2 font-semibold">Source</th>
            <th className="pb-2 text-right font-semibold">Found</th>
            <th className="pb-2 text-right font-semibold">Applied</th>
          </tr>
        </thead>
        <tbody>
          {data.sources.map((s) => (
            <tr key={s.source} className="border-t border-border">
              <td className="py-1.5">{SOURCE_LABELS[s.source] ?? s.source}</td>
              <td className="py-1.5 text-right tabular-nums">{s.foundCount}</td>
              <td className="py-1.5 text-right tabular-nums">{s.appliedCount}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="mt-3 space-y-0.5 text-xs text-muted">
        <p>{data.totalBooks} books in the catalog.</p>
        <p>{data.notFoundAnywhere} scanned but not found in any source.</p>
        <p>{data.neverScanned} never scanned.</p>
      </div>
    </Card>
  )
}
