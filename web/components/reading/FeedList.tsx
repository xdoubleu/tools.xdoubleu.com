import type { Feed } from '@/lib/gen/reading/v1/feeds_pb'

// Read-only list body for a set of RSS/Atom feed subscriptions, shared by the
// owner's SubscribedFeedsCard and the public shared-profile feeds card.
export default function FeedList({
  feeds,
  isLoading,
  error
}: {
  feeds: Feed[]
  isLoading?: boolean
  error?: Error
}) {
  return (
    <>
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
    </>
  )
}
