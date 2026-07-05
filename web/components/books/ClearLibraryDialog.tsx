'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useClearLibrary } from '@/hooks/useBooks'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { swrKeys } from '@/lib/swrKeys'

interface ClearLibraryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCleared?: () => void
}

const CONFIRM_WORD = 'DELETE'

export default function ClearLibraryDialog({
  open,
  onOpenChange,
  onCleared
}: ClearLibraryDialogProps) {
  const clearLibrary = useClearLibrary()
  const [typed, setTyped] = useState('')
  const [clearing, setClearing] = useState(false)
  const [error, setError] = useState('')

  function handleOpenChange(next: boolean) {
    if (!next) {
      setTyped('')
      setError('')
    }
    onOpenChange(next)
  }

  async function handleConfirm() {
    setClearing(true)
    setError('')
    try {
      await clearLibrary()
      await mutate(swrKeys.books)
      setTyped('')
      onOpenChange(false)
      onCleared?.()
    } catch {
      setError('Failed to clear library. Please try again.')
    } finally {
      setClearing(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Clear entire library</DialogTitle>
        </DialogHeader>

        <p className="text-sm text-muted">
          This will permanently delete all your books, reading progress, and uploaded files. This
          action cannot be undone.
        </p>

        <div className="mt-4 space-y-3">
          <label className="block text-sm text-subtle">
            Type <span className="font-semibold text-fg">{CONFIRM_WORD}</span> to confirm
          </label>
          <Input
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            placeholder={CONFIRM_WORD}
            data-testid="clear-library-confirm-input"
          />
        </div>

        {error && (
          <p className="mt-2 text-sm text-danger" data-testid="clear-library-error">
            {error}
          </p>
        )}

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" disabled={clearing} onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={typed !== CONFIRM_WORD || clearing}
            onClick={handleConfirm}
            data-testid="clear-library-confirm-btn"
          >
            {clearing ? 'Clearing…' : 'Clear library'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
