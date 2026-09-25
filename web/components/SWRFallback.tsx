'use client'

import { SWRConfig, unstable_serialize, type SWRConfigValue } from 'swr'
import type { Arguments } from 'swr'

// Injects server-fetched data into the SWR cache, merging with any parent
// fallback. Tuple/object keys go in `keyed` and are serialized here
// (unstable_serialize is client-only).
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
      value={(parent: SWRConfigValue | undefined) => ({
        ...parent,
        fallback: { ...parent?.fallback, ...merged }
      })}
    >
      {children}
    </SWRConfig>
  )
}
