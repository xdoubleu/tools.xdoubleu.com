'use client'

import { useState } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { mutate } from 'swr'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import { Label } from '@/components/ui/label'
import { Combobox } from '@/components/ui/combobox'
import { Button } from '@/components/ui/button'
import TogglePill from '@/components/reading/TogglePill'
import {
  SPECIAL_TAGS,
  BOOK_STATUSES,
  BUILT_IN_STATUSES,
  displayTags
} from '@/lib/reading/bookShelves'

interface BookShelfTagFieldsProps {
  userBook: UserBook
  /** All known custom shelf names (excludes built-in statuses). */
  knownShelves: string[]
  /** All known tag names for the checkbox list. */
  knownTags: string[]
  onSaved?: () => void
}

/**
 * Inline shelf/tag editor for the book detail page.
 * Renders shelf and tags as the same toggle-pill control (single-select for
 * shelf, multi-select for tags) — one click toggles, no popover or checkbox
 * list. New tags and new shelves are added via a combobox, hidden behind an
 * "Add" button until needed. Renaming or deleting shelves/tags is handled in
 * the sidebar Manage dialog.
 */
export default function BookShelfTagFields({
  userBook,
  knownShelves,
  knownTags,
  onSaved
}: BookShelfTagFieldsProps) {
  const [status, setStatus] = useState(userBook.status)
  const [tags, setTags] = useState<string[]>(displayTags(userBook.tags))
  const [newTag, setNewTag] = useState('')
  const [newShelf, setNewShelf] = useState('')
  const [showTagInput, setShowTagInput] = useState(false)
  const [showShelfInput, setShowShelfInput] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const updateBookStatus = useUpdateBookStatus()
  const toggleTag = useToggleTag()

  const customShelves = knownShelves.filter((s) => !BUILT_IN_STATUSES.has(s))

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
      mutate(swrKeys.books)
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
      await toggleTag(userBook.bookId, tag)
      mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setTags(prev)
      setError('Failed to update tag.')
    }
  }

  const handleAddTag = async (tag: string) => {
    setNewTag('')
    if (!tag || tags.includes(tag)) return
    await handleTagToggle(tag, true)
  }

  const handleAddShelf = async (shelf: string) => {
    setNewShelf('')
    setShowShelfInput(false)
    if (!shelf || shelf === status) return
    await handleStatusChange(shelf)
  }

  const visibleKnownTags = knownTags.filter((t) => !SPECIAL_TAGS.has(t))
  // Tags on this book not in the known list (edge case) plus known ones, deduped.
  const allTags = [...new Set([...visibleKnownTags, ...tags])]
  const addableTags = knownTags.filter((t) => !SPECIAL_TAGS.has(t) && !tags.includes(t))
  const addableShelves = customShelves.filter((s) => s !== status)

  return (
    <div className="space-y-4">
      {/* Shelf — single-select toggle pills */}
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-muted uppercase tracking-wide">Shelf</Label>
        <div className="flex flex-wrap items-center gap-1.5">
          {BOOK_STATUSES.map(({ value, label }) => (
            <TogglePill
              key={value}
              label={label}
              active={status === value}
              onClick={() => void handleStatusChange(value)}
            />
          ))}
          {customShelves.map((s) => (
            <TogglePill
              key={s}
              label={s}
              active={status === s}
              onClick={() => void handleStatusChange(s)}
            />
          ))}
          {!showShelfInput && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-xs"
              onClick={() => setShowShelfInput(true)}
            >
              + Add shelf
            </Button>
          )}
        </div>
        {showShelfInput && (
          <Combobox
            value={newShelf}
            onChange={setNewShelf}
            onSelect={(shelf) => void handleAddShelf(shelf)}
            onEnter={() => void handleAddShelf(newShelf)}
            suggestions={addableShelves}
            placeholder="New or existing shelf…"
            aria-label="Add a shelf"
            className="mt-1 max-w-56"
            autoFocus
          />
        )}
      </div>

      {/* Tags — clickable chips toggle in place, no popover/checkbox list */}
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-muted uppercase tracking-wide">Tags</Label>
        {allTags.length === 0 ? (
          <p className="text-xs text-muted">No tags yet.</p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {allTags.map((tag) => {
              const active = tags.includes(tag)
              return (
                <TogglePill
                  key={tag}
                  label={tag}
                  active={active}
                  onClick={() => void handleTagToggle(tag, !active)}
                />
              )
            })}
          </div>
        )}
        {showTagInput ? (
          <Combobox
            value={newTag}
            onChange={setNewTag}
            onSelect={(tag) => void handleAddTag(tag)}
            onEnter={() => void handleAddTag(newTag)}
            suggestions={addableTags}
            placeholder="Add a tag…"
            aria-label="Add a tag"
            className="mt-1 max-w-56"
            autoFocus
          />
        ) : (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="text-xs"
            onClick={() => setShowTagInput(true)}
          >
            + Add tag
          </Button>
        )}
      </div>

      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}
