'use client'

import { useState } from 'react'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBacklog'
import type { UpdateBookStatusInput } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'

const BOOK_STATUSES = ['wishlist', 'reading', 'finished', 'dnf']

interface BookEditModalProps {
  userBook: UserBook
  onClose: () => void
  onSaved: () => void
}

export default function BookEditModal({ userBook, onClose, onSaved }: BookEditModalProps) {
  const [status, setStatus] = useState(userBook.status)
  const [rating, setRating] = useState(userBook.rating)
  const [notes, setNotes] = useState(userBook.notes)
  const [favourite, setFavourite] = useState(userBook.tags.includes('favourite'))
  const [ownPhysical, setOwnPhysical] = useState(userBook.tags.includes('own-physical'))
  const [ownDigital, setOwnDigital] = useState(userBook.tags.includes('own-digital'))
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const toggleTag = useToggleTag()

  const book = userBook.book

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setError(null)
    try {
      const currentOwnPhysical = userBook.tags.includes('own-physical')
      const currentOwnDigital = userBook.tags.includes('own-digital')
      const req: UpdateBookStatusInput = {
        bookId: userBook.id,
        status,
        rating: String(rating),
        notes,
        favourite
      }
      await Promise.all([
        updateBookStatus(req),
        ownPhysical !== currentOwnPhysical && toggleTag(userBook.id, 'own-physical'),
        ownDigital !== currentOwnDigital && toggleTag(userBook.id, 'own-digital')
      ])
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update book.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{book?.title ?? 'Edit Book'}</DialogTitle>
          <DialogClose>×</DialogClose>
        </DialogHeader>
        {book && book.authors.length > 0 && (
          <p className="mb-4 text-sm text-muted">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="edit-status">Status</Label>
            <select
              id="edit-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              {BOOK_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s.charAt(0).toUpperCase() + s.slice(1)}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="edit-rating">Rating (0 = unrated)</Label>
            <select
              id="edit-rating"
              value={rating}
              onChange={(e) => setRating(Number(e.target.value))}
              className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              {[0, 1, 2, 3, 4, 5].map((r) => (
                <option key={r} value={r}>
                  {r === 0 ? 'No rating' : `${r} star${r > 1 ? 's' : ''}`}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="edit-notes">Notes</Label>
            <textarea
              id="edit-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              placeholder="Optional notes..."
              className="w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text resize-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            />
          </div>

          <div className="flex flex-wrap gap-4">
            {[
              { id: 'edit-favourite', state: favourite, setter: setFavourite, label: 'Favourite' },
              {
                id: 'edit-own-physical',
                state: ownPhysical,
                setter: setOwnPhysical,
                label: 'Own physical'
              },
              {
                id: 'edit-own-digital',
                state: ownDigital,
                setter: setOwnDigital,
                label: 'Own digital'
              }
            ].map(({ id, state, setter, label }) => (
              <div key={id} className="flex items-center gap-2">
                <input
                  id={id}
                  type="checkbox"
                  checked={state}
                  onChange={(e) => setter(e.target.checked)}
                  className="rounded accent-[rgb(var(--color-accent))]"
                />
                <label htmlFor={id} className="text-sm text-subtle cursor-pointer">
                  {label}
                </label>
              </div>
            ))}
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={isSubmitting} className="flex-1">
              {isSubmitting ? 'Saving...' : 'Save'}
            </Button>
            <Button type="button" variant="secondary" onClick={onClose} className="flex-1">
              Cancel
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
