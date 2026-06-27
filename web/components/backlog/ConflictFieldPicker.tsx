'use client'

import type { DuplicateGroup } from '@/lib/gen/backlog/v1/books_pb'
import type { BookConflictField, FieldConflict } from './duplicateConflicts'
import BookCover from '@/components/backlog/BookCover'

// ---------------------------------------------------------------------------
// Field label map
// ---------------------------------------------------------------------------

const FIELD_LABELS: Record<BookConflictField, string> = {
  title: 'Title',
  authors: 'Authors',
  isbn13: 'ISBN-13',
  isbn10: 'ISBN-10',
  cover: 'Cover',
  description: 'Description',
  pageCount: 'Page count',
  externalRefs: 'External refs'
}

// ---------------------------------------------------------------------------
// CoverChoice — renders a cover thumbnail for the cover field picker
// ---------------------------------------------------------------------------

interface CoverChoiceProps {
  bookId: string
  coverUrl: string
  title: string
  checked: boolean
  onChange: () => void
  groupKey: string
}

function CoverChoice({ bookId, coverUrl, title, checked, onChange, groupKey }: CoverChoiceProps) {
  return (
    <label className="flex flex-col items-center gap-1 cursor-pointer">
      <input
        type="radio"
        name={`cover-${groupKey}`}
        value={bookId}
        checked={checked}
        onChange={onChange}
        className="sr-only"
      />
      <div
        className={`rounded-lg overflow-hidden border-2 transition-colors ${
          checked ? 'border-primary' : 'border-transparent'
        }`}
      >
        <BookCover coverUrl={coverUrl} title={title} size="sm" />
      </div>
      <span className="text-xs text-muted">{checked ? 'Selected' : 'Use this'}</span>
    </label>
  )
}

// ---------------------------------------------------------------------------
// ConflictFieldPicker
// ---------------------------------------------------------------------------

interface ConflictFieldPickerProps {
  group: DuplicateGroup
  conflicts: FieldConflict[]
  /** fieldChoices[field] = bookId of the chosen entry */
  fieldChoices: Partial<Record<BookConflictField, string>>
  onChoiceChange: (field: BookConflictField, bookId: string) => void
  groupKey: string
}

export default function ConflictFieldPicker({
  group,
  conflicts,
  fieldChoices,
  onChoiceChange,
  groupKey
}: ConflictFieldPickerProps) {
  if (conflicts.length === 0) return null

  const bookById = new Map(group.entries.filter((e) => e.book).map((e) => [e.bookId, e.book!]))

  return (
    <div className="mt-3 space-y-3 border-t border-border pt-3">
      <p className="text-xs font-medium text-muted">
        Resolve {conflicts.length} conflicting {conflicts.length === 1 ? 'field' : 'fields'}
      </p>

      {conflicts.map(({ field, choices }) => {
        const chosen = fieldChoices[field]

        return (
          <div key={field} className="space-y-1">
            <p className="text-xs text-subtle">{FIELD_LABELS[field]}</p>

            {field === 'cover' ? (
              <div className="flex gap-3">
                {choices.map((c) => {
                  const book = bookById.get(c.bookId)
                  return (
                    <CoverChoice
                      key={c.bookId}
                      bookId={c.bookId}
                      coverUrl={book?.coverUrl ?? ''}
                      title={book?.title ?? ''}
                      checked={chosen === c.bookId}
                      onChange={() => onChoiceChange(field, c.bookId)}
                      groupKey={groupKey}
                    />
                  )
                })}
              </div>
            ) : (
              <div className="flex flex-wrap gap-2">
                {choices.map((c) => (
                  <label
                    key={c.bookId}
                    className={`flex items-center gap-1.5 cursor-pointer rounded-lg border px-2 py-1 text-xs transition-colors ${
                      chosen === c.bookId
                        ? 'border-primary bg-primary/5 text-foreground'
                        : 'border-border text-muted hover:border-muted'
                    }`}
                  >
                    <input
                      type="radio"
                      name={`${field}-${groupKey}`}
                      value={c.bookId}
                      checked={chosen === c.bookId}
                      onChange={() => onChoiceChange(field, c.bookId)}
                      className="sr-only"
                    />
                    <span className={c.hasValue ? '' : 'italic'}>{c.displayValue}</span>
                  </label>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
