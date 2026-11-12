import { type HTMLAttributes, type LiHTMLAttributes } from 'react'
import { cn } from '@/lib/cn'
import { controlMinHeightClass } from '@/components/ui/control'

/**
 * A list of single-line items on one rounded surface with hairline dividers, instead of a stack
 * of bordered cards. Rows are `GroupedListItem`s; give the list an `aria-label`.
 */
function GroupedList({ className, ...props }: HTMLAttributes<HTMLUListElement>) {
  return (
    <ul
      className={cn(
        'divide-y divide-border overflow-hidden rounded-2xl bg-card shadow-card',
        className
      )}
      {...props}
    />
  )
}

/** One row of a `GroupedList`, at least a tap target tall. */
function GroupedListItem({ className, ...props }: LiHTMLAttributes<HTMLLIElement>) {
  return (
    <li
      className={cn(
        'flex flex-wrap items-center gap-3 px-4 py-2',
        controlMinHeightClass,
        className
      )}
      {...props}
    />
  )
}

export { GroupedList, GroupedListItem }
