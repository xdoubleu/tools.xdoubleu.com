import { Card } from '@/components/ui/card'
import type { SharedFeed } from '@/lib/gen/dashboard/v1/reading_pb'

// SharedFeedsCard is the public reading dashboard's feeds widget — just the
// owner's subscribed feed names, linked to their public URL when they have
// one (email feeds don't). Unlike the private dashboard's
// ReadingFeedsSummaryCard, it carries no read/unread state, which isn't
// meaningful to a visitor of someone else's shared profile.
export default function SharedFeedsCard({ feeds }: { feeds?: SharedFeed[] }) {
  if (!feeds || feeds.length === 0) return null

  return (
    <Card className="flex flex-wrap items-center gap-3 p-3 text-sm">
      <span className="font-semibold">Feeds</span>
      {feeds.map((feed) =>
        feed.url ? (
          <a
            key={feed.title}
            href={feed.url}
            target="_blank"
            rel="noopener noreferrer"
            className="max-w-xs truncate text-muted hover:underline"
          >
            {feed.title}
          </a>
        ) : (
          <span key={feed.title} className="max-w-xs truncate text-muted">
            {feed.title}
          </span>
        )
      )}
    </Card>
  )
}
