'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { GetUnhealthyFeedsResponse } from '@/lib/gen/feeds/v1/feeds_pb'
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
              <li key={feed.url}>
                <Card variant="inset" className="text-sm">
                  <div className="flex items-start justify-between gap-2">
                    <Button
                      asChild
                      variant="link"
                      className="min-w-0 justify-start text-left text-fg wrap-anywhere hover:text-accent"
                    >
                      <a href={feed.url} target="_blank" rel="noopener noreferrer">
                        {feed.title}
                      </a>
                    </Button>
                    <Badge variant="danger">
                      {formatCount(feed.consecutiveFailures)} failure(s)
                    </Badge>
                  </div>
                  {feed.lastError && (
                    <p className="mt-1 break-words text-xs text-muted">{feed.lastError}</p>
                  )}
                </Card>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
