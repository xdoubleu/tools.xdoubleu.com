'use client'

import { type ReactNode } from 'react'
import { TogglePill } from '@/components/ui/toggle-pill'
import { cn } from '@/lib/cn'

interface SegmentedTabsOption<T extends string> {
  value: T
  label: ReactNode
}

interface SegmentedTabsProps<T extends string> {
  value: T
  onChange: (value: T) => void
  options: SegmentedTabsOption<T>[]
  /** Names the tablist for screen readers. */
  'aria-label': string
  className?: string
}

/** Single-choice switch between a few views or modes, rendered as a tablist. */
function SegmentedTabs<T extends string>({
  value,
  onChange,
  options,
  'aria-label': ariaLabel,
  className
}: SegmentedTabsProps<T>) {
  return (
    <div role="tablist" aria-label={ariaLabel} className={cn('flex flex-wrap gap-2', className)}>
      {options.map((option) => (
        <TogglePill
          key={option.value}
          role="tab"
          aria-selected={option.value === value}
          active={option.value === value}
          label={option.label}
          onClick={() => onChange(option.value)}
        />
      ))}
    </div>
  )
}

export { SegmentedTabs }
export type { SegmentedTabsProps, SegmentedTabsOption }
