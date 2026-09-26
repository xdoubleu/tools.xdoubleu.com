'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useFindDuplicates, useMergeBooks } from '@/hooks/useBooks'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Alert } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import DuplicateBookSummary from '@/components/books/DuplicateBookSummary'
import ConflictFieldPicker from '@/components/books/ConflictFieldPicker'
import { Radio } from '@/components/ui/radio-group'
import {
  detectConflicts,
  buildResolvedMetadata,
  pickAutoStatusBookId,
  resolveStatusChoice,
  ALL_CONFLICT_FIELDS,
  type BookConflictField
} from '@/components/books/duplicateConflicts'
import type { DuplicateGroup } from '@/lib/gen/books/v1/catalog_pb'
import { swrKeys } from '@/lib/swrKeys'

function reasonLabel(reason: string): string {
  switch (reason) {
    case 'isbn13':
      return 'Same ISBN-13'
    case 'title+author':
      return 'Same title + author'
    default:
      return reason
  }
}

interface DuplicateGroupCardProps {
  group: DuplicateGroup
  winnerId: string
  onWinnerChange: (id: string) => void
  onMerge: () => Promise<void>
  merging: boolean
  fieldChoices: Partial<Record<BookConflictField, string>>
  onFieldChoiceChange: (field: BookConflictField, bookId: string) => void
  groupKey: string
}

function DuplicateGroupCard({
  group,
  winnerId,
  onWinnerChange,
  onMerge,
  merging,
  fieldChoices,
  onFieldChoiceChange,
  groupKey
}: DuplicateGroupCardProps) {
  const conflicts = detectConflicts(group)

  return (
    <Card className="space-y-3 rounded-xl p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs text-muted">{reasonLabel(group.reason)}</span>
        <Button type="button" variant="secondary" size="sm" disabled={merging} onClick={onMerge}>
          {merging ? 'Merging…' : 'Merge'}
        </Button>
      </div>

      <div className="space-y-2">
        {group.entries.map((ub) => (
          <Label
            key={ub.bookId}
            className="flex cursor-pointer items-start gap-3 rounded-lg p-2 font-normal transition-colors hover:bg-surface"
          >
            <Radio
              name={`winner-${group.entries[0]?.bookId}`}
              value={ub.bookId}
              checked={winnerId === ub.bookId}
              onChange={() => onWinnerChange(ub.bookId)}
              className="mt-1 shrink-0 text-base"
            />
            <div className="flex-1 min-w-0">
              <DuplicateBookSummary ub={ub} />
              {winnerId === ub.bookId && (
                <p className="text-xs text-success mt-1">Keep this entry</p>
              )}
            </div>
          </Label>
        ))}
      </div>

      <ConflictFieldPicker
        group={group}
        conflicts={conflicts}
        fieldChoices={fieldChoices}
        onChoiceChange={onFieldChoiceChange}
        groupKey={groupKey}
      />
    </Card>
  )
}

