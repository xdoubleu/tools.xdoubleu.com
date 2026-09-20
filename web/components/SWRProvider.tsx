'use client'

import { useEffect } from 'react'
import { SWRConfig } from 'swr'
import posthog from 'posthog-js'
import { swrKeys } from '@/lib/swrKeys'
import type { GetCurrentUserResponse } from '@/lib/gen/auth/v1/auth_pb'

// Bridges server-fetched data into the SWR cache. The layout fetches the
// current user once per request and provides it as fallback for every
// consumer of swrKeys.currentUser (Navbar, HomeClient, settings, ...);
// hooks still revalidate client-side, which keeps the browser-side token
// refresh path alive when the server fetch came back null.
export default function SWRProvider({
  currentUser,
  children
}: {
  currentUser: GetCurrentUserResponse | null
  children: React.ReactNode
}) {
  // Ties PostHog's distinct_id to the real user (root CLAUDE.md's PostHog
  // decision — every family member is individually identified). A no-op
  // when PostHog wasn't initialized (missing key) or already identified as
  // this user.
  useEffect(() => {
    if (currentUser?.userId && posthog.get_distinct_id() !== currentUser.userId) {
      posthog.identify(currentUser.userId)
    }
  }, [currentUser?.userId])

  return (
    <SWRConfig
      value={{
        fallback: currentUser ? { [swrKeys.currentUser]: currentUser } : {}
      }}
    >
      {children}
    </SWRConfig>
  )
}
