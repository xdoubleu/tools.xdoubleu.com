import Link from 'next/link'
import { Card } from '@/components/ui/card'
import type { FeedsSummary } from '@/hooks/useFeeds'

// ReadingFeedsSummaryCard is the reading dashboard's read-only feeds widget
// (issue #737) — an unread count plus a few recent items, linking out to the
// full /feeds app. href is omitted on the public dashboard: a visitor's
// share token has no access to the owner's authenticated /feeds app.
// The public dashboard's own dashboardv1.FeedsSummary proto message is
// structurally identical, so it satisfies this type too.
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
