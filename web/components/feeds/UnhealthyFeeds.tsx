'use client'

import { useCurrentUser } from '@/hooks/useAuth'
import { useUnhealthyFeeds } from '@/hooks/useFeeds'
import FeedsCard from './FeedsCard'

// Admin-only RPC (covers every user's feeds).
export default function UnhealthyFeeds() {
  const { data: currentUser } = useCurrentUser()
  const isAdmin = currentUser?.role === 'admin'
  const unhealthyFeeds = useUnhealthyFeeds(isAdmin)

  if (!isAdmin) return null

  return (
    <div className="mb-6">
      <FeedsCard data={unhealthyFeeds.data} />
    </div>
  )
}
