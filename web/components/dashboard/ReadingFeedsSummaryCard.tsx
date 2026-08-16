import Link from 'next/link'
import { Card } from '@/components/ui/card'
import type { FeedsSummary } from '@/hooks/useFeeds'

// ReadingFeedsSummaryCard is the private (owner) reading dashboard's feeds
// widget (issue #737) — an unread count plus a few recent items, linking out
// to the full /feeds app. The public dashboard uses SharedFeedsCard instead,
// since a visitor gets no read state and no href into the authenticated
// /feeds app.
export default function ReadingFeedsSummaryCard({
  summary,
  href
}: {
  summary?: FeedsSummary
  href?: string
}) {
  if (!summary) return null

  return (
    <Card className="flex flex-wrap items-center gap-3 p-3 text-sm">
      {href ? (
        <Link href={href} className="font-semibold hover:underline">
          Feeds
        </Link>
      ) : (
        <span className="font-semibold">Feeds</span>
      )}
      <span className="text-muted">{summary.unreadCount} unread</span>
      {summary.items.length === 0 && <span className="text-muted">No unread items.</span>}
      {summary.items.slice(0, 3).map((item) => (
        <span key={item.sourceUrl + item.publishedAt} className="max-w-xs truncate text-muted">
          {item.title}
        </span>
      ))}
    </Card>
  )
}
