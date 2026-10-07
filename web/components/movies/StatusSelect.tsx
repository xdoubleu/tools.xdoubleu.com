'use client'

import { Select } from '@/components/ui/select'
import { STATUSES, STATUS_LABELS } from '@/lib/movies/format'

interface StatusSelectProps {
  id: string
  value: string
  onChange: (status: string) => void
  /** Needed when no visible label names the control. */
  'aria-label'?: string
  disabled?: boolean
}

export default function StatusSelect({
  id,
  value,
  onChange,
  disabled,
  'aria-label': ariaLabel
}: StatusSelectProps) {
  return (
    <Select
      id={id}
      aria-label={ariaLabel}
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
    >
      {STATUSES.map((s) => (
        <option key={s} value={s}>
          {STATUS_LABELS[s]}
        </option>
      ))}
    </Select>
  )
}
