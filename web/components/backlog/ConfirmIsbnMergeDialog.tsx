'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useMergeBooks } from '@/hooks/useBacklog'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'

export interface IsbnMergeTarget {
  /** The book that already holds the ISBN — this will be the winner. */
  winnerId: string
  winnerTitle: string
  winnerAuthors: string[]
  /** The catalog IDs of the group being edited — these become the losers. */
  loserIds: string[]
  loserTitle: string
  loserAuthors: string[]
}

interface Props {
  target: IsbnMergeTarget | null
  onClose: () => void
}

export default function ConfirmIsbnMergeDialog({ target, onClose }: Props) {
  const mergeBooks = useMergeBooks()
  const [merging, setMerging] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleConfirm() {
    if (!target) return
    setError(null)
    setMerging(true)
    try {
      await mergeBooks(target.winnerId, target.loserIds)
      void mutate('/backlog/books/catalog')
      onClose()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Merge failed.')
    } finally {
      setMerging(false)
    }
  }

  return (
    <Dialog open={target !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>ISBN already assigned — merge entries?</DialogTitle>
        </DialogHeader>

        {target && (
          <div className="space-y-4 text-sm">
            <p className="text-muted">
              This ISBN belongs to another catalog entry. Merging will keep that entry and fold the
              current one into it.
            </p>

            <div className="rounded-xl border border-border bg-surface p-3 space-y-2">
              <div>
                <p className="text-xs font-medium text-muted uppercase tracking-wide mb-0.5">
                  Will be kept (winner)
                </p>
                <p className="font-medium text-fg">{target.winnerTitle}</p>
                {target.winnerAuthors.length > 0 && (
                  <p className="text-xs text-muted">{target.winnerAuthors.join(', ')}</p>
                )}
              </div>

              <div className="border-t border-border pt-2">
                <p className="text-xs font-medium text-muted uppercase tracking-wide mb-0.5">
                  Will be merged in (loser)
                </p>
                <p className="font-medium text-fg">{target.loserTitle}</p>
                {target.loserAuthors.length > 0 && (
                  <p className="text-xs text-muted">{target.loserAuthors.join(', ')}</p>
                )}
              </div>
            </div>

            <p className="text-xs text-muted">
              User data (tags, status, progress, ratings) will be combined. This cannot be undone.
            </p>

            {error && <p className="text-sm text-danger">{error}</p>}

            <div className="flex justify-end gap-2">
              <Button variant="secondary" size="sm" onClick={onClose} disabled={merging}>
                Cancel
              </Button>
              <Button
                variant="default"
                size="sm"
                onClick={() => void handleConfirm()}
                disabled={merging}
              >
                {merging ? 'Merging…' : 'Merge entries'}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
