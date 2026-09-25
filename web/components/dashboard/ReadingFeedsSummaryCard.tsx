import Link from 'next/link'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { Badge } from '@/components/ui/badge'
import RssIcon from '@/components/RssIcon'
import { cn } from '@/lib/cn'
import type { FeedsSummary } from '@/hooks/useFeeds'

// Owner's reading-dashboard feeds widget: unread count plus recent items,
// linking to /feeds. The public dashboard uses SharedFeedsCard.
export default function ReadingFeedsSummaryCard({
  summary,
  href
}: {
  summary?: FeedsSummary
  href?: string
}) {
  if (!summary) return null

  const header = (
    <div className="flex items-center justify-between gap-3">
      <span className="flex items-center gap-2 font-semibold">
        <RssIcon className="text-muted" />
        Feeds
      </span>
      <Badge variant={summary.unreadCount > 0 ? 'default' : 'secondary'}>
        {summary.unreadCount} unread
      </Badge>
    </div>
  )

  const items =
    summary.items.length === 0 ? (
      <p className="text-sm text-muted">No unread items.</p>
    ) : (
      <ul className="flex flex-col gap-1">
        {summary.items.slice(0, 3).map((item) => (
          <li key={item.sourceUrl + item.publishedAt} className="truncate text-sm text-muted">
            {item.title}
          </li>
        ))}
      </ul>
    )

  if (href) {
    return (
      <Link href={href} className={cn(interactiveCardClass, 'relative flex flex-col gap-2 p-3')}>
        <CardLinkStatus />
        {header}
        {items}
      </Link>
    )
  }

  return (
    <Card className="flex flex-col gap-2 p-3">
      {header}
      {items}
    </Card>
  )
}
