import { cn } from '@/lib/cn'

/** Placeholder block that reserves a loading region's final size so content doesn't shift in. */
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={cn('animate-pulse rounded-md bg-fg/10', className)} />
}
