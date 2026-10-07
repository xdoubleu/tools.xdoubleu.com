'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { todayISO, watchDay } from '@/lib/movies/format'

interface WatchDatesProps {
  /** Unique per page; names the date inputs. */
  idPrefix: string
  /** RFC3339 watches; '' is a watch with an unknown date. */
  dates: string[]
  pending: boolean
  /** date is YYYY-MM-DD, or '' for an unknown date. */
  onAdd: (date: string) => void
  onEdit: (index: number, date: string) => void
  onRemove: (index: number) => void
}

// Typing a year passes through 0002, 0020, 0202; earlier years and cleared
// inputs wait for blur.
const MIN_COMMIT_YEAR = 1900

function WatchDateInput({
  id,
  index,
  watchedAt,
  onEdit
}: {
  id: string
  index: number
  watchedAt: string
  onEdit: (index: number, date: string) => void
}) {
  const saved = watchDay(watchedAt)
  // A draft is dropped once the saved date changes under it; sent is the
  // last date handed to onEdit, so blur doesn't resend it.
  const [draft, setDraft] = useState({ from: saved, value: saved, sent: saved })
  const current = draft.from === saved ? draft : { from: saved, value: saved, sent: saved }

  const change = (value: string, commit: boolean) => {
    const send = commit && value !== current.sent
    setDraft({ from: saved, value, sent: send ? value : current.sent })
    if (send) onEdit(index, value)
  }

  return (
    <DateInput
      id={id}
      aria-label={`Watch ${index + 1} date`}
      className="w-full sm:w-44"
      value={current.value}
      onChange={(date) => change(date, Number(date.slice(0, 4)) >= MIN_COMMIT_YEAR)}
      onBlur={() => change(current.value, true)}
    />
  )
}

/** Editable list of watches; clearing a date makes it unknown. */
export default function WatchDates({
  idPrefix,
  dates,
  pending,
  onAdd,
  onEdit,
  onRemove
}: WatchDatesProps) {
  return (
    <div className="space-y-2">
      {dates.length > 0 && (
        <ul className="space-y-2">
          {dates.map((w, i) => (
            // Index keys: a date edit must not remount (and blur) its input.
            <li key={i} className="flex flex-wrap items-center gap-2">
              <WatchDateInput id={`${idPrefix}-${i}`} index={i} watchedAt={w} onEdit={onEdit} />
              {!w && <span className="text-sm text-muted">Date unknown</span>}
              <Button variant="ghost" disabled={pending} onClick={() => onRemove(i)}>
                Remove
              </Button>
            </li>
          ))}
        </ul>
      )}
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" disabled={pending} onClick={() => onAdd(todayISO())}>
          Watched today
        </Button>
        <Button variant="secondary" disabled={pending} onClick={() => onAdd('')}>
          Add unknown date
        </Button>
      </div>
    </div>
  )
}