interface ManageDuplicatesDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function ManageDuplicatesDialog({
  open,
  onOpenChange
}: ManageDuplicatesDialogProps) {
  const { data, isLoading, mutate: mutateDupes } = useFindDuplicates()
  const mergeBooks = useMergeBooks()

  const [winnerIds, setWinnerIds] = useState<Record<string, string>>({})
  const [fieldChoices, setFieldChoices] = useState<
    Record<string, Partial<Record<BookConflictField, string>>>
  >({})
  const [mergingKey, setMergingKey] = useState<string | null>(null)
  const [mergeAllBusy, setMergeAllBusy] = useState(false)
  const [error, setError] = useState('')

  const groups = data?.groups ?? []

  function groupKey(g: DuplicateGroup): string {
    return g.entries[0]?.bookId ?? ''
  }

  function getWinnerId(g: DuplicateGroup): string {
    return winnerIds[groupKey(g)] ?? g.entries[0]?.bookId ?? ''
  }

  function getFieldChoices(g: DuplicateGroup): Partial<Record<BookConflictField, string>> {
    const key = groupKey(g)
    const winner = getWinnerId(g)
    const stored = fieldChoices[key] ?? {}

    // Fields default to the winner; status defaults to the auto-consolidation
    // pick (custom shelf beats built-ins).
    const autoStatusBookId = pickAutoStatusBookId(g)
    const defaults: Partial<Record<BookConflictField, string>> = {}
    for (const { field } of detectConflicts(g)) {
      if (field === 'status') {
        defaults[field] = stored[field] ?? autoStatusBookId
      } else {
        defaults[field] = stored[field] ?? winner
      }
    }

    return { ...defaults, ...stored }
  }

  async function mergeGroup(g: DuplicateGroup): Promise<void> {
    const winner = getWinnerId(g)
    const losers = g.entries.map((e) => e.bookId).filter((id) => id !== winner)
    const choices = getFieldChoices(g)

    const resolvedMetadata = buildResolvedMetadata(g, choices)
    const coverChoice = choices['cover']
    const resolvedStatus = resolveStatusChoice(g, choices)

    await mergeBooks(winner, losers, {
      resolvedMetadata,
      resolvedCoverSourceBookId: coverChoice && coverChoice !== winner ? coverChoice : undefined,
      resolvedStatus
    })

    await mutate(swrKeys.books)
    await mutateDupes()
  }

  async function handleMergeOne(g: DuplicateGroup) {
    const key = groupKey(g)
    setMergingKey(key)
    setError('')
    try {
      await mergeGroup(g)
    } catch {
      setError('Merge failed. Please try again.')
    } finally {
      setMergingKey(null)
    }
  }

  async function handleMergeAll() {
    setMergeAllBusy(true)
    setError('')
    try {
      for (const g of groups) {
        await mergeGroup(g)
      }
    } catch {
      setError('One or more merges failed. Please try again.')
    } finally {
      setMergeAllBusy(false)
    }
  }

  function handleWinnerChange(g: DuplicateGroup, id: string) {
    const key = groupKey(g)
    setWinnerIds((prev) => ({ ...prev, [key]: id }))
    // Catalog fields follow the new winner; status stays independent.
    const autoStatusBookId = pickAutoStatusBookId(g)
    setFieldChoices((prev) => {
      const existing = prev[key] ?? {}
      const reset: Partial<Record<BookConflictField, string>> = {}
      for (const field of ALL_CONFLICT_FIELDS) {
        if (field in existing) {
          reset[field] = field === 'status' ? autoStatusBookId : id
        }
      }
      return { ...prev, [key]: reset }
    })
  }

  function handleFieldChoiceChange(g: DuplicateGroup, field: BookConflictField, bookId: string) {
    const key = groupKey(g)
    setFieldChoices((prev) => ({
      ...prev,
      [key]: { ...(prev[key] ?? {}), [field]: bookId }
    }))
  }

  const busy = mergeAllBusy || mergingKey !== null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Find duplicates</DialogTitle>
          <DialogClose />
        </DialogHeader>

        <div className="space-y-4">
          {isLoading && <p className="py-4 text-center text-sm text-muted">Scanning library…</p>}

          {!isLoading && groups.length === 0 && (
            <p className="text-sm text-muted py-4 text-center">No duplicates found.</p>
          )}

          {groups.map((g) => {
            const key = groupKey(g)
            return (
              <DuplicateGroupCard
                key={key}
                group={g}
                winnerId={getWinnerId(g)}
                onWinnerChange={(id) => handleWinnerChange(g, id)}
                onMerge={() => handleMergeOne(g)}
                merging={mergingKey === key}
                fieldChoices={getFieldChoices(g)}
                onFieldChoiceChange={(field, bookId) => handleFieldChoiceChange(g, field, bookId)}
                groupKey={key}
              />
            )
          })}
        </div>

        {error && (
          <Alert tone="danger" className="mt-2">
            {error}
          </Alert>
        )}

        {groups.length > 1 && (
          <DialogFooter>
            <Button type="button" variant="default" disabled={busy} onClick={handleMergeAll}>
              {mergeAllBusy ? 'Merging all…' : `Merge all (${groups.length} groups)`}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
