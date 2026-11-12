import { type ReactNode } from 'react'
import { LinkCard } from '@/components/ui/link-card'

interface ItemCardProps {
  href: string
  title: string
  /** An `Artwork` or icon, left of the text. */
  artwork?: ReactNode
  /** One muted line under the title. */
  meta?: ReactNode
  /** A `Badge` or similar, under the meta line. */
  status?: ReactNode
  /** Controls in a footer that never navigates. */
  actions?: ReactNode
}

/** One entry of a collection: artwork, title, one meta line and an optional status. */
function ItemCard({ href, title, artwork, meta, status, actions }: ItemCardProps) {
  return (
    <LinkCard href={href} actions={actions} linkClassName="p-4">
      <div className="flex items-start gap-3">
        {artwork}
        <div className="min-w-0 flex-1 space-y-1">
          <p className="break-words font-medium">{title}</p>
          {meta !== undefined && <p className="text-sm text-muted">{meta}</p>}
          {status !== undefined && <div>{status}</div>}
        </div>
      </div>
    </LinkCard>
  )
}

export { ItemCard }
export type { ItemCardProps }
