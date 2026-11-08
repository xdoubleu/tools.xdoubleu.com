import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

type StatePlacement = 'page' | 'inline'

const placementClasses: Record<StatePlacement, string> = {
  page: 'py-16 text-center',
  inline: ''
}

interface StateProps {
  /** `page` centres the message with room around it, for a whole page or panel; `inline` sits in flow. */
  placement?: StatePlacement
  className?: string
}

/** Loading placeholder; `label` names what is loading ("recipe" → "Loading recipe…"). */
function LoadingState({ label, placement = 'inline', className }: StateProps & { label?: string }) {
  return (
    <p className={cn('text-sm text-muted', placementClasses[placement], className)} role="status">
      {label ? `Loading ${label}…` : 'Loading…'}
    </p>
  )
}

/** Fetch failure message: "Failed to load {what}." */
function ErrorState({ what, placement = 'inline', className }: StateProps & { what: string }) {
  return (
    <p className={cn('text-sm text-danger', placementClasses[placement], className)} role="alert">
      Failed to load {what}.
    </p>
  )
}

interface EmptyStateProps extends StateProps {
  children: ReactNode
  /** Optional call to action under the message, e.g. a "New recipe" button. */
  action?: ReactNode
}

/**
 * Message for a list or section with nothing in it yet. Centred (`page`) by
 * default; `inline` for an empty list inside a field, dialog section or panel.
 */
function EmptyState({ children, action, placement = 'page', className }: EmptyStateProps) {
  const page = placement === 'page'
  return (
    <div className={cn(page ? 'space-y-3' : 'space-y-2', placementClasses[placement], className)}>
      <p className="text-sm text-muted">{children}</p>
      {action && <div className={cn('flex', page && 'justify-center')}>{action}</div>}
    </div>
  )
}

export { LoadingState, ErrorState, EmptyState }
export type { EmptyStateProps, StatePlacement }
