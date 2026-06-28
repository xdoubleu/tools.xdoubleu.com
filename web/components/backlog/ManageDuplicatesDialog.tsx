'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useFindDuplicates, useMergeBooks } from '@/hooks/useBacklog'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import DuplicateBookSummary from '@/components/backlog/DuplicateBookSummary'
import ConflictFieldPicker from '@/components/backlog/ConflictFieldPicker'
import {
  detectConflicts,
  buildResolvedMetadata,
  ALL_CONFLICT_FIELDS,
  type BookConflictField
} from '@/components/backlog/duplicateConflicts'
import type { DuplicateGroup } from '@/lib/gen/backlog/v1/books_pb'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// DuplicateGroupCard
// ---------------------------------------------------------------------------

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
    <div className="rounded-xl border border-border bg-card p-4 space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted">{reasonLabel(group.reason)}</span>
        <Button type="button" variant="secondary" size="sm" disabled={merging} onClick={onMerge}>
          {merging ? 'Merging…' : 'Merge'}
        </Button>
      </div>

      <div className="space-y-2">
        {group.entries.map((ub) => (
          <label
            key={ub.bookId}
            className="flex items-start gap-3 cursor-pointer rounded-lg p-2 hover:bg-surface transition-colors"
          >
            <input
              type="radio"
              name={`winner-${group.entries[0]?.bookId}`}
              value={ub.bookId}
              checked={winnerId === ub.bookId}
              onChange={() => onWinnerChange(ub.bookId)}
              className="mt-1 shrink-0"
            />
            <div className="flex-1 min-w-0">
              <DuplicateBookSummary ub={ub} />
              {winnerId === ub.bookId && (
                <p className="text-xs text-success mt-1">Keep this entry</p>
              )}
            </div>
          </label>
        ))}
      </div>

      <ConflictFieldPicker
        group={group}
        conflicts={conflicts}
        fieldChoices={fieldChoices}
        onChoiceChange={onFieldChoiceChange}
        groupKey={groupKey}
      />
    </div>
  )
}

// ---------------------------------------------------------------------------
// ManageDuplicatesDialog
// ---------------------------------------------------------------------------

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

  // winnerIds[groupKey] = selected winner bookId for that group.
  const [winnerIds, setWinnerIds] = useState<Record<string, string>>({})
  // fieldChoices[groupKey][field] = bookId of the chosen entry for that field.
  const [fieldChoices, setFieldChoices] = useState<
    Record<string, Partial<Record<BookConflictField, string>>>
  >({})
  // mergingKey tracks which group (by key) is currently merging.
  const [mergingKey, setMergingKey] = useState<string | null>(null)
  const [mergeAllBusy, setMergeAllBusy] = useState(false)
  const [error, setError] = useState('')

  const groups = data?.groups ?? []

  // A stable group key is the winner candidate's bookId.
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

    // Default each conflicting field to the current winner's bookId.
    const defaults: Partial<Record<BookConflictField, string>> = {}
    for (const { field } of detectConflicts(g)) {
      defaults[field] = stored[field] ?? winner
    }

    return { ...defaults, ...stored }
  }

  async function mergeGroup(g: DuplicateGroup): Promise<void> {
    const winner = getWinnerId(g)
    const losers = g.entries.map((e) => e.bookId).filter((id) => id !== winner)
    const choices = getFieldChoices(g)

    const resolvedMetadata = buildResolvedMetadata(g, choices)
    const coverChoice = choices['cover']

    await mergeBooks(winner, losers, {
      resolvedMetadata,
      resolvedCoverSourceBookId: coverChoice && coverChoice !== winner ? coverChoice : undefined
    })

    await mutate('/backlog/books')
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
    // Re-default all field choices to the new winner (user can override).
    setFieldChoices((prev) => {
      const existing = prev[key] ?? {}
      const reset: Partial<Record<BookConflictField, string>> = {}
      for (const field of ALL_CONFLICT_FIELDS) {
        if (field in existing) reset[field] = id
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
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>Find duplicates</DialogTitle>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4 pr-1">
          {isLoading && <p className="text-sm text-muted py-4 text-center">Scanning library…</p>}

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

        {error && <p className="text-sm text-danger mt-2">{error}</p>}

        {groups.length > 1 && (
          <div className="flex justify-end pt-3 border-t border-border">
            <Button type="button" variant="default" disabled={busy} onClick={handleMergeAll}>
              {mergeAllBusy ? 'Merging all…' : `Merge all (${groups.length} groups)`}
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
