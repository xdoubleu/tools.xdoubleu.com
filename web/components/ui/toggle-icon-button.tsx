'use client'

import { type MouseEvent, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

/**
 * Amber-when-on treatment shared by every glyph toggle (favourite, bookmark,
 * rating star) so "this is set" reads the same everywhere.
 */
const toggleGlyphClass = (active: boolean) =>
  active ? 'text-amber-500' : 'text-border hover:text-amber-400 active:text-amber-400'

interface ToggleIconButtonProps {
  /** Current state — drives `aria-pressed` and the amber treatment. */
  active: boolean
  onToggle: (event: MouseEvent<HTMLButtonElement>) => void
  /** `aria-label` while off, e.g. "Add to favourites". */
  label: string
  /** `aria-label` while on, e.g. "Remove from favourites". */
  activeLabel: string
  /** The glyph — a character or an inline `<svg>`. */
  children: ReactNode
  className?: string
}

/**
 * A single on/off glyph button (favourite heart, bookmark flag). Exposes the
 * state as `aria-pressed` so it announces as a toggle rather than an action.
 */
function ToggleIconButton({
  active,
  onToggle,
  label,
  activeLabel,
  children,
  className
}: ToggleIconButtonProps) {
  return (
    <Button
      variant="ghost"
      size="iconSm"
      onClick={onToggle}
      aria-label={active ? activeLabel : label}
      aria-pressed={active}
      className={cn('leading-none hover:bg-transparent', toggleGlyphClass(active), className)}
    >
      {children}
    </Button>
  )
}

export { ToggleIconButton, toggleGlyphClass }
export type { ToggleIconButtonProps }
