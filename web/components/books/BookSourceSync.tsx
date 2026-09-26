'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useBookSources, useApplyBookSource, type SourceSearchOverride } from '@/hooks/useBooks'
import SourceCompare from '@/components/books/SourceCompare'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

// Admin control to live-fetch and apply an external source to any one book
// (the resync wizard only covers flagged books).
export default function BookSourceSync({ bookId }: { bookId: string }) {
  const [open, setOpen] = useState(false)
  const [override, setOverride] = useState<SourceSearchOverride | undefined>(undefined)
  const { data, isLoading, error: fetchError } = useBookSources(bookId, open, override)
  const applySource = useApplyBookSource()

  async function handleApply(source: string, index: number) {
    await applySource(bookId, source, index, override)
    await mutate(swrKeys.books)
    await mutate(swrKeys.bookSources(bookId, override?.title ?? '', override?.author ?? ''))
    await mutate(swrKeys.bookSourceStats)
  }

  if (!open) {
    return (
      <Button variant="secondary" size="sm" className="text-xs" onClick={() => setOpen(true)}>
        Sync metadata source
      </Button>
    )
  }

  return (
    <Card className="p-4">
      {isLoading && <p className="text-sm text-muted">Fetching sources…</p>}
      {fetchError && <p className="text-sm text-danger">Failed to fetch sources.</p>}
      {data?.proposal && (
        <SourceCompare
          proposal={data.proposal}
          onApply={handleApply}
          applyLabel={() => 'Apply'}
          onSearch={(title, author) => setOverride({ title, author })}
        />
      )}
    </Card>
  )
}
