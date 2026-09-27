'use client'

import type { DuplicateGroup } from '@/lib/gen/books/v1/catalog_pb'
import type { BookConflictField, FieldConflict } from './duplicateConflicts'
import BookCover from '@/components/books/BookCover'
import { Label } from '@/components/ui/label'
import { Radio } from '@/components/ui/radio-group'
import { cn } from '@/lib/cn'

const FIELD_LABELS: Record<BookConflictField, string> = {
  status: 'Shelf / status',
  title: 'Title',
  authors: 'Authors',
  isbn13: 'ISBN-13',
  cover: 'Cover',
  description: 'Description',
  pageCount: 'Page count'
}

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
    <Label className="flex min-w-11 cursor-pointer flex-col items-center gap-1 font-normal">
      <Radio
        name={`cover-${groupKey}`}
        value={bookId}
        checked={checked}
        onChange={onChange}
        className="sr-only text-base"
      />
      <div
        className={cn(
          'overflow-hidden rounded-lg border-2 transition-colors',
          checked ? 'border-accent' : 'border-transparent'
        )}
      >
        <BookCover coverUrl={coverUrl} title={title} size="sm" />
      </div>
      <span className="text-xs text-muted">{checked ? 'Selected' : 'Use this'}</span>
    </Label>
  )
}

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
              <div className="flex flex-wrap gap-3">
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
                  <Label
                    key={c.bookId}
                    className={cn(
                      'flex min-h-11 min-w-0 max-w-full cursor-pointer items-center gap-1.5 rounded-lg border px-3 py-1 text-xs font-normal transition-colors sm:min-h-8',
                      'has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-accent/50',
                      chosen === c.bookId
                        ? 'border-accent bg-accent/5 text-fg'
                        : 'border-border text-muted hover:border-muted'
                    )}
                  >
                    <Radio
                      name={`${field}-${groupKey}`}
                      value={c.bookId}
                      checked={chosen === c.bookId}
                      onChange={() => onChoiceChange(field, c.bookId)}
                      className="sr-only text-base"
                    />
                    <span className={cn('min-w-0 break-words', !c.hasValue && 'italic')}>
                      {c.displayValue}
                    </span>
                  </Label>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
