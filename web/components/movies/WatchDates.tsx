'use client'

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
              <DateInput
                id={`${idPrefix}-${i}`}
                aria-label={`Watch ${i + 1} date`}
                className="w-full sm:w-44"
                value={watchDay(w)}
                onChange={(date) => onEdit(i, date)}
              />
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
