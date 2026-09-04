import Link from 'next/link'
import { type ReactNode } from 'react'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'

type StatTone = 'default' | 'success' | 'warn' | 'danger'

const toneClasses: Record<StatTone, string> = {
  default: 'text-fg',
  success: 'text-success',
  warn: 'text-warn',
  danger: 'text-danger'
}

interface StatTileProps {
  label: string
  value: ReactNode
  /** Colours the value only — the label stays muted so tiles scan as one row. */
  tone?: StatTone
  /** Optional muted line under the value (a delta, a unit, a timestamp). */
  hint?: ReactNode
  /** Renders the tile as a navigable card with a pending-navigation spinner. */
  href?: string
  className?: string
}

/**
 * One labelled number in a stats row. Use for a single scalar reading; anything
 * with its own structure belongs in a `SectionCard` instead.
 */
function StatTile({ label, value, tone = 'default', hint, href, className }: StatTileProps) {
  const body = (
    <>
      <p className="text-xs font-medium uppercase tracking-wide text-muted">{label}</p>
      <p className={cn('mt-1 text-xl font-semibold', toneClasses[tone])}>{value}</p>
      {hint !== undefined && <p className="mt-0.5 text-xs text-muted">{hint}</p>}
    </>
  )

  if (href !== undefined) {
    return (
      <Link href={href} className={cn(interactiveCardClass, 'relative block p-4', className)}>
        <CardLinkStatus />
        {body}
      </Link>
    )
  }

  return <Card className={cn('p-4', className)}>{body}</Card>
}

/** Responsive grid for a row of `StatTile`s — two up on mobile, four from `sm`. */
function StatTileGrid({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn('grid grid-cols-2 gap-3 sm:grid-cols-4', className)}>{children}</div>
}

export { StatTile, StatTileGrid }
export type { StatTileProps, StatTone }
