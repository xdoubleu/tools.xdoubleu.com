'use client'

import Link from 'next/link'
import { useFeeds } from '@/hooks/useBookFeeds'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

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

      {isLoading && <p className="text-muted text-sm">Loading…</p>}
      {error && <p className="text-danger text-sm">Failed to load feeds.</p>}
      {!isLoading && !error && feeds.length === 0 && (
        <p className="text-muted text-sm">No feed subscriptions yet.</p>
      )}
      {feeds.length > 0 && (
        <ul className="min-h-0 space-y-1.5 overflow-y-auto pr-1">
          {feeds.map((feed) => (
            <li key={feed.id} className="rounded-lg border border-border bg-card px-3 py-2">
              <p className="truncate text-sm font-medium">{feed.title || feed.url}</p>
              {feed.lastError ? (
                <p className="truncate text-xs text-danger">Last poll failed</p>
              ) : (
                feed.lastFetchedAt && (
                  <p className="truncate text-xs text-muted">
                    Last fetched {new Date(feed.lastFetchedAt).toLocaleDateString()}
                  </p>
                )
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
