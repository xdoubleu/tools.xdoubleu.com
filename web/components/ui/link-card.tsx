import Link from 'next/link'
import { type ReactNode } from 'react'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'

interface LinkCardProps {
  href: string
  /** The navigating zone: cover, title, meta. Keep controls out of it. */
  children: ReactNode
  /** Controls in a separate footer that never navigates, so a near-miss can't open the page. */
  actions?: ReactNode
  /** Accessible name for the link when `children` has no clear text. */
  'aria-label'?: string
  className?: string
  linkClassName?: string
}

/** A card that navigates, with its controls kept out of the link area. */
function LinkCard({
  href,
  children,
  actions,
  'aria-label': ariaLabel,
  className,
  linkClassName
}: LinkCardProps) {
  return (
    <div
      className={cn(
        interactiveCardClass,
        'flex flex-col overflow-hidden active:scale-100',
        className
      )}
    >
      <Link
        href={href}
        aria-label={ariaLabel}
        className={cn(
          'relative block min-w-0 p-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent',
          linkClassName
        )}
      >
        {children}
        <CardLinkStatus />
      </Link>
      {actions && (
        <div className="flex flex-wrap items-center gap-2 border-t border-border px-3 py-2">
          {actions}
        </div>
      )}
    </div>
  )
}

export { LinkCard }
export type { LinkCardProps }
