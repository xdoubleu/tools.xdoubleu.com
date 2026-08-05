'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import type { Book } from '@/lib/gen/books/v1/library_pb'
import { useUpdateBook } from '@/hooks/useBooks'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { swrKeys } from '@/lib/swrKeys'

interface BookEditDialogProps {
  book: Book
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}

export default function BookEditDialog({ book, open, onOpenChange, onSaved }: BookEditDialogProps) {
  const updateBook = useUpdateBook()
  const [title, setTitle] = useState(book.title)
  const [authors, setAuthors] = useState(book.authors.join(', '))
  const [isbn13, setIsbn13] = useState(book.isbn13)
  const [description, setDescription] = useState(book.description)
  const [pageCount, setPageCount] = useState(String(book.pageCount || ''))
  const [coverUrl, setCoverUrl] = useState(book.coverUrl)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  function handleOpenChange(next: boolean) {
    if (!next) setError('')
    onOpenChange(next)
  }

  async function handleSave() {
    setSaving(true)
    setError('')
    try {
      await updateBook(book.id, {
        title,
        authors: authors
          .split(',')
          .map((a) => a.trim())
          .filter(Boolean),
        isbn13,
        description,
        pageCount: Number(pageCount) || 0,
        coverUrl
      })
      await mutate(swrKeys.books)
      onOpenChange(false)
      onSaved()
    } catch {
      setError('Failed to save changes. Please try again.')
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit book</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <div>
            <p className="text-xs text-muted mb-1">Title</p>
            <Input value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>

          <div>
            <p className="text-xs text-muted mb-1">Authors (comma-separated)</p>
            <Input value={authors} onChange={(e) => setAuthors(e.target.value)} />
          </div>

          <div>
            <p className="text-xs text-muted mb-1">ISBN-13</p>
            <Input value={isbn13} onChange={(e) => setIsbn13(e.target.value)} />
          </div>

          <div>
            <p className="text-xs text-muted mb-1">Description</p>
            <Textarea
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div>
            <p className="text-xs text-muted mb-1">Page count</p>
            <Input
              type="number"
              min={0}
              value={pageCount}
              onChange={(e) => setPageCount(e.target.value)}
            />
          </div>

          <div>
            <p className="text-xs text-muted mb-1">Cover URL</p>
            <div className="flex gap-2">
              <Input
                value={coverUrl}
                onChange={(e) => setCoverUrl(e.target.value)}
                placeholder="No cover"
              />
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={!coverUrl}
                onClick={() => setCoverUrl('')}
              >
                Remove
              </Button>
            </div>
          </div>
        </div>

        {error && (
          <p className="mt-2 text-sm text-danger" data-testid="edit-book-error">
            {error}
          </p>
        )}

        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={saving} onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving} data-testid="edit-book-save-btn">
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
