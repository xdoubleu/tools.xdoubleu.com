'use client'

import { Input } from '@/components/ui/input'
import { cn } from '@/lib/cn'

interface DateInputProps {
  /** 'YYYY-MM-DD' or '' */
  value: string
  onChange: (value: string) => void
  onBlur?: () => void
  id?: string
  className?: string
  'aria-label'?: string
}

/**
 * Native `<input type="date">` on every viewport — the browser/OS renders
 * its own locale formatting and picker UI, matching how the time field
 * (`Input type="time"`) already behaves. A `type="date"` input's value is
 * always 'YYYY-MM-DD' or '', so no display/ISO conversion layer is needed.
 */
export function DateInput({
  value,
  onChange,
  onBlur,
  id,
  className,
  'aria-label': ariaLabel
}: DateInputProps) {
  return (
    <Input
      type="date"
      id={id}
      aria-label={ariaLabel}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      className={cn('h-11 w-full', className)}
    />
  )
}
