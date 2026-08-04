'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { useRemoveBook, useUpdateBookStatus } from '@/hooks/useBooks'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

interface RemoveBookDialogProps {
  userBook: UserBook
  title: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onRemoved: () => void
}

export default function RemoveBookDialog({
  userBook,
  title,
  open,
  onOpenChange,
  onRemoved
}: RemoveBookDialogProps) {
  const removeBook = useRemoveBook()
  const updateBookStatus = useUpdateBookStatus()
  const [removing, setRemoving] = useState(false)
  const [markingOwned, setMarkingOwned] = useState(false)
  const [error, setError] = useState('')

  const owned = userBook.tags.includes('own-physical') || userBook.tags.includes('own-digital')
  const suggestOwned = owned && userBook.status !== 'owned'

  function handleOpenChange(next: boolean) {
    if (!next) setError('')
    onOpenChange(next)
  }

  async function handleConfirm() {
    setRemoving(true)
    setError('')
    try {
      await removeBook(userBook.bookId)
      await mutate(swrKeys.books)
      onOpenChange(false)
      onRemoved()
    } catch {
      setError('Failed to remove book. Please try again.')
      setRemoving(false)
    }
  }

  async function handleMarkOwned() {
    setMarkingOwned(true)
    setError('')
    try {
      await updateBookStatus({ bookId: userBook.bookId, status: 'owned' })
      await mutate(swrKeys.books)
      onOpenChange(false)
    } catch {
      setError('Failed to update book. Please try again.')
      setMarkingOwned(false)
    }
  }

  const busy = removing || markingOwned

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Remove from library</DialogTitle>
        </DialogHeader>

        <p className="text-sm text-muted">
          Remove <span className="font-semibold text-fg">{title}</span> from your library? Your
          reading progress and any uploaded files for this book will be deleted.
          {suggestOwned &&
            " You've marked this book as owned — removing it will lose that too. You can mark it Owned instead to keep it without tracking reading progress."}
        </p>

        {error && (
          <p className="mt-2 text-sm text-danger" data-testid="remove-book-error">
            {error}
          </p>
        )}

        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={busy} onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          {suggestOwned && (
            <Button
              variant="secondary"
              disabled={busy}
              onClick={handleMarkOwned}
              data-testid="remove-book-mark-owned-btn"
            >
              {markingOwned ? 'Marking as owned…' : 'Mark as owned instead'}
            </Button>
          )}
          <Button
            variant="destructive"
            disabled={busy}
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
