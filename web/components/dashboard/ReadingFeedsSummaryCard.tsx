import Link from 'next/link'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { Badge } from '@/components/ui/badge'
import RssIcon from '@/components/RssIcon'
import { cn } from '@/lib/cn'
import type { FeedsSummary } from '@/hooks/useFeeds'

// ReadingFeedsSummaryCard is the private (owner) reading dashboard's feeds
// widget (issue #737) — an unread count plus a few recent items, linking out
// to the full /feeds app. The public dashboard uses SharedFeedsCard instead,
// since a visitor gets no read state and no href into the authenticated
// /feeds app.
//
// A single flex-wrap row of the title, count, and every item title jammed
// together read as one cramped, indistinct line right above the rest of the
// dashboard (issue #1356) — laid out here as a proper card instead, with a
// header row (icon, title, unread badge) and each recent item on its own
// line so it reads like the other dashboard cards rather than a stray strip
// of text.
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
