import { Card } from '@/components/ui/card'
import RssIcon from '@/components/RssIcon'
import type { SharedFeed } from '@/lib/gen/dashboard/v1/reading_pb'

// SharedFeedsCard is the public reading dashboard's feeds widget — just the
// owner's subscribed feed names, linked to their public URL when they have
// one (email feeds don't). Unlike the private dashboard's
// ReadingFeedsSummaryCard, it carries no read/unread state, which isn't
// meaningful to a visitor of someone else's shared profile.
//
// Laid out as a header row plus one feed name per line (see
// ReadingFeedsSummaryCard's issue #1356 note) rather than every name wrapped
// into a single dense line.
export default function SharedFeedsCard({ feeds }: { feeds?: SharedFeed[] }) {
  if (!feeds || feeds.length === 0) return null

  return (
    <Card className="flex flex-col gap-2 p-3">
      <span className="flex items-center gap-2 font-semibold">
        <RssIcon className="text-muted" />
        Feeds
      </span>
      <ul className="flex flex-col gap-1">
        {feeds.map((feed) => (
          <li key={feed.title} className="truncate text-sm">
            {feed.url ? (
              <a
                href={feed.url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted hover:underline"
              >
                {feed.title}
              </a>
            ) : (
              <span className="text-muted">{feed.title}</span>
            )}
          </li>
        ))}
      </ul>
    </Card>
  )
}
