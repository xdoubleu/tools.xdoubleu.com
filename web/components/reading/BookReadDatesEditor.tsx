'use client'

import { useState } from 'react'
import { mutate } from 'swr'
import { useUpdateFinishedAt } from '@/hooks/useBooks'
import type { UserBook } from '@/lib/gen/reading/v1/library_pb'
import { DateInput } from '@/components/ui/date-input'
import { Button } from '@/components/ui/button'
import { swrKeys } from '@/lib/swrKeys'

interface BookReadDatesEditorProps {
  userBook: UserBook
  onSaved?: () => void
}

function toDateInputValue(iso: string): string {
  return iso.slice(0, 10)
}

/** Full CRUD over a book's read-date history (finished_at). */
export default function BookReadDatesEditor({ userBook, onSaved }: BookReadDatesEditorProps) {
  const [dates, setDates] = useState<string[]>(userBook.finishedAt.map(toDateInputValue))
  const [error, setError] = useState<string | null>(null)
  const updateFinishedAt = useUpdateFinishedAt()

  const save = async (next: string[]) => {
    const prev = dates
    setDates(next)
    setError(null)
    try {
      await updateFinishedAt(userBook.bookId, next.filter(Boolean))
      void mutate(swrKeys.books)
      onSaved?.()
    } catch {
      setDates(prev)
      setError('Failed to update read dates.')
    }
  }

  return (
    <div>
      <p className="text-xs text-muted mb-1">{dates.length === 1 ? 'Finished' : 'Read dates'}</p>
      <div className="flex flex-col gap-1.5">
        {dates.map((date, i) => (
          <div key={i} className="flex items-center gap-2">
            <DateInput
              value={date}
              onChange={(v) => {
                const next = [...dates]
                next[i] = v
                setDates(next)
              }}
              onBlur={() => void save(dates)}
              className="h-9 w-40"
              aria-label={`Read date ${i + 1}`}
            />
            <Button
              type="button"
              variant="ghost"
              size="iconSm"
              aria-label="Remove this date"
              onClick={() => void save(dates.filter((_, j) => j !== i))}
            >
              ×
            </Button>
          </div>
        ))}
        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="self-start text-xs"
          onClick={() => setDates([...dates, ''])}
        >
          Add date
        </Button>
      </div>
      {error && <p className="mt-1 text-xs text-danger">{error}</p>}
    </div>
  )
}
