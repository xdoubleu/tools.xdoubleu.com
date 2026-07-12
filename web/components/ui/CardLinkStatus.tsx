'use client'

import { useLinkStatus } from 'next/link'

/**
 * Drop inside a navigable card's `<Link>` (which must be `relative`) to show
 * a spinner while that link's navigation is pending — otherwise a slow route
 * transition looks like the tap did nothing.
 */
export function CardLinkStatus() {
  const { pending } = useLinkStatus()
  if (!pending) return null
  return (
    <span
      aria-hidden
      className="absolute right-2 top-2 h-4 w-4 animate-spin rounded-full border-2 border-accent/30 border-t-accent"
    />
  )
}
