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

/** Native date input on every viewport; its value is always 'YYYY-MM-DD' or ''. */
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
