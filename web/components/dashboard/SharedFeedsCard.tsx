import { Card } from '@/components/ui/card'
import RssIcon from '@/components/RssIcon'
import type { SharedFeed } from '@/lib/gen/dashboard/v1/reading_pb'

// Public dashboard's feeds widget: feed names linked to their public URL
// (email feeds have none). No read state for visitors.
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
