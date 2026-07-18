'use client'

import { useState } from 'react'
import { useCreateBook } from '@/hooks/useBooks'
import type { CreateBookInput } from '@/hooks/useBooks'
import type { ExternalBookResult } from '@/lib/gen/reading/v1/library_pb'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { BOOK_STATUSES } from '@/lib/reading/bookShelves'

interface BookModalProps {
  book: ExternalBookResult | null
  onClose: () => void
  onAdded: () => void
}

export default function BookModal({ book, onClose, onAdded }: BookModalProps) {
  const [status, setStatus] = useState('to-read')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const addBook = useCreateBook()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!book) return
    setIsSubmitting(true)
    setError(null)
    try {
      const req: CreateBookInput = {
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
      }
      await addBook(req)
      onAdded()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add book.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={!!book} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{book?.title ?? ''}</DialogTitle>
          <DialogClose>×</DialogClose>
        </DialogHeader>
        {book?.authors && book.authors.length > 0 && (
          <p className="mb-4 text-sm text-muted">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="status-select">Status</Label>
            <Select id="status-select" value={status} onChange={(e) => setStatus(e.target.value)}>
              {BOOK_STATUSES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </Select>
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={isSubmitting} className="flex-1">
              {isSubmitting ? 'Adding…' : 'Add Book'}
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
