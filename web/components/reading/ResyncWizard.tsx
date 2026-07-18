'use client'

import { useEffect, useState } from 'react'
import { mutate } from 'swr'
import {
  useResyncProposals,
  useApplyResyncChoice,
  useBookSources,
  useApplyBookSource,
  type SourceSearchOverride
} from '@/hooks/useBooks'
import SourceCompare from '@/components/reading/SourceCompare'
import TogglePill from '@/components/reading/TogglePill'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { swrKeys } from '@/lib/swrKeys'

export default function ResyncWizard() {
  const { data, isLoading } = useResyncProposals()
  const applyChoice = useApplyResyncChoice()
  const applySource = useApplyBookSource()
  const [index, setIndex] = useState(0)
  const [onlyNotFound, setOnlyNotFound] = useState(false)
  const [override, setOverride] = useState<SourceSearchOverride | undefined>(undefined)

  const proposals = data?.proposals ?? []
  const notFoundCount = proposals.filter((p) => p.sources.length === 0).length
  const visible = onlyNotFound ? proposals.filter((p) => p.sources.length === 0) : proposals

  // Clamp after the list shrinks (e.g. applying the last book on the page).
  useEffect(() => {
    if (index > 0 && index >= visible.length) setIndex(visible.length - 1)
  }, [index, visible.length])
  const current = visible[index]
  const currentBookId = current?.bookId

  // A tweaked search only applies to the book it was typed for.
  useEffect(() => {
    setOverride(undefined)
  }, [currentBookId])

  // Live re-fetch with the admin's tweaked title/author terms.
  const {
    data: liveData,
    isLoading: liveLoading,
    error: liveError
  } = useBookSources(currentBookId ?? '', Boolean(currentBookId && override), override)

  function goTo(next: number) {
    setIndex(Math.max(0, Math.min(next, visible.length - 1)))
  }

  async function handleApply(choice: string, choiceIndex: number) {
    if (!current) return
    if (override) {
      // ApplyBookSource re-runs the tweaked search server-side and also
      // clears the stored proposal, so the wizard advances exactly as below.
      await applySource(current.bookId, choice, choiceIndex, override)
    } else {
      // The stored wizard proposal always has one candidate per source.
      await applyChoice(current.bookId, choice)
    }
    await mutate(swrKeys.resyncProposals)
    await mutate(swrKeys.books)
    await mutate(swrKeys.bookSourceStats)
    // The list shrinks by one — stay on the same index to see the next book.
  }

  if (isLoading) return <p className="text-xs text-muted">Loading…</p>
  if (proposals.length === 0) {
    return (
      <p className="text-xs text-muted">No flagged differences. Run a scan to check for updates.</p>
    )
  }

  const filterPill = (
    <TogglePill
      label={`Not found only (${notFoundCount})`}
      active={onlyNotFound}
      onClick={() => {
        setOnlyNotFound(!onlyNotFound)
        setIndex(0)
      }}
    />
  )

  if (visible.length === 0) {
    return (
      <Card className="mt-2 rounded-2xl p-4">
        {filterPill}
        <p className="mt-3 text-xs text-muted">Every source found every flagged book.</p>
      </Card>
    )
  }
  // Briefly undefined the render after the list shrinks, before the clamp effect runs.
  if (!current) return null

  const displayed = override && liveData?.proposal ? liveData.proposal : current

  return (
    <Card className="mt-2 rounded-2xl p-4">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2 text-xs text-muted">
        <span className="flex items-center gap-2">
          <span>
            Book {index + 1} of {visible.length}
          </span>
          {filterPill}
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
            disabled={index === visible.length - 1}
            onClick={() => goTo(index + 1)}
          >
            Next
          </Button>
        </span>
      </div>

      {override && liveLoading && <p className="text-sm text-muted">Fetching sources…</p>}
      {override && liveError && <p className="text-sm text-danger">Failed to fetch sources.</p>}
      <SourceCompare
        key={current.bookId + (override ? '-override' : '')}
        proposal={displayed}
        onApply={handleApply}
        applyLabel={(choice) => (choice === '' ? 'Dismiss' : 'Apply & next')}
        onSearch={(title, author) => setOverride({ title, author })}
      />

      <Badge variant={displayed.sources.length === 0 ? 'warn' : 'secondary'} className="mt-3">
        {displayed.sources.length === 0
          ? 'Not found in any source'
          : `${displayed.sources.length} source${displayed.sources.length !== 1 ? 's' : ''} differ`}
      </Badge>
    </Card>
  )
}
