import { type ReactNode } from 'react'
import { Card } from '@/components/ui/card'
import { LinkCard } from '@/components/ui/link-card'

interface ItemCardProps {
  /** Where the card navigates; without it the card is static. */
  href?: string
  title: string
  /** An `Artwork` or icon, left of the text. */
  artwork?: ReactNode
  /** One muted line under the title. */
  meta?: ReactNode
  /** A `Badge` or similar, under the meta line. */
  status?: ReactNode
  /** Controls in a footer that never navigates. */
  actions?: ReactNode
  /** A full-width, stacked region in the footer for richer controls (e.g. ratings, progress). */
  details?: ReactNode
  /** Accessible name for the link when the body has more than the title. */
  'aria-label'?: string
}

/** One entry of a collection: artwork, title, one meta line and an optional status. */
function ItemCard({
  href,
  title,
  artwork,
  meta,
  status,
  actions,
  details,
  'aria-label': ariaLabel
}: ItemCardProps) {
  const body = (
    <div className="flex items-start gap-3">
      {artwork}
      <div className="min-w-0 flex-1 space-y-1">
        <p className="break-words font-medium">{title}</p>
        {meta !== undefined && <p className="text-sm text-muted">{meta}</p>}
        {status !== undefined && <div>{status}</div>}
      </div>
    </div>
  )
  const footer =
    details === undefined ? (
      actions
    ) : (
      <div className="w-full min-w-0 space-y-2">
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
        {details}
      </div>
    )
  if (href === undefined) {
    return (
      <Card className="space-y-3">
        {body}
        {footer && <div className="flex flex-wrap items-center gap-2">{footer}</div>}
      </Card>
    )
  }
  return (
    <LinkCard href={href} aria-label={ariaLabel} actions={footer} linkClassName="p-4">
      {body}
    </LinkCard>
  )
}

export { ItemCard }
export type { ItemCardProps }
