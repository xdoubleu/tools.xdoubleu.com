'use client'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface TogglePillProps {
  label: string
  active: boolean
  onClick: () => void
  disabled?: boolean
  className?: string
}

/**
 * Shared pill control for editable single/multi-select attributes (shelf,
 * tags, ownership). Filled when active, outlined when not — the visual
 * contrast is what signals "this is a control," distinct from read-only
 * Badges used for static facts (e.g. PDF/EPUB format).
 */
export default function TogglePill({
  label,
  active,
  onClick,
  disabled,
  className
}: TogglePillProps) {
  return (
    <Button
      type="button"
      size="sm"
      variant={active ? 'default' : 'secondary'}
      className={cn('h-auto rounded-full px-2.5 py-0.5 text-xs', className)}
      aria-pressed={active}
      disabled={disabled}
      onClick={onClick}
    >
      {label}
    </Button>
  )
}
