'use client'

import Link from 'next/link'
import { useFeeds } from '@/hooks/useBookFeeds'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import FeedList from '@/components/reading/FeedList'

// SubscribedFeedsCard is a compact, read-only view of the user's RSS/Atom
// subscriptions for the dashboard. Managing feeds (add/remove/kobo-sync) lives
// on the settings page.
export default function SubscribedFeedsCard() {
  const { data, error, isLoading } = useFeeds()
  const feeds = data?.feeds ?? []

  return (
    <Card className="flex min-h-0 flex-col p-4">
      <div className="mb-2 flex items-center justify-between gap-2">
        <h2 className="text-base font-semibold">Subscribed feeds</h2>
        <Button asChild variant="ghost" size="sm">
          <Link href="/reading/settings">Manage</Link>
        </Button>
      </div>

      <FeedList feeds={feeds} isLoading={isLoading} error={error} />
    </Card>
  )
}
