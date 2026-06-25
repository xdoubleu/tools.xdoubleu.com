'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBacklog'
import type { UserBook } from '@/lib/gen/backlog/v1/books_pb'
import { Popover, PopoverTrigger } from '@/components/ui/popover'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Combobox } from '@/components/ui/combobox'
import { Button } from '@/components/ui/button'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Checkbox } from '@/components/ui/checkbox'
import {
  SPECIAL_TAGS,
  BOOK_STATUSES,
  BUILT_IN_STATUSES,
  statusLabel,
  displayTags
} from '@/lib/backlog/bookShelves'

interface BookShelfTagCellProps {
  userBook: UserBook
  /** All known shelf names for autocomplete (custom + built-in). */
  knownShelves: string[]
  /** All known tag names for the checkbox list. */
  knownTags: string[]
  onSaved?: () => void
}

export default function BookShelfTagCell({
  userBook,
  knownShelves,
  knownTags,
  onSaved
}: BookShelfTagCellProps) {
  const [status, setStatus] = useState(userBook.status)
  const [tags, setTags] = useState<string[]>(displayTags(userBook.tags))
  const [newShelfInput, setNewShelfInput] = useState('')
  const [error, setError] = useState<string | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const toggleTag = useToggleTag()

  // All known shelves = built-in BOOK_STATUSES + custom (from library.shelves names)
  // Filter out ones already represented in the fixed list
  const customShelves = knownShelves.filter((s) => !BUILT_IN_STATUSES.has(s))
  const allShelfOptions = [...BOOK_STATUSES.map((s) => s.value), ...customShelves]

  const handleStatusChange = async (newStatus: string) => {
    const prev = status
    setStatus(newStatus)
    setError(null)
    try {
      await updateBookStatus({
        bookId: userBook.id,
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

  const handleTagToggle = async (tag: string, checked: boolean) => {
    const prev = [...tags]
    setTags(checked ? [...tags, tag] : tags.filter((t) => t !== tag))
    setError(null)
    try {
      await toggleTag(userBook.id, tag)
      mutate('/backlog/books')
      onSaved?.()
    } catch {
      setTags(prev)
      setError('Failed to update tag.')
    }
  }

  const addCustomShelf = async (name: string) => {
    const trimmed = name.trim()
    if (!trimmed) return
    setNewShelfInput('')
    setError(null)
    await handleStatusChange(trimmed)
  }

  // Shelf label for trigger
  const shelfDisplay = statusLabel(status)
  const tagCount = tags.length

  // Tags not already on the book for "add new tag" suggestions
  const tagSuggestions = knownTags.filter((t) => !tags.includes(t) && !SPECIAL_TAGS.has(t))

  return (
    <Popover
      align="right"
      trigger={({ open, onClick }) => (
        <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Edit shelf and tags">
          <span className="text-sm">{shelfDisplay}</span>
          {tagCount > 0 && <span className="ml-1 text-xs text-muted">+{tagCount}</span>}
        </PopoverTrigger>
      )}
    >
      <div className="space-y-4 min-w-56 max-w-72">
        {/* Shelf section (radio — mutually exclusive) */}
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold text-muted uppercase tracking-wide">Shelf</Label>
          <RadioGroup
            name={`shelf-${userBook.id}`}
            value={status}
            onChange={(v) => void handleStatusChange(v)}
          >
            {BOOK_STATUSES.map(({ value, label }) => (
              <RadioGroupItem key={value} value={value} label={label} />
            ))}
            {customShelves.map((s) => (
              <RadioGroupItem key={s} value={s} label={s} />
            ))}
          </RadioGroup>

          {/* Add a new custom shelf */}
          <div className="flex gap-1.5 mt-1">
            <Combobox
              value={newShelfInput}
              onChange={setNewShelfInput}
              onSelect={(v) => void addCustomShelf(v)}
              onEnter={() => void addCustomShelf(newShelfInput)}
              suggestions={allShelfOptions.filter((s) => !BOOK_STATUSES.some((b) => b.value === s))}
              placeholder="Custom shelf..."
              className="flex-1"
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => void addCustomShelf(newShelfInput)}
              disabled={!newShelfInput.trim()}
            >
              Set
            </Button>
          </div>
        </div>

        {/* Tags section (checkboxes — multi-select) */}
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold text-muted uppercase tracking-wide">Tags</Label>

          {knownTags.filter((t) => !SPECIAL_TAGS.has(t)).length === 0 && tags.length === 0 ? (
            <p className="text-xs text-muted">No tags yet.</p>
          ) : (
            <div className="flex flex-col gap-1 max-h-40 overflow-y-auto pr-1">
              {/* Known tags as checkboxes */}
              {knownTags
                .filter((t) => !SPECIAL_TAGS.has(t))
                .map((tag) => (
                  <Checkbox
                    key={tag}
                    id={`tag-${userBook.id}-${tag}`}
                    label={tag}
                    checked={tags.includes(tag)}
                    onChange={(e) => void handleTagToggle(tag, e.target.checked)}
                  />
                ))}
              {/* Render current tags not in knownTags list (edge case) */}
              {tags
                .filter((t) => !knownTags.includes(t) && !SPECIAL_TAGS.has(t))
                .map((tag) => (
                  <Checkbox
                    key={tag}
                    id={`tag-${userBook.id}-${tag}`}
                    label={tag}
                    checked
                    onChange={(e) => void handleTagToggle(tag, e.target.checked)}
                  />
                ))}
            </div>
          )}

          {/* Add new tag */}
          <div className="flex gap-1.5 mt-1">
            <Combobox
              value={''}
              onChange={() => {}}
              onSelect={(v) => void handleTagToggle(v, !tags.includes(v))}
              onEnter={() => {}}
              suggestions={tagSuggestions}
              placeholder="Add tag..."
              className="flex-1"
            />
          </div>
        </div>

        {/* Active tags as removable badges */}
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {tags.map((tag) => (
              <Badge key={tag} variant="secondary" className="gap-1">
                {tag}
                <button
                  type="button"
                  onClick={() => void handleTagToggle(tag, false)}
                  className="ml-1 text-muted hover:text-foreground leading-none"
                  aria-label={`Remove tag ${tag}`}
                >
                  x
                </button>
              </Badge>
            ))}
          </div>
        )}

        {error && <p className="text-xs text-danger">{error}</p>}
      </div>
    </Popover>
  )
}
