'use client'

import { useState } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { mutate } from 'swr'
import { useUpdateBookStatus, useToggleTag } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/books/v1/library_pb'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Checkbox } from '@/components/ui/checkbox'
import {
  SPECIAL_TAGS,
  BOOK_STATUSES,
  BUILT_IN_STATUSES,
  displayTags
} from '@/lib/books/bookShelves'

interface BookShelfTagFieldsProps {
  userBook: UserBook
  /** All known custom shelf names (excludes built-in statuses). */
  knownShelves: string[]
  /** All known tag names for the checkbox list. */
  knownTags: string[]
  onSaved?: () => void
}

/**
 * Shared select-only body for per-book shelf/tag popovers.
 * Renders a radio group (shelf) and checkboxes (tags) for existing items only.
 * Creating or deleting shelves/tags is handled in the sidebar Manage dialog.
 */
export default function BookShelfTagFields({
  userBook,
  knownShelves,
  knownTags,
  onSaved
}: BookShelfTagFieldsProps) {
  const [status, setStatus] = useState(userBook.status)
  const [tags, setTags] = useState<string[]>(displayTags(userBook.tags))
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

  const visibleKnownTags = knownTags.filter((t) => !SPECIAL_TAGS.has(t))
  const orphanTags = tags.filter((t) => !knownTags.includes(t) && !SPECIAL_TAGS.has(t))
  const noTags = visibleKnownTags.length === 0 && tags.length === 0

  return (
    <div className="space-y-4 min-w-56 max-w-72">
      {/* Shelf — single-select via radio group */}
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
      </div>

      {/* Tags — multi-select via checkboxes */}
      <div className="space-y-1.5">
        <Label className="text-xs font-semibold text-muted uppercase tracking-wide">Tags</Label>
        {noTags ? (
          <p className="text-xs text-muted">No tags yet.</p>
        ) : (
          <div className="flex flex-col gap-1 max-h-40 overflow-y-auto pr-1">
            {visibleKnownTags.map((tag) => (
              <Checkbox
                key={tag}
                id={`tag-${userBook.id}-${tag}`}
                label={tag}
                checked={tags.includes(tag)}
                onChange={(e) => void handleTagToggle(tag, e.target.checked)}
              />
            ))}
            {/* Tags on this book not in the known list (edge case) */}
            {orphanTags.map((tag) => (
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
      </div>

      {error && <p className="text-xs text-danger">{error}</p>}
    </div>
  )
}
