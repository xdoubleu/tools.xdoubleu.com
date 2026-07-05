'use client'

import { SWRConfig, unstable_serialize, type SWRConfiguration } from 'swr'
import type { Arguments } from 'swr'

// Injects server-fetched data into the SWR cache for the given keys without
// touching hook or component signatures. Merges with any parent fallback
// (e.g. the root layout's current-user entry) instead of replacing it.
// String keys go in `fallback` directly; tuple/object keys go in `keyed`
// and are serialized here (unstable_serialize is client-only).
export default function SWRFallback({
  fallback,
  keyed = [],
  children
}: {
  fallback: Record<string, unknown>
  keyed?: [Arguments, unknown][]
  children: React.ReactNode
}) {
  const merged: Record<string, unknown> = { ...fallback }
  for (const [key, data] of keyed) {
    merged[unstable_serialize(key)] = data
  }

  return (
    <SWRConfig
      value={(parent: SWRConfiguration | undefined) => ({
        ...parent,
        fallback: { ...parent?.fallback, ...merged }
      })}
    >
      {children}
    </SWRConfig>
  )
}
