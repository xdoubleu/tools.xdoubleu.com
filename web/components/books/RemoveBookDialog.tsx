'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useRemoveBook } from '@/hooks/useBooks'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

interface RemoveBookDialogProps {
  bookId: string
  title: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onRemoved: () => void
}

export default function RemoveBookDialog({
  bookId,
  title,
  open,
  onOpenChange,
  onRemoved
}: RemoveBookDialogProps) {
  const removeBook = useRemoveBook()
  const [removing, setRemoving] = useState(false)
  const [error, setError] = useState('')

  function handleOpenChange(next: boolean) {
    if (!next) setError('')
    onOpenChange(next)
  }

  async function handleConfirm() {
    setRemoving(true)
    setError('')
    try {
      await removeBook(bookId)
      await mutate(swrKeys.books)
      onOpenChange(false)
      onRemoved()
    } catch {
      setError('Failed to remove book. Please try again.')
      setRemoving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Remove from library</DialogTitle>
        </DialogHeader>

        <p className="text-sm text-muted">
          Remove <span className="font-semibold text-fg">{title}</span> from your library? Your
          reading progress and any uploaded files for this book will be deleted.
        </p>

        {error && (
          <p className="mt-2 text-sm text-danger" data-testid="remove-book-error">
            {error}
          </p>
        )}

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" disabled={removing} onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={removing}
            onClick={handleConfirm}
            data-testid="remove-book-confirm-btn"
          >
            {removing ? 'Removing…' : 'Remove'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
