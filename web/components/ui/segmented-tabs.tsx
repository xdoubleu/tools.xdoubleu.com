import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'
import { controlHeightClass } from '@/components/ui/control'

interface SegmentedTabsOption<T extends string> {
  value: T
  label: ReactNode
}

interface SegmentedTabsProps<T extends string> {
  /** `null` when no option is selected. */
  value: T | null
  onChange: (value: T) => void
  options: SegmentedTabsOption<T>[]
  /** Names the tablist for screen readers. */
  'aria-label': string
  className?: string
}

// Stryker disable StringLiteral: class names are presentation; the selected state is tested.
/**
 * The one way to switch between a few views, sibling sections or a scope (Mine/Family):
 * a single track whose selected segment slides to the tapped option. Segments share the
 * track equally. Filters are `TogglePill`s and actions are `Button`s, never this.
 */
function SegmentedTabs<T extends string>({
  value,
  onChange,
  options,
  'aria-label': ariaLabel,
  className
}: SegmentedTabsProps<T>) {
  const selected = options.findIndex((option) => option.value === value)
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={cn(
        'relative inline-grid auto-cols-fr grid-flow-col rounded-xl bg-hover p-0.5',
        className
      )}
    >
      {selected >= 0 && (
        <span
          aria-hidden
          className="absolute inset-y-0.5 left-0.5 rounded-[10px] bg-card shadow-card transition-transform duration-settle ease-spring motion-reduce:transition-none"
          style={{
            width: `calc((100% - 4px) / ${options.length})`,
            transform: `translateX(${selected * 100}%)`
          }}
        />
      )}
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={option.value === value}
          onClick={() => onChange(option.value)}
          className={cn(
            'relative z-10 flex min-w-11 cursor-pointer items-center justify-center whitespace-nowrap px-3 text-sm font-medium',
            controlHeightClass,
            'rounded-[10px] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
            option.value === value ? 'cursor-default text-fg' : 'text-muted hover:text-fg'
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}

// Stryker restore StringLiteral

export { SegmentedTabs }
export type { SegmentedTabsProps, SegmentedTabsOption }
