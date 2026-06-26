'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { Popover, PopoverTrigger } from '@/components/ui/popover'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Combobox } from '@/components/ui/combobox'
import { Button } from '@/components/ui/button'

import { SPECIAL_TAGS, BOOK_STATUSES } from '@/lib/backlog/bookShelves'

interface BookShelfPopoverProps {
  userBook: UserBook
  knownShelves: string[]
  onSaved?: () => void
}

export default function BookShelfPopover({
  userBook,
  knownShelves,
  onSaved
}: BookShelfPopoverProps) {
  const [status, setStatus] = useState(userBook.status)
  const [shelves, setShelves] = useState<string[]>(
    userBook.tags.filter((t) => !SPECIAL_TAGS.has(t))
  )
  const [shelfInput, setShelfInput] = useState('')
  const [error, setError] = useState<string | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const toggleTag = useToggleTag()

  const handleStatusChange = async (newStatus: string) => {
    const prev = status
    setStatus(newStatus)
    setError(null)
    try {
      await updateBookStatus({
        bookId: userBook.bookId,
        status: newStatus,
        favourite: userBook.tags.includes('favourite'),
        rating: String(userBook.rating)
      })
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setStatus(prev)
      setError('Failed to update status.')
    }
  }

  const addTag = async (name: string) => {
    const trimmed = name.trim()
    if (!trimmed || shelves.includes(trimmed)) {
      setShelfInput('')
      return
    }
    setShelves((prev) => [...prev, trimmed])
    setShelfInput('')
    setError(null)
    try {
      await toggleTag(userBook.bookId, trimmed)
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setShelves((prev) => prev.filter((s) => s !== trimmed))
      setError('Failed to add tag.')
    }
  }

  const removeTag = async (name: string) => {
    setShelves((prev) => prev.filter((s) => s !== name))
    setError(null)
    try {
      await toggleTag(userBook.bookId, name)
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setShelves((prev) => [...prev, name])
      setError('Failed to remove tag.')
    }
  }

  const suggestions = knownShelves.filter((s) => !SPECIAL_TAGS.has(s) && !shelves.includes(s))

  const triggerLabel = shelves.length > 0 ? `Shelves & tags (${shelves.length})` : 'Shelves & tags'

  return (
    <Popover
      align="right"
      trigger={({ open, onClick }) => (
        <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Edit shelves and tags">
          {triggerLabel}
        </PopoverTrigger>
      )}
    >
      <div className="space-y-3 min-w-50">
        <div className="space-y-1">
          <Label htmlFor="shelf-popover-status" className="text-xs">
            Status
          </Label>
          <Select
            id="shelf-popover-status"
            value={status}
            onChange={(e) => void handleStatusChange(e.target.value)}
          >
            {BOOK_STATUSES.map(({ value, label }) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </Select>
        </div>

        <div className="space-y-1">
          <Label className="text-xs">Shelves & tags</Label>
          {shelves.length > 0 && (
            <div className="flex flex-wrap gap-1 mb-1.5">
              {shelves.map((shelf) => (
                <Badge key={shelf} variant="secondary" className="gap-1">
                  {shelf}
                  <button
                    type="button"
                    onClick={() => void removeTag(shelf)}
                    className="ml-1 text-muted hover:text-foreground leading-none"
                    aria-label={`Remove ${shelf}`}
                  >
                    ×
                  </button>
                </Badge>
              ))}
            </div>
          )}
          <div className="flex gap-1.5">
            <Combobox
              value={shelfInput}
              onChange={setShelfInput}
              onSelect={(v) => void addTag(v)}
              onEnter={() => void addTag(shelfInput)}
              suggestions={suggestions}
              placeholder="Add a shelf or tag..."
              aria-label="Shelf or tag name"
              className="flex-1"
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => void addTag(shelfInput)}
              disabled={!shelfInput.trim()}
            >
              Add
            </Button>
          </div>
        </div>

        {error && <p className="text-xs text-danger">{error}</p>}
      </div>
    </Popover>
  )
}
