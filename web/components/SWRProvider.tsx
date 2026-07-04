'use client'

import { SWRConfig } from 'swr'
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
