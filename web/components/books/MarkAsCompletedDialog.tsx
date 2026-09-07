'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import { ToggleIconButton } from '@/components/ui/toggle-icon-button'
import { cn } from '@/lib/cn'
import { swrKeys } from '@/lib/swrKeys'

interface MarkAsCompletedDialogProps {
  userBook: UserBook
  open: boolean
  onOpenChange: (open: boolean) => void
  onCompleted?: () => void
}

/**
 * Confirms moving a currently-reading book to "read", stamping today's date
 * (the server sets `finishedAt` when `updateBookStatus` transitions status to
 * `read`) while letting the user set a rating and favourite in the same call.
 */
export default function MarkAsCompletedDialog({
  userBook,
  open,
  onOpenChange,
  onCompleted
}: MarkAsCompletedDialogProps) {
  const [rating, setRating] = useState(userBook.rating)
  const [hover, setHover] = useState(0)
  const [favourite, setFavourite] = useState(userBook.tags.includes('favourite'))
  const [completing, setCompleting] = useState(false)
  const [error, setError] = useState('')
  const updateBookStatus = useUpdateBookStatus()

  function handleOpenChange(next: boolean) {
    if (!next) {
      setError('')
      setRating(userBook.rating)
      setFavourite(userBook.tags.includes('favourite'))
    }
    onOpenChange(next)
  }

  async function handleConfirm() {
    setCompleting(true)
    setError('')
    try {
      await updateBookStatus({
        bookId: userBook.bookId,
        status: 'read',
        favourite,
        rating: String(rating)
      })
      await mutate(swrKeys.books)
      onOpenChange(false)
      onCompleted?.()
    } catch {
      setError('Failed to mark as completed. Please try again.')
    } finally {
      setCompleting(false)
    }
  }

  const displayed = hover > 0 ? hover : rating

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={handleOpenChange}
      title="Mark as completed"
      description="This moves the book to your read shelf and records today as the finish date."
      confirmLabel="Mark as completed"
      pendingLabel="Completing…"
      pending={completing}
      onConfirm={handleConfirm}
    >
      <div className="mt-4 flex items-center justify-between gap-3">
        <div
          className="flex items-center gap-0.5"
          aria-label={rating > 0 ? `${rating} out of 5 stars` : 'No rating'}
          onMouseLeave={() => setHover(0)}
        >
          {[1, 2, 3, 4, 5].map((star) => (
            <Button
              key={star}
              variant="ghost"
              size="iconSm"
              onClick={() => setRating(star === rating ? 0 : star)}
              onMouseEnter={() => setHover(star)}
              aria-label={`Rate ${star} star${star > 1 ? 's' : ''}`}
              className={cn(
                'h-auto w-auto p-0 leading-none hover:bg-transparent hover:text-amber-400',
                star <= displayed ? 'text-amber-400' : 'text-border'
              )}
            >
              ★
            </Button>
          ))}
        </div>

        <ToggleIconButton
          active={favourite}
          onToggle={() => setFavourite((prev) => !prev)}
          label="Add to favourites"
          activeLabel="Remove from favourites"
        >
          ♥
        </ToggleIconButton>
      </div>

      {error && (
        <p className="mt-2 text-sm text-danger" data-testid="mark-completed-error">
          {error}
        </p>
      )}
    </ConfirmDialog>
  )
}
