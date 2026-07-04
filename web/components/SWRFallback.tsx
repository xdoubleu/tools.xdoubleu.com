'use client'

import { SWRConfig, type SWRConfiguration } from 'swr'

// Injects server-fetched data into the SWR cache for the given keys without
// touching hook or component signatures. Merges with any parent fallback
// (e.g. the root layout's current-user entry) instead of replacing it.
// Keys must be the serialized SWR key — use swrKeys entries directly for
// string keys and unstable_serialize() for tuple keys.
export default function SWRFallback({
  fallback,
  children
}: {
  fallback: Record<string, unknown>
  children: React.ReactNode
}) {
  return (
    <SWRConfig
      value={(parent: SWRConfiguration | undefined) => ({
        ...parent,
        fallback: { ...parent?.fallback, ...fallback }
      })}
    >
      {children}
    </SWRConfig>
  )
}
