import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

interface ToolbarProps {
  /** Leading search field; it takes the remaining width and wraps to its own row on phones. */
  search?: ReactNode
  /** Filter controls (`Select`, `TogglePill`s, `SegmentedTabs`) after the search. */
  children?: ReactNode
  /** Right-aligned trailing slot: a result count or a view toggle. */
  trailing?: ReactNode
  /** Names the toolbar for screen readers, e.g. "Filter books". */
  'aria-label'?: string
  className?: string
}

/** The search-and-filter row between `PageHeader` and a list; every list page uses this one. */
function Toolbar({ search, children, trailing, 'aria-label': ariaLabel, className }: ToolbarProps) {
  return (
    <div
      role="toolbar"
      aria-label={ariaLabel}
      className={cn('mb-4 flex flex-wrap items-center gap-2', className)}
    >
      {search !== undefined && (
        <div className="min-w-0 basis-full sm:basis-60 sm:flex-1">{search}</div>
      )}
      {children}
      {trailing !== undefined && (
        <div className="ml-auto flex shrink-0 items-center gap-2 text-sm text-muted">
          {trailing}
        </div>
      )}
    </div>
  )
}

export { Toolbar }
export type { ToolbarProps }
