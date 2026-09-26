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
 * Pill for selectable attributes and filter chips: filled when active,
 * outlined when not (unlike a read-only `Badge`). Sets `aria-pressed`; pass
 * `role="tab"`/`aria-selected` in a tablist.
 */
function TogglePill({ label, active, className, ...props }: TogglePillProps) {
  return (
    <Button
      type="button"
      size="sm"
      variant={active ? 'default' : 'secondary'}
      className={cn('rounded-full px-3 sm:h-auto sm:px-2.5 sm:py-0.5', className)}
      aria-pressed={active}
      {...props}
    >
      {label}
    </Button>
  )
}

export { TogglePill }
export type { TogglePillProps }
