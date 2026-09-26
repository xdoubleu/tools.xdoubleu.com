import { type ReactNode } from 'react'
import { cn } from '@/lib/cn'

/** Loading placeholder; `label` names what is loading ("recipe" → "Loading recipe…"). */
function LoadingState({ label, className }: { label?: string; className?: string }) {
  return (
    <p className={cn('text-muted', className)} role="status">
      {label ? `Loading ${label}…` : 'Loading…'}
    </p>
  )
}

/** Fetch failure message: "Failed to load {what}." */
function ErrorState({ what, className }: { what: string; className?: string }) {
  return (
    <p className={cn('text-danger', className)} role="alert">
      Failed to load {what}.
    </p>
  )
}

interface EmptyStateProps {
  children: ReactNode
  /** Optional call to action under the message, e.g. a "New recipe" button. */
  action?: ReactNode
  className?: string
}

/** Message for a list or section with nothing in it yet. */
function EmptyState({ children, action, className }: EmptyStateProps) {
  return (
    <div className={cn('space-y-3 py-8 text-center', className)}>
      <p className="text-sm text-muted">{children}</p>
      {action && <div className="flex justify-center">{action}</div>}
    </div>
  )
}

export { LoadingState, ErrorState, EmptyState }
export type { EmptyStateProps }
