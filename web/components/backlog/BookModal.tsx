'use client'

import { useState } from 'react'
import { useAddBook } from '@/hooks/useBacklog'
import { AddBookRequest } from '@/lib/gen/backlog/v1/books_pb'
import type { ExternalBookResult } from '@/lib/gen/backlog/v1/books_pb'

const BOOK_STATUSES = ['wishlist', 'reading', 'finished', 'dnf']

interface BookModalProps {
  book: ExternalBookResult | null
  onClose: () => void
  onAdded: () => void
}

export default function BookModal({ book, onClose, onAdded }: BookModalProps) {
  const [status, setStatus] = useState('wishlist')
  const [notes, setNotes] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const addBook = useAddBook()

  if (!book) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setError(null)
    try {
      await addBook(
        new AddBookRequest({
          provider: book.provider,
          providerId: book.providerId,
          title: book.title,
          author: book.authors.join(', '),
          status,
          isbn13: book.isbn13,
          coverUrl: book.coverUrl,
          description: book.description,
          ownPhysical: false,
          ownDigital: false
        })
      )
      onAdded()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add book.')
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
        <h2 className="text-xl font-bold mb-1">{book.title}</h2>
        {book.authors.length > 0 && (
          <p className="text-sm text-muted mb-4">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="status-select" className="block text-sm font-medium text-subtle mb-1">
              Status
            </label>
            <select
              id="status-select"
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
            <label htmlFor="notes-area" className="block text-sm font-medium text-subtle mb-1">
              Notes
            </label>
            <textarea
              id="notes-area"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              rows={3}
              placeholder="Optional notes..."
              className="w-full px-3 py-2 border border-input-border bg-input text-input-text rounded resize-none"
            />
          </div>

          {error && <p className="text-red-600 text-sm">{error}</p>}

          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
            >
              {isSubmitting ? 'Adding...' : 'Add Book'}
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
