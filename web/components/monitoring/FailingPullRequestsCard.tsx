'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetFailingPullRequestsResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'
import { formatDate } from '@/lib/dates'

export default function FailingPullRequestsCard({
  data
}: {
  data?: GetFailingPullRequestsResponse
}) {
  const pullRequests = data?.pullRequests ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>Failing pull requests</CardTitle>
        <CardDescription>
          {data ? `${formatCount(data.failingCount)} with a failing check.` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">GitHub is not configured.</p>
        ) : pullRequests.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No failing pull requests.</p>
        ) : (
          <ul className="space-y-2">
            {pullRequests.map((pr) => (
              <li
                key={pr.number}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={pr.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="break-words font-medium text-fg hover:text-accent"
                  >
                    <span className="mr-1 font-mono text-xs text-muted">#{pr.number}</span>
                    {pr.title}
                  </a>
                  <span className="shrink-0 text-xs text-muted">
                    {pr.author} &middot; {formatDate(pr.updatedAt)}
                  </span>
                </div>
                {pr.failingChecks.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {pr.failingChecks.map((check) => (
                      <Badge key={check.name} variant="danger">
                        {check.name}
                      </Badge>
                    ))}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
