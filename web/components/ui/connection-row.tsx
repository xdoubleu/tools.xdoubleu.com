import { type ReactNode } from 'react'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { cn } from '@/lib/cn'

interface ConnectionRowProps {
  /** Provider name, e.g. "Todoist". */
  name: ReactNode
  connected: boolean
  /** Muted line under the name, e.g. when it was connected. */
  detail?: ReactNode
  /** Connect/disconnect controls; they wrap under the name on narrow screens. */
  actions?: ReactNode
  className?: string
}

/** One third-party integration with its connection status and controls. */
function ConnectionRow({ name, connected, detail, actions, className }: ConnectionRowProps) {
  return (
    <Card
      variant="inset"
      className={cn('flex flex-wrap items-center justify-between gap-3 text-sm', className)}
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium text-fg">{name}</span>
          <Badge variant={connected ? 'success' : 'secondary'}>
            {connected ? 'Connected' : 'Not connected'}
          </Badge>
        </div>
        {detail && <p className="mt-1 text-xs text-muted">{detail}</p>}
      </div>
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </Card>
  )
}

export { ConnectionRow }
export type { ConnectionRowProps }
