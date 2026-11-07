import { type HTMLAttributes } from 'react'
import { cn } from '@/lib/cn'

/**
 * A page's one primary action. On phones it docks to the bottom thumb zone in
 * a glass bar, above the safe area and `--bottom-offset` (set by a bottom tab
 * bar); from `sm` up it is an ordinary inline child. Pass a `Button`, and add
 * `ThumbActionSpacer` at the end of the page so the bar covers no content.
 */
export function ThumbAction({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'max-sm:fixed max-sm:inset-x-0 max-sm:bottom-[var(--bottom-offset,0px)] max-sm:z-40',
        'max-sm:border-t max-sm:border-border/60 max-sm:bg-glass max-sm:shadow-glass max-sm:backdrop-blur-xl',
        'max-sm:px-[var(--inset-left)] max-sm:pt-3 max-sm:pb-[max(0.75rem,env(safe-area-inset-bottom))]',
        'max-sm:*:w-full',
        className
      )}
      {...props}
    />
  )
}

/** Clears the docked `ThumbAction` bar at the end of the page (phones only). */
export function ThumbActionSpacer() {
  return (
    <div
      aria-hidden
      className="h-[calc(5rem+var(--bottom-offset,0px)+env(safe-area-inset-bottom))] sm:hidden"
    />
  )
}
