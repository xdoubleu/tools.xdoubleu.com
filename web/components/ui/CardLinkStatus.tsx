'use client'

import { useLinkStatus } from 'next/link'

/** Spinner inside a card's (relative) `<Link>` while its navigation is pending. */
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
