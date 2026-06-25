'use client'

import { useState } from 'react'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBacklog'
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
import { Select } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Combobox } from '@/components/ui/combobox'

// Tags that have reserved UI treatment — not custom shelves.
const SPECIAL_TAGS = new Set([
  'favourite',
  'own-physical',
  'own-digital',
  'kobo-sync',
  'kobo-format-pdf'
])

const BOOK_STATUSES: { value: string; label: string }[] = [
  { value: 'to-read', label: 'Want to read' },
  { value: 'currently-reading', label: 'Currently reading' },
  { value: 'read', label: 'Read' },
  { value: 'dropped', label: 'Dropped' }
]

interface BookShelfModalProps {
  userBook: UserBook
  /** All shelf names in the user's library, used to seed the shelf combobox. */
  knownShelves: string[]
  onClose: () => void
  onSaved: () => void
}

export default function BookShelfModal({
  userBook,
  knownShelves,
  onClose,
  onSaved
}: BookShelfModalProps) {
  const [status, setStatus] = useState(userBook.status)
  const [shelves, setShelves] = useState<string[]>(
    userBook.tags.filter((t) => !SPECIAL_TAGS.has(t))
  )
  const [shelfInput, setShelfInput] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const toggleTag = useToggleTag()

  const book = userBook.book

  const originalShelves = userBook.tags.filter((t) => !SPECIAL_TAGS.has(t))

  const addShelf = (name: string) => {
    const trimmed = name.trim()
    if (!trimmed || shelves.includes(trimmed)) return
    setShelves((prev) => [...prev, trimmed])
    setShelfInput('')
  }

  const removeShelf = (name: string) => {
    setShelves((prev) => prev.filter((s) => s !== name))
  }

  // Suggestions = known shelves the book is not already on.
  const suggestions = knownShelves.filter((s) => !SPECIAL_TAGS.has(s) && !shelves.includes(s))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    setError(null)
    try {
      // Determine which shelf tags to add or remove.
      const toAdd = shelves.filter((s) => !originalShelves.includes(s))
      const toRemove = originalShelves.filter((s) => !shelves.includes(s))

      await Promise.all([
        updateBookStatus({
          bookId: userBook.id,
          status,
          // Pass through fields this modal doesn't own.
          favourite: userBook.tags.includes('favourite'),
          rating: String(userBook.rating),
          notes: userBook.notes
        }),
        ...toAdd.map((s) => toggleTag(userBook.id, s)),
        ...toRemove.map((s) => toggleTag(userBook.id, s))
      ])
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update shelf.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{book?.title ?? 'Move in library'}</DialogTitle>
          <DialogClose>×</DialogClose>
        </DialogHeader>
        {book && book.authors.length > 0 && (
          <p className="mb-4 text-sm text-muted">{book.authors.join(', ')}</p>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="shelf-status">Status</Label>
            <Select id="shelf-status" value={status} onChange={(e) => setStatus(e.target.value)}>
              {BOOK_STATUSES.map(({ value, label }) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </Select>
          </div>

          <div className="space-y-1.5">
            <Label>Shelves</Label>
            {shelves.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-2">
                {shelves.map((shelf) => (
                  <Badge key={shelf} variant="secondary" className="gap-1">
                    {shelf}
                    <button
                      type="button"
                      onClick={() => removeShelf(shelf)}
                      className="ml-1 text-muted hover:text-foreground leading-none"
                      aria-label={`Remove shelf ${shelf}`}
                    >
                      ×
                    </button>
                  </Badge>
                ))}
              </div>
            )}
            <div className="flex gap-2">
              <Combobox
                value={shelfInput}
                onChange={setShelfInput}
                onSelect={addShelf}
                onEnter={() => addShelf(shelfInput)}
                suggestions={suggestions}
                placeholder="Add a shelf..."
                aria-label="Shelf name"
                className="flex-1"
              />
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => addShelf(shelfInput)}
                disabled={!shelfInput.trim()}
              >
                Add
              </Button>
            </div>
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
