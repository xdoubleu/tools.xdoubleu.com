import { type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface TogglePillProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  /** Pill contents. A plain string in the common case; a fragment when it carries a count. */
  label: ReactNode
  active: boolean
  className?: string
}

/**
 * Filter chip for selectable attributes: a light outline when off, a tinted accent fill
 * with a check when on (unlike a read-only `Badge`, and visibly lighter than a `Button`).
 * Sets `aria-pressed`. Switching views is `SegmentedTabs`, not a row of these.
 */
// Stryker disable StringLiteral: class names are presentation; the on/off tint is tested.
function TogglePill({ label, active, className, ...props }: TogglePillProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      className={cn(
        // A one-word label ("All") would otherwise fall under the 44px tap width.
        'gap-1.5 border px-3 min-w-11 sm:min-w-9',
        active
          ? 'border-accent bg-accent/10 text-accent hover:bg-accent/15'
          : 'border-border text-muted hover:text-fg',
        className
      )}
      aria-pressed={active}
      {...props}
    >
      {active && <span aria-hidden>✓</span>}
      {label}
    </Button>
  )
}

// Stryker restore StringLiteral

export { TogglePill }
export type { TogglePillProps }
