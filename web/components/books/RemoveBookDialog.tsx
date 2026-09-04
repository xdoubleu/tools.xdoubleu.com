'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { useRemoveBook } from '@/hooks/useBooks'
import { ConfirmDialog } from '@/components/ui/dialog'
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
      await removeBook(userBook.bookId)
      await mutate(swrKeys.books)
      onOpenChange(false)
      onRemoved()
    } catch {
      setError('Failed to remove book. Please try again.')
      setRemoving(false)
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={handleOpenChange}
      title="Remove from library"
      description={
        <>
          Remove <span className="font-semibold text-fg">{title}</span> from your library? Your
          reading progress and any uploaded files for this book will be deleted.
        </>
      }
      confirmLabel="Remove"
      pendingLabel="Removing…"
      destructive
      pending={removing}
      onConfirm={handleConfirm}
    >
      {error && (
        <p className="mt-2 text-sm text-danger" data-testid="remove-book-error">
          {error}
        </p>
      )}
    </ConfirmDialog>
  )
}
