'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetUnhealthyFeedsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'

export default function FeedsCard({ data }: { data?: GetUnhealthyFeedsResponse }) {
  const feeds = data?.feeds ?? []

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle>Unhealthy feeds</CardTitle>
          {feeds.length > 0 && <Badge variant="danger">{formatCount(feeds.length)}</Badge>}
        </div>
        <CardDescription>
          {data ? `${formatCount(feeds.length)} feed(s) failing to poll.` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {feeds.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">All feeds are healthy.</p>
        ) : (
          <ul className="space-y-2">
            {feeds.map((feed) => (
              <li key={feed.url} className="rounded-lg border border-border bg-surface p-3 text-sm">
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={feed.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="break-words font-medium text-fg hover:text-accent"
                  >
                    {feed.title}
                  </a>
                  <Badge variant="danger">{formatCount(feed.consecutiveFailures)} failure(s)</Badge>
                </div>
                {feed.lastError && <p className="mt-1 text-xs text-muted">{feed.lastError}</p>}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
