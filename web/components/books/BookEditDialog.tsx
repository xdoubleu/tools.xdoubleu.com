'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import type { Book } from '@/lib/gen/books/v1/library_pb'
import { useUpdateBook } from '@/hooks/useBooks'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Alert } from '@/components/ui/alert'
import { Field } from '@/components/ui/field'
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
      <DialogContent side="sheet">
        <DialogHeader>
          <DialogTitle>Edit book</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <Field label="Title" htmlFor="edit-book-title">
            <Input id="edit-book-title" value={title} onChange={(e) => setTitle(e.target.value)} />
          </Field>

          <Field label="Authors (comma-separated)" htmlFor="edit-book-authors">
            <Input
              id="edit-book-authors"
              value={authors}
              onChange={(e) => setAuthors(e.target.value)}
            />
          </Field>

          <Field label="ISBN-13" htmlFor="edit-book-isbn">
            <Input
              id="edit-book-isbn"
              inputMode="numeric"
              value={isbn13}
              onChange={(e) => setIsbn13(e.target.value)}
            />
          </Field>

          <Field label="Description" htmlFor="edit-book-description">
            <Textarea
              id="edit-book-description"
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </Field>

          <Field label="Page count" htmlFor="edit-book-pages">
            <Input
              id="edit-book-pages"
              type="number"
              inputMode="numeric"
              min={0}
              value={pageCount}
              onChange={(e) => setPageCount(e.target.value)}
            />
          </Field>

          <Field label="Cover URL" htmlFor="edit-book-cover">
            <div className="flex gap-2">
              <Input
                id="edit-book-cover"
                type="url"
                className="min-w-0 flex-1"
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
          </Field>
        </div>

        {error && (
          <Alert tone="danger" className="mt-2" data-testid="edit-book-error">
            {error}
          </Alert>
        )}

        <DialogFooter>
          <Button variant="ghost" disabled={saving} onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving} data-testid="edit-book-save-btn">
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
