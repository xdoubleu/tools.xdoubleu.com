'use client'

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
 * Pill control for selectable attributes (shelf, tag, ownership) and for
 * filter chips. Filled when active, outlined when not — that contrast is what
 * signals "this is a control", distinct from a read-only `Badge` stating a
 * static fact.
 *
 * Sets `aria-pressed` by default; pass `role="tab"`/`aria-selected` instead
 * when the pills form a tablist.
 */
function TogglePill({ label, active, className, ...props }: TogglePillProps) {
  return (
    <Button
      type="button"
      size="sm"
      variant={active ? 'default' : 'secondary'}
      className={cn('h-auto rounded-full px-2.5 py-0.5 text-xs', className)}
      aria-pressed={active}
      {...props}
    >
      {label}
    </Button>
  )
}

export { TogglePill }
export type { TogglePillProps }
