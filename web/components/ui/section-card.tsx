import { type ReactNode } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/cn'

interface SectionCardProps {
  title: ReactNode
  /** Muted line under the title. */
  description?: ReactNode
  /** Right-aligned controls on the title row (a refresh button, a filter, a count). */
  action?: ReactNode
  children: ReactNode
  className?: string
  /** Applied to the body wrapper, not the card. */
  contentClassName?: string
}

/**
 * A `Card` with the standard title/description/action header already composed.
 * Reach for this instead of assembling `Card` + `CardHeader` + `CardTitle` by
 * hand — that hand-assembly is what drifted across the monitoring cards.
 */
function SectionCard({
  title,
  description,
  action,
  children,
  className,
  contentClassName
}: SectionCardProps) {
  return (
    <Card className={className}>
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div className="flex flex-col space-y-1">
            <CardTitle>{title}</CardTitle>
            {description !== undefined && <CardDescription>{description}</CardDescription>}
          </div>
          {action !== undefined && <div className="shrink-0">{action}</div>}
        </div>
      </CardHeader>
      <CardContent className={cn(contentClassName)}>{children}</CardContent>
    </Card>
  )
}

export { SectionCard }
export type { SectionCardProps }
