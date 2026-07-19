'use client'

import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { GetGithubIssuesResponse } from '@/lib/gen/observability/v1/observability_pb'
import { formatCount } from '@/lib/observability'
import { formatDate } from '@/lib/dates'

export default function GithubIssuesCard({ data }: { data?: GetGithubIssuesResponse }) {
  const issues = data?.issues ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>GitHub issues</CardTitle>
        <CardDescription>
          {data ? `${formatCount(data.openCount)} open on the repository.` : 'Loading…'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {data && !data.configured ? (
          <p className="py-8 text-center text-sm text-muted">GitHub is not configured.</p>
        ) : issues.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted">No open issues.</p>
        ) : (
          <ul className="space-y-2">
            {issues.map((issue) => (
              <li
                key={issue.number}
                className="rounded-lg border border-border bg-surface p-3 text-sm"
              >
                <div className="flex items-start justify-between gap-2">
                  <a
                    href={issue.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="break-words font-medium text-fg hover:text-accent"
                  >
                    <span className="mr-1 font-mono text-xs text-muted">#{issue.number}</span>
                    {issue.title}
                  </a>
                  <span className="shrink-0 text-xs text-muted">{formatDate(issue.createdAt)}</span>
                </div>
                {issue.labels.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {issue.labels.map((label) => (
                      <Badge key={label} variant="secondary">
                        {label}
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
