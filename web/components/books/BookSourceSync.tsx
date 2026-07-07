'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useBookSources, useApplyBookSource } from '@/hooks/useBooks'
import SourceCompare from '@/components/books/SourceCompare'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

// BookSourceSync is the admin-only control on a book's detail page for
// live-fetching and applying an external metadata source to that one book —
// works on any book on demand, unlike the resync wizard which only shows
// books a prior scan already flagged.
export default function BookSourceSync({ bookId }: { bookId: string }) {
  const [open, setOpen] = useState(false)
  const { data, isLoading, error: fetchError } = useBookSources(bookId, open)
  const applySource = useApplyBookSource()

  async function handleApply(source: string) {
    await applySource(bookId, source)
    await mutate(swrKeys.books)
    await mutate(swrKeys.bookSources(bookId))
  }

  if (!open) {
    return (
      <Button variant="secondary" size="sm" className="text-xs" onClick={() => setOpen(true)}>
        Sync metadata source
      </Button>
    )
  }

  return (
    <div className="rounded-2xl border border-border bg-card shadow-card p-4">
      {isLoading && <p className="text-sm text-muted">Fetching sources…</p>}
      {fetchError && <p className="text-sm text-danger">Failed to fetch sources.</p>}
      {data?.proposal && (
        <SourceCompare proposal={data.proposal} onApply={handleApply} applyLabel={() => 'Apply'} />
      )}
    </div>
  )
}
