'use client'

import { useState } from 'react'
import { useCreateBook, useLibrary } from '@/hooks/useBooks'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { BOOK_STATUSES, BUILT_IN_STATUSES } from '@/lib/books/bookShelves'

interface AddManualBookDialogProps {
  initialTitle?: string
  onClose: () => void
  onAdded: () => void
}

// Rendered only while the "add manually" flow is active (see BooksLibrary) —
// unlike BookDialog, which stays mounted and toggles via `open={!!book}`,
// this component owns its own form state, so the parent conditionally
// mounts/unmounts it instead of passing an `open` flag.
export default function AddManualBookDialog({
  initialTitle = '',
  onClose,
  onAdded
}: AddManualBookDialogProps) {
  const [title, setTitle] = useState(initialTitle)
  const [author, setAuthor] = useState('')
  const [isbn13, setIsbn13] = useState('')
  const [coverUrl, setCoverUrl] = useState('')
  const [description, setDescription] = useState('')
  const [status, setStatus] = useState('to-read')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const addBook = useCreateBook()
  const { data: libraryData } = useLibrary()
  const customShelves = (libraryData?.library?.shelves ?? [])
    .map((s) => s.name)
    .filter((s) => !BUILT_IN_STATUSES.has(s))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    setIsSubmitting(true)
    setError(null)
    try {
      await addBook({
        provider: 'manual',
        providerId: '',
        title: title.trim(),
        author: author.trim(),
        status,
        isbn13: isbn13.trim(),
        coverUrl: coverUrl.trim(),
        description: description.trim(),
        ownPhysical: false,
        ownDigital: false
      })
      onAdded()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add book.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add book manually</DialogTitle>
          <DialogClose>×</DialogClose>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="manual-title">Title</Label>
            <Input
              id="manual-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="manual-author">Author</Label>
            <Input id="manual-author" value={author} onChange={(e) => setAuthor(e.target.value)} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="manual-isbn13">ISBN-13</Label>
            <Input id="manual-isbn13" value={isbn13} onChange={(e) => setIsbn13(e.target.value)} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="manual-cover-url">Cover URL</Label>
            <Input
              id="manual-cover-url"
              value={coverUrl}
              onChange={(e) => setCoverUrl(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="manual-description">Description</Label>
            <Textarea
              id="manual-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="manual-status-select">Status</Label>
            <Select
              id="manual-status-select"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
            >
              {BOOK_STATUSES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
              {customShelves.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </Select>
          </div>

          {error && <p className="text-sm text-danger">{error}</p>}

          <div className="flex gap-2 pt-2">
            <Button type="submit" disabled={isSubmitting || !title.trim()} className="flex-1">
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
