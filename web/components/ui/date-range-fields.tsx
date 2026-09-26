'use client'

import { DateInput } from '@/components/ui/date-input'
import { Field } from '@/components/ui/field'
import { cn } from '@/lib/cn'

interface DateRangeFieldsProps {
  /** Prefix for the two input ids (`<prefix>-from`, `<prefix>-to`); unique per page. */
  idPrefix: string
  start: string
  onStartChange: (value: string) => void
  end: string
  onEndChange: (value: string) => void
  className?: string
}

/** Labelled From/To date pair: stacked on phones, side by side from `sm`. */
function DateRangeFields({
  idPrefix,
  start,
  onStartChange,
  end,
  onEndChange,
  className
}: DateRangeFieldsProps) {
  const fromId = `${idPrefix}-from`
  const toId = `${idPrefix}-to`
  return (
    <div className={cn('flex w-full flex-col gap-3 sm:w-auto sm:flex-row', className)}>
      <Field label="From" htmlFor={fromId} className="sm:w-40">
        <DateInput id={fromId} value={start} onChange={onStartChange} />
      </Field>
      <Field label="To" htmlFor={toId} className="sm:w-40">
        <DateInput id={toId} value={end} onChange={onEndChange} />
      </Field>
    </div>
  )
}

export { DateRangeFields }
export type { DateRangeFieldsProps }
