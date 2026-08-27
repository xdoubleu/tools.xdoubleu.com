'use client'

import { useCurrentUser } from '@/hooks/useAuth'
import { useUnhealthyFeeds } from '@/hooks/useFeeds'
import FeedsCard from './FeedsCard'

// GetUnhealthyFeeds reports every user's feeds, not just the caller's own, so
// it's admin-only server-side — only render/fetch it for an admin viewer.
export default function UnhealthyFeedsSection() {
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
