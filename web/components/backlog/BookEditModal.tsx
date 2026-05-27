'use client'

import { useState } from 'react'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBacklog'
import type { UpdateBookStatusInput } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'

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
      const req: UpdateBookStatusInput = { bookId: userBook.id, status, rating: String(rating), notes, favourite }
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
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={onClose}
    >
      <div
        className="bg-card rounded-lg shadow-xl p-6 w-full max-w-md mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-xl font-bold mb-1">{book?.title ?? 'Edit Book'}</h2>
        {book && book.authors.length > 0 && (
          <p className="text-sm text-muted mb-4">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="edit-status" className="block text-sm font-medium text-subtle mb-1">
              Status
            </label>
            <select
              id="edit-status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="w-full px-3 py-2 border border-input-border bg-input text-input-text rounded"
            >
              {BOOK_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s.charAt(0).toUpperCase() + s.slice(1)}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="edit-rating" className="block text-sm font-medium text-subtle mb-1">
              Rating (0 = unrated)
            </label>
            <select
              id="edit-rating"
              value={rating}
              onChange={(e) => setRating(Number(e.target.value))}
              className="w-full px-3 py-2 border border-input-border bg-input text-input-text rounded"
            >
              {[0, 1, 2, 3, 4, 5].map((r) => (
                <option key={r} value={r}>
                  {r === 0 ? 'No rating' : `${r} star${r > 1 ? 's' : ''}`}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="edit-notes" className="block text-sm font-medium text-subtle mb-1">
              Notes
            </label>
            <textarea
              id="edit-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              placeholder="Optional notes..."
              className="w-full px-3 py-2 border border-input-border bg-input text-input-text rounded resize-none"
            />
          </div>

          <div className="flex flex-wrap gap-4">
            <div className="flex items-center gap-2">
              <input
                id="edit-favourite"
                type="checkbox"
                checked={favourite}
                onChange={(e) => setFavourite(e.target.checked)}
                className="rounded"
              />
              <label htmlFor="edit-favourite" className="text-sm text-subtle cursor-pointer">
                Favourite
              </label>
            </div>
            <div className="flex items-center gap-2">
              <input
                id="edit-own-physical"
                type="checkbox"
                checked={ownPhysical}
                onChange={(e) => setOwnPhysical(e.target.checked)}
                className="rounded"
              />
              <label htmlFor="edit-own-physical" className="text-sm text-subtle cursor-pointer">
                Own physical
              </label>
            </div>
            <div className="flex items-center gap-2">
              <input
                id="edit-own-digital"
                type="checkbox"
                checked={ownDigital}
                onChange={(e) => setOwnDigital(e.target.checked)}
                className="rounded"
              />
              <label htmlFor="edit-own-digital" className="text-sm text-subtle cursor-pointer">
                Own digital
              </label>
            </div>
          </div>

          {error && <p className="text-red-600 text-sm">{error}</p>}

          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
            >
              {isSubmitting ? 'Saving...' : 'Save'}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 bg-surface text-fg rounded hover:bg-border"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
