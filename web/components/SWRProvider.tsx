'use client'

import { useEffect } from 'react'
import { SWRConfig } from 'swr'
import posthog from 'posthog-js'
import { persistMiddleware } from '@/lib/offline/persist'
import { claimOfflineData } from '@/lib/offline/session'
import { swrKeys } from '@/lib/swrKeys'
import type { GetCurrentUserResponse } from '@/lib/gen/auth/v1/auth_pb'

// Seeds the SWR cache with server-fetched data (e.g. the current user);
// hooks still revalidate, keeping the browser-side token refresh alive.
// Responses are saved for offline use (lib/offline/persist.ts).
export default function SWRProvider({
  currentUser,
  children
}: {
  currentUser: GetCurrentUserResponse | null
  children: React.ReactNode
}) {
  // Identify the PostHog user (every family member individually).
  useEffect(() => {
    if (currentUser?.userId && posthog.get_distinct_id() !== currentUser.userId) {
      posthog.identify(currentUser.userId)
    }
  }, [currentUser?.userId])

  useEffect(() => {
    if (currentUser?.userId) void claimOfflineData(currentUser.userId)
  }, [currentUser?.userId])

  return (
    <SWRConfig
      value={{
        fallback: currentUser ? { [swrKeys.currentUser]: currentUser } : {},
        use: [persistMiddleware]
      }}
    >
      {children}
    </SWRConfig>
  )
}
