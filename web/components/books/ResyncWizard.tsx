'use client'

import { useEffect, useState } from 'react'
import { mutate } from 'swr'
import { useResyncProposals, useApplyResyncChoice } from '@/hooks/useBooks'
import SourceCompare from '@/components/books/SourceCompare'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { swrKeys } from '@/lib/swrKeys'

export default function ResyncWizard() {
  const { data, isLoading } = useResyncProposals()
  const applyChoice = useApplyResyncChoice()
  const [index, setIndex] = useState(0)

  const proposals = data?.proposals ?? []
  // Clamp after the list shrinks (e.g. applying the last book on the page).
  useEffect(() => {
    if (index > 0 && index >= proposals.length) setIndex(proposals.length - 1)
  }, [index, proposals.length])
  const current = proposals[index]

  function goTo(next: number) {
    setIndex(Math.max(0, Math.min(next, proposals.length - 1)))
  }

  async function handleApply(choice: string) {
    if (!current) return
    await applyChoice(current.bookId, choice)
    await mutate(swrKeys.resyncProposals)
    await mutate(swrKeys.books)
    // The list shrinks by one — stay on the same index to see the next book.
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

      <SourceCompare
        key={current.bookId}
        proposal={current}
        onApply={handleApply}
        applyLabel={(choice) => (choice === '' ? 'Dismiss' : 'Apply & next')}
      />

      <Badge variant={current.sources.length === 0 ? 'warn' : 'secondary'} className="mt-3">
        {current.sources.length === 0
          ? 'Not found in any source'
          : `${current.sources.length} source${current.sources.length !== 1 ? 's' : ''} differ`}
      </Badge>
    </Card>
  )
}
